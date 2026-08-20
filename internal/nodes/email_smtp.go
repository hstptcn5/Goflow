package nodes

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"net/mail"
	"net/smtp"
	"strconv"
	"strings"
	"time"
)

const maxSMTPMessageBytes = 1 << 20

type EmailSMTPExecutor struct{}

func NewEmailSMTPExecutor() *EmailSMTPExecutor { return &EmailSMTPExecutor{} }

func (e *EmailSMTPExecutor) Execute(ctx *ExecutionContext, node *Node) (interface{}, error) {
	params, err := parseSMTPParams(node)
	if err != nil {
		return nil, err
	}
	password, err := resolveNodeCredential(ctx, node, "password", "SMTP password")
	if err != nil {
		return nil, err
	}
	msg := []byte(fmt.Sprintf("From: %s\r\nTo: %s\r\nSubject: %s\r\nMIME-Version: 1.0\r\nContent-Type: text/html; charset=UTF-8\r\n\r\n%s", params.Username, strings.Join(params.Recipients, ", "), params.Subject, params.Body))
	if len(msg) > maxSMTPMessageBytes {
		return nil, fmt.Errorf("SMTP message exceeds %d byte limit", maxSMTPMessageBytes)
	}
	if err := sendSMTPWithContext(ctx.Context, params.Host, params.Port, params.Username, password, params.Recipients, msg); err != nil {
		return nil, fmt.Errorf("failed to send SMTP email: %w", err)
	}
	return map[string]interface{}{"status": "sent", "recipients": params.Recipients, "subject": params.Subject}, nil
}

type smtpParams struct {
	Host       string
	Port       int
	Username   string
	Recipients []string
	Subject    string
	Body       string
}

func parseSMTPParams(node *Node) (smtpParams, error) {
	var out smtpParams
	out.Host, _ = node.Params["host"].(string)
	out.Host = strings.TrimSpace(out.Host)
	portStr, _ := node.Params["port"].(string)
	portStr = strings.TrimSpace(portStr)
	if portStr == "" {
		portStr = "587"
	}
	port, err := strconv.Atoi(portStr)
	if err != nil || port < 1 || port > 65535 {
		return out, fmt.Errorf("SMTP port must be an integer between 1 and 65535")
	}
	out.Port = port
	out.Username, _ = node.Params["username"].(string)
	out.Username = strings.TrimSpace(out.Username)
	to, _ := node.Params["to"].(string)
	out.Subject, _ = node.Params["subject"].(string)
	out.Body, _ = node.Params["body"].(string)
	if out.Host == "" || out.Username == "" || strings.TrimSpace(to) == "" {
		return out, fmt.Errorf("SMTP host, username/sender, and destination 'to' address are required")
	}
	if strings.ContainsAny(out.Host, "\r\n") || strings.ContainsAny(out.Subject, "\r\n") || strings.ContainsAny(out.Username, "\r\n") || strings.ContainsAny(to, "\r\n") {
		return out, fmt.Errorf("SMTP header fields must not contain line breaks")
	}
	if _, err := mail.ParseAddress(out.Username); err != nil {
		return out, fmt.Errorf("SMTP username/sender must be a valid email address")
	}
	for _, raw := range strings.Split(to, ",") {
		raw = strings.TrimSpace(raw)
		if raw == "" {
			continue
		}
		parsed, err := mail.ParseAddress(raw)
		if err != nil {
			return out, fmt.Errorf("invalid SMTP recipient %q", raw)
		}
		out.Recipients = append(out.Recipients, parsed.Address)
	}
	if len(out.Recipients) == 0 || len(out.Recipients) > 50 {
		return out, fmt.Errorf("SMTP requires between 1 and 50 recipients")
	}
	if len([]rune(out.Subject)) > 998 {
		return out, fmt.Errorf("SMTP subject is too long")
	}
	if len(out.Body) > maxSMTPMessageBytes {
		return out, fmt.Errorf("SMTP body exceeds %d byte limit", maxSMTPMessageBytes)
	}
	credentialID, _ := node.Params["credential_id"].(string)
	password, _ := node.Params["password"].(string)
	if strings.TrimSpace(credentialID) == "" && strings.TrimSpace(password) == "" {
		return out, fmt.Errorf("SMTP password or encrypted credential is required")
	}
	return out, nil
}

func sendSMTPWithContext(ctx context.Context, host string, port int, username, password string, recipients []string, msg []byte) error {
	if ctx == nil {
		ctx = context.Background()
	}
	addr := net.JoinHostPort(host, strconv.Itoa(port))
	dialer := &net.Dialer{Timeout: 15 * time.Second}
	var conn net.Conn
	var err error
	tlsConfig := &tls.Config{ServerName: host, MinVersion: tls.VersionTLS12}
	if port == 465 {
		tlsDialer := &tls.Dialer{NetDialer: dialer, Config: tlsConfig}
		conn, err = tlsDialer.DialContext(ctx, "tcp", addr)
	} else {
		conn, err = dialer.DialContext(ctx, "tcp", addr)
	}
	if err != nil {
		return err
	}
	defer conn.Close()
	deadline := time.Now().Add(30 * time.Second)
	if ctxDeadline, ok := ctx.Deadline(); ok && ctxDeadline.Before(deadline) {
		deadline = ctxDeadline
	}
	_ = conn.SetDeadline(deadline)
	client, err := smtp.NewClient(conn, host)
	if err != nil {
		return err
	}
	defer client.Close()
	if port != 465 {
		if ok, _ := client.Extension("STARTTLS"); ok {
			if err := client.StartTLS(tlsConfig); err != nil {
				return err
			}
		}
	}
	if username != "" {
		if err := client.Auth(smtp.PlainAuth("", username, password, host)); err != nil {
			return err
		}
	}
	if err := client.Mail(username); err != nil {
		return err
	}
	for _, recipient := range recipients {
		if err := client.Rcpt(recipient); err != nil {
			return err
		}
	}
	writer, err := client.Data()
	if err != nil {
		return err
	}
	if _, err := writer.Write(msg); err != nil {
		_ = writer.Close()
		return err
	}
	if err := writer.Close(); err != nil {
		return err
	}
	return client.Quit()
}

func (e *EmailSMTPExecutor) Validate(node *Node) error {
	_, err := parseSMTPParams(node)
	return err
}

func (e *EmailSMTPExecutor) GetDefinition() NodeDefinition {
	return NodeDefinition{
		Type: TypeEmailSMTP, Name: "SMTP Email", Description: "Sends email through SMTP providers such as Gmail or a custom SMTP server", Icon: "Mail", Category: "ACTION",
		Params: []ParamDefinition{
			{Name: "host", Label: "SMTP Host", Type: "text", Default: "smtp.gmail.com", Required: true, Description: "SMTP server address"},
			{Name: "port", Label: "SMTP Port", Type: "text", Default: "587", Required: true, Description: "Usually 587 for STARTTLS or 465 for implicit TLS"},
			{Name: "username", Label: "SMTP Username / Sender", Type: "text", Default: "", Required: true, Description: "SMTP username and sender email"},
			{Name: "password", Label: "SMTP Password (legacy)", Type: "password", Default: "", Required: false, Description: "Legacy direct app password. Prefer an encrypted credential."},
			{Name: "credential_id", Label: "Credential Secret", Type: "credential", Default: "", Required: false, Description: "Encrypted SMTP password saved in Credentials"},
			{Name: "to", Label: "Recipients (To)", Type: "text", Default: "", Required: true, Description: "Comma-separated recipient email addresses"},
			{Name: "subject", Label: "Email Subject", Type: "text", Default: "Goflow Notification", Required: true, Description: "Email subject"},
			{Name: "body", Label: "Email Body (HTML)", Type: "textarea", Default: "<h3>Goflow Notification</h3><p>Your workflow completed successfully!</p>", Required: true, Description: "Email body in HTML"},
		},
	}
}
