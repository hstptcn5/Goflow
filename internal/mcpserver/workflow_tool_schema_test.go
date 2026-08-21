package mcpserver

import (
	"encoding/json"
	"testing"
)

func TestDynamicWorkflowInputSchemaAddsGoflowControls(t *testing.T) {
	raw := `{"type":"object","properties":{"order_id":{"type":"string"}},"required":["order_id"],"additionalProperties":false}`
	result := dynamicWorkflowInputSchema(raw)
	var schema map[string]interface{}
	if err := json.Unmarshal(result, &schema); err != nil {
		t.Fatal(err)
	}
	properties := schema["properties"].(map[string]interface{})
	if properties["order_id"] == nil || properties["_goflow"] == nil {
		t.Fatalf("schema properties = %#v", properties)
	}
	if schema["additionalProperties"] != false {
		t.Fatalf("additionalProperties changed: %#v", schema["additionalProperties"])
	}
}

func TestDynamicWorkflowInputSchemaFallsBackToObject(t *testing.T) {
	result := dynamicWorkflowInputSchema("not-json")
	var schema map[string]interface{}
	if err := json.Unmarshal(result, &schema); err != nil {
		t.Fatal(err)
	}
	properties, ok := schema["properties"].(map[string]interface{})
	if !ok || properties["_goflow"] == nil {
		t.Fatalf("fallback schema = %#v", schema)
	}
}
