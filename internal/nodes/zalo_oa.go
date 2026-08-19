package nodes

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const (
	defaultZaloOAMessageEndpoint       = "https://openapi.zalo.me/v3.0/oa/message/cs"
	maxZaloOAResponseBytes       int64 = 256 << 10
	maxZaloOATextCharacters            = 2000
)

type ZaloOAExecutor struct {
	client   *http.Client
	endpoint string
}

func NewZaloOAExecutor() *ZaloOAExecutor {
	return &ZaloOAExecutor{
		client:   &http.Client{Timeout: 20 * time.Second},
		endpoint: defaultZaloOAMessageEndpoint,
	}
}

func NewZaloOAExecutorWithClient(client *http.Client, endpoint string) *ZaloOAExecutor {
	if client == nil {
		client = &http.Client{Timeout: 20 * time.Second}
	}
	if strings.TrimSpace(endpoint) == "" {
		endpoint = defaultZaloOAMessageEndpoint
	}
	return &ZaloOAExecutor{client: client, endpoint: strings.TrimSpace(endpoint)}
}

func (e *ZaloOAExecutor) Execute(ctx *ExecutionContext, node *Node) (interface{}, error) {
	accessToken, userID, message, endpoint, err := zaloOARequestParams(ctx, node, e.endpoint)
	if err != nil {
		return nil, err
	}
	payload := map[string]interface{}{
		"recipient": map[string]interface{}{"user_id": userID},
		"message":   map[string]interface{}{"text": message},
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("Zalo OA request could not be encoded")
	}
	req, err := http.NewRequestWithContext(ctx.Context, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("Zalo OA request could not be created")
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("access_token", accessToken)

	resp, err := e.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("Goflow could not connect to Zalo OA")
	}
	defer resp.Body.Close()
	respBody, err := io.ReadAll(io.LimitReader(resp.Body, maxZaloOAResponseBytes+1))
	if err != nil {
		return nil, fmt.Errorf("Zalo OA response could not be read")
	}
	if int64(len(respBody)) > maxZaloOAResponseBytes {
		return nil, fmt.Errorf("Zalo OA response exceeds %d byte limit", maxZaloOAResponseBytes)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(respBody, &result); err != nil {
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			return nil, fmt.Errorf("Zalo OA returned HTTP %d", resp.StatusCode)
		}
		return nil, fmt.Errorf("Zalo OA returned an invalid JSON response")
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("Zalo OA returned HTTP %d: %s", resp.StatusCode, zaloOAErrorMessage(result))
	}
	if errorCode := zaloOAErrorCode(result); errorCode != 0 {
		return nil, fmt.Errorf("Zalo OA rejected the message (error %d): %s", errorCode, zaloOAErrorMessage(result))
	}

	return map[string]interface{}{
		"ok":         true,
		"user_id":    userID,
		"message_id": zaloOAMessageID(result),
		"response":   result,
	}, nil
}

func (e *ZaloOAExecutor) Validate(node *Node) error {
	_, _, _, _, err := zaloOARequestParams(nil, node, e.endpoint)
	return err
}

func zaloOARequestParams(ctx *ExecutionContext, node *Node, defaultEndpoint string) (string, string, string, string, error) {
	accessToken, _ := node.Params["access_token"].(string)
	credentialID, _ := node.Params["credential_id"].(string)
	userID, _ := node.Params["user_id"].(string)
	message, _ := node.Params["message"].(string)
	endpoint, _ := node.Params["endpoint"].(string)
	accessToken = strings.TrimSpace(accessToken)
	credentialID = strings.TrimSpace(credentialID)
	userID = strings.TrimSpace(userID)
	message = strings.TrimSpace(message)
	endpoint = strings.TrimSpace(endpoint)
	if endpoint == "" {
		endpoint = defaultEndpoint
	}
	if credentialID != "" {
		if ctx == nil {
			accessToken = "validation-placeholder"
		} else {
			secret, ok := ctx.Credentials[credentialID]
			if !ok || strings.TrimSpace(secret) == "" {
				return "", "", "", "", fmt.Errorf("Zalo OA credential is not available")
			}
			accessToken = strings.TrimSpace(secret)
		}
	}
	if accessToken == "" {
		return "", "", "", "", fmt.Errorf("Zalo OA requires access_token or credential_id")
	}
	if userID == "" {
		return "", "", "", "", fmt.Errorf("Zalo OA requires user_id")
	}
	if message == "" {
		return "", "", "", "", fmt.Errorf("Zalo OA requires message")
	}
	if len([]rune(message)) > maxZaloOATextCharacters {
		return "", "", "", "", fmt.Errorf("Zalo OA text message exceeds %d character limit", maxZaloOATextCharacters)
	}
	parsed, err := url.Parse(endpoint)
	if err != nil || !parsed.IsAbs() || parsed.Host == "" || (parsed.Scheme != "http" && parsed.Scheme != "https") {
		return "", "", "", "", fmt.Errorf("Zalo OA endpoint must be an absolute http/https URL")
	}
	return accessToken, userID, message, endpoint, nil
}

func zaloOAErrorCode(result map[string]interface{}) int {
	value, ok := result["error"]
	if !ok {
		return 0
	}
	switch typed := value.(type) {
	case float64:
		return int(typed)
	case int:
		return typed
	case json.Number:
		parsed, _ := typed.Int64()
		return int(parsed)
	}
	return -1
}

func zaloOAErrorMessage(result map[string]interface{}) string {
	for _, key := range []string{"message", "error_name", "error_description"} {
		if value, ok := result[key].(string); ok && strings.TrimSpace(value) != "" {
			text := strings.TrimSpace(value)
			if len(text) > 512 {
				text = text[:512]
			}
			return text
		}
	}
	return "request failed"
}

func zaloOAMessageID(result map[string]interface{}) interface{} {
	data, _ := result["data"].(map[string]interface{})
	if data != nil {
		if value, ok := data["message_id"]; ok {
			return value
		}
	}
	if value, ok := result["message_id"]; ok {
		return value
	}
	return nil
}

func (e *ZaloOAExecutor) GetDefinition() NodeDefinition {
	return NodeDefinition{
		Type:        TypeZaloOA,
		Name:        "Zalo OA",
		Description: "Sends a text message through the Zalo Official Account OpenAPI customer-service endpoint",
		Icon:        "MessageCircle",
		Category:    "COMMUNICATION",
		Retryable:   false,
		Params: []ParamDefinition{
			{Name: "access_token", Label: "OA Access Token", Type: "text", Default: "", Required: false, Description: "Prefer an encrypted credential instead of pasting a token"},
			{Name: "credential_id", Label: "OA Access Token Credential", Type: "credential", Default: "", Required: false},
			{Name: "user_id", Label: "Recipient Zalo User ID", Type: "text", Required: true, Description: "Recipient must satisfy the current Zalo OA messaging, interaction, and quota rules"},
			{Name: "message", Label: "Message", Type: "textarea", Required: true, Description: "Plain text, maximum 2,000 characters"},
			{Name: "endpoint", Label: "OA Message Endpoint", Type: "text", Default: defaultZaloOAMessageEndpoint, Required: true, Description: "Advanced override for future OA OpenAPI endpoint changes or controlled testing"},
		},
	}
}
