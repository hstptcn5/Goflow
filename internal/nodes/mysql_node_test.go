package nodes

import (
	"strings"
	"testing"
)

func TestMySQLQueryExecutorOffline(t *testing.T) {
	executor := NewMySQLQueryExecutor()
	ctx := NewExecutionContext("wf-1", "exec-1")

	// Test 1: Empty connection string error
	nodeEmptyConn := &Node{
		Params: map[string]interface{}{
			"connection_string": "",
			"query":             "SELECT 1;",
			"query_type":        "SELECT",
		},
	}
	_, err := executor.Execute(ctx, nodeEmptyConn)
	if err == nil || !strings.Contains(err.Error(), "connection_string is empty") {
		t.Errorf("Expected connection_string is empty error, got: %v", err)
	}

	// Test 2: Empty query error
	nodeEmptyQuery := &Node{
		Params: map[string]interface{}{
			"connection_string": "root:pass@tcp(localhost:3306)/db",
			"query":             "",
			"query_type":        "SELECT",
		},
	}
	_, err = executor.Execute(ctx, nodeEmptyQuery)
	if err == nil || !strings.Contains(err.Error(), "SQL query is empty") {
		t.Errorf("Expected SQL query is empty error, got: %v", err)
	}

	// Test 3: Connection failure error (Offline port)
	nodeOffline := &Node{
		Params: map[string]interface{}{
			"connection_string": "root:pass@tcp(localhost:53306)/db",
			"query":             "SELECT 1;",
			"query_type":        "SELECT",
		},
	}
	_, err = executor.Execute(ctx, nodeOffline)
	if err == nil {
		t.Errorf("Expected mysql connection error, but got nil")
	}
}
