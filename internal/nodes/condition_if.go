package nodes

import (
	"fmt"
	"strings"

	"github.com/google/uuid"
)

type ConditionIFExecutor struct{}

func NewConditionIFExecutor() *ConditionIFExecutor {
	return &ConditionIFExecutor{}
}

func (e *ConditionIFExecutor) Execute(ctx *ExecutionContext, node *Node) (interface{}, error) {
	field, _ := node.Params["field"].(string)
	operator, _ := node.Params["operator"].(string)
	value, _ := node.Params["value"].(string)

	isTrue := false

	switch operator {
	case "equals", "==":
		isTrue = field == value
	case "not_equals", "!=":
		isTrue = field != value
	case "contains":
		isTrue = strings.Contains(field, value)
	case "is_not_empty":
		isTrue = field != ""
	default:
		isTrue = field == value
	}

	resultHandle := "false"
	if isTrue {
		resultHandle = "true"
	}

	return map[string]interface{}{
		"result":        isTrue,
		"target_handle": resultHandle,
		"evaluated":     fmt.Sprintf("'%s' %s '%s'", field, operator, value),
		"execution_tag": uuid.New().String(),
	}, nil
}

func (e *ConditionIFExecutor) Validate(node *Node) error {
	return nil
}

func (e *ConditionIFExecutor) GetDefinition() NodeDefinition {
	return NodeDefinition{
		Type:        TypeConditionIF,
		Name:        "IF / ELSE Condition",
		Description: "Rẽ nhánh thực thi workflow dựa theo điều kiện so sánh",
		Icon:        "GitBranch",
		Category:    "LOGIC",
		Params: []ParamDefinition{
			{
				Name:        "field",
				Label:       "Input Value",
				Type:        "text",
				Default:     "",
				Required:    true,
				Description: "Giá trị truyền vào cần so sánh",
			},
			{
				Name:        "operator",
				Label:       "Operator",
				Type:        "select",
				Default:     "equals",
				Options:     []string{"equals", "not_equals", "contains", "is_not_empty"},
				Required:    true,
				Description: "Toán tử so sánh",
			},
			{
				Name:        "value",
				Label:       "Compare Value",
				Type:        "text",
				Default:     "",
				Required:    false,
				Description: "Giá trị đích để đối chiếu",
			},
		},
	}
}
