package nodes

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

const (
	maxGoogleAPIResponseBytes int64 = 2 << 20
	maxSheetsValuesBytes            = 1 << 20
	maxSheetsValuesCount            = 1000
	maxSheetsRowsCount              = 1000
)

type GoogleSheetsExecutor struct{}

func NewGoogleSheetsExecutor() *GoogleSheetsExecutor { return &GoogleSheetsExecutor{} }

func parseSheetsAction(node *Node) (string, error) {
	action, _ := node.Params["action"].(string)
	action = strings.ToUpper(strings.TrimSpace(action))
	if action == "" {
		action = "APPEND"
	}
	switch action {
	case "APPEND", "READ", "UPDATE", "UPSERT":
		return action, nil
	default:
		return "", fmt.Errorf("unsupported Google Sheets action: %s", action)
	}
}

// parseSheetsRows accepts the legacy single-row array as well as a multi-row
// array. This preserves existing workflows while allowing bulk APPEND/UPDATE.
func parseSheetsRows(raw interface{}) ([][]interface{}, error) {
	if raw == nil {
		return nil, fmt.Errorf("values_json array is empty")
	}
	var decoded interface{}
	switch typed := raw.(type) {
	case string:
		text := strings.TrimSpace(typed)
		if text == "" {
			return nil, fmt.Errorf("values_json array is empty")
		}
		if len(text) > maxSheetsValuesBytes {
			return nil, fmt.Errorf("values_json exceeds %d byte limit", maxSheetsValuesBytes)
		}
		if err := json.Unmarshal([]byte(text), &decoded); err != nil {
			return nil, fmt.Errorf("values_json must be a valid JSON array: %w", err)
		}
	default:
		decoded = raw
	}

	encoded, err := json.Marshal(decoded)
	if err != nil || len(encoded) > maxSheetsValuesBytes {
		return nil, fmt.Errorf("values_json exceeds %d byte limit", maxSheetsValuesBytes)
	}
	outer, ok := decoded.([]interface{})
	if !ok || len(outer) == 0 {
		return nil, fmt.Errorf("values_json must resolve to a non-empty JSON array")
	}
	if len(outer) > maxSheetsRowsCount {
		return nil, fmt.Errorf("values_json contains %d rows/values; maximum is %d", len(outer), maxSheetsRowsCount)
	}

	// Legacy format: ["A", 1, true]
	if _, nested := outer[0].([]interface{}); !nested {
		if len(outer) > maxSheetsValuesCount {
			return nil, fmt.Errorf("values_json contains %d values; maximum is %d", len(outer), maxSheetsValuesCount)
		}
		return [][]interface{}{outer}, nil
	}

	rows := make([][]interface{}, 0, len(outer))
	for i, rawRow := range outer {
		row, ok := rawRow.([]interface{})
		if !ok {
			return nil, fmt.Errorf("values_json row %d must be an array", i+1)
		}
		if len(row) > maxSheetsValuesCount {
			return nil, fmt.Errorf("values_json row %d contains more than %d values", i+1, maxSheetsValuesCount)
		}
		rows = append(rows, row)
	}
	return rows, nil
}

// parseSheetsValues remains for callers/tests that depend on the legacy helper.
func parseSheetsValues(raw interface{}) ([]interface{}, error) {
	rows, err := parseSheetsRows(raw)
	if err != nil {
		return nil, err
	}
	if len(rows) != 1 {
		return nil, fmt.Errorf("values_json must contain exactly one row")
	}
	return rows[0], nil
}

func validateGoogleSheetsNode(node *Node) error {
	spreadsheetID, _ := node.Params["spreadsheet_id"].(string)
	if strings.TrimSpace(spreadsheetID) == "" {
		return fmt.Errorf("spreadsheet_id is required")
	}
	if strings.ContainsAny(spreadsheetID, "\r\n") || len(spreadsheetID) > 512 {
		return fmt.Errorf("spreadsheet_id is invalid")
	}
	sheetName, _ := node.Params["sheet_name"].(string)
	if len(sheetName) > 1024 || strings.ContainsAny(sheetName, "\r\n") {
		return fmt.Errorf("sheet_name is invalid")
	}
	action, err := parseSheetsAction(node)
	if err != nil {
		return err
	}
	credentialID, _ := node.Params["credential_id"].(string)
	directSA, _ := node.Params["service_account_json"].(string)
	if strings.TrimSpace(credentialID) == "" && strings.TrimSpace(directSA) == "" {
		return fmt.Errorf("Google Sheets requires an encrypted credential or service_account_json")
	}
	if action != "READ" {
		if text, ok := node.Params["values_json"].(string); ok && containsTemplateExpression(text) {
			return nil
		}
		if _, err = parseSheetsRows(node.Params["values_json"]); err != nil {
			return err
		}
	}
	if action == "UPSERT" {
		matchColumn, err := parseSheetsMatchColumn(node.Params["match_column"])
		if err != nil {
			return err
		}
		if matchColumn < 1 {
			return fmt.Errorf("Google Sheets UPSERT requires match_column >= 1")
		}
		if strings.TrimSpace(conditionValueString(node.Params["match_value"])) == "" {
			return fmt.Errorf("Google Sheets UPSERT requires match_value")
		}
	}
	return nil
}

func (e *GoogleSheetsExecutor) Execute(ctx *ExecutionContext, node *Node) (interface{}, error) {
	if err := validateGoogleSheetsNode(node); err != nil {
		return nil, err
	}
	spreadsheetID, _ := node.Params["spreadsheet_id"].(string)
	sheetName, _ := node.Params["sheet_name"].(string)
	if strings.TrimSpace(sheetName) == "" {
		sheetName = "Sheet1"
	}
	action, _ := parseSheetsAction(node)
	material, err := resolveGoogleAuth(ctx, node)
	if err != nil {
		return nil, err
	}
	accessToken, err := googleAccessToken(ctx.Context, material, "https://www.googleapis.com/auth/spreadsheets")
	if err != nil {
		return nil, err
	}
	client := &http.Client{Timeout: 15 * time.Second}
	base := "https://sheets.googleapis.com/v4/spreadsheets/" + url.PathEscape(strings.TrimSpace(spreadsheetID)) + "/values/" + url.PathEscape(sheetName)

	switch action {
	case "READ":
		return googleSheetsRead(ctx, client, accessToken, base)
	case "APPEND":
		rows, err := parseSheetsRows(node.Params["values_json"])
		if err != nil {
			return nil, err
		}
		return googleSheetsWrite(ctx, client, http.MethodPost, accessToken, base+":append?valueInputOption=USER_ENTERED", rows)
	case "UPDATE":
		rows, err := parseSheetsRows(node.Params["values_json"])
		if err != nil {
			return nil, err
		}
		return googleSheetsWrite(ctx, client, http.MethodPut, accessToken, base+"?valueInputOption=USER_ENTERED", rows)
	case "UPSERT":
		rows, err := parseSheetsRows(node.Params["values_json"])
		if err != nil {
			return nil, err
		}
		if len(rows) != 1 {
			return nil, fmt.Errorf("Google Sheets UPSERT accepts exactly one row")
		}
		matchColumn, _ := parseSheetsMatchColumn(node.Params["match_column"])
		matchValue := conditionValueString(node.Params["match_value"])
		readResult, err := googleSheetsRead(ctx, client, accessToken, base)
		if err != nil {
			return nil, err
		}
		rowNumber := findSheetsMatchRow(readResult["values"], matchColumn, matchValue)
		if rowNumber == 0 {
			result, err := googleSheetsWrite(ctx, client, http.MethodPost, accessToken, base+":append?valueInputOption=USER_ENTERED", rows)
			if err == nil {
				result["upserted"] = "appended"
			}
			return result, err
		}
		rowRange := sheetsRowRange(sheetName, rowNumber, len(rows[0]))
		rowBase := "https://sheets.googleapis.com/v4/spreadsheets/" + url.PathEscape(strings.TrimSpace(spreadsheetID)) + "/values/" + url.PathEscape(rowRange)
		result, err := googleSheetsWrite(ctx, client, http.MethodPut, accessToken, rowBase+"?valueInputOption=USER_ENTERED", rows)
		if err == nil {
			result["upserted"] = "updated"
			result["matched_row"] = rowNumber
		}
		return result, err
	}
	return nil, fmt.Errorf("unsupported Google Sheets action")
}

func googleSheetsRead(ctx *ExecutionContext, client *http.Client, accessToken, requestURL string) (map[string]interface{}, error) {
	req, err := http.NewRequestWithContext(ctx.Context, http.MethodGet, requestURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create Google Sheets request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)
	return doGoogleJSONRequest(client, req, http.StatusOK)
}

func googleSheetsWrite(ctx *ExecutionContext, client *http.Client, method, accessToken, requestURL string, rows [][]interface{}) (map[string]interface{}, error) {
	payloadBytes, err := json.Marshal(map[string]interface{}{"values": rows})
	if err != nil {
		return nil, fmt.Errorf("failed to marshal Google Sheets payload: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx.Context, method, requestURL, bytes.NewReader(payloadBytes))
	if err != nil {
		return nil, fmt.Errorf("failed to create Google Sheets request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Content-Type", "application/json")
	return doGoogleJSONRequest(client, req, http.StatusOK)
}

func parseSheetsMatchColumn(raw interface{}) (int, error) {
	if raw == nil || strings.TrimSpace(conditionValueString(raw)) == "" {
		return 1, nil
	}
	switch typed := raw.(type) {
	case int:
		return typed, nil
	case int64:
		return int(typed), nil
	case float64:
		if typed != float64(int(typed)) {
			return 0, fmt.Errorf("match_column must be an integer")
		}
		return int(typed), nil
	default:
		value, err := strconv.Atoi(strings.TrimSpace(conditionValueString(raw)))
		if err != nil {
			return 0, fmt.Errorf("match_column must be an integer")
		}
		return value, nil
	}
}

func findSheetsMatchRow(raw interface{}, oneBasedColumn int, match string) int {
	rows, ok := raw.([]interface{})
	if !ok || oneBasedColumn < 1 {
		return 0
	}
	index := oneBasedColumn - 1
	for rowIndex, rawRow := range rows {
		row, ok := rawRow.([]interface{})
		if !ok || index >= len(row) {
			continue
		}
		if conditionValueString(row[index]) == match {
			return rowIndex + 1
		}
	}
	return 0
}

func sheetsRowRange(sheet string, row, width int) string {
	if width < 1 {
		width = 1
	}
	plainSheet := strings.SplitN(sheet, "!", 2)[0]
	return fmt.Sprintf("%s!A%d:%s%d", plainSheet, row, sheetsColumnName(width), row)
}

func sheetsColumnName(index int) string {
	if index <= 0 {
		return "A"
	}
	name := ""
	for index > 0 {
		index--
		name = string(rune('A'+index%26)) + name
		index /= 26
	}
	return name
}

func doGoogleJSONRequest(client *http.Client, req *http.Request, successCodes ...int) (map[string]interface{}, error) {
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("Google API request failed: %w", err)
	}
	defer resp.Body.Close()
	body, err := readNodeResponseBody(resp, maxGoogleAPIResponseBytes)
	if err != nil {
		return nil, fmt.Errorf("Google API %w", err)
	}
	success := false
	for _, code := range successCodes {
		if resp.StatusCode == code {
			success = true
			break
		}
	}
	if !success {
		return nil, fmt.Errorf("Google API error (status %d): %s", resp.StatusCode, boundedNodeErrorText(body))
	}
	var result map[string]interface{}
	if len(strings.TrimSpace(string(body))) == 0 {
		return map[string]interface{}{}, nil
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("Google API returned invalid JSON: %w", err)
	}
	return result, nil
}

func (e *GoogleSheetsExecutor) Validate(node *Node) error { return validateGoogleSheetsNode(node) }

func (e *GoogleSheetsExecutor) GetDefinition() NodeDefinition {
	return NodeDefinition{
		Type: TypeGoogleSheets, Name: "Google Sheets", Description: "Reads, appends, updates, or upserts bounded rows in Google Sheets", Icon: "Table", Category: "COMMUNICATION", Retryable: true,
		Params: []ParamDefinition{
			{Name: "credential_id", Label: "Select Encrypted Credential", Type: "credential", Required: false, Description: "Encrypted Google OAuth2 access token or service account JSON"},
			{Name: "service_account_json", Label: "Service Account JSON Key (legacy)", Type: "textarea", Required: false, Description: "Legacy inline service account JSON. Prefer an encrypted credential."},
			{Name: "spreadsheet_id", Label: "Spreadsheet ID", Type: "text", Required: true, Description: "Spreadsheet ID from the Google Sheets URL"},
			{Name: "sheet_name", Label: "Sheet Name / Range", Type: "text", Default: "Sheet1", Required: true, Description: "Sheet name or range, for example Sheet1 or Sheet1!A:C"},
			{Name: "action", Label: "Action", Type: "select", Default: "APPEND", Options: []string{"READ", "APPEND", "UPDATE", "UPSERT"}, Required: true},
			{Name: "values_json", Label: "Values / Rows", Type: "json", Default: "[\n  \"Value 1\",\n  \"Value 2\"\n]", Required: false, Description: "One row or an array of rows; supports placeholders."},
			{Name: "match_column", Label: "UPSERT Match Column", Type: "number", Default: 1, Required: false, Description: "1-based column number used by UPSERT"},
			{Name: "match_value", Label: "UPSERT Match Value", Type: "text", Default: "", Required: false},
		},
	}
}
