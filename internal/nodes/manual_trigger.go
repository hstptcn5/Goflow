package nodes

import "time"

type ManualTriggerExecutor struct{}

func NewManualTriggerExecutor() *ManualTriggerExecutor { return &ManualTriggerExecutor{} }

func (e *ManualTriggerExecutor) Execute(ctx *ExecutionContext, node *Node) (interface{}, error) {
	if ctx != nil {
		if payload, ok := ctx.GetOutput("$trigger"); ok && payload != nil {
			return payload, nil
		}
	}
	return map[string]interface{}{
		"triggered_at": time.Now().UTC().Format(time.RFC3339),
		"source":       "manual",
	}, nil
}

func (e *ManualTriggerExecutor) Validate(node *Node) error { return nil }

func (e *ManualTriggerExecutor) GetDefinition() NodeDefinition {
	return NodeDefinition{
		Type:        TypeManualTrigger,
		Name:        "Manual Trigger",
		Description: "Starts a workflow when the operator runs it manually",
		Icon:        "Play",
		Category:    "TRIGGER",
		Retryable:   true,
		Params:      []ParamDefinition{},
	}
}
