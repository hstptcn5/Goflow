package api

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"goflow/internal/storage"

	"github.com/go-chi/chi/v5"
)

func TestListWorkflowsWithScopedTokenReturnsAllowlistedSummaries(t *testing.T) {
	store := newTestWorkflowStore(t)
	handler := NewWorkflowHandler(store, nil, 60)
	allowed := &storage.Workflow{
		ID:          "wf-allowed",
		Name:        "Allowed",
		Description: "Visible summary",
		IsActive:    true,
		NodesJSON:   `[{"id":"secret","type":"http_request","params":{"credential_id":"cred-1"}}]`,
		EdgesJSON:   `[{"source":"secret","target":"other"}]`,
		Slug:        "allowed",
		ExposeCLI:   true,
		ExposeMCP:   true,
		RiskLevel:   "low",
	}
	if err := store.Create(allowed); err != nil {
		t.Fatalf("create allowed workflow: %v", err)
	}
	blocked := &storage.Workflow{
		ID:        "wf-blocked",
		Name:      "Blocked",
		IsActive:  true,
		NodesJSON: "[]",
		EdgesJSON: "[]",
		ExposeCLI: true,
	}
	if err := store.Create(blocked); err != nil {
		t.Fatalf("create blocked workflow: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/workflows", nil)
	req = req.WithContext(context.WithValue(req.Context(), authContextKey{}, AuthInfo{
		Subject: "token:test",
		Token:   &storage.AccessToken{Scopes: []string{"workflow:list"}, AllowedWorkflows: []string{"wf-allowed"}},
	}))
	rec := httptest.NewRecorder()

	handler.ListWorkflows(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var got []map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(got) != 1 || got[0]["id"] != "wf-allowed" {
		t.Fatalf("expected only allowlisted workflow, got %#v", got)
	}
	if _, leaked := got[0]["nodes_json"]; leaked {
		t.Fatalf("workflow:list leaked nodes_json: %#v", got[0])
	}
	if _, leaked := got[0]["edges_json"]; leaked {
		t.Fatalf("workflow:list leaked edges_json: %#v", got[0])
	}
}

func TestUpdateWorkflowPreservesInterfaceMetadata(t *testing.T) {
	store := newTestWorkflowStore(t)
	handler := NewWorkflowHandler(store, nil, 60)
	workflow := createTestWorkflow(t, store)
	workflow.ExposeMCP = true
	workflow.MCPToolName = "release_check"
	workflow.MCPDescription = "Checks release status"
	workflow.Slug = "release-check"
	if err := store.Update(workflow); err != nil {
		t.Fatalf("update interface metadata: %v", err)
	}

	body := bytes.NewBufferString(`{
		"name":"Renamed",
		"description":"Updated",
		"is_active":true,
		"nodes_json":"[]",
		"edges_json":"[]"
	}`)
	req := requestWithWorkflowID(http.MethodPut, "/api/v1/workflows/"+workflow.ID, workflow.ID, body)
	rec := httptest.NewRecorder()

	handler.UpdateWorkflow(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	updated, err := store.GetByID(workflow.ID)
	if err != nil {
		t.Fatalf("get updated workflow: %v", err)
	}
	if !updated.ExposeMCP || updated.MCPToolName != "release_check" || updated.Slug != "release-check" {
		t.Fatalf("interface metadata was not preserved: %#v", updated)
	}
}

func TestUpdateWorkflowInterfaceValidatesJSONSchema(t *testing.T) {
	store := newTestWorkflowStore(t)
	handler := NewWorkflowHandler(store, nil, 60)
	workflow := createTestWorkflow(t, store)

	body := bytes.NewBufferString(`{"input_schema_json":"[]","output_schema_json":"{}","risk_level":"medium","concurrency_policy":"global"}`)
	req := requestWithWorkflowID(http.MethodPut, "/api/v1/workflows/"+workflow.ID+"/interface", workflow.ID, body)
	rec := httptest.NewRecorder()

	handler.UpdateWorkflowInterface(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
}

func TestUpdateWorkflowInterface(t *testing.T) {
	store := newTestWorkflowStore(t)
	handler := NewWorkflowHandler(store, nil, 60)
	workflow := createTestWorkflow(t, store)

	payload := workflowInterface{
		Slug:              "daily-report",
		InputSchemaJSON:   `{"type":"object"}`,
		OutputSchemaJSON:  `{"type":"object"}`,
		ExposeCLI:         true,
		ExposeMCP:         true,
		MCPToolName:       "daily_report",
		MCPDescription:    "Runs the daily report workflow",
		RiskLevel:         "low",
		MaxConcurrentRuns: 1,
		ConcurrencyPolicy: "reject",
	}
	data, _ := json.Marshal(payload)
	req := requestWithWorkflowID(http.MethodPut, "/api/v1/workflows/"+workflow.ID+"/interface", workflow.ID, bytes.NewReader(data))
	rec := httptest.NewRecorder()

	handler.UpdateWorkflowInterface(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	updated, err := store.GetByID(workflow.ID)
	if err != nil {
		t.Fatalf("get updated workflow: %v", err)
	}
	if !updated.ExposeMCP || updated.MCPToolName != "daily_report" || updated.RiskLevel != "low" || updated.ConcurrencyPolicy != "reject" {
		t.Fatalf("unexpected interface update: %#v", updated)
	}
}

func TestUpdateWorkflowInterfaceCanDisableCLIExposure(t *testing.T) {
	store := newTestWorkflowStore(t)
	handler := NewWorkflowHandler(store, nil, 60)
	workflow := createTestWorkflow(t, store)

	payload := workflowInterface{
		ExposeCLI:         false,
		ExposeMCP:         false,
		InputSchemaJSON:   `{}`,
		OutputSchemaJSON:  `{}`,
		RiskLevel:         "medium",
		ConcurrencyPolicy: "global",
	}
	data, _ := json.Marshal(payload)
	req := requestWithWorkflowID(http.MethodPut, "/api/v1/workflows/"+workflow.ID+"/interface", workflow.ID, bytes.NewReader(data))
	rec := httptest.NewRecorder()

	handler.UpdateWorkflowInterface(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	updated, err := store.GetByID(workflow.ID)
	if err != nil {
		t.Fatalf("get updated workflow: %v", err)
	}
	if updated.ExposeCLI {
		t.Fatalf("expected expose_cli=false to be persisted")
	}
}

func TestRequestPrincipalIgnoresSpoofedHeader(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/api/v1/workflows/wf/executions", nil)
	req.Header.Set("X-Goflow-Principal", "attacker")
	req = req.WithContext(context.WithValue(req.Context(), authContextKey{}, AuthInfo{
		Subject: "token:real:runner",
		Token:   &storage.AccessToken{Scopes: []string{"workflow:run"}},
	}))

	if got := requestPrincipal(req); got != "token:real:runner" {
		t.Fatalf("expected authenticated principal, got %q", got)
	}
}

func newTestWorkflowStore(t *testing.T) *storage.WorkflowStore {
	t.Helper()
	db, err := storage.NewDB(filepath.Join(t.TempDir(), "goflow.db"))
	if err != nil {
		t.Fatalf("new db: %v", err)
	}
	t.Cleanup(db.Close)
	return storage.NewWorkflowStore(db)
}

func createTestWorkflow(t *testing.T, store *storage.WorkflowStore) *storage.Workflow {
	t.Helper()
	workflow := &storage.Workflow{
		Name:      "Test Workflow",
		IsActive:  true,
		NodesJSON: "[]",
		EdgesJSON: "[]",
		ExposeCLI: true,
	}
	if err := store.Create(workflow); err != nil {
		t.Fatalf("create workflow: %v", err)
	}
	return workflow
}

func requestWithWorkflowID(method, target, workflowID string, body io.Reader) *http.Request {
	req := httptest.NewRequest(method, target, body)
	routeCtx := chi.NewRouteContext()
	routeCtx.URLParams.Add("id", workflowID)
	return req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, routeCtx))
}
