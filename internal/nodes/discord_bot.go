package nodes

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

const maxDiscordResponseBytes int64 = 256 << 10

type DiscordBotExecutor struct {
	client *http.Client
}

func NewDiscordBotExecutor() *DiscordBotExecutor {
	return &DiscordBotExecutor{client: &http.Client{Timeout: 15 * time.Second}}
}

func (e *DiscordBotExecutor) Execute(ctx *ExecutionContext, node *Node) (interface{}, error) {
	if err := validateDiscordNode(node); err != nil {
		return nil, err
	}
	webhookURL, err := resolveNodeCredential(ctx, node, "webhook_url", "Discord webhook URL", "discord")
	if err != nil {
		return nil, err
	}
	if err := validateDiscordWebhookURL(webhookURL); err != nil {
		return nil, err
	}
	content, _ := node.Params["content"].(string)
	username, _ := node.Params["username"].(string)
	embedTitle, _ := node.Params["embed_title"].(string)
	embedDesc, _ := node.Params["embed_desc"].(string)

	payloadMap := map[string]interface{}{}
	if content != "" {
		payloadMap["content"] = content
	}
	if username != "" {
		payloadMap["username"] = username
	}
	if embedTitle != "" || embedDesc != "" {
		payloadMap["embeds"] = []map[string]interface{}{{
			"title": embedTitle, "description": embedDesc, "color": 3447003,
			"timestamp": time.Now().UTC().Format(time.RFC3339),
		}}
	}
	jsonBytes, err := json.Marshal(payloadMap)
	if err != nil {
		return nil, fmt.Errorf("Discord payload could not be encoded: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx.Context, http.MethodPost, webhookURL, bytes.NewReader(jsonBytes))
	if err != nil {
		return nil, fmt.Errorf("failed to create Discord request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := e.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("Discord webhook request failed: %w", err)
	}
	defer resp.Body.Close()
	body, err := readNodeResponseBody(resp, maxDiscordResponseBytes)
	if err != nil {
		return nil, fmt.Errorf("Discord webhook %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("Discord Webhook error (%d): %s", resp.StatusCode, boundedNodeErrorText(body))
	}
	return map[string]interface{}{"status": "sent"}, nil
}

func validateDiscordWebhookURL(raw string) error {
	if err := validateHTTPSHost(raw, "discord.com", ".discord.com", "discordapp.com", ".discordapp.com"); err != nil {
		return fmt.Errorf("Discord webhook URL is invalid: %w", err)
	}
	parsed, _ := validateAbsoluteHTTPURL(raw)
	if parsed == nil || !strings.HasPrefix(parsed.Path, "/api/webhooks/") {
		return fmt.Errorf("Discord webhook URL must use the /api/webhooks/ path")
	}
	return nil
}

func validateDiscordNode(node *Node) error {
	webhookURL, _ := node.Params["webhook_url"].(string)
	credentialID, _ := node.Params["credential_id"].(string)
	if strings.TrimSpace(webhookURL) == "" && strings.TrimSpace(credentialID) == "" {
		return fmt.Errorf("Discord Webhook URL or credential is required")
	}
	if strings.TrimSpace(webhookURL) != "" && !containsTemplateExpression(webhookURL) {
		if err := validateDiscordWebhookURL(webhookURL); err != nil {
			return err
		}
	}
	content, _ := node.Params["content"].(string)
	username, _ := node.Params["username"].(string)
	title, _ := node.Params["embed_title"].(string)
	desc, _ := node.Params["embed_desc"].(string)
	if len([]rune(content)) > 2000 {
		return fmt.Errorf("Discord message content exceeds 2000 character limit")
	}
	if len([]rune(username)) > 80 {
		return fmt.Errorf("Discord username exceeds 80 character limit")
	}
	if len([]rune(title)) > 256 {
		return fmt.Errorf("Discord embed title exceeds 256 character limit")
	}
	if len([]rune(desc)) > 4096 {
		return fmt.Errorf("Discord embed description exceeds 4096 character limit")
	}
	if strings.TrimSpace(content) == "" && strings.TrimSpace(title) == "" && strings.TrimSpace(desc) == "" {
		return fmt.Errorf("Discord message requires content or an embed")
	}
	return nil
}

func (e *DiscordBotExecutor) Validate(node *Node) error { return validateDiscordNode(node) }

func (e *DiscordBotExecutor) GetDefinition() NodeDefinition {
	return NodeDefinition{
		Type: TypeDiscordBot, Name: "Discord Webhook", Description: "Sends messages and embed cards to a Discord channel", Icon: "MessageSquare", Category: "COMMUNICATION",
		Params: []ParamDefinition{
			{Name: "credential_id", Label: "Discord Webhook Credential", Type: "credential", Default: "", Required: false, Description: "Encrypted credential containing the complete Discord webhook URL", CredentialKinds: []string{"CUSTOM", "API_KEY"}, CredentialProviders: []string{"discord"}},
			{Name: "webhook_url", Label: "Discord Webhook URL (legacy)", Type: "password", Default: "", Required: false, Description: "Legacy direct webhook URL. Prefer an encrypted credential."},
			{Name: "username", Label: "Bot Display Name", Type: "text", Default: "Goflow Bot", Required: false, Description: "Display name used by the Discord webhook"},
			{Name: "content", Label: "Message Content", Type: "textarea", Default: "Goflow Alert: Workflow executed successfully!", Required: false, Description: "Plain text message content, maximum 2,000 characters"},
			{Name: "embed_title", Label: "Embed Title", Type: "text", Default: "Workflow Completed", Required: false, Description: "Embed card title"},
			{Name: "embed_desc", Label: "Embed Description", Type: "textarea", Default: "Status: SUCCESS", Required: false, Description: "Embed card description"},
		},
	}
}
