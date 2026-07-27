package nodes

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"
)

type DelaySleepExecutor struct{}

const maxDelaySeconds = 24 * 60 * 60

func NewDelaySleepExecutor() *DelaySleepExecutor {
	return &DelaySleepExecutor{}
}

func (e *DelaySleepExecutor) Execute(ctx *ExecutionContext, node *Node) (interface{}, error) {
	seconds, err := parseDelaySeconds(node.Params["seconds"])
	if err != nil {
		return nil, err
	}

	select {
	case <-time.After(time.Duration(seconds) * time.Second):
	case <-ctx.Context.Done():
		return nil, context.Cause(ctx.Context)
	}

	return map[string]interface{}{
		"delayed_seconds": seconds,
		"resumed_at":      time.Now().Format(time.RFC3339),
	}, nil
}

func (e *DelaySleepExecutor) Validate(node *Node) error {
	if raw, ok := node.Params["seconds"].(string); ok && strings.Contains(raw, "{{") {
		return nil
	}
	_, err := parseDelaySeconds(node.Params["seconds"])
	return err
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
				Type:        "integer",
				Default:     "3",
				Required:    true,
				Description: "Number of seconds to pause, for example 3",
			},
		},
	}
}

func parseDelaySeconds(value interface{}) (int, error) {
	var seconds int64
	switch v := value.(type) {
	case nil:
		return 0, fmt.Errorf("delay seconds is required")
	case string:
		parsed, err := strconv.ParseInt(v, 10, 64)
		if err != nil {
			return 0, fmt.Errorf("delay seconds must be an integer")
		}
		seconds = parsed
	case int:
		seconds = int64(v)
	case int32:
		seconds = int64(v)
	case int64:
		seconds = v
	case float32:
		parsed := float64(v)
		if math.IsNaN(parsed) || math.IsInf(parsed, 0) || math.Trunc(parsed) != parsed {
			return 0, fmt.Errorf("delay seconds must be a finite integer")
		}
		seconds = int64(parsed)
	case float64:
		if math.IsNaN(v) || math.IsInf(v, 0) || math.Trunc(v) != v {
			return 0, fmt.Errorf("delay seconds must be a finite integer")
		}
		seconds = int64(v)
	case json.Number:
		parsed, err := strconv.ParseInt(v.String(), 10, 64)
		if err != nil {
			return 0, fmt.Errorf("delay seconds must be an integer")
		}
		seconds = parsed
	default:
		return 0, fmt.Errorf("delay seconds must be an integer")
	}
	if seconds <= 0 {
		return 0, fmt.Errorf("delay seconds must be greater than zero")
	}
	if seconds > maxDelaySeconds {
		return 0, fmt.Errorf("delay seconds must be less than or equal to %d", maxDelaySeconds)
	}
	return int(seconds), nil
}
