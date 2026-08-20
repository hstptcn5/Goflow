package nodes

import (
	"strings"
	"testing"
)

func TestGoogleDriveExecutorOffline(t *testing.T) {
	executor := NewGoogleDriveExecutor()
	ctx := NewExecutionContext("wf-1", "exec-1")

	// Test 1: Missing Google auth material.
	nodeEmpty := &Node{Params: map[string]interface{}{
		"service_account_json": "",
		"action":               "LIST",
	}}
	_, err := executor.Execute(ctx, nodeEmpty)
	if err == nil || !strings.Contains(err.Error(), "requires an encrypted credential or service_account_json") {
		t.Errorf("Expected missing Google Drive credential error, got: %v", err)
	}

	// Test 2: Invalid service-account JSON format.
	nodeInvalid := &Node{Params: map[string]interface{}{
		"service_account_json": "{bad-json}",
		"action":               "LIST",
	}}
	_, err = executor.Execute(ctx, nodeInvalid)
	if err == nil || !strings.Contains(err.Error(), "invalid service account JSON") {
		t.Errorf("Expected invalid service account JSON error, got: %v", err)
	}
}
