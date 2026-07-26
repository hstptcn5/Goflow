package mcpserver

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"goflow/internal/client"

	"github.com/modelcontextprotocol/go-sdk/mcp"
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

func TestRunWorkflowRejectsWhenPerClientInflightLimitReached(t *testing.T) {
	server := New(Options{BaseURL: "http://127.0.0.1:1", MaxInflight: 1})
	server.runInflight <- struct{}{}

	_, _, err := server.runWorkflow(context.Background(), nil, runWorkflowInput{Workflow: "anything"})
	if err == nil {
		t.Fatalf("expected inflight limit error")
	}
	if !strings.Contains(err.Error(), "inflight limit") {
		t.Fatalf("expected inflight limit error, got %v", err)
	}
}

func TestOptionsFromEnvReadsMCPInflightLimit(t *testing.T) {
	t.Setenv("GOFLOW_MCP_MAX_INFLIGHT_PER_CLIENT", "7")

	opts := OptionsFromEnv()
	if opts.MaxInflight != 7 {
		t.Fatalf("expected max inflight 7, got %d", opts.MaxInflight)
	}
}

func TestDynamicToolNameUsesMCPToolNameSlugThenName(t *testing.T) {
	cases := []struct {
		workflow client.Workflow
		want     string
	}{
		{workflow: client.Workflow{MCPToolName: "daily_report", Slug: "ignored", Name: "Ignored"}, want: "goflow.daily_report"},
		{workflow: client.Workflow{Slug: "release-check", Name: "Ignored"}, want: "goflow.release-check"},
		{workflow: client.Workflow{Name: "Release Check!"}, want: "goflow.Release_Check"},
	}
	for _, tc := range cases {
		if got := dynamicToolName(tc.workflow); got != tc.want {
			t.Fatalf("dynamicToolName(%#v) = %q, want %q", tc.workflow, got, tc.want)
		}
	}
}

func TestDynamicWorkflowHandlerRunsWorkflow(t *testing.T) {
	var runPath string
	var runBody map[string]interface{}
	httpServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.Method != http.MethodPost || r.URL.Path != "/api/v1/workflows/wf-1/executions" {
			http.NotFound(w, r)
			return
		}
		runPath = r.URL.Path
		_ = json.NewDecoder(r.Body).Decode(&runBody)
		_ = json.NewEncoder(w).Encode(client.ExecutionAccepted{
			ExecutionID:  "exec-1",
			WorkflowID:   "wf-1",
			Status:       "RUNNING",
			Deduplicated: false,
		})
	}))
	t.Cleanup(httpServer.Close)

	server := New(Options{BaseURL: httpServer.URL})
	handler := server.dynamicWorkflowHandler(client.Workflow{ID: "wf-1", Name: "Daily", IsActive: true, ExposeMCP: true})
	args := json.RawMessage(`{"date":"2026-07-26","idempotency_key":"idem-1"}`)
	result, err := handler(context.Background(), &mcp.CallToolRequest{Params: &mcp.CallToolParamsRaw{Arguments: args}})
	if err != nil {
		t.Fatalf("dynamic workflow handler returned protocol error: %v", err)
	}
	if result.IsError {
		t.Fatalf("dynamic workflow handler returned tool error: %#v", result)
	}
	if runPath != "/api/v1/workflows/wf-1/executions" {
		t.Fatalf("unexpected run path: %s", runPath)
	}
	if runBody["idempotency_key"] != "idem-1" {
		t.Fatalf("idempotency key was not forwarded: %#v", runBody)
	}
	input, ok := runBody["input"].(map[string]interface{})
	if !ok || input["date"] != "2026-07-26" {
		t.Fatalf("workflow input was not forwarded correctly: %#v", runBody)
	}
	output, ok := result.StructuredContent.(runWorkflowOutput)
	if !ok || output.ExecutionID != "exec-1" {
		t.Fatalf("unexpected structured output: %#v", result.StructuredContent)
	}
}

func TestHTTPHandlerServesStreamableInitialize(t *testing.T) {
	restServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == "/api/v1/workflows" {
			_, _ = w.Write([]byte(`[]`))
			return
		}
		http.NotFound(w, r)
	}))
	t.Cleanup(restServer.Close)

	handler := NewHTTPHandler(HTTPOptions{BaseURL: restServer.URL, AllowedOrigins: []string{"http://127.0.0.1:8080"}})
	req := httptest.NewRequest(http.MethodPost, "/mcp", bytes.NewBufferString(`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{}}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json, text/event-stream")
	req.Header.Set("Origin", "http://127.0.0.1:8080")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	data, _ := io.ReadAll(rec.Body)
	if !strings.Contains(string(data), `"serverInfo":{"name":"goflow"`) {
		t.Fatalf("unexpected initialize response: %s", data)
	}
}

func TestHTTPHandlerRejectsDisallowedOrigin(t *testing.T) {
	handler := NewHTTPHandler(HTTPOptions{BaseURL: "http://127.0.0.1:1", AllowedOrigins: []string{"http://127.0.0.1:8080"}})
	req := httptest.NewRequest(http.MethodPost, "/mcp", bytes.NewBufferString(`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{}}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json, text/event-stream")
	req.Header.Set("Origin", "http://evil.example")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestSchemaOrEmptyObjectAddsObjectType(t *testing.T) {
	schema := schemaOrEmptyObject(`{}`)
	var decoded map[string]interface{}
	if err := json.Unmarshal(schema, &decoded); err != nil {
		t.Fatalf("schema was not valid JSON: %v", err)
	}
	if decoded["type"] != "object" {
		t.Fatalf("expected object schema, got %#v", decoded)
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
