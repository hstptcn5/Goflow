package nodes

import (
	"strings"
	"testing"
)

func TestNotionPageExecutorOffline(t *testing.T) {
	executor := NewNotionPageExecutor()
	ctx := NewExecutionContext("wf-1", "exec-1")

	// Test 1: Empty database ID validation error.
	nodeEmpty := &Node{Params: map[string]interface{}{"database_id": ""}}
	err := executor.Validate(nodeEmpty)
	if err == nil || !strings.Contains(err.Error(), "database_id is required") {
		t.Errorf("Expected validate database_id error, got: %v", err)
	}

	// Test 2: Missing token/credential is rejected before outbound request.
	nodeNoToken := &Node{Params: map[string]interface{}{
		"database_id":    "db-123",
		"notion_token":   "",
		"properties_json": `{"Name":{"title":[{"text":{"content":"test"}}]}}`,
	}}
	_, err = executor.Execute(ctx, nodeNoToken)
	if err == nil || !strings.Contains(err.Error(), "token or encrypted credential is required") {
		t.Errorf("Expected missing Notion credential error, got: %v", err)
	}
}
