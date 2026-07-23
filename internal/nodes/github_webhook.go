package nodes

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
)

type GithubWebhookExecutor struct{}

func NewGithubWebhookExecutor() *GithubWebhookExecutor {
	return &GithubWebhookExecutor{}
}

func (e *GithubWebhookExecutor) Execute(ctx *ExecutionContext, node *Node) (interface{}, error) {
	triggerData, ok := ctx.GetOutput("$trigger")
	if !ok {
		return nil, fmt.Errorf("GitHub Webhook Trigger requires a trigger payload")
	}

	triggerMap, ok := triggerData.(map[string]interface{})
	if !ok {
		return triggerData, nil
	}

	// 1. Check Secret validation if set
	secret, _ := node.Params["secret"].(string)
	if secret != "" {
		headers, _ := triggerMap["headers"].(map[string]interface{})
		if headers == nil {
			headers = make(map[string]interface{})
		}

		// Look for signature header
		sigHeader := ""
		for k, v := range headers {
			if strings.ToLower(k) == "x-hub-signature-256" {
				if strList, isList := v.([]string); isList && len(strList) > 0 {
					sigHeader = strList[0]
				} else if strVal, isStr := v.(string); isStr {
					sigHeader = strVal
				}
				break
			}
		}

		if sigHeader == "" {
			return nil, fmt.Errorf("GitHub signature validation failed: X-Hub-Signature-256 header is missing")
		}

		// Get raw body
		bodyBytesStr, _ := triggerMap["body_raw"].(string)
		if bodyBytesStr == "" {
			// Fallback to body map marshal if body_raw is not available
			bodyMap, _ := triggerMap["body"].(map[string]interface{})
			if bodyMap != nil {
				if jsonBytes, err := json.Marshal(bodyMap); err == nil {
					bodyBytesStr = string(jsonBytes)
				}
			}
		}

		// Verify SHA-256 HMAC signature
		if !strings.HasPrefix(sigHeader, "sha256=") {
			return nil, fmt.Errorf("GitHub signature validation failed: signature format must start with sha256=")
		}
		expectedSig := sigHeader[7:]

		mac := hmac.New(sha256.New, []byte(secret))
		mac.Write([]byte(bodyBytesStr))
		computedSig := hex.EncodeToString(mac.Sum(nil))

		if !hmac.Equal([]byte(computedSig), []byte(expectedSig)) {
			return nil, fmt.Errorf("GitHub signature validation failed: signature mismatch")
		}
	}

	// 2. Return payload
	body, _ := triggerMap["body"]
	if body != nil {
		return body, nil
	}

	return triggerMap, nil
}

func (e *GithubWebhookExecutor) Validate(node *Node) error {
	return nil
}

func (e *GithubWebhookExecutor) GetDefinition() NodeDefinition {
	return NodeDefinition{
		Type:        TypeGithubWebhook,
		Name:        "GitHub Webhook",
		Description: "Kích hoạt workflow bằng các sự kiện GitHub Webhooks, hỗ trợ kiểm tra chữ ký Secret",
		Icon:        "GitBranch",
		Category:    "TRIGGER",
		Retryable:   true,
		Params: []ParamDefinition{
			{
				Name:        "secret",
				Label:       "Webhook Secret",
				Type:        "password",
				Required:    false,
				Description: "Khóa bảo mật Secret để xác thực chữ ký GitHub gửi tới (X-Hub-Signature-256)",
			},
		},
	}
}
