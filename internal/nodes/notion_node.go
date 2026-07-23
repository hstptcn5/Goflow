package nodes

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

type NotionPageExecutor struct{}

func NewNotionPageExecutor() *NotionPageExecutor {
	return &NotionPageExecutor{}
}

func (e *NotionPageExecutor) Execute(ctx *ExecutionContext, node *Node) (interface{}, error) {
	// 1. Resolve parameters
	credID, _ := node.Params["credential_id"].(string)
	directToken, _ := node.Params["notion_token"].(string)
	databaseID, _ := node.Params["database_id"].(string)
	propsJSON, _ := node.Params["properties_json"].(string)

	if databaseID == "" {
		return nil, fmt.Errorf("database_id is required")
	}

	token := directToken
	if credID != "" {
		ctx.mu.RLock()
		decrypted, ok := ctx.Credentials[credID]
		ctx.mu.RUnlock()
		if ok && decrypted != "" {
			token = decrypted
		}
	}

	if strings.TrimSpace(token) == "" {
		return nil, fmt.Errorf("notion_token is empty (please set it directly or select a valid credential)")
	}

	// Parse properties JSON
	var properties map[string]interface{}
	if strings.TrimSpace(propsJSON) == "" {
		return nil, fmt.Errorf("properties_json is empty")
	}
	if err := json.Unmarshal([]byte(propsJSON), &properties); err != nil {
		return nil, fmt.Errorf("invalid properties JSON: %w", err)
	}

	// 2. Setup request body
	requestBody := map[string]interface{}{
		"parent": map[string]string{
			"database_id": databaseID,
		},
		"properties": properties,
	}
	bodyBytes, err := json.Marshal(requestBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal notion payload: %w", err)
	}

	// 3. Make HTTP call
	httpClient := &http.Client{Timeout: 10 * time.Second}
	req, err := http.NewRequest("POST", "https://api.notion.com/v1/pages", bytes.NewBuffer(bodyBytes))
	if err != nil {
		return nil, fmt.Errorf("failed to create http request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Notion-Version", "2022-06-28")
	req.Header.Set("Content-Type", "application/json")

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("notion API request failed: %w", err)
	}
	defer resp.Body.Close()

	respBytes, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("Notion API error (status %d): %s", resp.StatusCode, string(respBytes))
	}

	var apiResult map[string]interface{}
	_ = json.Unmarshal(respBytes, &apiResult)
	return apiResult, nil
}

func (e *NotionPageExecutor) Validate(node *Node) error {
	databaseID, _ := node.Params["database_id"].(string)
	if strings.TrimSpace(databaseID) == "" {
		return fmt.Errorf("database_id is required")
	}
	return nil
}

func (e *NotionPageExecutor) GetDefinition() NodeDefinition {
	return NodeDefinition{
		Type:        TypeNotionPage,
		Name:        "Notion Page",
		Description: "Creates a new page inside a Notion database using an API token",
		Icon:        "BookOpen",
		Category:    "COMMUNICATION",
		Params: []ParamDefinition{
			{
				Name:        "credential_id",
				Label:       "Select Encrypted Credential",
				Type:        "credential",
				Required:    false,
				Description: "Select an encrypted Notion integration token from the vault",
			},
			{
				Name:        "notion_token",
				Label:       "Notion API Token",
				Type:        "password",
				Required:    false,
				Description: "Paste the Notion API token directly if not using the vault",
			},
			{
				Name:        "database_id",
				Label:       "Database ID",
				Type:        "text",
				Required:    true,
				Description: "Database ID from the Notion database URL",
			},
			{
				Name:        "properties_json",
				Label:       "Properties (Notion JSON Format)",
				Type:        "textarea",
				Default:     "{\n  \"Name\": {\n    \"title\": [\n      {\n        \"text\": {\n          \"content\": \"New Task from Goflow\"\n        }\n      }\n    ]\n  }\n}",
				Required:    true,
				Description: "Page properties as a JSON object using the Notion API format",
			},
		},
	}
}
