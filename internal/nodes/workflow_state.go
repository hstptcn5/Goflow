package nodes

import (
	"encoding/json"
	"fmt"
	"strings"
)

type WorkflowStateExecutor struct{}

func NewWorkflowStateExecutor() *WorkflowStateExecutor { return &WorkflowStateExecutor{} }

func normalizeStateOperation(raw interface{}) (string, error) {
	op := strings.ToUpper(strings.TrimSpace(conditionValueString(raw)))
	if op == "" {
		op = "GET"
	}
	switch op {
	case "GET", "SET", "DELETE", "INCREMENT":
		return op, nil
	default:
		return "", fmt.Errorf("state operation must be GET, SET, DELETE, or INCREMENT")
	}
}

func normalizeStateScope(raw interface{}) (string, error) {
	scope := strings.ToLower(strings.TrimSpace(conditionValueString(raw)))
	if scope == "" {
		scope = "workflow"
	}
	if scope != "workflow" && scope != "global" {
		return "", fmt.Errorf("state scope must be Workflow or Global")
	}
	return scope, nil
}

func stateNodeValue(raw interface{}) interface{} {
	if text, ok := raw.(string); ok {
		text = strings.TrimSpace(text)
		if text == "" {
			return ""
		}
		var value interface{}
		if json.Unmarshal([]byte(text), &value) == nil {
			return value
		}
	}
	return raw
}

func (e *WorkflowStateExecutor) Execute(ctx *ExecutionContext, node *Node) (interface{}, error) {
	op, err := normalizeStateOperation(node.Params["operation"])
	if err != nil {
		return nil, err
	}
	scope, err := normalizeStateScope(node.Params["scope"])
	if err != nil {
		return nil, err
	}
	key := strings.TrimSpace(conditionValueString(node.Params["key"]))
	if key == "" {
		return nil, fmt.Errorf("state key is required")
	}

	switch op {
	case "GET":
		value, found, err := workflowStateGet(ctx, scope, key)
		if err != nil {
			return nil, err
		}
		return map[string]interface{}{"found": found, "value": value, "key": key, "scope": scope}, nil
	case "SET":
		value := stateNodeValue(node.Params["value"])
		if err := workflowStateSet(ctx, scope, key, value); err != nil {
			return nil, err
		}
		return map[string]interface{}{"stored": true, "value": value, "key": key, "scope": scope}, nil
	case "DELETE":
		deleted, err := workflowStateDelete(ctx, scope, key)
		if err != nil {
			return nil, err
		}
		return map[string]interface{}{"deleted": deleted, "key": key, "scope": scope}, nil
	case "INCREMENT":
		delta, ok := conditionNumber(node.Params["delta"])
		if !ok {
			return nil, fmt.Errorf("state increment delta must be numeric")
		}
		value, err := workflowStateIncrement(ctx, scope, key, delta)
		if err != nil {
			return nil, err
		}
		return map[string]interface{}{"value": value, "delta": delta, "key": key, "scope": scope}, nil
	}
	return nil, fmt.Errorf("unsupported state operation")
}

func (e *WorkflowStateExecutor) Validate(node *Node) error {
	if _, err := normalizeStateOperation(node.Params["operation"]); err != nil {
		return err
	}
	if _, err := normalizeStateScope(node.Params["scope"]); err != nil {
		return err
	}
	if key, ok := node.Params["key"].(string); ok && !containsTemplateExpression(key) && strings.TrimSpace(key) == "" {
		return fmt.Errorf("state key is required")
	}
	return nil
}

func (e *WorkflowStateExecutor) GetDefinition() NodeDefinition {
	return NodeDefinition{
		Type: TypeWorkflowState, Name: "Workflow State", Description: "Reads and mutates small persistent SQLite-backed workflow or global state", Icon: "Database", Category: "LOGIC & UTILITY", Retryable: false,
		Params: []ParamDefinition{
			{Name: "operation", Label: "Operation", Type: "select", Default: "GET", Options: []string{"GET", "SET", "DELETE", "INCREMENT"}, Required: true},
			{Name: "scope", Label: "Scope", Type: "select", Default: "Workflow", Options: []string{"Workflow", "Global"}, Required: true},
			{Name: "key", Label: "Key", Type: "text", Default: "", Required: true, Description: "State key; supports expressions"},
			{Name: "value", Label: "Value", Type: "json", Default: "null", Required: false, Description: "Value for SET; complete expressions preserve their type"},
			{Name: "delta", Label: "Increment By", Type: "number", Default: 1, Required: false, Description: "Numeric delta for INCREMENT"},
		},
	}
}
