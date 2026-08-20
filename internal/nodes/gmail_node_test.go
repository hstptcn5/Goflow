package nodes

import (
	"strings"
	"testing"
)

func TestGmailRESTExecutorOffline(t *testing.T) {
	executor := NewGmailRESTExecutor()
	ctx := NewExecutionContext("wf-1", "exec-1")

	// Test 1: Missing Google credential fails before any outbound request.
	nodeEmpty := &Node{Params: map[string]interface{}{
		"service_account_json": "",
		"to":                   "test@example.com",
		"subject":              "test",
		"body":                 "hello",
	}}
	_, err := executor.Execute(ctx, nodeEmpty)
	if err == nil || !strings.Contains(err.Error(), "requires an encrypted credential or service_account_json") {
		t.Errorf("Expected missing Gmail credential error, got: %v", err)
	}

	// Test 2: Validation failure (empty recipient).
	nodeNoRecipient := &Node{Params: map[string]interface{}{
		"credential_id": "cred",
		"to":            "",
		"subject":       "test",
		"body":          "hello",
	}}
	err = executor.Validate(nodeNoRecipient)
	if err == nil || !strings.Contains(err.Error(), "recipient email 'to' is required") {
		t.Errorf("Expected validation failure for recipient, got: %v", err)
	}
}
