package api

import (
	"testing"

	"goflow/internal/nodes"
)

func TestAgentAutoTestSafetyAllowsDeterministicGraph(t *testing.T) {
	draft := workflowDraft{
		Name: "safe",
		Nodes: []nodes.Node{
			{ID: "manual", Type: nodes.TypeManualTrigger, Params: map[string]interface{}{}},
			{ID: "transform", Type: nodes.TypeJSONTransform, Params: map[string]interface{}{"json_template": `{"ok":true}`}},
			{ID: "if", Type: nodes.TypeConditionIF, Params: map[string]interface{}{}},
			{ID: "switch", Type: nodes.TypeSwitch, Params: map[string]interface{}{}},
		},
	}
	ok, reasons := agentAutoTestSafety(draft)
	if !ok || len(reasons) != 0 {
		t.Fatalf("expected deterministic graph to be auto-testable, ok=%v reasons=%v", ok, reasons)
	}
}

func TestAgentAutoTestSafetyBlocksExternalAndTrustedExecution(t *testing.T) {
	draft := workflowDraft{
		Name: "unsafe",
		Nodes: []nodes.Node{
			{ID: "http", Type: nodes.TypeHTTPRequest, Params: map[string]interface{}{"method": "GET", "url": "https://example.com"}},
			{ID: "python", Type: nodes.TypePythonCode, Params: map[string]interface{}{"code": "output = 1"}},
			{ID: "mail", Type: nodes.TypeEmailSMTP, Params: map[string]interface{}{}},
		},
	}
	ok, reasons := agentAutoTestSafety(draft)
	if ok {
		t.Fatal("expected graph with external/trusted nodes to be blocked")
	}
	if len(reasons) != 3 {
		t.Fatalf("expected one reason per blocked node, got %v", reasons)
	}
}

func TestNormalizeAgentIterations(t *testing.T) {
	if got := normalizeAgentIterations(0); got != defaultAgentIterations {
		t.Fatalf("default iterations = %d", got)
	}
	if got := normalizeAgentIterations(99); got != maxAgentIterations {
		t.Fatalf("max iterations = %d", got)
	}
	if got := normalizeAgentIterations(1); got != 1 {
		t.Fatalf("explicit iterations = %d", got)
	}
}
