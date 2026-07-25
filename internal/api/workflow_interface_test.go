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
