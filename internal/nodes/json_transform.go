package nodes

import (
	"encoding/json"
	"fmt"
)

type JSONTransformExecutor struct{}

func NewJSONTransformExecutor() *JSONTransformExecutor {
	return &JSONTransformExecutor{}
}

func (e *JSONTransformExecutor) Execute(ctx *ExecutionContext, node *Node) (interface{}, error) {
	jsonTemplate, _ := node.Params["json_template"].(string)
	if jsonTemplate == "" {
		jsonTemplate = "{}"
	}

	var parsed map[string]interface{}
	if err := json.Unmarshal([]byte(jsonTemplate), &parsed); err != nil {
		return nil, fmt.Errorf("invalid json_template: %w", err)
	}

	// ????a th??m context outputs c???a c??c node tr?????c v??o k???t qu??? transform
	result := map[string]interface{}{
		"transformed": parsed,
		"context":     ctx.Outputs,
	}

	return result, nil
}

func (e *JSONTransformExecutor) Validate(node *Node) error {
	return nil
}

func (e *JSONTransformExecutor) GetDefinition() NodeDefinition {
	return NodeDefinition{
		Type:        TypeJSONTransform,
		Name:        "JSON Transform",
		Description: "Creates or transforms JSON data structures",
		Icon:        "Code",
		Category:    "ACTION",
		Retryable:   true,
		Params: []ParamDefinition{
			{
				Name:        "json_template",
				Label:       "JSON Structure",
				Type:        "json",
				Default:     "{\n  \"status\": \"success\",\n  \"processed\": true\n}",
				Required:    true,
				Description: "Desired JSON output structure",
			},
		},
	}
}
