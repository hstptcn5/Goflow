package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"goflow/internal/storage"

	"github.com/go-chi/chi/v5"
)

func TestExecutionInspectorDTORedactsSecrets(t *testing.T) {
	db, err := storage.NewDB(filepath.Join(t.TempDir(), "goflow.db"))
	if err != nil {
		t.Fatalf("new db: %v", err)
	}
	t.Cleanup(db.Close)
	execStore := storage.NewExecutionStore(db)
	wfStore := storage.NewWorkflowStore(db)
	handler := NewExecutionHandler(execStore, nil)
	if err := wfStore.Create(&storage.Workflow{ID: "wf-redact", Name: "Redact", NodesJSON: "[]", EdgesJSON: "[]"}); err != nil {
		t.Fatalf("create workflow: %v", err)
	}

	logs := []map[string]interface{}{
		{
			"node_id": "node_1",
			"status":  "FAILED",
			"output": map[string]interface{}{
				"headers": map[string]interface{}{
					"Authorization": []interface{}{"Bearer raw-token"},
					"Set-Cookie":    []interface{}{"session=secret-cookie"},
				},
				"nested": map[string]interface{}{
					"access_token": "secret-token",
					"items": []interface{}{
						map[string]interface{}{"api_key": "sk-test-secret-1234567890"},
					},
				},
			},
			"error": "Authorization: Bearer raw-token access_token=secret-token ghp_abcdefghijklmnopqrstuvwxyz",
		},
	}
	logsJSON, _ := json.Marshal(logs)
	inputJSON := `{"password":"plain-password","headers":{"cookie":"session=secret-cookie"},"private_key":"-----BEGIN PRIVATE KEY-----\nabc\n-----END PRIVATE KEY-----"}`
	errMessage := "failed with xoxb-123456789012345678 and github_pat_abcdefghijklmnopqrstuvwxyz"
	exec := &storage.Execution{
		ID:            "exec-redact",
		WorkflowID:    "wf-redact",
		Status:        "FAILED",
		DurationMs:    10,
		LogsJSON:      string(logsJSON),
		InputJSON:     inputJSON,
		ErrorMessage:  errMessage,
		StartedAt:     time.Now(),
		TriggerSource: "ui",
	}
	if err := execStore.Create(exec); err != nil {
		t.Fatalf("create execution: %v", err)
	}
	if err := execStore.UpdateStatusWithError(exec.ID, exec.Status, exec.DurationMs, exec.LogsJSON, exec.ErrorMessage); err != nil {
		t.Fatalf("update execution: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/executions/exec-redact", nil)
	routeCtx := chi.NewRouteContext()
	routeCtx.URLParams.Add("id", exec.ID)
	req = req.WithContext(contextWithRoute(req, routeCtx))
	rec := httptest.NewRecorder()

	handler.GetExecution(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	for _, secret := range []string{
		"plain-password",
		"raw-token",
		"secret-token",
		"secret-cookie",
		"sk-test-secret",
		"ghp_abcdefghijklmnopqrstuvwxyz",
		"github_pat_abcdefghijklmnopqrstuvwxyz",
		"xoxb-123456789012345678",
		"BEGIN PRIVATE KEY",
	} {
		if strings.Contains(body, secret) {
			t.Fatalf("execution inspector response leaked %q: %s", secret, body)
		}
	}
	if !strings.Contains(body, `"node_logs"`) || !strings.Contains(body, `[REDACTED]`) {
		t.Fatalf("expected structured redacted node logs, got %s", body)
	}
}

func TestExecutionInspectorScopedTokenWorkflowAllowlist(t *testing.T) {
	db := newAuthTestDB(t)
	execStore := storage.NewExecutionStore(db)
	wfStore := storage.NewWorkflowStore(db)
	tokenStore := storage.NewAccessTokenStore(db)
	_, rawToken, err := tokenStore.Create("reader", []string{"execution:read"}, []string{"wf-allowed"})
	if err != nil {
		t.Fatalf("create token: %v", err)
	}
	for _, wf := range []*storage.Workflow{
		{ID: "wf-allowed", Name: "Allowed", NodesJSON: "[]", EdgesJSON: "[]"},
		{ID: "wf-denied", Name: "Denied", NodesJSON: "[]", EdgesJSON: "[]"},
	} {
		if err := wfStore.Create(wf); err != nil {
			t.Fatalf("create workflow %s: %v", wf.ID, err)
		}
	}
	for _, exec := range []*storage.Execution{
		{ID: "exec-allowed", WorkflowID: "wf-allowed", Status: "SUCCESS", LogsJSON: "[]"},
		{ID: "exec-denied", WorkflowID: "wf-denied", Status: "SUCCESS", LogsJSON: "[]"},
	} {
		if err := execStore.Create(exec); err != nil {
			t.Fatalf("create execution %s: %v", exec.ID, err)
		}
	}

	router := chi.NewRouter()
	handler := NewExecutionHandler(execStore, nil)
	router.Route("/api/v1", func(r chi.Router) {
		r.Use(authMiddleware("admin-key", tokenStore, nil, false))
		r.Get("/executions/{id}", handler.GetExecution)
	})

	allowedReq := httptest.NewRequest(http.MethodGet, "/api/v1/executions/exec-allowed", nil)
	allowedReq.Header.Set("Authorization", "Bearer "+rawToken)
	allowedRec := httptest.NewRecorder()
	router.ServeHTTP(allowedRec, allowedReq)
	if allowedRec.Code != http.StatusOK {
		t.Fatalf("expected allowlisted execution to pass, got %d: %s", allowedRec.Code, allowedRec.Body.String())
	}

	deniedReq := httptest.NewRequest(http.MethodGet, "/api/v1/executions/exec-denied", nil)
	deniedReq.Header.Set("Authorization", "Bearer "+rawToken)
	deniedRec := httptest.NewRecorder()
	router.ServeHTTP(deniedRec, deniedReq)
	if deniedRec.Code != http.StatusForbidden {
		t.Fatalf("expected denied workflow execution to be forbidden, got %d: %s", deniedRec.Code, deniedRec.Body.String())
	}
}
