package nodes

import (
	"encoding/json"
	"fmt"
	"reflect"
	"regexp"
	"strconv"
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
	case "exists":
		return "exists", nil
	case "not_exists":
		return "not_exists", nil
	case "empty", "is_empty":
		return "empty", nil
	case "not_empty", "is_not_empty":
		return "not_empty", nil
	case "greater_than", ">":
		return "greater_than", nil
	case "greater_or_equal", ">=", "greater_than_or_equal":
		return "greater_or_equal", nil
	case "less_than", "<":
		return "less_than", nil
	case "less_or_equal", "<=", "less_than_or_equal":
		return "less_or_equal", nil
	case "contains":
		return "contains", nil
	case "not_contains":
		return "not_contains", nil
	case "starts_with":
		return "starts_with", nil
	case "ends_with":
		return "ends_with", nil
	case "regex", "matches_regex":
		return "regex", nil
	case "true", "is_true":
		return "true", nil
	case "false", "is_false":
		return "false", nil
	default:
		return "", fmt.Errorf("unsupported IF operator %q", operator)
	}
}

func conditionIsEmpty(raw interface{}) bool {
	if raw == nil {
		return true
	}
	switch typed := raw.(type) {
	case string:
		return strings.TrimSpace(typed) == ""
	case []byte:
		return len(typed) == 0
	case []interface{}:
		return len(typed) == 0
	case map[string]interface{}:
		return len(typed) == 0
	}
	value := reflect.ValueOf(raw)
	if value.IsValid() && (value.Kind() == reflect.Slice || value.Kind() == reflect.Map || value.Kind() == reflect.Array) {
		return value.Len() == 0
	}
	return false
}

func conditionNumber(raw interface{}) (float64, bool) {
	switch typed := raw.(type) {
	case int:
		return float64(typed), true
	case int8:
		return float64(typed), true
	case int16:
		return float64(typed), true
	case int32:
		return float64(typed), true
	case int64:
		return float64(typed), true
	case uint:
		return float64(typed), true
	case uint8:
		return float64(typed), true
	case uint16:
		return float64(typed), true
	case uint32:
		return float64(typed), true
	case uint64:
		return float64(typed), true
	case float32:
		return float64(typed), true
	case float64:
		return typed, true
	case json.Number:
		value, err := typed.Float64()
		return value, err == nil
	case string:
		value, err := strconv.ParseFloat(strings.TrimSpace(typed), 64)
		return value, err == nil
	default:
		return 0, false
	}
}

func conditionBool(raw interface{}) (bool, bool) {
	switch typed := raw.(type) {
	case bool:
		return typed, true
	case string:
		value, err := strconv.ParseBool(strings.TrimSpace(typed))
		return value, err == nil
	default:
		return false, false
	}
}

func conditionEquals(left, right interface{}) bool {
	if left == nil || right == nil {
		return left == nil && right == nil
	}
	if lnum, lok := conditionNumber(left); lok {
		if rnum, rok := conditionNumber(right); rok {
			return lnum == rnum
		}
	}
	return reflect.DeepEqual(left, right)
}

func evaluateCondition(field interface{}, operator string, value interface{}) (bool, error) {
	switch operator {
	case "equals":
		return conditionEquals(field, value), nil
	case "not_equals":
		return !conditionEquals(field, value), nil
	case "exists":
		return field != nil, nil
	case "not_exists":
		return field == nil, nil
	case "empty":
		return conditionIsEmpty(field), nil
	case "not_empty":
		return !conditionIsEmpty(field), nil
	case "greater_than", "greater_or_equal", "less_than", "less_or_equal":
		left, leftOK := conditionNumber(field)
		right, rightOK := conditionNumber(value)
		if !leftOK || !rightOK {
			return false, fmt.Errorf("operator %s requires numeric values", operator)
		}
		switch operator {
		case "greater_than":
			return left > right, nil
		case "greater_or_equal":
			return left >= right, nil
		case "less_than":
			return left < right, nil
		default:
			return left <= right, nil
		}
	case "contains", "not_contains", "starts_with", "ends_with", "regex":
		left := conditionValueString(field)
		right := conditionValueString(value)
		switch operator {
		case "contains":
			return strings.Contains(left, right), nil
		case "not_contains":
			return !strings.Contains(left, right), nil
		case "starts_with":
			return strings.HasPrefix(left, right), nil
		case "ends_with":
			return strings.HasSuffix(left, right), nil
		default:
			pattern, err := regexp.Compile(right)
			if err != nil {
				return false, fmt.Errorf("invalid IF regex: %w", err)
			}
			return pattern.MatchString(left), nil
		}
	case "true", "false":
		actual, ok := conditionBool(field)
		if !ok {
			return false, fmt.Errorf("operator %s requires a boolean value", operator)
		}
		if operator == "true" {
			return actual, nil
		}
		return !actual, nil
	default:
		return false, fmt.Errorf("unsupported IF operator %q", operator)
	}
}

func (e *ConditionIFExecutor) Execute(ctx *ExecutionContext, node *Node) (interface{}, error) {
	field := node.Params["field"]
	value := node.Params["value"]
	operator, err := normalizeConditionOperator(node.Params["operator"])
	if err != nil {
		return nil, err
	}
	isTrue, err := evaluateCondition(field, operator, value)
	if err != nil {
		return nil, err
	}
	resultHandle := "false"
	if isTrue {
		resultHandle = "true"
	}
	return map[string]interface{}{
		"result":        isTrue,
		"target_handle": resultHandle,
		"evaluated":     fmt.Sprintf("%s %s %s", conditionValueString(field), operator, conditionValueString(value)),
		"execution_tag": uuid.New().String(),
	}, nil
}

func (e *ConditionIFExecutor) Validate(node *Node) error {
	operator, err := normalizeConditionOperator(node.Params["operator"])
	if err != nil {
		return err
	}
	if operator == "regex" {
		_, err = evaluateCondition("", operator, node.Params["value"])
	}
	return err
}

func (e *ConditionIFExecutor) GetDefinition() NodeDefinition {
	return NodeDefinition{
		Type: TypeConditionIF, Name: "IF / ELSE Condition", Description: "Branches workflow execution using typed comparisons", Icon: "GitBranch", Category: "LOGIC", Retryable: true,
		Params: []ParamDefinition{
			{Name: "field", Label: "Input Value", Type: "text", Default: "", Required: true, Description: "Input value to compare. A complete expression preserves the upstream value type."},
			{Name: "operator", Label: "Operator", Type: "select", Default: "equals", Options: []string{"equals", "not_equals", "exists", "not_exists", "empty", "not_empty", "greater_than", "greater_or_equal", "less_than", "less_or_equal", "contains", "not_contains", "starts_with", "ends_with", "regex", "true", "false"}, Required: true, Description: "Type-aware comparison operator"},
			{Name: "value", Label: "Compare Value", Type: "text", Default: "", Required: false, Description: "Expected comparison value; number operators accept numeric values or numeric literals"},
		},
	}
}
