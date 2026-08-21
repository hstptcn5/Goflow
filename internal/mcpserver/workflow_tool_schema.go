package mcpserver

import "encoding/json"

func dynamicWorkflowInputSchema(raw string) json.RawMessage {
	base := schemaOrEmptyObject(raw)
	var schema map[string]interface{}
	if err := json.Unmarshal(base, &schema); err != nil {
		schema = map[string]interface{}{"type": "object"}
	}
	properties, _ := schema["properties"].(map[string]interface{})
	if properties == nil {
		properties = map[string]interface{}{}
	}
	properties["_goflow"] = map[string]interface{}{
		"type":        "object",
		"description": "Optional Goflow execution controls. This object is removed before workflow input is delivered.",
		"properties": map[string]interface{}{
			"idempotency_key": map[string]interface{}{
				"type":        "string",
				"maxLength":   256,
				"description": "Stable key used to avoid duplicate side effects if an MCP client retries a tool call.",
			},
		},
		"additionalProperties": false,
	}
	schema["properties"] = properties
	encoded, _ := json.Marshal(schema)
	return encoded
}
