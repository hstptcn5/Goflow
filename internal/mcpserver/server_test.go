package mcpserver

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"goflow/internal/client"
)

func TestListWorkflowsFiltersToActiveMCPExposed(t *testing.T) {
	server := newMCPTestServer(t, []client.Workflow{
		{ID: "active-exposed", Name: "Active Exposed", IsActive: true, ExposeMCP: true},
		{ID: "inactive-exposed", Name: "Inactive Exposed", IsActive: false, ExposeMCP: true},
		{ID: "active-hidden", Name: "Active Hidden", IsActive: true, ExposeMCP: false},
	})

	_, output, err := server.listWorkflows(context.Background(), nil, listWorkflowsInput{})
	if err != nil {
		t.Fatalf("list workflows: %v", err)
	}
	if output.Count != 1 || output.Workflows[0].ID != "active-exposed" {
		t.Fatalf("unexpected filtered workflows: %#v", output)
	}
}

func TestResolveAllowedWorkflowRejectsHiddenWorkflow(t *testing.T) {
	server := newMCPTestServer(t, []client.Workflow{
		{ID: "hidden", Name: "Hidden", IsActive: true, ExposeMCP: false},
	})

	if _, err := server.resolveAllowedWorkflow("hidden"); err == nil {
		t.Fatalf("expected hidden workflow to be rejected")
	}
}

func TestResolveAllowedWorkflowRejectsApprovalWorkflow(t *testing.T) {
	server := newMCPTestServer(t, []client.Workflow{
		{ID: "approval", Name: "Approval", IsActive: true, ExposeMCP: true, RequiresApproval: true},
	})

	if _, err := server.resolveAllowedWorkflow("approval"); err == nil {
		t.Fatalf("expected approval workflow to be rejected")
	}
}

func newMCPTestServer(t *testing.T, workflows []client.Workflow) *Server {
	t.Helper()
	byID := make(map[string]client.Workflow, len(workflows))
	for _, workflow := range workflows {
		byID[workflow.ID] = workflow
	}
	httpServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/v1/workflows":
			_ = json.NewEncoder(w).Encode(workflows)
			return
		default:
			const prefix = "/api/v1/workflows/"
			if len(r.URL.Path) > len(prefix) && r.URL.Path[:len(prefix)] == prefix {
				id := r.URL.Path[len(prefix):]
				workflow, ok := byID[id]
				if !ok {
					http.NotFound(w, r)
					return
				}
				_ = json.NewEncoder(w).Encode(workflow)
				return
			}
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(httpServer.Close)
	return New(Options{BaseURL: httpServer.URL})
}
