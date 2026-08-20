package nodes

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

const maxSlackResponseBytes int64 = 256 << 10

type SlackBotExecutor struct {
	client *http.Client
}

func NewSlackBotExecutor() *SlackBotExecutor {
	return &SlackBotExecutor{client: &http.Client{Timeout: 15 * time.Second}}
}

func (e *SlackBotExecutor) Execute(ctx *ExecutionContext, node *Node) (interface{}, error) {
	if err := validateSlackNode(node); err != nil {
		return nil, err
	}
	webhookURL, err := resolveNodeCredential(ctx, node, "webhook_url", "Slack webhook URL", "slack")
	if err != nil {
		return nil, err
	}
	if err := validateSlackWebhookURL(webhookURL); err != nil {
		return nil, err
	}
	text, _ := node.Params["text"].(string)
	channel, _ := node.Params["channel"].(string)
	username, _ := node.Params["username"].(string)
	payloadMap := map[string]interface{}{"text": text}
	if strings.TrimSpace(channel) != "" {
		payloadMap["channel"] = channel
	}
	if strings.TrimSpace(username) != "" {
		payloadMap["username"] = username
	}
	jsonBytes, err := json.Marshal(payloadMap)
	if err != nil {
		return nil, fmt.Errorf("Slack payload could not be encoded: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx.Context, http.MethodPost, webhookURL, bytes.NewReader(jsonBytes))
	if err != nil {
		return nil, fmt.Errorf("failed to create Slack request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := e.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("Slack webhook request failed: %w", err)
	}
	defer resp.Body.Close()
	body, err := readNodeResponseBody(resp, maxSlackResponseBytes)
	if err != nil {
		return nil, fmt.Errorf("Slack webhook %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("Slack Webhook error (%d): %s", resp.StatusCode, boundedNodeErrorText(body))
	}
	return map[string]interface{}{"status": "sent"}, nil
}

func validateSlackWebhookURL(raw string) error {
	if err := validateHTTPSHost(raw, "hooks.slack.com"); err != nil {
		return fmt.Errorf("Slack webhook URL is invalid: %w", err)
	}
	parsed, _ := validateAbsoluteHTTPURL(raw)
	if parsed == nil || !strings.HasPrefix(parsed.Path, "/services/") {
		return fmt.Errorf("Slack webhook URL must use the /services/ path")
	}
	return nil
}

func validateSlackNode(node *Node) error {
	webhookURL, _ := node.Params["webhook_url"].(string)
	credentialID, _ := node.Params["credential_id"].(string)
	if strings.TrimSpace(webhookURL) == "" && strings.TrimSpace(credentialID) == "" {
		return fmt.Errorf("Slack Webhook URL or credential is required")
	}
	if strings.TrimSpace(webhookURL) != "" && !containsTemplateExpression(webhookURL) {
		if err := validateSlackWebhookURL(webhookURL); err != nil {
			return err
		}
	}
	text, _ := node.Params["text"].(string)
	if strings.TrimSpace(text) == "" {
		return fmt.Errorf("Slack message text is required")
	}
	if len([]rune(text)) > 40000 {
		return fmt.Errorf("Slack message exceeds 40000 character limit")
	}
	username, _ := node.Params["username"].(string)
	if len([]rune(username)) > 80 {
		return fmt.Errorf("Slack username exceeds 80 character limit")
	}
	return nil
}

func (e *SlackBotExecutor) Validate(node *Node) error { return validateSlackNode(node) }

func (e *SlackBotExecutor) GetDefinition() NodeDefinition {
	return NodeDefinition{
		Type: TypeSlackBot, Name: "Slack Webhook", Description: "Sends notification messages to a Slack channel", Icon: "Slack", Category: "COMMUNICATION",
		Params: []ParamDefinition{
			{Name: "credential_id", Label: "Slack Webhook Credential", Type: "credential", Default: "", Required: false, Description: "Encrypted credential containing the complete Slack webhook URL", CredentialKinds: []string{"CUSTOM", "API_KEY"}, CredentialProviders: []string{"slack"}},
			{Name: "webhook_url", Label: "Slack Webhook URL (legacy)", Type: "password", Default: "", Required: false, Description: "Legacy direct incoming webhook URL. Prefer an encrypted credential."},
			{Name: "text", Label: "Message Text", Type: "textarea", Default: "Goflow Alert: Slack message sent successfully!", Required: true, Description: "Message text"},
			{Name: "channel", Label: "Channel Override", Type: "text", Default: "", Required: false, Description: "Optional channel override when supported by the webhook"},
			{Name: "username", Label: "Bot Name", Type: "text", Default: "Goflow Bot", Required: false, Description: "Sender display name when supported by the webhook"},
		},
	}
}
