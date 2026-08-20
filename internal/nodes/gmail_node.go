package nodes

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"mime"
	"net/http"
	"net/mail"
	"net/url"
	"strings"
	"time"
)

const maxGmailMessageBytes = 5 << 20

type GmailRESTExecutor struct{}

func NewGmailRESTExecutor() *GmailRESTExecutor { return &GmailRESTExecutor{} }

func validateGmailNode(node *Node) error {
	to, _ := node.Params["to"].(string)
	to = strings.TrimSpace(to)
	if to == "" {
		return fmt.Errorf("recipient email 'to' is required")
	}
	if strings.ContainsAny(to, "\r\n") {
		return fmt.Errorf("recipient email must not contain line breaks")
	}
	if _, err := mail.ParseAddress(to); err != nil {
		return fmt.Errorf("recipient email 'to' is invalid")
	}
	subject, _ := node.Params["subject"].(string)
	if strings.ContainsAny(subject, "\r\n") || len([]rune(subject)) > 998 {
		return fmt.Errorf("Gmail subject is invalid")
	}
	body, _ := node.Params["body"].(string)
	if len(body) > maxGmailMessageBytes {
		return fmt.Errorf("Gmail body exceeds %d byte limit", maxGmailMessageBytes)
	}
	impersonate, _ := node.Params["impersonate_user"].(string)
	impersonate = strings.TrimSpace(impersonate)
	if impersonate != "" {
		if strings.ContainsAny(impersonate, "\r\n") {
			return fmt.Errorf("impersonate_user must not contain line breaks")
		}
		if parsed, err := mail.ParseAddress(impersonate); err != nil || parsed.Address != impersonate {
			return fmt.Errorf("impersonate_user must be a valid email address")
		}
	}
	credentialID, _ := node.Params["credential_id"].(string)
	directSA, _ := node.Params["service_account_json"].(string)
	if strings.TrimSpace(credentialID) == "" && strings.TrimSpace(directSA) == "" {
		return fmt.Errorf("Gmail requires an encrypted credential or service_account_json")
	}
	return nil
}

func (e *GmailRESTExecutor) Execute(ctx *ExecutionContext, node *Node) (interface{}, error) {
	if err := validateGmailNode(node); err != nil {
		return nil, err
	}
	to, _ := node.Params["to"].(string)
	subject, _ := node.Params["subject"].(string)
	body, _ := node.Params["body"].(string)
	impersonateUser, _ := node.Params["impersonate_user"].(string)
	impersonateUser = strings.TrimSpace(impersonateUser)
	material, err := resolveGoogleAuth(ctx, node)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(material.ServiceAccountJSON) != "" && impersonateUser == "" {
		return nil, fmt.Errorf("Gmail service account credentials require impersonate_user with Google Workspace domain-wide delegation")
	}
	accessToken, err := googleAccessTokenForSubject(ctx.Context, material, impersonateUser, "https://www.googleapis.com/auth/gmail.send")
	if err != nil {
		return nil, err
	}

	parsedTo, _ := mail.ParseAddress(strings.TrimSpace(to))
	encodedSubject := mime.QEncoding.Encode("UTF-8", subject)
	rawMessage := fmt.Sprintf("To: %s\r\nSubject: %s\r\nMIME-Version: 1.0\r\nContent-Type: text/html; charset=UTF-8\r\n\r\n%s", parsedTo.String(), encodedSubject, body)
	if len(rawMessage) > maxGmailMessageBytes {
		return nil, fmt.Errorf("Gmail encoded message exceeds %d byte limit", maxGmailMessageBytes)
	}
	payloadBytes, err := json.Marshal(map[string]string{"raw": base64.RawURLEncoding.EncodeToString([]byte(rawMessage))})
	if err != nil {
		return nil, fmt.Errorf("failed to marshal Gmail payload: %w", err)
	}
	userPath := "me"
	if impersonateUser != "" {
		userPath = url.PathEscape(impersonateUser)
	}
	endpoint := "https://gmail.googleapis.com/gmail/v1/users/" + userPath + "/messages/send"
	req, err := http.NewRequestWithContext(ctx.Context, http.MethodPost, endpoint, bytes.NewReader(payloadBytes))
	if err != nil {
		return nil, fmt.Errorf("failed to create Gmail request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Content-Type", "application/json")
	client := &http.Client{Timeout: 20 * time.Second}
	return doGoogleJSONRequest(client, req, http.StatusOK)
}

func (e *GmailRESTExecutor) Validate(node *Node) error { return validateGmailNode(node) }

func (e *GmailRESTExecutor) GetDefinition() NodeDefinition {
	return NodeDefinition{
		Type: TypeGmailREST, Name: "Gmail REST API", Description: "Sends bounded HTML email through the Gmail REST API", Icon: "Mail", Category: "COMMUNICATION",
		Params: []ParamDefinition{
			{Name: "credential_id", Label: "Select Encrypted Credential", Type: "credential", Required: false, Description: "Encrypted Google OAuth2 access token or service account JSON"},
			{Name: "service_account_json", Label: "Service Account JSON Key (legacy)", Type: "textarea", Required: false, Description: "Legacy inline service account JSON. Prefer an encrypted credential."},
			{Name: "impersonate_user", Label: "Impersonate User Email (Google Workspace)", Type: "text", Required: false, Description: "Required for service accounts using domain-wide delegation. OAuth2 user credentials can leave this empty."},
			{Name: "to", Label: "Recipient Email (To)", Type: "text", Required: true, Description: "Single recipient email address"},
			{Name: "subject", Label: "Email Subject", Type: "text", Default: "Notification from Goflow", Required: true, Description: "Email subject"},
			{Name: "body", Label: "HTML Body", Type: "textarea", Default: "<h1>Hello!</h1><p>This is a custom notification email sent by Goflow.</p>", Required: true, Description: "HTML email body, maximum 5 MiB"},
		},
	}
}
