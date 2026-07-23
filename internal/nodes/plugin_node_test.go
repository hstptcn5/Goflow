package nodes

import (
	"strings"
	"testing"
)

func TestGoflowPluginExecutorOffline(t *testing.T) {
	executor := NewGoflowPluginExecutor()
	ctx := NewExecutionContext("wf-1", "exec-1")

	// Test 1: Validation failure (empty plugin name)
	nodeEmpty := &Node{
		Params: map[string]interface{}{
			"plugin_name": "",
		},
	}
	err := executor.Validate(nodeEmpty)
	if err == nil || !strings.Contains(err.Error(), "plugin_name is required") {
		t.Errorf("Expected validation failure for empty plugin name, got: %v", err)
	}

	// Test 2: Execution failure (offline executable file not exist)
	nodeOffline := &Node{
		Params: map[string]interface{}{
			"plugin_name": "non_existent_plugin_filename_12345",
		},
	}
	_, err = executor.Execute(ctx, nodeOffline)
	if err == nil || !strings.Contains(err.Error(), "plugin executable not found") {
		t.Errorf("Expected plugin not found error, got: %v", err)
	}

	// Test 3: Validation rejects path traversal
	nodeTraversal := &Node{
		Params: map[string]interface{}{
			"plugin_name": "../outside",
		},
	}
	err = executor.Validate(nodeTraversal)
	if err == nil || !strings.Contains(err.Error(), "file name in the plugins directory") {
		t.Errorf("Expected path traversal validation failure, got: %v", err)
	}
}
