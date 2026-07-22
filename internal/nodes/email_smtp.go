package nodes

import (
	"fmt"
	"net/smtp"
	"strconv"
	"strings"
)

type EmailSMTPExecutor struct{}

func NewEmailSMTPExecutor() *EmailSMTPExecutor {
	return &EmailSMTPExecutor{}
}

func (e *EmailSMTPExecutor) Execute(ctx *ExecutionContext, node *Node) (interface{}, error) {
	host, _ := node.Params["host"].(string)
	portStr, _ := node.Params["port"].(string)
	user, _ := node.Params["username"].(string)
	pass, _ := node.Params["password"].(string)
	to, _ := node.Params["to"].(string)
	subject, _ := node.Params["subject"].(string)
	body, _ := node.Params["body"].(string)

	credID, _ := node.Params["credential_id"].(string)
	if credID != "" {
		if secret, ok := ctx.Credentials[credID]; ok {
			pass = secret
		}
	}

	if host == "" || to == "" {
		return nil, fmt.Errorf("SMTP host and destination 'to' address are required")
	}

	if portStr == "" {
		portStr = "587"
	}
	port, _ := strconv.Atoi(portStr)

	addr := fmt.Sprintf("%s:%d", host, port)
	auth := smtp.PlainAuth("", user, pass, host)

	// Format Email Message Header
	msg := []byte(fmt.Sprintf("From: %s\r\nTo: %s\r\nSubject: %s\r\nMIME-Version: 1.0\r\nContent-Type: text/html; charset=UTF-8\r\n\r\n%s", user, to, subject, body))

	toAddresses := strings.Split(to, ",")
	for i := range toAddresses {
		toAddresses[i] = strings.TrimSpace(toAddresses[i])
	}

	err := smtp.SendMail(addr, auth, user, toAddresses, msg)
	if err != nil {
		return nil, fmt.Errorf("failed to send SMTP email: %w", err)
	}

	return map[string]interface{}{
		"status":     "sent",
		"recipients": toAddresses,
		"subject":    subject,
	}, nil
}

func (e *EmailSMTPExecutor) Validate(node *Node) error {
	to, _ := node.Params["to"].(string)
	if strings.TrimSpace(to) == "" {
		return fmt.Errorf("Email SMTP Node requires 'to' email address")
	}
	return nil
}

func (e *EmailSMTPExecutor) GetDefinition() NodeDefinition {
	return NodeDefinition{
		Type:        TypeEmailSMTP,
		Name:        "SMTP Email",
		Description: "Tu dong gui Email thong qua SMTP (Gmail, Custom SMTP)",
		Icon:        "Mail",
		Category:    "ACTION",
		Params: []ParamDefinition{
			{
				Name:        "host",
				Label:       "SMTP Host",
				Type:        "text",
				Default:     "smtp.gmail.com",
				Required:    true,
				Description: "Dia chi server SMTP (smtp.gmail.com)",
			},
			{
				Name:        "port",
				Label:       "SMTP Port",
				Type:        "text",
				Default:     "587",
				Required:    true,
				Description: "Cong SMTP (587 cho TLS, 465 cho SSL)",
			},
			{
				Name:        "username",
				Label:       "SMTP Username / Sender",
				Type:        "text",
				Default:     "",
				Required:    true,
				Description: "Ten dang nhap / Email nguoi gui",
			},
			{
				Name:        "password",
				Label:       "SMTP Password",
				Type:        "text",
				Default:     "",
				Required:    false,
				Description: "Mat khau ung dung (App Password)",
			},
			{
				Name:        "credential_id",
				Label:       "Credential Secret",
				Type:        "credential",
				Default:     "",
				Required:    false,
				Description: "Mat khau da ma hoa trong Credentials",
			},
			{
				Name:        "to",
				Label:       "Recipients (To)",
				Type:        "text",
				Default:     "",
				Required:    true,
				Description: "Email nguoi nhan",
			},
			{
				Name:        "subject",
				Label:       "Email Subject",
				Type:        "text",
				Default:     "Goflow Notification",
				Required:    true,
				Description: "Tieu de thu",
			},
			{
				Name:        "body",
				Label:       "Email Body (HTML)",
				Type:        "textarea",
				Default:     "<h3>Goflow Notification</h3><p>Your workflow completed successfully!</p>",
				Required:    true,
				Description: "Noi dung email (HTML)",
			},
		},
	}
}
