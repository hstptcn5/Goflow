package nodes

import (
	"fmt"
	"strings"
)

type WebhookTriggerExecutor struct{}

func NewWebhookTriggerExecutor() *WebhookTriggerExecutor { return &WebhookTriggerExecutor{} }

func (e *WebhookTriggerExecutor) Execute(ctx *ExecutionContext, node *Node) (interface{}, error) {
	if triggerData, ok := ctx.GetOutput("$trigger"); ok {
		return triggerData, nil
	}
	return map[string]interface{}{"message": "Webhook triggered without body"}, nil
}

func (e *WebhookTriggerExecutor) Validate(node *Node) error {
	path, _ := node.Params["path"].(string)
	path = strings.TrimSpace(path)
	if path != "" {
		if len(path) > 512 || strings.ContainsAny(path, "\r\n?\x00") || !strings.HasPrefix(path, "/") || strings.Contains(path, "..") {
			return fmt.Errorf("webhook path must start with / and must not contain query strings, line breaks, or .. segments")
		}
	}
	secret, _ := node.Params["secret"].(string)
	if len(secret) > 4096 {
		return fmt.Errorf("webhook secret exceeds 4096 byte limit")
	}
	return nil
}

func (e *WebhookTriggerExecutor) GetDefinition() NodeDefinition {
	return NodeDefinition{
		Type: TypeWebhookTrigger, Name: "Webhook Trigger", Description: "Start a workflow from an HTTP webhook request.", Icon: "Webhook", Category: "TRIGGER", Retryable: true,
		Params: []ParamDefinition{
			{Name: "path", Label: "Webhook Subpath", Type: "text", Default: "/trigger", Required: false, Description: "Optional subpath beginning with /. Query strings and traversal segments are rejected."},
			{Name: "secret", Label: "Webhook Secret", Type: "password", Required: false, Description: "Optional shared secret required in the X-Goflow-Webhook-Secret header."},
		},
	}
}
