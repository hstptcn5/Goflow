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

type DiscordBotExecutor struct {
	client *http.Client
}

func NewDiscordBotExecutor() *DiscordBotExecutor {
	return &DiscordBotExecutor{
		client: &http.Client{
			Timeout: 15 * time.Second,
		},
	}
}

func (e *DiscordBotExecutor) Execute(ctx *ExecutionContext, node *Node) (interface{}, error) {
	webhookURL, _ := node.Params["webhook_url"].(string)
	content, _ := node.Params["content"].(string)
	username, _ := node.Params["username"].(string)
	embedTitle, _ := node.Params["embed_title"].(string)
	embedDesc, _ := node.Params["embed_desc"].(string)

	if webhookURL == "" {
		return nil, fmt.Errorf("Discord Webhook URL is required")
	}

	payloadMap := map[string]interface{}{}
	if content != "" {
		payloadMap["content"] = content
	}
	if username != "" {
		payloadMap["username"] = username
	}

	if embedTitle != "" || embedDesc != "" {
		payloadMap["embeds"] = []map[string]interface{}{
			{
				"title":       embedTitle,
				"description": embedDesc,
				"color":       3447003, // Royal Blue
				"timestamp":   time.Now().Format(time.RFC3339),
			},
		}
	}

	jsonBytes, _ := json.Marshal(payloadMap)
	req, err := http.NewRequest("POST", webhookURL, bytes.NewBuffer(jsonBytes))
	if err != nil {
		return nil, fmt.Errorf("failed to create Discord request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := e.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("Discord webhook request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("Discord Webhook error (%d): %s", resp.StatusCode, string(body))
	}

	return map[string]interface{}{
		"status":      "sent",
		"webhook_url": webhookURL,
		"content":     content,
	}, nil
}

func (e *DiscordBotExecutor) Validate(node *Node) error {
	webhookURL, _ := node.Params["webhook_url"].(string)
	if strings.TrimSpace(webhookURL) == "" {
		return fmt.Errorf("Discord Webhook URL is required")
	}
	return nil
}

func (e *DiscordBotExecutor) GetDefinition() NodeDefinition {
	return NodeDefinition{
		Type:        TypeDiscordBot,
		Name:        "Discord Webhook",
		Description: "Sends messages and embed cards to a Discord channel",
		Icon:        "MessageSquare",
		Category:    "COMMUNICATION",
		Params: []ParamDefinition{
			{
				Name:        "webhook_url",
				Label:       "Discord Webhook URL",
				Type:        "text",
				Default:     "",
				Required:    true,
				Description: "Webhook URL created from Discord channel settings",
			},
			{
				Name:        "username",
				Label:       "Bot Display Name",
				Type:        "text",
				Default:     "Goflow Bot",
				Required:    false,
				Description: "Display name used by the Discord bot",
			},
			{
				Name:        "content",
				Label:       "Message Content",
				Type:        "textarea",
				Default:     "Goflow Alert: Workflow executed successfully!",
				Required:    false,
				Description: "Plain text message content",
			},
			{
				Name:        "embed_title",
				Label:       "Embed Title",
				Type:        "text",
				Default:     "Workflow Completed",
				Required:    false,
				Description: "Embed card title",
			},
			{
				Name:        "embed_desc",
				Label:       "Embed Description",
				Type:        "textarea",
				Default:     "Status: SUCCESS",
				Required:    false,
				Description: "Embed card description",
			},
		},
	}
}
