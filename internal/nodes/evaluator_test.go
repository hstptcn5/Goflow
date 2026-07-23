package nodes

import (
	"reflect"
	"testing"
)

func TestResolveParams(t *testing.T) {
	ctx := NewExecutionContext("wf-1", "exec-1")
	ctx.SetOutput("node1", map[string]interface{}{
		"status_code": 200,
		"data": map[string]interface{}{
			"id":   "user-123",
			"name": "Alice",
		},
		"tags": []interface{}{"admin", "developer"},
	})
	ctx.SetOutput("$trigger", map[string]interface{}{
		"query": "hello world",
	})

	params := map[string]interface{}{
		"url":           "https://api.example.com/users/{{node1.data.id}}",
		"exact_obj":     "{{node1.data}}",
		"exact_int":     "{{node1.status_code}}",
		"tag_first":     "{{node1.tags.0}}",
		"trigger_query": "Query was: {{$trigger.query}}",
		"nested_map": map[string]interface{}{
			"auth": "Bearer {{node1.data.id}}",
		},
		"nested_slice": []interface{}{
			"Name: {{node1.data.name}}",
			123,
		},
		"non_existent": "{{node2.some_field}}",
		"regular_str":  "just a string",
		"number":       456,
	}

	resolved := ResolveParams(ctx, params)

	expectedURL := "https://api.example.com/users/user-123"
	if resolved["url"] != expectedURL {
		t.Errorf("Expected url to be %q, got %q", expectedURL, resolved["url"])
	}

	expectedObj := map[string]interface{}{
		"id":   "user-123",
		"name": "Alice",
	}
	if !reflect.DeepEqual(resolved["exact_obj"], expectedObj) {
		t.Errorf("Expected exact_obj to be %v, got %v", expectedObj, resolved["exact_obj"])
	}

	if resolved["exact_int"] != 200 {
		t.Errorf("Expected exact_int to be 200, got %v", resolved["exact_int"])
	}

	if resolved["tag_first"] != "admin" {
		t.Errorf("Expected tag_first to be 'admin', got %v", resolved["tag_first"])
	}

	expectedQuery := "Query was: hello world"
	if resolved["trigger_query"] != expectedQuery {
		t.Errorf("Expected trigger_query to be %q, got %q", expectedQuery, resolved["trigger_query"])
	}

	expectedNestedMap := map[string]interface{}{
		"auth": "Bearer user-123",
	}
	if !reflect.DeepEqual(resolved["nested_map"], expectedNestedMap) {
		t.Errorf("Expected nested_map to be %v, got %v", expectedNestedMap, resolved["nested_map"])
	}

	expectedNestedSlice := []interface{}{
		"Name: Alice",
		123,
	}
	if !reflect.DeepEqual(resolved["nested_slice"], expectedNestedSlice) {
		t.Errorf("Expected nested_slice to be %v, got %v", expectedNestedSlice, resolved["nested_slice"])
	}

	if resolved["non_existent"] != "" {
		t.Errorf("Expected non_existent to be empty string, got %v", resolved["non_existent"])
	}

	if resolved["regular_str"] != "just a string" {
		t.Errorf("Expected regular_str to be unchanged, got %v", resolved["regular_str"])
	}

	if resolved["number"] != 456 {
		t.Errorf("Expected number to be unchanged, got %v", resolved["number"])
	}
}
