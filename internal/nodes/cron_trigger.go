package nodes

import (
	"fmt"
	"strings"
	"time"

	"github.com/robfig/cron/v3"
)

type CronTriggerExecutor struct{}

func NewCronTriggerExecutor() *CronTriggerExecutor { return &CronTriggerExecutor{} }

func (e *CronTriggerExecutor) Execute(ctx *ExecutionContext, node *Node) (interface{}, error) {
	if err := e.Validate(node); err != nil {
		return nil, err
	}
	expression, _ := node.Params["cron_expression"].(string)
	return map[string]interface{}{
		"triggered_at": time.Now().UTC().Format(time.RFC3339),
		"schedule":     strings.TrimSpace(expression),
	}, nil
}

func (e *CronTriggerExecutor) Validate(node *Node) error {
	expression, _ := node.Params["cron_expression"].(string)
	expression = strings.TrimSpace(expression)
	if expression == "" {
		return fmt.Errorf("cron_expression is required")
	}
	if len(expression) > 256 {
		return fmt.Errorf("cron_expression exceeds 256 character limit")
	}
	if _, err := cron.ParseStandard(expression); err != nil {
		return fmt.Errorf("invalid cron_expression: %w", err)
	}
	return nil
}

func (e *CronTriggerExecutor) GetDefinition() NodeDefinition {
	return NodeDefinition{
		Type: TypeCronTrigger, Name: "Cron Schedule", Description: "Runs the workflow automatically on a standard five-field cron schedule", Icon: "Clock", Category: "TRIGGER", Retryable: true,
		Params: []ParamDefinition{{Name: "cron_expression", Label: "Cron Expression", Type: "text", Default: "*/5 * * * *", Required: true, Description: "Standard 5-field cron expression, for example */5 * * * * for every 5 minutes"}},
	}
}
