package nodes

import (
	"fmt"
	"testing"
)

func newLoopPolicyContext(t *testing.T, calls *int) *ExecutionContext {
	t.Helper()
	ctx := NewExecutionContext("wf", "exec")
	ctx.ExecuteWorkflow = func(workflowID string, payload interface{}) (interface{}, error) {
		(*calls)++
		if number, ok := payload.(float64); ok && number == 2 {
			return nil, fmt.Errorf("item two failed")
		}
		return map[string]interface{}{"value": payload}, nil
	}
	return ctx
}

func TestSubWorkflowLoopStopAllStopsSequentialScheduling(t *testing.T) {
	calls := 0
	ctx := newLoopPolicyContext(t, &calls)
	executor := NewSubWorkflowExecutor()
	_, err := executor.Execute(ctx, &Node{Params: map[string]interface{}{
		"sub_workflow_id": "child",
		"payload_json":    `[1,2,3]`,
		"loop_mode":       true,
		"parallel":        false,
		"error_policy":    "Stop all",
	}})
	if err == nil {
		t.Fatal("Stop all unexpectedly swallowed child failure")
	}
	if calls != 2 {
		t.Fatalf("child calls = %d, want 2 before stop", calls)
	}
}

func TestSubWorkflowLoopContinueReturnsPerItemStatus(t *testing.T) {
	calls := 0
	ctx := newLoopPolicyContext(t, &calls)
	executor := NewSubWorkflowExecutor()
	out, err := executor.Execute(ctx, &Node{Params: map[string]interface{}{
		"sub_workflow_id": "child",
		"payload_json":    `[1,2,3]`,
		"loop_mode":       true,
		"parallel":        false,
		"error_policy":    "Continue",
	}})
	if err != nil {
		t.Fatalf("Continue returned error: %v", err)
	}
	if calls != 3 {
		t.Fatalf("child calls = %d, want 3", calls)
	}
	result := out.(map[string]interface{})
	if result["success_count"] != 2 || result["error_count"] != 1 {
		t.Fatalf("unexpected counts: %#v", result)
	}
	items := result["items"].([]interface{})
	failed := items[1].(map[string]interface{})
	if failed["ok"] != false || failed["error"] != "item two failed" {
		t.Fatalf("unexpected failed item: %#v", failed)
	}
}

func TestSubWorkflowLoopCollectErrorsReturnsStructuredCollections(t *testing.T) {
	calls := 0
	ctx := newLoopPolicyContext(t, &calls)
	executor := NewSubWorkflowExecutor()
	out, err := executor.Execute(ctx, &Node{Params: map[string]interface{}{
		"sub_workflow_id": "child",
		"payload_json":    `[1,2,3]`,
		"loop_mode":       true,
		"parallel":        false,
		"error_policy":    "Collect errors",
	}})
	if err != nil {
		t.Fatalf("Collect errors returned error: %v", err)
	}
	result := out.(map[string]interface{})
	if result["success_count"] != 2 || result["error_count"] != 1 {
		t.Fatalf("unexpected counts: %#v", result)
	}
	errorsList := result["errors"].([]map[string]interface{})
	if len(errorsList) != 1 || errorsList[0]["index"] != 1 || errorsList[0]["error"] != "item two failed" {
		t.Fatalf("unexpected errors: %#v", errorsList)
	}
	successes := result["successes"].([]map[string]interface{})
	if len(successes) != 2 {
		t.Fatalf("successes = %#v, want 2 entries", successes)
	}
}

func TestSubWorkflowLoopErrorPolicyDefaultsToStopAll(t *testing.T) {
	policy, err := parseSubWorkflowLoopErrorPolicy(nil)
	if err != nil {
		t.Fatalf("parse policy: %v", err)
	}
	if policy != SubWorkflowStopAll {
		t.Fatalf("default policy = %q, want %q", policy, SubWorkflowStopAll)
	}
}
