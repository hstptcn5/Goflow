package nodes

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

const maxNotionPropertiesBytes = 1 << 20

type NotionPageExecutor struct{}

func NewNotionPageExecutor() *NotionPageExecutor { return &NotionPageExecutor{} }

func parseNotionProperties(raw interface{}) (map[string]interface{}, error) {
	if raw == nil {
		return nil, fmt.Errorf("properties_json is empty")
	}
	var properties map[string]interface{}
	switch typed := raw.(type) {
	case map[string]interface{}:
		properties = typed
	case string:
		text := strings.TrimSpace(typed)
		if text == "" {
			return nil, fmt.Errorf("properties_json is empty")
		}
		if len(text) > maxNotionPropertiesBytes {
			return nil, fmt.Errorf("properties_json exceeds %d byte limit", maxNotionPropertiesBytes)
		}
		if err := json.Unmarshal([]byte(text), &properties); err != nil {
			return nil, fmt.Errorf("invalid properties JSON: %w", err)
		}
	default:
		encoded, err := json.Marshal(raw)
		if err != nil {
			return nil, fmt.Errorf("properties_json could not be encoded: %w", err)
		}
		if len(encoded) > maxNotionPropertiesBytes {
			return nil, fmt.Errorf("properties_json exceeds %d byte limit", maxNotionPropertiesBytes)
		}
		if err := json.Unmarshal(encoded, &properties); err != nil {
			return nil, fmt.Errorf("properties_json must resolve to a JSON object")
		}
	}
	if len(properties) == 0 {
		return nil, fmt.Errorf("properties_json object is empty")
	}
	encoded, err := json.Marshal(properties)
	if err != nil || len(encoded) > maxNotionPropertiesBytes {
		return nil, fmt.Errorf("properties_json exceeds %d byte limit", maxNotionPropertiesBytes)
	}
	return properties, nil
}

func validateNotionNode(node *Node) error {
	databaseID, _ := node.Params["database_id"].(string)
	if strings.TrimSpace(databaseID) == "" {
		return fmt.Errorf("database_id is required")
	}
	if len(databaseID) > 512 || strings.ContainsAny(databaseID, "\r\n") {
		return fmt.Errorf("database_id is invalid")
	}
	credentialID, _ := node.Params["credential_id"].(string)
	directToken, _ := node.Params["notion_token"].(string)
	if strings.TrimSpace(credentialID) == "" && strings.TrimSpace(directToken) == "" {
		return fmt.Errorf("Notion token or encrypted credential is required")
	}
	if text, ok := node.Params["properties_json"].(string); ok && containsTemplateExpression(text) {
		return nil
	}
	_, err := parseNotionProperties(node.Params["properties_json"])
	return err
}

func (e *NotionPageExecutor) Execute(ctx *ExecutionContext, node *Node) (interface{}, error) {
	if err := validateNotionNode(node); err != nil {
		return nil, err
	}
	databaseID, _ := node.Params["database_id"].(string)
	properties, err := parseNotionProperties(node.Params["properties_json"])
	if err != nil {
		return nil, err
	}
	token, err := resolveNodeCredential(ctx, node, "notion_token", "Notion API token", "notion")
	if err != nil {
		return nil, err
	}
	bodyBytes, err := json.Marshal(map[string]interface{}{
		"parent":     map[string]string{"database_id": strings.TrimSpace(databaseID)},
		"properties": properties,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to marshal Notion payload: %w", err)
	}
	if len(bodyBytes) > maxNotionPropertiesBytes+(64<<10) {
		return nil, fmt.Errorf("Notion request payload is too large")
	}
	req, err := http.NewRequestWithContext(ctx.Context, http.MethodPost, "https://api.notion.com/v1/pages", bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, fmt.Errorf("failed to create Notion request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Notion-Version", "2022-06-28")
	req.Header.Set("Content-Type", "application/json")
	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("Notion API request failed: %w", err)
	}
	defer resp.Body.Close()
	respBytes, err := readNodeResponseBody(resp, maxGoogleAPIResponseBytes)
	if err != nil {
		return nil, fmt.Errorf("Notion API %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("Notion API error (status %d): %s", resp.StatusCode, boundedNodeErrorText(respBytes))
	}
	var result map[string]interface{}
	if err := json.Unmarshal(respBytes, &result); err != nil {
		return nil, fmt.Errorf("Notion API returned invalid JSON: %w", err)
	}
	return result, nil
}

func (e *NotionPageExecutor) Validate(node *Node) error { return validateNotionNode(node) }

func (e *NotionPageExecutor) GetDefinition() NodeDefinition {
	return NodeDefinition{
		Type: TypeNotionPage, Name: "Notion Page", Description: "Creates a bounded new page inside a Notion database", Icon: "BookOpen", Category: "COMMUNICATION",
		Params: []ParamDefinition{
			{Name: "credential_id", Label: "Select Encrypted Credential", Type: "credential", Required: false, Description: "Encrypted Notion integration token", CredentialProviders: []string{"notion"}},
			{Name: "notion_token", Label: "Notion API Token (legacy)", Type: "password", Required: false, Description: "Legacy direct Notion API token. Prefer an encrypted credential."},
			{Name: "database_id", Label: "Database ID", Type: "text", Required: true, Description: "Database ID from the Notion database URL"},
			{Name: "properties_json", Label: "Properties (Notion JSON Format)", Type: "textarea", Default: "{\n  \"Name\": {\n    \"title\": [{\n      \"text\": {\"content\": \"New Task from Goflow\"}\n    }]\n  }\n}", Required: true, Description: "Page properties as a JSON object using the Notion API format"},
		},
	}
}
