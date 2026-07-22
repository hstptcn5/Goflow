package nodes

import (
	"strings"
	"testing"
)

func TestSSHRunnerExecutorOffline(t *testing.T) {
	executor := NewSSHRunnerExecutor()
	ctx := NewExecutionContext("wf-1", "exec-1")

	// Test 1: Validation failure (empty address)
	nodeEmpty := &Node{
		Params: map[string]interface{}{
			"address": "",
		},
	}
	err := executor.Validate(nodeEmpty)
	if err == nil || !strings.Contains(err.Error(), "address is required") {
		t.Errorf("Expected validate address error, got: %v", err)
	}

	// Test 2: Connection failure (offline server)
	nodeOffline := &Node{
		Params: map[string]interface{}{
			"address":  "127.0.0.1:50022",
			"username": "root",
			"password": "wrong-password",
			"command":  "whoami",
		},
	}
	_, err = executor.Execute(ctx, nodeOffline)
	if err == nil {
		t.Errorf("Expected SSH connection failure error, but got nil")
	}
}
