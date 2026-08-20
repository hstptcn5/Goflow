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
	"unicode/utf8"

	"goflow/internal/apperror"
)

const (
	maxTelegramResponseBytes int64 = 256 << 10
	maxTelegramMessageRunes        = 4096
)

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

func normalizeTelegramParseMode(raw string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "", "plain", "plain text", "none":
		return "", nil
	case "html":
		return "HTML", nil
	case "markdownv2", "markdown v2":
		return "MarkdownV2", nil
	default:
		return "", fmt.Errorf("Telegram parse_mode must be Plain text, HTML, or MarkdownV2")
	}
}

func validateTelegramMessage(message string) error {
	if strings.TrimSpace(message) == "" {
		return fmt.Errorf("Telegram Node requires 'message'")
	}
	if utf8.RuneCountInString(message) > maxTelegramMessageRunes {
		return apperror.New("telegram_message_too_long", fmt.Sprintf("Tin nhắn Telegram vượt quá giới hạn %d ký tự. Hãy rút gọn nội dung trước khi gửi.", maxTelegramMessageRunes))
	}
	return nil
}

func (e *TelegramBotExecutor) Execute(ctx *ExecutionContext, node *Node) (interface{}, error) {
	botToken, _ := node.Params["bot_token"].(string)
	chatID, _ := node.Params["chat_id"].(string)
	message, _ := node.Params["message"].(string)
	parseModeRaw, _ := node.Params["parse_mode"].(string)

	credID, _ := node.Params["credential_id"].(string)
	if credID != "" {
		token, ok := ctx.Credentials[credID]
		if !ok || strings.TrimSpace(token) == "" {
			return nil, fmt.Errorf("telegram credential is not available")
		}
		botToken = token
	}

	if strings.TrimSpace(botToken) == "" || strings.TrimSpace(chatID) == "" {
		return nil, fmt.Errorf("bot_token and chat_id are required")
	}
	if err := validateTelegramMessage(message); err != nil {
		return nil, err
	}
	parseMode, err := normalizeTelegramParseMode(parseModeRaw)
	if err != nil {
		return nil, err
	}

	urlStr, err := e.telegramMethodURL(botToken, "sendMessage")
	if err != nil {
		return nil, err
	}
	payload := map[string]interface{}{
		"chat_id": chatID,
		"text":    message,
	}
	if parseMode != "" {
		payload["parse_mode"] = parseMode
	}

	payloadBytes, _ := json.Marshal(payload)
	req, err := http.NewRequestWithContext(ctx.Context, http.MethodPost, urlStr, bytes.NewBuffer(payloadBytes))
	if err != nil {
		return nil, fmt.Errorf("telegram API request could not be created")
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := e.client.Do(req)
	if err != nil {
		return nil, apperror.New("telegram_unreachable", "GoFlow không thể kết nối tới Telegram. Hãy thử lại sau.")
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
	_ = json.Unmarshal(respBytes, &result)

	ok, _ := result["ok"].(bool)
	if resp.StatusCode != http.StatusOK || !ok {
		return nil, telegramAPIError(resp.StatusCode, result, respBytes)
	}

	return result, nil
}

func telegramAPIError(status int, result map[string]interface{}, body []byte) error {
	description, _ := result["description"].(string)
	description = strings.TrimSpace(redactTelegramText(description))
	lower := strings.ToLower(description)

	if status == http.StatusUnauthorized {
		return apperror.New("telegram_unauthorized", "Telegram từ chối Bot Token. Hãy kiểm tra lại credential của bot.")
	}
	if status == http.StatusTooManyRequests {
		return apperror.New("telegram_rate_limited", "Telegram đang giới hạn tần suất gửi tin. Hãy chờ một lúc rồi thử lại.")
	}
	if strings.Contains(lower, "can't parse entities") || strings.Contains(lower, "can't find end tag") || strings.Contains(lower, "unsupported start tag") {
		return apperror.New("telegram_message_format_invalid", "Telegram không đọc được định dạng tin nhắn. Hãy dùng Văn bản thường hoặc sửa cú pháp HTML/MarkdownV2.")
	}
	if strings.Contains(lower, "message is too long") {
		return apperror.New("telegram_message_too_long", fmt.Sprintf("Tin nhắn Telegram vượt quá giới hạn %d ký tự. Hãy rút gọn nội dung trước khi gửi.", maxTelegramMessageRunes))
	}
	if strings.Contains(lower, "bot was blocked by the user") {
		return apperror.New("telegram_bot_blocked", "Người nhận đã chặn bot Telegram. Hãy bỏ chặn bot rồi thử lại.")
	}
	if strings.Contains(lower, "chat not found") || strings.Contains(lower, "bot can't initiate conversation") || strings.Contains(lower, "bot cannot initiate conversation") {
		return apperror.New("telegram_chat_inaccessible", "Telegram không tìm thấy hoặc chưa cho bot truy cập chat này. Hãy gửi /start cho bot và kiểm tra Chat ID.")
	}
	if strings.Contains(lower, "not enough rights") || strings.Contains(lower, "have no rights") || strings.Contains(lower, "forbidden") {
		return apperror.New("telegram_permission_denied", "Bot Telegram không có đủ quyền gửi tin vào chat hoặc kênh này.")
	}

	if description == "" {
		description = redactTelegramErrorBody(body)
	}
	if len(description) > 1000 {
		description = description[:1000]
	}
	if description == "" {
		description = fmt.Sprintf("HTTP %d", status)
	}
	return apperror.New("telegram_api_error", fmt.Sprintf("Telegram từ chối gửi tin: %s", description))
}

func (e *TelegramBotExecutor) Validate(node *Node) error {
	chatID, _ := node.Params["chat_id"].(string)
	if strings.TrimSpace(chatID) == "" {
		return fmt.Errorf("Telegram Node requires 'chat_id'")
	}
	message, _ := node.Params["message"].(string)
	if err := validateTelegramMessage(message); err != nil {
		return err
	}
	parseMode, _ := node.Params["parse_mode"].(string)
	if _, err := normalizeTelegramParseMode(parseMode); err != nil {
		return err
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
				Description: "Message content. Plain text is safest for AI-generated output.",
			},
			{
				Name:        "parse_mode",
				Label:       "Parse Mode",
				Type:        "select",
				Default:     "Plain text",
				Options:     []string{"Plain text", "HTML", "MarkdownV2"},
				Required:    false,
				Description: "Plain text sends content literally. Use HTML or MarkdownV2 only when the message is intentionally formatted.",
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
