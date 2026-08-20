package nodes

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const (
	maxGoogleAPIResponseBytes int64 = 2 << 20
	maxSheetsValuesBytes            = 1 << 20
	maxSheetsValuesCount            = 1000
)

type GoogleSheetsExecutor struct{}

func NewGoogleSheetsExecutor() *GoogleSheetsExecutor { return &GoogleSheetsExecutor{} }

func parseSheetsAction(node *Node) (string, error) {
	action, _ := node.Params["action"].(string)
	action = strings.ToUpper(strings.TrimSpace(action))
	if action == "" {
		action = "APPEND"
	}
	if action != "APPEND" && action != "READ" {
		return "", fmt.Errorf("unsupported Google Sheets action: %s", action)
	}
	return action, nil
}

func parseSheetsValues(raw interface{}) ([]interface{}, error) {
	if raw == nil {
		return nil, fmt.Errorf("values_json array is empty")
	}
	var values []interface{}
	switch typed := raw.(type) {
	case []interface{}:
		values = typed
	case string:
		text := strings.TrimSpace(typed)
		if text == "" {
			return nil, fmt.Errorf("values_json array is empty")
		}
		if len(text) > maxSheetsValuesBytes {
			return nil, fmt.Errorf("values_json exceeds %d byte limit", maxSheetsValuesBytes)
		}
		if err := json.Unmarshal([]byte(text), &values); err != nil {
			return nil, fmt.Errorf("values_json must be a valid JSON array: %w", err)
		}
	default:
		encoded, err := json.Marshal(raw)
		if err != nil {
			return nil, fmt.Errorf("values_json could not be encoded: %w", err)
		}
		if len(encoded) > maxSheetsValuesBytes {
			return nil, fmt.Errorf("values_json exceeds %d byte limit", maxSheetsValuesBytes)
		}
		if err := json.Unmarshal(encoded, &values); err != nil {
			return nil, fmt.Errorf("values_json must resolve to a JSON array")
		}
	}
	if len(values) == 0 {
		return nil, fmt.Errorf("values_json array is empty")
	}
	if len(values) > maxSheetsValuesCount {
		return nil, fmt.Errorf("values_json contains %d values; maximum is %d", len(values), maxSheetsValuesCount)
	}
	encoded, err := json.Marshal(values)
	if err != nil || len(encoded) > maxSheetsValuesBytes {
		return nil, fmt.Errorf("values_json exceeds %d byte limit", maxSheetsValuesBytes)
	}
	return values, nil
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
	if action == "APPEND" {
		if text, ok := node.Params["values_json"].(string); ok && containsTemplateExpression(text) {
			return nil
		}
		_, err = parseSheetsValues(node.Params["values_json"])
	}
	return err
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

	if action == "APPEND" {
		values, err := parseSheetsValues(node.Params["values_json"])
		if err != nil {
			return nil, err
		}
		payloadBytes, err := json.Marshal(map[string]interface{}{"values": [][]interface{}{values}})
		if err != nil {
			return nil, fmt.Errorf("failed to marshal Google Sheets payload: %w", err)
		}
		req, err := http.NewRequestWithContext(ctx.Context, http.MethodPost, base+":append?valueInputOption=USER_ENTERED", bytes.NewReader(payloadBytes))
		if err != nil {
			return nil, fmt.Errorf("failed to create Google Sheets request: %w", err)
		}
		req.Header.Set("Authorization", "Bearer "+accessToken)
		req.Header.Set("Content-Type", "application/json")
		return doGoogleJSONRequest(client, req, http.StatusOK)
	}
	req, err := http.NewRequestWithContext(ctx.Context, http.MethodGet, base, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create Google Sheets request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)
	return doGoogleJSONRequest(client, req, http.StatusOK)
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
		Type: TypeGoogleSheets, Name: "Google Sheets", Description: "Reads from or appends a bounded row to Google Sheets", Icon: "Table", Category: "COMMUNICATION", Retryable: true,
		Params: []ParamDefinition{
			{Name: "credential_id", Label: "Select Encrypted Credential", Type: "credential", Required: false, Description: "Encrypted Google OAuth2 access token or service account JSON"},
			{Name: "service_account_json", Label: "Service Account JSON Key (legacy)", Type: "textarea", Required: false, Description: "Legacy inline service account JSON. Prefer an encrypted credential."},
			{Name: "spreadsheet_id", Label: "Spreadsheet ID", Type: "text", Required: true, Description: "Spreadsheet ID from the Google Sheets URL"},
			{Name: "sheet_name", Label: "Sheet Name / Range", Type: "text", Default: "Sheet1", Required: true, Description: "Sheet name or range, for example Sheet1 or Sheet1!A:C"},
			{Name: "action", Label: "Action", Type: "select", Default: "APPEND", Options: []string{"APPEND", "READ"}, Required: true, Description: "Choose APPEND to add one row or READ to fetch data"},
			{Name: "values_json", Label: "Values Array (For APPEND)", Type: "textarea", Default: "[\n  \"Value 1\",\n  \"Value 2\"\n]", Required: false, Description: "Valid JSON array of up to 1,000 column values. Supports placeholders."},
		},
	}
}
