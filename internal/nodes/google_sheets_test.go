package nodes

import (
	"strings"
	"testing"
)

func TestGoogleSheetsExecutorOffline(t *testing.T) {
	executor := NewGoogleSheetsExecutor()
	ctx := NewExecutionContext("wf-1", "exec-1")

	// Test 1: Empty spreadsheet ID.
	nodeEmptySheetID := &Node{Params: map[string]interface{}{
		"spreadsheet_id":       "",
		"service_account_json": `{"client_email":"x@x.iam.gserviceaccount.com"}`,
		"action":               "READ",
	}}
	_, err := executor.Execute(ctx, nodeEmptySheetID)
	if err == nil || !strings.Contains(err.Error(), "spreadsheet_id is required") {
		t.Errorf("Expected spreadsheet_id is required error, got: %v", err)
	}

	// Test 2: Validation sees the same error.
	err = executor.Validate(nodeEmptySheetID)
	if err == nil || !strings.Contains(err.Error(), "spreadsheet_id is required") {
		t.Errorf("Expected validate spreadsheet_id is required error, got: %v", err)
	}

	// Test 3: Missing auth material fails before outbound request.
	nodeEmptySA := &Node{Params: map[string]interface{}{
		"spreadsheet_id":       "1abc123",
		"service_account_json": "",
		"action":               "READ",
	}}
	_, err = executor.Execute(ctx, nodeEmptySA)
	if err == nil || !strings.Contains(err.Error(), "requires an encrypted credential or service_account_json") {
		t.Errorf("Expected missing Google Sheets credential error, got: %v", err)
	}

	// Test 4: Invalid service-account JSON is rejected before network access.
	nodeInvalidSA := &Node{Params: map[string]interface{}{
		"spreadsheet_id":       "1abc123",
		"service_account_json": "{invalid-json}",
		"action":               "READ",
	}}
	_, err = executor.Execute(ctx, nodeInvalidSA)
	if err == nil || !strings.Contains(err.Error(), "invalid service account JSON") {
		t.Errorf("Expected invalid service account JSON error, got: %v", err)
	}
}
