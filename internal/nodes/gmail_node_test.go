package nodes

import (
	"strings"
	"testing"
)

func TestGmailRESTExecutorOffline(t *testing.T) {
	executor := NewGmailRESTExecutor()
	ctx := NewExecutionContext("wf-1", "exec-1")

	// Test 1: Empty service account JSON
	nodeEmpty := &Node{
		Params: map[string]interface{}{
			"service_account_json": "",
			"to":                   "test@example.com",
		},
	}
	_, err := executor.Execute(ctx, nodeEmpty)
	if err == nil || !strings.Contains(err.Error(), "service_account_json is empty") {
		t.Errorf("Expected service_account_json is empty error, got: %v", err)
	}

	// Test 2: Validation failure (empty recipient)
	nodeNoRecipient := &Node{
		Params: map[string]interface{}{
			"to": "",
		},
	}
	err = executor.Validate(nodeNoRecipient)
	if err == nil || !strings.Contains(err.Error(), "recipient email 'to' is required") {
		t.Errorf("Expected validation failure for recipient, got: %v", err)
	}
}
