package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"goflow/internal/application"
	"goflow/internal/engine"
	"goflow/internal/nodes"
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
	handler := NewExecutionHandler(execStore, nil, nil)
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
	handler := NewExecutionHandler(execStore, nil, nil)
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

type replayTestAction struct{}

func (a *replayTestAction) Execute(ctx *nodes.ExecutionContext, node *nodes.Node) (interface{}, error) {
	input, _ := ctx.GetOutput("$trigger")
	return map[string]interface{}{"input": input}, nil
}

func (a *replayTestAction) Validate(node *nodes.Node) error { return nil }

func (a *replayTestAction) GetDefinition() nodes.NodeDefinition {
	return nodes.NodeDefinition{Type: "replayTestAction"}
}

type blockingReplayAction struct {
	release <-chan struct{}
}

func (a *blockingReplayAction) Execute(ctx *nodes.ExecutionContext, node *nodes.Node) (interface{}, error) {
	select {
	case <-a.release:
		return map[string]interface{}{"released": true}, nil
	case <-ctx.Context.Done():
		return nil, ctx.Context.Err()
	}
}

func (a *blockingReplayAction) Validate(node *nodes.Node) error { return nil }

func (a *blockingReplayAction) GetDefinition() nodes.NodeDefinition {
	return nodes.NodeDefinition{Type: "blockingReplayAction"}
}

type replayAPIFixture struct {
	router     *chi.Mux
	wfStore    *storage.WorkflowStore
	execStore  *storage.ExecutionStore
	tokenStore *storage.AccessTokenStore
	service    *application.TriggerService
}

func TestReplayExecutionUsesStoredInputAndAuthenticatedContext(t *testing.T) {
	fixture := newReplayAPIFixture(t, &replayTestAction{}, "replayTestAction", 0)
	workflow := fixture.createWorkflow(t, "wf-replay", true, "replayTestAction", nil)
	original := fixture.createExecution(t, workflow.ID, `{"source":"original","count":2}`, "old-idempotency-key")
	token, rawToken, err := fixture.tokenStore.Create("runner", []string{"workflow:run"}, []string{workflow.ID})
	if err != nil {
		t.Fatalf("create token: %v", err)
	}

	rec := fixture.replay(t, original.ID, rawToken, "req-replay-1")
	if rec.Code != http.StatusAccepted {
		t.Fatalf("expected 202, got %d: %s", rec.Code, rec.Body.String())
	}
	replayedID := executionIDFromReplayResponse(t, rec)
	if replayedID == original.ID {
		t.Fatalf("replay reused original execution id")
	}

	replayed := fixture.waitForExecution(t, replayedID)
	if replayed.InputJSON != `{"count":2,"source":"original"}` && replayed.InputJSON != `{"source":"original","count":2}` {
		t.Fatalf("replay did not preserve original input, got %s", replayed.InputJSON)
	}
	if replayed.IdempotencyKey != "" {
		t.Fatalf("replay copied idempotency key %q", replayed.IdempotencyKey)
	}
	if replayed.TriggerSource != string(application.SourceUI) {
		t.Fatalf("expected ui source, got %q", replayed.TriggerSource)
	}
	wantPrincipal := "token:" + token.ID + ":runner"
	if replayed.TriggerPrincipal != wantPrincipal {
		t.Fatalf("expected principal %q, got %q", wantPrincipal, replayed.TriggerPrincipal)
	}
	if replayed.RequestID != "req-replay-1" {
		t.Fatalf("expected request id to be stored, got %q", replayed.RequestID)
	}
}

func TestReplayExecutionEnforcesWorkflowAllowlist(t *testing.T) {
	fixture := newReplayAPIFixture(t, &replayTestAction{}, "replayTestAction", 0)
	workflow := fixture.createWorkflow(t, "wf-denied-replay", true, "replayTestAction", nil)
	original := fixture.createExecution(t, workflow.ID, `{"source":"denied"}`, "")
	_, rawToken, err := fixture.tokenStore.Create("runner", []string{"workflow:run"}, []string{"wf-other"})
	if err != nil {
		t.Fatalf("create token: %v", err)
	}

	rec := fixture.replay(t, original.ID, rawToken, "")
	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestReplayExecutionReturnsTriggerServiceStatusCodes(t *testing.T) {
	tests := []struct {
		name       string
		active     bool
		schema     string
		inputJSON  string
		wantStatus int
	}{
		{
			name:       "inactive",
			active:     false,
			inputJSON:  `{"source":"inactive"}`,
			wantStatus: http.StatusConflict,
		},
		{
			name:       "invalid-input",
			active:     true,
			schema:     `{"type":"object","required":["source"],"properties":{"source":{"type":"string"}}}`,
			inputJSON:  `{"other":"missing"}`,
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "stored-invalid-json",
			active:     true,
			inputJSON:  `{bad json`,
			wantStatus: http.StatusConflict,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			fixture := newReplayAPIFixture(t, &replayTestAction{}, "replayTestAction", 0)
			workflow := fixture.createWorkflow(t, "wf-"+tc.name, tc.active, "replayTestAction", map[string]string{
				"input_schema_json": tc.schema,
			})
			original := fixture.createExecution(t, workflow.ID, tc.inputJSON, "")
			rec := fixture.replay(t, original.ID, "admin-key", "")
			if rec.Code != tc.wantStatus {
				t.Fatalf("expected %d, got %d: %s", tc.wantStatus, rec.Code, rec.Body.String())
			}
		})
	}
}

func TestReplayExecutionReturnsConcurrencyLimit(t *testing.T) {
	release := make(chan struct{})
	defer close(release)
	fixture := newReplayAPIFixture(t, &blockingReplayAction{release: release}, "blockingReplayAction", 0)
	workflow := fixture.createWorkflow(t, "wf-replay-concurrency", true, "blockingReplayAction", map[string]string{
		"concurrency_policy": "reject",
	})
	workflow.MaxConcurrentRuns = 1
	if err := fixture.wfStore.Update(workflow); err != nil {
		t.Fatalf("update workflow: %v", err)
	}
	original := fixture.createExecution(t, workflow.ID, `{"source":"concurrency"}`, "")

	if _, err := fixture.service.Trigger(context.Background(), application.TriggerRequest{
		WorkflowID: workflow.ID,
		Input:      map[string]interface{}{"source": "occupy-slot"},
		Mode:       application.ModeAsync,
		Source:     application.SourceUI,
	}); err != nil {
		t.Fatalf("start occupying execution: %v", err)
	}

	rec := fixture.replay(t, original.ID, "admin-key", "")
	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("expected 429, got %d: %s", rec.Code, rec.Body.String())
	}
}

func newReplayAPIFixture(t *testing.T, executor nodes.NodeExecutor, nodeType string, maxConcurrent int) replayAPIFixture {
	t.Helper()
	db, err := storage.NewDB(filepath.Join(t.TempDir(), "goflow.db"))
	if err != nil {
		t.Fatalf("new db: %v", err)
	}
	t.Cleanup(db.Close)

	registry := nodes.NewPluginRegistry()
	if err := registry.Register(executor); err != nil {
		t.Fatalf("register executor: %v", err)
	}
	wfStore := storage.NewWorkflowStore(db)
	execStore := storage.NewExecutionStore(db)
	tokenStore := storage.NewAccessTokenStore(db)
	eng := engine.NewEngine(registry, execStore, storage.NewCredentialStore(db, nil), engine.NewEventBus(), wfStore, maxConcurrent)
	service := application.NewTriggerService(wfStore, eng)
	handler := NewExecutionHandler(execStore, eng, service)
	router := chi.NewRouter()
	router.Route("/api/v1", func(r chi.Router) {
		r.Use(authMiddleware("admin-key", tokenStore, nil, false))
		r.Post("/executions/{id}/replay", handler.ReplayExecution)
	})
	return replayAPIFixture{
		router:     router,
		wfStore:    wfStore,
		execStore:  execStore,
		tokenStore: tokenStore,
		service:    service,
	}
}

func (f replayAPIFixture) createWorkflow(t *testing.T, id string, active bool, nodeType string, options map[string]string) *storage.Workflow {
	t.Helper()
	nodeList := []nodes.Node{{ID: "action", Type: nodes.NodeType(nodeType), Name: "Action", Params: map[string]interface{}{}}}
	nodesJSON, _ := json.Marshal(nodeList)
	workflow := &storage.Workflow{
		ID:                id,
		Name:              id,
		IsActive:          active,
		NodesJSON:         string(nodesJSON),
		EdgesJSON:         "[]",
		InputSchemaJSON:   "{}",
		ConcurrencyPolicy: "global",
	}
	if options != nil {
		if options["input_schema_json"] != "" {
			workflow.InputSchemaJSON = options["input_schema_json"]
		}
		if options["concurrency_policy"] != "" {
			workflow.ConcurrencyPolicy = options["concurrency_policy"]
		}
	}
	if err := f.wfStore.Create(workflow); err != nil {
		t.Fatalf("create workflow: %v", err)
	}
	return workflow
}

func (f replayAPIFixture) createExecution(t *testing.T, workflowID string, inputJSON string, idempotencyKey string) *storage.Execution {
	t.Helper()
	exec := &storage.Execution{
		ID:             "exec-" + workflowID,
		WorkflowID:     workflowID,
		Status:         "FAILED",
		LogsJSON:       "[]",
		InputJSON:      inputJSON,
		IdempotencyKey: idempotencyKey,
	}
	if err := f.execStore.Create(exec); err != nil {
		t.Fatalf("create execution: %v", err)
	}
	if err := f.execStore.UpdateStatusWithError(exec.ID, "FAILED", 1, "[]", "original failure"); err != nil {
		t.Fatalf("mark original failed: %v", err)
	}
	return exec
}

func (f replayAPIFixture) replay(t *testing.T, executionID string, token string, requestID string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/executions/"+executionID+"/replay", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	if requestID != "" {
		req.Header.Set("X-Request-ID", requestID)
	}
	rec := httptest.NewRecorder()
	f.router.ServeHTTP(rec, req)
	return rec
}

func (f replayAPIFixture) waitForExecution(t *testing.T, executionID string) *storage.Execution {
	t.Helper()
	var exec *storage.Execution
	var err error
	for i := 0; i < 20; i++ {
		exec, err = f.execStore.GetByID(executionID)
		if err == nil && exec.Status != "RUNNING" && exec.Status != "QUEUED" {
			return exec
		}
		time.Sleep(50 * time.Millisecond)
	}
	if err != nil {
		t.Fatalf("get replayed execution: %v", err)
	}
	return exec
}

func executionIDFromReplayResponse(t *testing.T, rec *httptest.ResponseRecorder) string {
	t.Helper()
	var body struct {
		ExecutionID string `json:"execution_id"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode replay response: %v", err)
	}
	if body.ExecutionID == "" {
		t.Fatalf("replay response missing execution_id: %s", rec.Body.String())
	}
	return body.ExecutionID
}
