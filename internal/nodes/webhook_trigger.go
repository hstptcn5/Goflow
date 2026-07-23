package nodes

type WebhookTriggerExecutor struct{}

func NewWebhookTriggerExecutor() *WebhookTriggerExecutor {
	return &WebhookTriggerExecutor{}
}

func (e *WebhookTriggerExecutor) Execute(ctx *ExecutionContext, node *Node) (interface{}, error) {
	// Webhook Trigger nhận payload được truyền từ trigger handler vào ExecutionContext với key `$trigger`
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
		Description: "Khởi tạo workflow bằng một HTTP Webhook Request",
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
				Description: "Tùy chọn đường dẫn phụ cho Webhook Endpoint",
			},
		},
	}
}
