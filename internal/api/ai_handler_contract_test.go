package api

import (
	"testing"

	"goflow/internal/nodes"
)

func TestValidateWorkflowDraftAcceptsAdvertisedOnErrorPolicy(t *testing.T) {
	registry := nodes.NewBuiltinRegistry()
	handler := NewAIHandler(nil, registry)

	draft := workflowDraft{
		Name: "Grounded Python",
		Nodes: []nodes.Node{
			{
				ID:   "pythonCode",
				Type: nodes.TypePythonCode,
				Name: "Python Code",
				Params: map[string]interface{}{
					"code":     `output = {"category": "NORMAL"}`,
					"on_error": nodes.ErrorPolicyStopLabel,
				},
			},
		},
		Edges: []nodes.Edge{},
	}

	if issues := handler.validateWorkflowDraft(draft); len(issues) != 0 {
		t.Fatalf("advertised on_error policy should validate, got: %#v", issues)
	}
}
