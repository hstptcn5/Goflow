package nodes

import (
	"encoding/json"
	"fmt"
	"strings"
)

const maxJSONTransformBytes = 1 << 20

type JSONTransformExecutor struct{}

func NewJSONTransformExecutor() *JSONTransformExecutor { return &JSONTransformExecutor{} }

func (e *JSONTransformExecutor) Execute(ctx *ExecutionContext, node *Node) (interface{}, error) {
	parsed, err := parseJSONTransformValue(node.Params["json_template"])
	if err != nil {
		return nil, err
	}
	return map[string]interface{}{
		"transformed": parsed,
		"context":     ctx.GetOutputs(),
	}, nil
}

func parseJSONTransformValue(raw interface{}) (map[string]interface{}, error) {
	if raw == nil {
		return map[string]interface{}{}, nil
	}
	switch typed := raw.(type) {
	case map[string]interface{}:
		encoded, err := json.Marshal(typed)
		if err != nil {
			return nil, fmt.Errorf("json_template could not be encoded: %w", err)
		}
		if len(encoded) > maxJSONTransformBytes {
			return nil, fmt.Errorf("json_template exceeds %d byte limit", maxJSONTransformBytes)
		}
		var copied map[string]interface{}
		if err := json.Unmarshal(encoded, &copied); err != nil {
			return nil, fmt.Errorf("json_template could not be copied: %w", err)
		}
		return copied, nil
	case string:
		text := strings.TrimSpace(typed)
		if text == "" {
			return map[string]interface{}{}, nil
		}
		if len(text) > maxJSONTransformBytes {
			return nil, fmt.Errorf("json_template exceeds %d byte limit", maxJSONTransformBytes)
		}
		var parsed map[string]interface{}
		if err := json.Unmarshal([]byte(text), &parsed); err != nil {
			return nil, fmt.Errorf("invalid json_template: %w", err)
		}
		return parsed, nil
	default:
		return nil, fmt.Errorf("json_template must resolve to a JSON object or JSON object string")
	}
}

func (e *JSONTransformExecutor) Validate(node *Node) error {
	raw := node.Params["json_template"]
	if text, ok := raw.(string); ok && containsTemplateExpression(text) {
		return nil
	}
	_, err := parseJSONTransformValue(raw)
	return err
}

func (e *JSONTransformExecutor) GetDefinition() NodeDefinition {
	return NodeDefinition{
		Type: TypeJSONTransform, Name: "JSON Transform", Description: "Creates or transforms JSON object data structures", Icon: "Code", Category: "ACTION", Retryable: true,
		Params: []ParamDefinition{{Name: "json_template", Label: "JSON Structure", Type: "json", Default: "{\n  \"status\": \"success\",\n  \"processed\": true\n}", Required: true, Description: "Desired JSON object output structure"}},
	}
}
