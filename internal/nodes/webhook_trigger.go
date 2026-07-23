package nodes

type WebhookTriggerExecutor struct{}

func NewWebhookTriggerExecutor() *WebhookTriggerExecutor {
	return &WebhookTriggerExecutor{}
}

func (e *WebhookTriggerExecutor) Execute(ctx *ExecutionContext, node *Node) (interface{}, error) {
	if triggerData, ok := ctx.GetOutput("$trigger"); ok {
		return triggerData, nil
	}
	return map[string]interface{}{
		"message": "Webhook triggered without body",
	}, nil
}

func (e *WebhookTriggerExecutor) Validate(node *Node) error {
	return nil
}

func (e *WebhookTriggerExecutor) GetDefinition() NodeDefinition {
	return NodeDefinition{
		Type:        TypeWebhookTrigger,
		Name:        "Webhook Trigger",
		Description: "Start a workflow from an HTTP webhook request.",
		Icon:        "Webhook",
		Category:    "TRIGGER",
		Retryable:   true,
		Params: []ParamDefinition{
			{
				Name:        "path",
				Label:       "Webhook Subpath",
				Type:        "text",
				Default:     "/trigger",
				Required:    false,
				Description: "Optional subpath for documenting the webhook endpoint.",
			},
			{
				Name:        "secret",
				Label:       "Webhook Secret",
				Type:        "password",
				Required:    false,
				Description: "Optional shared secret required in the X-Goflow-Webhook-Secret header.",
			},
		},
	}
}
