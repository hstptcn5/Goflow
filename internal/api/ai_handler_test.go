package api

import (
	"strings"
	"testing"

	"goflow/internal/nodes"
)

func newTestAIHandler(t *testing.T) *AIHandler {
	t.Helper()
	registry := nodes.NewPluginRegistry()
	if err := registry.Register(nodes.NewWebhookTriggerExecutor()); err != nil {
		t.Fatal(err)
	}
	if err := registry.Register(nodes.NewHTTPRequestExecutor()); err != nil {
		t.Fatal(err)
	}
	return NewAIHandler(nil, registry)
}

func TestValidateWorkflowDraftAcceptsValidWorkflow(t *testing.T) {
	handler := newTestAIHandler(t)
	issues := handler.validateWorkflowDraft(workflowDraft{
		Name: "Valid workflow",
		Nodes: []nodes.Node{
			{ID: "webhook_1", Type: nodes.TypeWebhookTrigger, Name: "Webhook", Params: map[string]interface{}{}},
			{ID: "http_1", Type: nodes.TypeHTTPRequest, Name: "HTTP", Params: map[string]interface{}{
				"method": "GET",
				"url":    "https://example.com",
			}},
		},
		Edges: []nodes.Edge{
			{ID: "edge_1", Source: "webhook_1", Target: "http_1"},
		},
	})
	if len(issues) != 0 {
		t.Fatalf("expected valid workflow, got issues: %v", issues)
	}
}

func TestValidateWorkflowDraftReportsStructuralIssues(t *testing.T) {
	handler := newTestAIHandler(t)
	issues := handler.validateWorkflowDraft(workflowDraft{
		Name: "Broken workflow",
		Nodes: []nodes.Node{
			{ID: "http_1", Type: nodes.TypeHTTPRequest, Name: "HTTP", Params: map[string]interface{}{
				"url":        "",
				"unexpected": "value",
			}},
			{ID: "unknown_1", Type: nodes.NodeType("missingNode"), Name: "Unknown", Params: map[string]interface{}{}},
		},
		Edges: []nodes.Edge{
			{ID: "edge_1", Source: "http_1", Target: "missing_target"},
		},
	})

	expected := []string{
		`node "http_1" has unsupported parameter "unexpected"`,
		`node "http_1" is missing required parameter "url"`,
		`node "http_1" validation failed`,
		`node "unknown_1" uses unknown type "missingNode"`,
		`edge "edge_1" references unknown target node "missing_target"`,
	}
	for _, want := range expected {
		if !containsIssue(issues, want) {
			t.Fatalf("expected issue containing %q, got %v", want, issues)
		}
	}
}

func containsIssue(issues []string, want string) bool {
	for _, issue := range issues {
		if strings.Contains(issue, want) {
			return true
		}
	}
	return false
}
