package nodes

import (
	"strings"
	"testing"
)

func TestRedisCommandExecutorOffline(t *testing.T) {
	executor := NewRedisCommandExecutor()
	ctx := NewExecutionContext("wf-1", "exec-1")

	// Test 1: Empty Key error
	nodeEmptyKey := &Node{
		Params: map[string]interface{}{
			"address":  "localhost:6379",
			"command":  "GET",
			"key":      "",
		},
	}
	_, err := executor.Execute(ctx, nodeEmptyKey)
	if err == nil || !strings.Contains(err.Error(), "Key is required") {
		t.Errorf("Expected Redis Key is required error, got: %v", err)
	}

	// Test 2: Validation test
	err = executor.Validate(nodeEmptyKey)
	if err == nil || !strings.Contains(err.Error(), "key is required") {
		t.Errorf("Expected validate key is required error, got: %v", err)
	}

	// Test 3: Connection failure error (Offline port)
	nodeOffline := &Node{
		Params: map[string]interface{}{
			"address":  "localhost:56379",
			"command":  "GET",
			"key":      "test-key",
		},
	}
	_, err = executor.Execute(ctx, nodeOffline)
	if err == nil {
		t.Errorf("Expected Redis connection error, but got nil")
	}
}
