package nodes

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"
)

const maxTelegramResponseBytes int64 = 256 << 10

type TelegramBotExecutor struct {
	client  *http.Client
	baseURL string
}

func NewTelegramBotExecutor() *TelegramBotExecutor {
	return &TelegramBotExecutor{
		client:  &http.Client{Timeout: 15 * time.Second},
		baseURL: "https://api.telegram.org",
	}
}

func NewTelegramBotExecutorWithClient(client *http.Client, baseURL string) *TelegramBotExecutor {
	if client == nil {
		client = &http.Client{Timeout: 15 * time.Second}
	}
	if strings.TrimSpace(baseURL) == "" {
		baseURL = "https://api.telegram.org"
	}
	return &TelegramBotExecutor{client: client, baseURL: baseURL}
}

func (e *TelegramBotExecutor) Execute(ctx *ExecutionContext, node *Node) (interface{}, error) {
	botToken, _ := node.Params["bot_token"].(string)
	chatID, _ := node.Params["chat_id"].(string)
	message, _ := node.Params["message"].(string)

	credID, _ := node.Params["credential_id"].(string)
	if credID != "" {
		token, ok := ctx.Credentials[credID]
		if !ok || strings.TrimSpace(token) == "" {
			return nil, fmt.Errorf("telegram credential is not available")
		}
		botToken = token
	}

	if botToken == "" || chatID == "" {
		return nil, fmt.Errorf("bot_token and chat_id are required")
	}

	urlStr, err := e.telegramMethodURL(botToken, "sendMessage")
	if err != nil {
		return nil, err
	}
	payload := map[string]interface{}{
		"chat_id":    chatID,
		"text":       message,
		"parse_mode": "HTML",
	}

	payloadBytes, _ := json.Marshal(payload)
	req, err := http.NewRequestWithContext(ctx.Context, http.MethodPost, urlStr, bytes.NewBuffer(payloadBytes))
	if err != nil {
		return nil, fmt.Errorf("telegram API request could not be created")
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := e.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("telegram API request failed: %s", redactTelegramText(err.Error()))
	}
	defer resp.Body.Close()

	respBytes, err := io.ReadAll(io.LimitReader(resp.Body, maxTelegramResponseBytes+1))
	if err != nil {
		return nil, fmt.Errorf("telegram API response could not be read")
	}
	if int64(len(respBytes)) > maxTelegramResponseBytes {
		return nil, fmt.Errorf("telegram API response exceeds %d byte limit", maxTelegramResponseBytes)
	}
	var result map[string]interface{}
	json.Unmarshal(respBytes, &result)

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("telegram API error (%d): %s", resp.StatusCode, redactTelegramErrorBody(respBytes))
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

func (e *TelegramBotExecutor) telegramMethodURL(botToken, method string) (string, error) {
	base := strings.TrimRight(e.baseURL, "/")
	parsed, err := url.Parse(base)
	if err != nil || !parsed.IsAbs() || parsed.Host == "" || (parsed.Scheme != "http" && parsed.Scheme != "https") {
		return "", fmt.Errorf("telegram API base URL is invalid")
	}
	return fmt.Sprintf("%s/bot%s/%s", base, botToken, method), nil
}

func redactTelegramErrorBody(body []byte) string {
	text := string(body)
	if len(text) > 4096 {
		text = text[:4096]
	}
	return redactTelegramText(text)
}

func redactTelegramText(text string) string {
	patterns := []*regexp.Regexp{
		regexp.MustCompile(`(?i)bot[0-9]+:[A-Za-z0-9_-]+`),
		regexp.MustCompile(`(?i)(bot_token|token|authorization|password|secret)["'=:\s]+[^"',}\s]+`),
	}
	for _, pattern := range patterns {
		text = pattern.ReplaceAllString(text, "[REDACTED]")
	}
	return text
}
