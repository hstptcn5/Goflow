package nodes

import (
	"strconv"
	"time"
)

type DelaySleepExecutor struct{}

func NewDelaySleepExecutor() *DelaySleepExecutor {
	return &DelaySleepExecutor{}
}

func (e *DelaySleepExecutor) Execute(ctx *ExecutionContext, node *Node) (interface{}, error) {
	durationStr, _ := node.Params["seconds"].(string)
	seconds, _ := strconv.Atoi(durationStr)
	if seconds <= 0 {
		seconds = 1
	}

	time.Sleep(time.Duration(seconds) * time.Second)

	return map[string]interface{}{
		"delayed_seconds": seconds,
		"resumed_at":      time.Now().Format(time.RFC3339),
	}, nil
}

func (e *DelaySleepExecutor) Validate(node *Node) error {
	return nil
}

func (e *DelaySleepExecutor) GetDefinition() NodeDefinition {
	return NodeDefinition{
		Type:        TypeDelaySleep,
		Name:        "Delay / Sleep",
		Description: "Pauses workflow execution for the configured duration",
		Icon:        "Hourglass",
		Category:    "LOGIC",
		Retryable:   true,
		Params: []ParamDefinition{
			{
				Name:        "seconds",
				Label:       "Delay Duration (Seconds)",
				Type:        "text",
				Default:     "3",
				Required:    true,
				Description: "Number of seconds to pause, for example 3",
			},
		},
	}
}
