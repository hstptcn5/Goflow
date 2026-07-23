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

type SlackBotExecutor struct {
	client *http.Client
}

func NewSlackBotExecutor() *SlackBotExecutor {
	return &SlackBotExecutor{
		client: &http.Client{
			Timeout: 15 * time.Second,
		},
	}
}

func (e *SlackBotExecutor) Execute(ctx *ExecutionContext, node *Node) (interface{}, error) {
	webhookURL, _ := node.Params["webhook_url"].(string)
	text, _ := node.Params["text"].(string)
	channel, _ := node.Params["channel"].(string)
	username, _ := node.Params["username"].(string)

	if webhookURL == "" {
		return nil, fmt.Errorf("Slack Webhook URL is required")
	}

	payloadMap := map[string]interface{}{
		"text": text,
	}
	if channel != "" {
		payloadMap["channel"] = channel
	}
	if username != "" {
		payloadMap["username"] = username
	}

	jsonBytes, _ := json.Marshal(payloadMap)
	req, err := http.NewRequest("POST", webhookURL, bytes.NewBuffer(jsonBytes))
	if err != nil {
		return nil, fmt.Errorf("failed to create Slack request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := e.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("Slack webhook request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("Slack Webhook error (%d): %s", resp.StatusCode, string(body))
	}

	return map[string]interface{}{
		"status": "sent",
		"text":   text,
	}, nil
}

func (e *SlackBotExecutor) Validate(node *Node) error {
	webhookURL, _ := node.Params["webhook_url"].(string)
	if strings.TrimSpace(webhookURL) == "" {
		return fmt.Errorf("Slack Webhook URL is required")
	}
	return nil
}

func (e *SlackBotExecutor) GetDefinition() NodeDefinition {
	return NodeDefinition{
		Type:        TypeSlackBot,
		Name:        "Slack Webhook",
		Description: "Sends notification messages to a Slack channel",
		Icon:        "Slack",
		Category:    "COMMUNICATION",
		Params: []ParamDefinition{
			{
				Name:        "webhook_url",
				Label:       "Slack Webhook URL",
				Type:        "text",
				Default:     "",
				Required:    true,
				Description: "Incoming webhook URL from Slack Apps",
			},
			{
				Name:        "text",
				Label:       "Message Text",
				Type:        "textarea",
				Default:     "Goflow Alert: Slack message sent successfully!",
				Required:    true,
				Description: "Message text",
			},
			{
				Name:        "username",
				Label:       "Bot Name",
				Type:        "text",
				Default:     "Goflow Bot",
				Required:    false,
				Description: "Sender display name",
			},
		},
	}
}
