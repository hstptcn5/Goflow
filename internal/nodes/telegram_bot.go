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

type TelegramBotExecutor struct {
	client *http.Client
}

func NewTelegramBotExecutor() *TelegramBotExecutor {
	return &TelegramBotExecutor{
		client: &http.Client{Timeout: 15 * time.Second},
	}
}

func (e *TelegramBotExecutor) Execute(ctx *ExecutionContext, node *Node) (interface{}, error) {
	botToken, _ := node.Params["bot_token"].(string)
	chatID, _ := node.Params["chat_id"].(string)
	message, _ := node.Params["message"].(string)

	// N???u d??ng Credential ID
	credID, _ := node.Params["credential_id"].(string)
	if credID != "" {
		if token, ok := ctx.Credentials[credID]; ok {
			botToken = token
		}
	}

	if botToken == "" || chatID == "" {
		return nil, fmt.Errorf("bot_token and chat_id are required")
	}

	urlStr := fmt.Sprintf("https://api.telegram.org/bot%s/sendMessage", botToken)
	payload := map[string]interface{}{
		"chat_id":    chatID,
		"text":       message,
		"parse_mode": "HTML",
	}

	payloadBytes, _ := json.Marshal(payload)
	resp, err := e.client.Post(urlStr, "application/json", bytes.NewBuffer(payloadBytes))
	if err != nil {
		return nil, fmt.Errorf("telegram API request failed: %w", err)
	}
	defer resp.Body.Close()

	respBytes, _ := io.ReadAll(resp.Body)
	var result map[string]interface{}
	json.Unmarshal(respBytes, &result)

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("telegram API error (%d): %s", resp.StatusCode, string(respBytes))
	}

	return result, nil
}

func (e *TelegramBotExecutor) Validate(node *Node) error {
	chatID, _ := node.Params["chat_id"].(string)
	if strings.TrimSpace(chatID) == "" {
		return fmt.Errorf("Telegram Node requires 'chat_id'")
	}
	return nil
}

func (e *TelegramBotExecutor) GetDefinition() NodeDefinition {
	return NodeDefinition{
		Type:        TypeTelegramBot,
		Name:        "Telegram Bot",
		Description: "Sends notification messages to a Telegram chat or channel",
		Icon:        "Send",
		Category:    "ACTION",
		Params: []ParamDefinition{
			{
				Name:        "bot_token",
				Label:       "Bot Token",
				Type:        "text",
				Default:     "",
				Required:    false,
				Description: "Bot token from @BotFather",
			},
			{
				Name:        "credential_id",
				Label:       "Credential Token",
				Type:        "credential",
				Default:     "",
				Required:    false,
				Description: "Or select an encrypted token saved in Credentials",
			},
			{
				Name:        "chat_id",
				Label:       "Chat ID / Channel Name",
				Type:        "text",
				Default:     "",
				Required:    true,
				Description: "Chat ID, group ID, or @channel_name",
			},
			{
				Name:        "message",
				Label:       "Message Content",
				Type:        "textarea",
				Default:     "Goflow Execution Completed!",
				Required:    true,
				Description: "Message content. HTML tags are supported",
			},
		},
	}
}
