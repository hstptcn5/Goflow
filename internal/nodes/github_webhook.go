package nodes

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
)

type GithubWebhookExecutor struct{}

func NewGithubWebhookExecutor() *GithubWebhookExecutor { return &GithubWebhookExecutor{} }

func (e *GithubWebhookExecutor) Execute(ctx *ExecutionContext, node *Node) (interface{}, error) {
	triggerData, ok := ctx.GetOutput("$trigger")
	if !ok {
		return nil, fmt.Errorf("GitHub Webhook Trigger requires a trigger payload")
	}
	triggerMap, ok := triggerData.(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("GitHub Webhook Trigger payload must be an object")
	}

	secret, _ := node.Params["secret"].(string)
	secret = strings.TrimSpace(secret)
	if secret != "" {
		headers := webhookHeaderMap(triggerMap["headers"])
		sigHeader := firstHeaderValue(headers, "x-hub-signature-256")
		if sigHeader == "" {
			return nil, fmt.Errorf("GitHub signature validation failed: X-Hub-Signature-256 header is missing")
		}
		if !strings.HasPrefix(sigHeader, "sha256=") {
			return nil, fmt.Errorf("GitHub signature validation failed: signature format must start with sha256=")
		}
		expectedSig, err := hex.DecodeString(strings.TrimPrefix(sigHeader, "sha256="))
		if err != nil || len(expectedSig) != sha256.Size {
			return nil, fmt.Errorf("GitHub signature validation failed: signature is not a valid SHA-256 digest")
		}

		// GitHub signs the exact HTTP request body bytes. Re-marshalling a decoded
		// JSON object can change whitespace/order and must never be used for HMAC.
		bodyRaw, ok := triggerMap["body_raw"].(string)
		if !ok || bodyRaw == "" {
			return nil, fmt.Errorf("GitHub signature validation failed: exact body_raw is required when a webhook secret is configured")
		}
		mac := hmac.New(sha256.New, []byte(secret))
		_, _ = mac.Write([]byte(bodyRaw))
		if !hmac.Equal(mac.Sum(nil), expectedSig) {
			return nil, fmt.Errorf("GitHub signature validation failed: signature mismatch")
		}
	}

	if body, exists := triggerMap["body"]; exists && body != nil {
		return body, nil
	}
	return triggerMap, nil
}

func webhookHeaderMap(raw interface{}) map[string]interface{} {
	if headers, ok := raw.(map[string]interface{}); ok {
		return headers
	}
	if headers, ok := raw.(map[string]string); ok {
		result := make(map[string]interface{}, len(headers))
		for key, value := range headers {
			result[key] = value
		}
		return result
	}
	return map[string]interface{}{}
}

func firstHeaderValue(headers map[string]interface{}, name string) string {
	for key, value := range headers {
		if !strings.EqualFold(key, name) {
			continue
		}
		switch typed := value.(type) {
		case string:
			return strings.TrimSpace(typed)
		case []string:
			if len(typed) > 0 {
				return strings.TrimSpace(typed[0])
			}
		case []interface{}:
			if len(typed) > 0 {
				if text, ok := typed[0].(string); ok {
					return strings.TrimSpace(text)
				}
			}
		}
	}
	return ""
}

func (e *GithubWebhookExecutor) Validate(node *Node) error {
	secret, _ := node.Params["secret"].(string)
	if len([]byte(secret)) > 4096 {
		return fmt.Errorf("GitHub webhook secret exceeds 4096 byte limit")
	}
	return nil
}

func (e *GithubWebhookExecutor) GetDefinition() NodeDefinition {
	return NodeDefinition{
		Type: TypeGithubWebhook, Name: "GitHub Webhook", Description: "Starts a workflow from GitHub webhook events with signature verification", Icon: "GitBranch", Category: "TRIGGER", Retryable: true,
		Params: []ParamDefinition{{Name: "secret", Label: "Webhook Secret", Type: "password", Required: false, Description: "Shared secret used to verify GitHub X-Hub-Signature-256 against the exact raw request body"}},
	}
}
