package nodes

import "time"

type CronTriggerExecutor struct{}

func NewCronTriggerExecutor() *CronTriggerExecutor {
	return &CronTriggerExecutor{}
}

func (e *CronTriggerExecutor) Execute(ctx *ExecutionContext, node *Node) (interface{}, error) {
	return map[string]interface{}{
		"triggered_at": time.Now().Format(time.RFC3339),
		"schedule":     node.Params["cron_expression"],
	}, nil
}

func (e *CronTriggerExecutor) Validate(node *Node) error {
	return nil
}

func (e *CronTriggerExecutor) GetDefinition() NodeDefinition {
	return NodeDefinition{
		Type:        TypeCronTrigger,
		Name:        "Cron Schedule",
		Description: "Runs the workflow automatically on a cron schedule",
		Icon:        "Clock",
		Category:    "TRIGGER",
		Retryable:   true,
		Params: []ParamDefinition{
			{
				Name:        "cron_expression",
				Label:       "Cron Expression",
				Type:        "text",
				Default:     "*/5 * * * *",
				Required:    true,
				Description: "Cron expression, for example */5 * * * * for every 5 minutes",
			},
		},
	}
}
