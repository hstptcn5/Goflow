package nodes

import (
	"strings"
	"testing"
)

func TestMongoDBCommandExecutorOffline(t *testing.T) {
	executor := NewMongoDBCommandExecutor()
	ctx := NewExecutionContext("wf-1", "exec-1")

	// Test 1: Validation failure (empty database/collection)
	nodeEmpty := &Node{
		Params: map[string]interface{}{
			"database":   "",
			"collection": "",
		},
	}
	err := executor.Validate(nodeEmpty)
	if err == nil || !strings.Contains(err.Error(), "database and collection parameters are required") {
		t.Errorf("Expected validation error, got: %v", err)
	}

	// Test 2: Connection failure (offline server)
	nodeOffline := &Node{
		Params: map[string]interface{}{
			"connection_string": "mongodb://localhost:57017",
			"database":          "testdb",
			"collection":        "users",
			"command":           "FIND_ONE",
			"filter_json":       `{"id": 1}`,
		},
	}
	_, err = executor.Execute(ctx, nodeOffline)
	if err == nil {
		t.Errorf("Expected mongodb connection error, but got nil")
	}
}
