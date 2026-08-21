package nodes

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"mime/multipart"
	"net"
	"net/mail"
	"net/smtp"
	"net/textproto"
	"strconv"
	"strings"
	"time"

	"goflow/internal/fileref"
)

const (
	maxSMTPBodyBytes       = 1 << 20
	maxSMTPMessageBytes    = 25 << 20
	maxSMTPAttachments     = 10
	maxSMTPAttachmentBytes = 20 << 20
)

type EmailSMTPExecutor struct{ store *fileref.Store }

func NewEmailSMTPExecutor() *EmailSMTPExecutor {
	return &EmailSMTPExecutor{store: fileref.DefaultStore()}
}
func NewEmailSMTPExecutorWithStore(store *fileref.Store) *EmailSMTPExecutor {
	if store == nil {
		store = fileref.DefaultStore()
	}
	return &EmailSMTPExecutor{store: store}
}

func (e *EmailSMTPExecutor) Execute(ctx *ExecutionContext, node *Node) (interface{}, error) {
	params, err := parseSMTPParams(node)
	if err != nil {
		return nil, err
	}
	password, err := resolveNodeCredential(ctx, node, "password", "SMTP password")
	if err != nil {
		return nil, err
	}
	attachments, err := parseSMTPAttachmentRefs(node.Params["attachments"])
	if err != nil {
		return nil, err
	}
	msg, err := buildSMTPMessage(params, attachments, e.store)
	if err != nil {
		return nil, err
	}
	if len(msg) > maxSMTPMessageBytes {
		return nil, fmt.Errorf("SMTP message exceeds %d byte limit", maxSMTPMessageBytes)
	}
	if err := sendSMTPWithContext(ctx.Context, params.Host, params.Port, params.Username, password, params.Recipients, msg); err != nil {
		return nil, fmt.Errorf("failed to send SMTP email: %w", err)
	}
	return map[string]interface{}{"status": "sent", "recipients": params.Recipients, "subject": params.Subject, "attachments": len(attachments)}, nil
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
	out.Host = strings.TrimSpace(conditionValueString(node.Params["host"]))
	portStr := strings.TrimSpace(conditionValueString(node.Params["port"]))
	if portStr == "" {
		portStr = "587"
	}
	port, err := strconv.Atoi(portStr)
	if err != nil || port < 1 || port > 65535 {
		return out, fmt.Errorf("SMTP port must be an integer between 1 and 65535")
	}
	out.Port = port
	out.Username = strings.TrimSpace(conditionValueString(node.Params["username"]))
	to := conditionValueString(node.Params["to"])
	out.Subject = conditionValueString(node.Params["subject"])
	out.Body = conditionValueString(node.Params["body"])
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
	if len(out.Body) > maxSMTPBodyBytes {
		return out, fmt.Errorf("SMTP body exceeds %d byte limit", maxSMTPBodyBytes)
	}
	credentialID := strings.TrimSpace(conditionValueString(node.Params["credential_id"]))
	password := strings.TrimSpace(conditionValueString(node.Params["password"]))
	if credentialID == "" && password == "" {
		return out, fmt.Errorf("SMTP password or encrypted credential is required")
	}
	return out, nil
}

func parseSMTPAttachmentRefs(raw interface{}) ([]fileref.Ref, error) {
	if raw == nil || strings.TrimSpace(conditionValueString(raw)) == "" {
		return nil, nil
	}
	var values []interface{}
	if text, ok := raw.(string); ok {
		var decoded interface{}
		if err := json.Unmarshal([]byte(text), &decoded); err != nil {
			return nil, fmt.Errorf("SMTP attachments must be a FileRef or JSON array of FileRefs")
		}
		if array, ok := decoded.([]interface{}); ok {
			values = array
		} else {
			values = []interface{}{decoded}
		}
	} else if array, ok := raw.([]interface{}); ok {
		values = array
	} else {
		values = []interface{}{raw}
	}
	if len(values) > maxSMTPAttachments {
		return nil, fmt.Errorf("SMTP supports at most %d attachments", maxSMTPAttachments)
	}
	refs := make([]fileref.Ref, 0, len(values))
	for _, value := range values {
		ref, err := fileref.Parse(value)
		if err != nil {
			return nil, err
		}
		if ref.Size > maxSMTPAttachmentBytes {
			return nil, fmt.Errorf("SMTP attachment %q exceeds %d byte limit", ref.Name, maxSMTPAttachmentBytes)
		}
		refs = append(refs, ref)
	}
	return refs, nil
}

func buildSMTPMessage(params smtpParams, attachments []fileref.Ref, store *fileref.Store) ([]byte, error) {
	var buffer bytes.Buffer
	fmt.Fprintf(&buffer, "From: %s\r\nTo: %s\r\nSubject: %s\r\nMIME-Version: 1.0\r\n", params.Username, strings.Join(params.Recipients, ", "), params.Subject)
	if len(attachments) == 0 {
		fmt.Fprintf(&buffer, "Content-Type: text/html; charset=UTF-8\r\n\r\n%s", params.Body)
		return buffer.Bytes(), nil
	}
	writer := multipart.NewWriter(&buffer)
	fmt.Fprintf(&buffer, "Content-Type: multipart/mixed; boundary=%q\r\n\r\n", writer.Boundary())
	bodyHeader := make(textproto.MIMEHeader)
	bodyHeader.Set("Content-Type", "text/html; charset=UTF-8")
	bodyPart, err := writer.CreatePart(bodyHeader)
	if err != nil {
		return nil, err
	}
	if _, err := bodyPart.Write([]byte(params.Body)); err != nil {
		return nil, err
	}
	for _, ref := range attachments {
		data, err := store.ReadAll(ref)
		if err != nil {
			return nil, err
		}
		if len(data) > maxSMTPAttachmentBytes {
			return nil, fmt.Errorf("SMTP attachment %q exceeds %d byte limit", ref.Name, maxSMTPAttachmentBytes)
		}
		mimeType := strings.TrimSpace(ref.MIME)
		if mimeType == "" {
			mimeType = "application/octet-stream"
		}
		header := make(textproto.MIMEHeader)
		header.Set("Content-Type", fmt.Sprintf("%s; name=%q", mimeType, ref.Name))
		header.Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", ref.Name))
		header.Set("Content-Transfer-Encoding", "base64")
		part, err := writer.CreatePart(header)
		if err != nil {
			return nil, err
		}
		encoded := base64.StdEncoding.EncodeToString(data)
		for len(encoded) > 76 {
			if _, err := fmt.Fprintf(part, "%s\r\n", encoded[:76]); err != nil {
				return nil, err
			}
			encoded = encoded[76:]
		}
		if _, err := fmt.Fprintf(part, "%s\r\n", encoded); err != nil {
			return nil, err
		}
	}
	if err := writer.Close(); err != nil {
		return nil, err
	}
	return buffer.Bytes(), nil
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
	if _, err := parseSMTPParams(node); err != nil {
		return err
	}
	_, err := parseSMTPAttachmentRefs(node.Params["attachments"])
	return err
}

func (e *EmailSMTPExecutor) GetDefinition() NodeDefinition {
	return NodeDefinition{
		Type: TypeEmailSMTP, Name: "SMTP Email", Description: "Sends HTML email with optional managed FileRef attachments through SMTP", Icon: "Mail", Category: "ACTION",
		Params: []ParamDefinition{
			{Name: "host", Label: "SMTP Host", Type: "text", Default: "smtp.gmail.com", Required: true, Description: "SMTP server address"},
			{Name: "port", Label: "SMTP Port", Type: "text", Default: "587", Required: true, Description: "Usually 587 for STARTTLS or 465 for implicit TLS"},
			{Name: "username", Label: "SMTP Username / Sender", Type: "text", Default: "", Required: true, Description: "SMTP username and sender email"},
			{Name: "password", Label: "SMTP Password (legacy)", Type: "password", Default: "", Required: false, Description: "Legacy direct app password. Prefer an encrypted credential."},
			{Name: "credential_id", Label: "Credential Secret", Type: "credential", Default: "", Required: false, Description: "Encrypted SMTP password saved in Credentials"},
			{Name: "to", Label: "Recipients (To)", Type: "text", Default: "", Required: true, Description: "Comma-separated recipient email addresses"},
			{Name: "subject", Label: "Email Subject", Type: "text", Default: "Goflow Notification", Required: true, Description: "Email subject"},
			{Name: "body", Label: "Email Body (HTML)", Type: "textarea", Default: "<h3>Goflow Notification</h3><p>Your workflow completed successfully!</p>", Required: true, Description: "Email body in HTML"},
			{Name: "attachments", Label: "Attachments", Type: "json", Default: "", Required: false, Description: "One FileRef or JSON array of managed FileRefs"},
		},
	}
}
