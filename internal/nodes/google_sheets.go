package nodes

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"golang.org/x/oauth2/google"
)

type GoogleSheetsExecutor struct{}

func NewGoogleSheetsExecutor() *GoogleSheetsExecutor {
	return &GoogleSheetsExecutor{}
}

func (e *GoogleSheetsExecutor) Execute(ctx *ExecutionContext, node *Node) (interface{}, error) {
	// 1. Resolve parameters
	credID, _ := node.Params["credential_id"].(string)
	directSA, _ := node.Params["service_account_json"].(string)
	spreadsheetID, _ := node.Params["spreadsheet_id"].(string)
	sheetName, _ := node.Params["sheet_name"].(string)
	action, _ := node.Params["action"].(string)
	valuesJSON, _ := node.Params["values_json"].(string)

	if spreadsheetID == "" {
		return nil, fmt.Errorf("spreadsheet_id is required")
	}
	if sheetName == "" {
		sheetName = "Sheet1"
	}
	action = strings.ToUpper(strings.TrimSpace(action))
	if action == "" {
		action = "APPEND"
	}

	// Resolve Service Account JSON key (prioritize Vault credential)
	saJSON := directSA
	if credID != "" {
		ctx.mu.RLock()
		decrypted, ok := ctx.Credentials[credID]
		ctx.mu.RUnlock()
		if ok && decrypted != "" {
			saJSON = decrypted
		}
	}

	if strings.TrimSpace(saJSON) == "" {
		return nil, fmt.Errorf("service_account_json is empty (please set it directly or select a valid credential)")
	}

	// 2. Generate Google OAuth2 Token using JWT config
	jwtConfig, err := google.JWTConfigFromJSON([]byte(saJSON), "https://www.googleapis.com/auth/spreadsheets")
	if err != nil {
		return nil, fmt.Errorf("invalid service account JSON: %w", err)
	}

	httpClient := &http.Client{Timeout: 10 * time.Second}
	ts := jwtConfig.TokenSource(context.Background())
	token, err := ts.Token()
	if err != nil {
		return nil, fmt.Errorf("failed to generate OAuth2 token: %w", err)
	}

	// 3. Perform REST requests
	if action == "APPEND" {
		if strings.TrimSpace(valuesJSON) == "" {
			return nil, fmt.Errorf("values_json array is empty")
		}

		var values []interface{}
		if err := json.Unmarshal([]byte(valuesJSON), &values); err != nil {
			// Fallback: parse as single string if it's not a JSON array
			values = []interface{}{valuesJSON}
		}

		// Google Sheets API expects a 2D array: [ [col1, col2, ...] ]
		payload := map[string]interface{}{
			"values": [][]interface{}{values},
		}
		payloadBytes, err := json.Marshal(payload)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal payload: %w", err)
		}

		url := fmt.Sprintf("https://sheets.googleapis.com/v4/spreadsheets/%s/values/%s:append?valueInputOption=USER_ENTERED",
			spreadsheetID, sheetName)

		req, err := http.NewRequest("POST", url, bytes.NewBuffer(payloadBytes))
		if err != nil {
			return nil, fmt.Errorf("failed to create http request: %w", err)
		}
		req.Header.Set("Authorization", "Bearer "+token.AccessToken)
		req.Header.Set("Content-Type", "application/json")

		resp, err := httpClient.Do(req)
		if err != nil {
			return nil, fmt.Errorf("http request failed: %w", err)
		}
		defer resp.Body.Close()

		respBytes, _ := io.ReadAll(resp.Body)
		if resp.StatusCode != http.StatusOK {
			return nil, fmt.Errorf("Google Sheets API error (status %d): %s", resp.StatusCode, string(respBytes))
		}

		var apiResult map[string]interface{}
		_ = json.Unmarshal(respBytes, &apiResult)

		return apiResult, nil
	} else if action == "READ" {
		url := fmt.Sprintf("https://sheets.googleapis.com/v4/spreadsheets/%s/values/%s", spreadsheetID, sheetName)

		req, err := http.NewRequest("GET", url, nil)
		if err != nil {
			return nil, fmt.Errorf("failed to create http request: %w", err)
		}
		req.Header.Set("Authorization", "Bearer "+token.AccessToken)

		resp, err := httpClient.Do(req)
		if err != nil {
			return nil, fmt.Errorf("http request failed: %w", err)
		}
		defer resp.Body.Close()

		respBytes, _ := io.ReadAll(resp.Body)
		if resp.StatusCode != http.StatusOK {
			return nil, fmt.Errorf("Google Sheets API error (status %d): %s", resp.StatusCode, string(respBytes))
		}

		var apiResult map[string]interface{}
		_ = json.Unmarshal(respBytes, &apiResult)

		return apiResult, nil
	}

	return nil, fmt.Errorf("unsupported Google Sheets action: %s", action)
}

func (e *GoogleSheetsExecutor) Validate(node *Node) error {
	spreadsheetID, _ := node.Params["spreadsheet_id"].(string)
	if strings.TrimSpace(spreadsheetID) == "" {
		return fmt.Errorf("spreadsheet_id is required")
	}
	return nil
}

func (e *GoogleSheetsExecutor) GetDefinition() NodeDefinition {
	return NodeDefinition{
		Type:        TypeGoogleSheets,
		Name:        "Google Sheets",
		Description: "Đọc hoặc thêm dòng mới vào Google Sheets bằng Service Account",
		Icon:        "Table",
		Category:    "COMMUNICATION",
		Params: []ParamDefinition{
			{
				Name:        "credential_id",
				Label:       "Select Encrypted Credential",
				Type:        "credential",
				Required:    false,
				Description: "Chọn tệp khóa Service Account JSON đã mã hóa từ Vault",
			},
			{
				Name:        "service_account_json",
				Label:       "Service Account JSON Key",
				Type:        "textarea",
				Required:    false,
				Description: "Dán nội dung khóa Service Account JSON trực tiếp (nếu không dùng Vault)",
			},
			{
				Name:        "spreadsheet_id",
				Label:       "Spreadsheet ID",
				Type:        "text",
				Required:    true,
				Description: "ID của bảng tính (lấy từ đường dẫn Google Sheets URL)",
			},
			{
				Name:        "sheet_name",
				Label:       "Sheet Name / Range",
				Type:        "text",
				Default:     "Sheet1",
				Required:    true,
				Description: "Tên thẻ trang tính (ví dụ: Sheet1 hoặc Sheet1!A:C)",
			},
			{
				Name:        "action",
				Label:       "Action",
				Type:        "select",
				Default:     "APPEND",
				Options:     []string{"APPEND", "READ"},
				Required:    true,
				Description: "Chọn thêm mới dòng (APPEND) hoặc đọc dữ liệu (READ)",
			},
			{
				Name:        "values_json",
				Label:       "Values Array (For APPEND)",
				Type:        "textarea",
				Default:     "[\n  \"Value 1\",\n  \"Value 2\"\n]",
				Required:    false,
				Description: "Mảng JSON chứa các giá trị cho các cột, ví dụ: [\"{{trigger.name}}\", \"{{trigger.email}}\"]",
			},
		},
	}
}
