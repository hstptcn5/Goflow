package nodes

import (
	"strings"
	"testing"
)

func TestGoogleSheetsExecutorOffline(t *testing.T) {
	executor := NewGoogleSheetsExecutor()
	ctx := NewExecutionContext("wf-1", "exec-1")

	// Test 1: Empty spreadsheet ID
	nodeEmptySheetID := &Node{
		Params: map[string]interface{}{
			"spreadsheet_id":       "",
			"service_account_json": `{"client_email": "x@x.iam.gserviceaccount.com"}`,
		},
	}
	_, err := executor.Execute(ctx, nodeEmptySheetID)
	if err == nil || !strings.Contains(err.Error(), "spreadsheet_id is required") {
		t.Errorf("Expected spreadsheet_id is required error, got: %v", err)
	}

	// Test 2: Validation
	err = executor.Validate(nodeEmptySheetID)
	if err == nil || !strings.Contains(err.Error(), "spreadsheet_id is required") {
		t.Errorf("Expected validate spreadsheet_id is required error, got: %v", err)
	}

	// Test 3: Empty Service Account JSON
	nodeEmptySA := &Node{
		Params: map[string]interface{}{
			"spreadsheet_id":       "1abc123",
			"service_account_json": "",
		},
	}
	_, err = executor.Execute(ctx, nodeEmptySA)
	if err == nil || !strings.Contains(err.Error(), "service_account_json is empty") {
		t.Errorf("Expected service_account_json is empty error, got: %v", err)
	}

	// Test 4: Invalid Service Account JSON structure
	nodeInvalidSA := &Node{
		Params: map[string]interface{}{
			"spreadsheet_id":       "1abc123",
			"service_account_json": "{invalid-json}",
		},
	}
	_, err = executor.Execute(ctx, nodeInvalidSA)
	if err == nil || !strings.Contains(err.Error(), "invalid service account JSON") {
		t.Errorf("Expected invalid service account JSON error, got: %v", err)
	}
}
