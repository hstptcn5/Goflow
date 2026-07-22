package nodes

import (
	"strings"
	"testing"
)

func TestNotionPageExecutorOffline(t *testing.T) {
	executor := NewNotionPageExecutor()
	ctx := NewExecutionContext("wf-1", "exec-1")

	// Test 1: Empty database ID validation error
	nodeEmpty := &Node{
		Params: map[string]interface{}{
			"database_id": "",
		},
	}
	err := executor.Validate(nodeEmpty)
	if err == nil || !strings.Contains(err.Error(), "database_id is required") {
		t.Errorf("Expected validate database_id error, got: %v", err)
	}

	// Test 2: Empty notion token execution error
	nodeNoToken := &Node{
		Params: map[string]interface{}{
			"database_id":  "db-123",
			"notion_token": "",
		},
	}
	_, err = executor.Execute(ctx, nodeNoToken)
	if err == nil || !strings.Contains(err.Error(), "notion_token is empty") {
		t.Errorf("Expected notion_token is empty error, got: %v", err)
	}
}
