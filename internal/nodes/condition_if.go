package nodes

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/google/uuid"
)

type ConditionIFExecutor struct{}

func NewConditionIFExecutor() *ConditionIFExecutor { return &ConditionIFExecutor{} }

func conditionValueString(raw interface{}) string {
	if raw == nil {
		return ""
	}
	if text, ok := raw.(string); ok {
		return text
	}
	switch raw.(type) {
	case map[string]interface{}, []interface{}:
		if encoded, err := json.Marshal(raw); err == nil {
			return string(encoded)
		}
	}
	return fmt.Sprint(raw)
}

func normalizeConditionOperator(raw interface{}) (string, error) {
	operator := strings.ToLower(strings.TrimSpace(conditionValueString(raw)))
	if operator == "" {
		operator = "equals"
	}
	switch operator {
	case "equals", "==":
		return "equals", nil
	case "not_equals", "!=":
		return "not_equals", nil
	case "contains":
		return "contains", nil
	case "is_not_empty":
		return "is_not_empty", nil
	default:
		return "", fmt.Errorf("unsupported IF operator %q", operator)
	}
}

func (e *ConditionIFExecutor) Execute(ctx *ExecutionContext, node *Node) (interface{}, error) {
	field := conditionValueString(node.Params["field"])
	value := conditionValueString(node.Params["value"])
	operator, err := normalizeConditionOperator(node.Params["operator"])
	if err != nil {
		return nil, err
	}

	var isTrue bool
	switch operator {
	case "equals":
		isTrue = field == value
	case "not_equals":
		isTrue = field != value
	case "contains":
		isTrue = strings.Contains(field, value)
	case "is_not_empty":
		isTrue = strings.TrimSpace(field) != ""
	}
	resultHandle := "false"
	if isTrue {
		resultHandle = "true"
	}
	return map[string]interface{}{
		"result": isTrue, "target_handle": resultHandle,
		"evaluated": fmt.Sprintf("%s %s %s", field, operator, value),
		"execution_tag": uuid.New().String(),
	}, nil
}

func (e *ConditionIFExecutor) Validate(node *Node) error {
	_, err := normalizeConditionOperator(node.Params["operator"])
	return err
}

func (e *ConditionIFExecutor) GetDefinition() NodeDefinition {
	return NodeDefinition{
		Type: TypeConditionIF, Name: "IF / ELSE Condition", Description: "Branches workflow execution based on a comparison condition", Icon: "GitBranch", Category: "LOGIC", Retryable: true,
		Params: []ParamDefinition{
			{Name: "field", Label: "Input Value", Type: "text", Default: "", Required: true, Description: "Input value to compare. Structured values are converted to JSON text after expression resolution."},
			{Name: "operator", Label: "Operator", Type: "select", Default: "equals", Options: []string{"equals", "not_equals", "contains", "is_not_empty"}, Required: true, Description: "Comparison operator"},
			{Name: "value", Label: "Compare Value", Type: "text", Default: "", Required: false, Description: "Expected value to compare against"},
		},
	}
}
