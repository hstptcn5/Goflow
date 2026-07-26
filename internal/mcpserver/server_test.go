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
	"time"

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
	var triggerSource string
	httpServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.Method != http.MethodPost || r.URL.Path != "/api/v1/workflows/wf-1/executions" {
			http.NotFound(w, r)
			return
		}
		runPath = r.URL.Path
		triggerSource = r.Header.Get("X-Goflow-Trigger-Source")
		_ = json.NewDecoder(r.Body).Decode(&runBody)
		_ = json.NewEncoder(w).Encode(client.ExecutionAccepted{
			ExecutionID:  "exec-1",
			WorkflowID:   "wf-1",
			Status:       "RUNNING",
			Deduplicated: false,
		})
	}))
	t.Cleanup(httpServer.Close)

	server := New(Options{BaseURL: httpServer.URL, TriggerSource: "mcp_stdio"})
	handler := server.dynamicWorkflowHandler(client.Workflow{ID: "wf-1", Name: "Daily", IsActive: true, ExposeMCP: true})
	args := json.RawMessage(`{"date":"2026-07-26","idempotency_key":"business-value","_goflow":{"idempotency_key":"idem-1"}}`)
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
	if triggerSource != "mcp_stdio" {
		t.Fatalf("expected mcp_stdio trigger source header, got %q", triggerSource)
	}
	if runBody["idempotency_key"] != "idem-1" {
		t.Fatalf("idempotency key was not forwarded: %#v", runBody)
	}
	input, ok := runBody["input"].(map[string]interface{})
	if !ok || input["date"] != "2026-07-26" || input["idempotency_key"] != "business-value" {
		t.Fatalf("workflow input was not forwarded correctly: %#v", runBody)
	}
	if _, leaked := input["_goflow"]; leaked {
		t.Fatalf("_goflow control envelope leaked into workflow input: %#v", input)
	}
	output, ok := result.StructuredContent.(runWorkflowOutput)
	if !ok || output.ExecutionID != "exec-1" {
		t.Fatalf("unexpected structured output: %#v", result.StructuredContent)
	}
}

func TestMCPGetExecutionDoesNotExposeInputJSON(t *testing.T) {
	httpServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path != "/api/v1/executions/exec-1" {
			http.NotFound(w, r)
			return
		}
		_ = json.NewEncoder(w).Encode(client.Execution{
			ID:            "exec-1",
			WorkflowID:    "wf-1",
			Status:        "SUCCESS",
			LogsJSON:      `[]`,
			InputJSON:     `{"password":"secret-value"}`,
			TriggerSource: "mcp_http",
		})
	}))
	t.Cleanup(httpServer.Close)

	server := New(Options{BaseURL: httpServer.URL})
	_, output, err := server.getExecution(context.Background(), nil, executionRefInput{ExecutionID: "exec-1"})
	if err != nil {
		t.Fatalf("getExecution failed: %v", err)
	}
	data, _ := json.Marshal(output)
	if strings.Contains(string(data), "input_json") || strings.Contains(string(data), "secret-value") {
		t.Fatalf("MCP execution output leaked input_json: %s", data)
	}
	if output.Execution.Status != "SUCCESS" || output.Execution.TriggerSource != "mcp_http" {
		t.Fatalf("safe execution lost metadata: %#v", output.Execution)
	}
}

func TestMCPListExecutionsDoesNotExposeInputJSON(t *testing.T) {
	workflows := []client.Workflow{{ID: "wf-1", Name: "Daily", IsActive: true, ExposeMCP: true}}
	server := newMCPTestServerWithHandler(t, workflows, func(w http.ResponseWriter, r *http.Request) bool {
		if r.URL.Path != "/api/v1/workflows/wf-1/executions" {
			return false
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode([]client.Execution{{
			ID:         "exec-1",
			WorkflowID: "wf-1",
			Status:     "SUCCESS",
			LogsJSON:   `[]`,
			InputJSON:  `{"api_key":"secret-value"}`,
		}})
		return true
	})

	_, output, err := server.listExecutions(context.Background(), nil, workflowRefInput{Workflow: "wf-1"})
	if err != nil {
		t.Fatalf("listExecutions failed: %v", err)
	}
	data, _ := json.Marshal(output)
	if strings.Contains(string(data), "input_json") || strings.Contains(string(data), "secret-value") {
		t.Fatalf("MCP execution list leaked input_json: %s", data)
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

func TestHTTPHandlerRateLimitsByHashedBearerAcrossRequests(t *testing.T) {
	handler := NewHTTPHandler(HTTPOptions{BaseURL: "http://127.0.0.1:1", RateLimitPerMinute: 1})
	body := `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{}}`

	for i, want := range []int{http.StatusOK, http.StatusTooManyRequests} {
		req := httptest.NewRequest(http.MethodPost, "/mcp", bytes.NewBufferString(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Accept", "application/json, text/event-stream")
		req.Header.Set("Authorization", "Bearer secret-token")
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)
		if rec.Code != want {
			t.Fatalf("request %d expected status %d, got %d: %s", i+1, want, rec.Code, rec.Body.String())
		}
	}
	if principal := requestPrincipalKey(httptest.NewRequest(http.MethodPost, "/mcp", nil)); strings.Contains(principal, "secret-token") {
		t.Fatalf("principal key leaked raw token: %q", principal)
	}
	req := httptest.NewRequest(http.MethodPost, "/mcp", nil)
	req.Header.Set("Authorization", "Bearer secret-token")
	if principal := requestPrincipalKey(req); strings.Contains(principal, "secret-token") || !strings.HasPrefix(principal, "bearer_sha256:") {
		t.Fatalf("principal key should hash bearer token, got %q", principal)
	}
}

func TestHTTPClientLimitersPersistAcrossRequestsForSameToken(t *testing.T) {
	limiters := newHTTPClientLimiters(1)
	firstReq := httptest.NewRequest(http.MethodPost, "/mcp", nil)
	firstReq.Header.Set("Authorization", "Bearer token-1")
	secondReq := httptest.NewRequest(http.MethodPost, "/mcp", nil)
	secondReq.Header.Set("Authorization", "Bearer token-1")
	otherReq := httptest.NewRequest(http.MethodPost, "/mcp", nil)
	otherReq.Header.Set("Authorization", "Bearer token-2")

	first := limiters.limiterFor(firstReq)
	second := limiters.limiterFor(secondReq)
	other := limiters.limiterFor(otherReq)
	if first != second {
		t.Fatalf("expected same limiter for same token")
	}
	if first == other {
		t.Fatalf("expected different limiter for different token")
	}
	first <- struct{}{}
	select {
	case second <- struct{}{}:
		t.Fatalf("expected shared limiter to be full")
	default:
	}
}

func TestHTTPHandlerWorksWithSDKStreamableClient(t *testing.T) {
	restServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/v1/workflows":
			_ = json.NewEncoder(w).Encode([]client.Workflow{
				{ID: "wf-1", Name: "Daily", IsActive: true, ExposeMCP: true, RiskLevel: "low"},
				{ID: "wf-approval", Name: "Approval", IsActive: true, ExposeMCP: true, RiskLevel: "high"},
			})
			return
		case "/api/v1/workflows/wf-1/interface":
			_ = json.NewEncoder(w).Encode(client.Workflow{
				ID:              "wf-1",
				Slug:            "daily",
				InputSchemaJSON: `{"type":"object","properties":{"date":{"type":"string"}}}`,
				ExposeMCP:       true,
				MCPToolName:     "daily_report",
				MCPDescription:  "Prepare daily report",
				RiskLevel:       "low",
			})
			return
		case "/api/v1/workflows/wf-approval/interface":
			_ = json.NewEncoder(w).Encode(client.Workflow{
				ID:               "wf-approval",
				Slug:             "approval",
				InputSchemaJSON:  `{"type":"object"}`,
				ExposeMCP:        true,
				RequiresApproval: true,
				MCPToolName:      "approval_report",
			})
			return
		case "/api/v1/workflows/wf-1":
			_ = json.NewEncoder(w).Encode(client.Workflow{ID: "wf-1", Name: "Daily", IsActive: true, ExposeMCP: true, Slug: "daily"})
			return
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(restServer.Close)

	mcpHTTP := httptest.NewServer(NewHTTPHandler(HTTPOptions{BaseURL: restServer.URL}))
	t.Cleanup(mcpHTTP.Close)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	session, err := mcp.NewClient(&mcp.Implementation{
		Name:    "goflow-http-client-compat-test",
		Version: "0.0.0",
	}, nil).Connect(ctx, &mcp.StreamableClientTransport{Endpoint: mcpHTTP.URL}, nil)
	if err != nil {
		t.Fatalf("connect streamable client: %v", err)
	}
	defer session.Close()

	tools, err := session.ListTools(ctx, nil)
	if err != nil {
		t.Fatalf("ListTools failed: %v", err)
	}
	names := map[string]bool{}
	for _, tool := range tools.Tools {
		names[tool.Name] = true
	}
	for _, want := range []string{"goflow_list_workflows", "goflow_run_workflow", "goflow.daily_report"} {
		if !names[want] {
			t.Fatalf("expected tool %s, got %#v", want, names)
		}
	}
	if names["goflow.approval_report"] {
		t.Fatalf("requires_approval workflow was exposed as dynamic tool: %#v", names)
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
	return newMCPTestServerWithHandler(t, workflows, nil)
}

func newMCPTestServerWithHandler(t *testing.T, workflows []client.Workflow, extra func(http.ResponseWriter, *http.Request) bool) *Server {
	t.Helper()
	byID := make(map[string]client.Workflow, len(workflows))
	for _, workflow := range workflows {
		byID[workflow.ID] = workflow
	}
	httpServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if extra != nil && extra(w, r) {
			return
		}
		switch r.URL.Path {
		case "/api/v1/workflows":
			_ = json.NewEncoder(w).Encode(workflows)
			return
		default:
			const prefix = "/api/v1/workflows/"
			if len(r.URL.Path) > len(prefix) && r.URL.Path[:len(prefix)] == prefix {
				id := strings.TrimSuffix(r.URL.Path[len(prefix):], "/interface")
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
