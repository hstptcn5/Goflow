package nodes

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"golang.org/x/oauth2/google"
)

type GmailRESTExecutor struct{}

func NewGmailRESTExecutor() *GmailRESTExecutor {
	return &GmailRESTExecutor{}
}

func (e *GmailRESTExecutor) Execute(ctx *ExecutionContext, node *Node) (interface{}, error) {
	// 1. Resolve parameters
	credID, _ := node.Params["credential_id"].(string)
	directSA, _ := node.Params["service_account_json"].(string)
	to, _ := node.Params["to"].(string)
	subject, _ := node.Params["subject"].(string)
	body, _ := node.Params["body"].(string)

	if to == "" {
		return nil, fmt.Errorf("recipient email 'to' is required")
	}

	saJSON := directSA
	if credID != "" {
		ctx.mu.RLock()
		decrypted, ok := ctx.Credentials[credID]
		ctx.mu.RUnlock()
		if ok && decrypted != "" {
			saJSON = decrypted
		}
	}

	if strings.TrimSpace(saJSON) == "" {
		return nil, fmt.Errorf("service_account_json is empty (please set it directly or select a valid credential)")
	}

	// 2. Generate Google OAuth2 Token using JWT config
	// Gmail REST API requires scopes: https://www.googleapis.com/auth/gmail.send
	// Note: Gmail API might require domain-wide delegation for service account to impersonate users
	// However, we still support direct sending using the service account credentials context
	jwtConfig, err := google.JWTConfigFromJSON([]byte(saJSON), "https://www.googleapis.com/auth/gmail.send")
	if err != nil {
		return nil, fmt.Errorf("invalid service account JSON: %w", err)
	}

	// If user wants to impersonate a specific user (Required for Gmail service accounts)
	impersonateUser, _ := node.Params["impersonate_user"].(string)
	if impersonateUser != "" {
		jwtConfig.Subject = impersonateUser
	}

	httpClient := &http.Client{Timeout: 15 * time.Second}
	ts := jwtConfig.TokenSource(context.Background())
	token, err := ts.Token()
	if err != nil {
		return nil, fmt.Errorf("failed to generate OAuth2 token for Gmail: %w", err)
	}

	// 3. Construct RFC 822 message
	rawMessage := fmt.Sprintf("To: %s\r\nSubject: %s\r\nContent-Type: text/html; charset=UTF-8\r\n\r\n%s", to, subject, body)
	encodedMessage := base64.URLEncoding.EncodeToString([]byte(rawMessage))

	payload := map[string]string{
		"raw": encodedMessage,
	}
	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal gmail payload: %w", err)
	}

	url := "https://gmail.googleapis.com/gmail/v1/users/me/messages/send"
	if impersonateUser != "" {
		url = fmt.Sprintf("https://gmail.googleapis.com/gmail/v1/users/%s/messages/send", impersonateUser)
	}

	req, err := http.NewRequest("POST", url, bytes.NewBuffer(payloadBytes))
	if err != nil {
		return nil, fmt.Errorf("failed to create http request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+token.AccessToken)
	req.Header.Set("Content-Type", "application/json")

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("http request failed: %w", err)
	}
	defer resp.Body.Close()

	respBytes, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("Gmail API send error (status %d): %s", resp.StatusCode, string(respBytes))
	}

	var apiResult map[string]interface{}
	_ = json.Unmarshal(respBytes, &apiResult)
	return apiResult, nil
}

func (e *GmailRESTExecutor) Validate(node *Node) error {
	to, _ := node.Params["to"].(string)
	if strings.TrimSpace(to) == "" {
		return fmt.Errorf("recipient email 'to' is required")
	}
	return nil
}

func (e *GmailRESTExecutor) GetDefinition() NodeDefinition {
	return NodeDefinition{
		Type:        TypeGmailREST,
		Name:        "Gmail REST API",
		Description: "Gửi Email HTML bảo mật thông qua dịch vụ Google Gmail REST API sử dụng Service Account",
		Icon:        "Mail",
		Category:    "COMMUNICATION",
		Params: []ParamDefinition{
			{
				Name:        "credential_id",
				Label:       "Select Encrypted Credential",
				Type:        "credential",
				Required:    false,
				Description: "Chọn tệp khóa Service Account JSON đã mã hóa từ Vault",
			},
			{
				Name:        "service_account_json",
				Label:       "Service Account JSON Key",
				Type:        "textarea",
				Required:    false,
				Description: "Dán nội dung khóa Service Account JSON trực tiếp (nếu không dùng Vault)",
			},
			{
				Name:        "impersonate_user",
				Label:       "Impersonate User Email (G-Suite)",
				Type:        "text",
				Required:    false,
				Description: "Địa chỉ email người gửi cần giả lập (bắt buộc khi dùng G-Suite Service Account)",
			},
			{
				Name:        "to",
				Label:       "Recipient Email (To)",
				Type:        "text",
				Required:    true,
				Description: "Địa chỉ nhận email (ví dụ: recipient@example.com)",
			},
			{
				Name:        "subject",
				Label:       "Email Subject",
				Type:        "text",
				Default:     "Notification from Goflow",
				Required:    true,
				Description: "Tiêu đề của Email",
			},
			{
				Name:        "body",
				Label:       "HTML Body",
				Type:        "textarea",
				Default:     "<h1>Hello!</h1><p>This is a custom notification email sent by Goflow automation flow.</p>",
				Required:    true,
				Description: "Nội dung văn bản định dạng HTML của email",
			},
		},
	}
}
