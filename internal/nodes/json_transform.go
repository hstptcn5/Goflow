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

	// Đưa thêm context outputs của các node trước vào kết quả transform
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
		Description: "Tạo hoặc trích xuất biến đổi cấu trúc dữ liệu JSON",
		Icon:        "Code",
		Category:    "ACTION",
		Params: []ParamDefinition{
			{
				Name:        "json_template",
				Label:       "JSON Structure",
				Type:        "json",
				Default:     "{\n  \"status\": \"success\",\n  \"processed\": true\n}",
				Required:    true,
				Description: "Cấu trúc JSON mong muốn",
			},
		},
	}
}
