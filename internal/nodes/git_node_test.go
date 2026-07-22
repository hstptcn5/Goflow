package nodes

import (
	"strings"
	"testing"
)

func TestGitCommandExecutorOffline(t *testing.T) {
	executor := NewGitCommandExecutor()
	ctx := NewExecutionContext("wf-1", "exec-1")

	// Test 1: Empty repo or directory parameter validation on clone
	nodeEmptyClone := &Node{
		Params: map[string]interface{}{
			"action":         "CLONE",
			"repository_url": "",
			"directory":      "",
		},
	}
	_, err := executor.Execute(ctx, nodeEmptyClone)
	if err == nil || !strings.Contains(err.Error(), "required for CLONE") {
		t.Errorf("Expected clone empty parameters error, got: %v", err)
	}

	// Test 2: Empty directory on pull
	nodeEmptyPull := &Node{
		Params: map[string]interface{}{
			"action":           "PULL",
			"target_directory": "",
		},
	}
	_, err = executor.Execute(ctx, nodeEmptyPull)
	if err == nil || !strings.Contains(err.Error(), "target_directory is required") {
		t.Errorf("Expected pull empty directory error, got: %v", err)
	}
}
