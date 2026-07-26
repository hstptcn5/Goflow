package api

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"goflow/internal/application"
	"goflow/internal/engine"
	"goflow/internal/nodes"
	"goflow/internal/storage"

	"github.com/go-chi/chi/v5"
)

type apiSecurityAction struct{}

func (a *apiSecurityAction) Execute(ctx *nodes.ExecutionContext, node *nodes.Node) (interface{}, error) {
	return map[string]interface{}{"ok": true}, nil
}

func (a *apiSecurityAction) Validate(node *nodes.Node) error { return nil }

func (a *apiSecurityAction) GetDefinition() nodes.NodeDefinition {
	return nodes.NodeDefinition{Type: "apiSecurityAction"}
}

func TestWebhookDoesNotPersistSensitiveHeaders(t *testing.T) {
	db, err := storage.NewDB(filepath.Join(t.TempDir(), "goflow.db"))
	if err != nil {
		t.Fatalf("new db: %v", err)
	}
	t.Cleanup(db.Close)

	registry := nodes.NewPluginRegistry()
	if err := registry.Register(&apiSecurityAction{}); err != nil {
		t.Fatalf("register action: %v", err)
	}
	wfStore := storage.NewWorkflowStore(db)
	execStore := storage.NewExecutionStore(db)
	eng := engine.NewEngine(registry, execStore, storage.NewCredentialStore(db, nil), engine.NewEventBus(), wfStore)
	handler := NewWorkflowHandler(wfStore, application.NewTriggerService(wfStore, eng), 60)

	nodeList := []nodes.Node{{ID: "a", Type: "apiSecurityAction", Name: "Action", Params: map[string]interface{}{}}}
	nodesJSON, _ := json.Marshal(nodeList)
	workflow := &storage.Workflow{
		ID:        "wf-webhook",
		Name:      "Webhook Security",
		IsActive:  true,
		NodesJSON: string(nodesJSON),
		EdgesJSON: "[]",
		ExposeCLI: true,
	}
	if err := wfStore.Create(workflow); err != nil {
		t.Fatalf("create workflow: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/webhook/wf-webhook?async=true", bytes.NewBufferString(`{"event":"test"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer secret-token")
	req.Header.Set("Cookie", "session=secret-cookie")
	req.Header.Set("X-Goflow-Webhook-Secret", "webhook-secret")
	req.Header.Set("X-API-Key", "api-secret")
	routeCtx := chi.NewRouteContext()
	routeCtx.URLParams.Add("workflowId", workflow.ID)
	req = req.WithContext(contextWithRoute(req, routeCtx))
	rec := httptest.NewRecorder()

	handler.TriggerWebhook(rec, req)
	if rec.Code != http.StatusAccepted {
		t.Fatalf("expected 202, got %d: %s", rec.Code, rec.Body.String())
	}

	var accepted map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &accepted); err != nil {
		t.Fatalf("decode accepted response: %v", err)
	}
	execID, _ := accepted["execution_id"].(string)
	execRecord, err := execStore.GetByID(execID)
	if err != nil {
		t.Fatalf("get execution: %v", err)
	}
	lowerInput := strings.ToLower(execRecord.InputJSON)
	for _, secret := range []string{"secret-token", "secret-cookie", "webhook-secret", "api-secret"} {
		if strings.Contains(lowerInput, secret) {
			t.Fatalf("execution input leaked sensitive header value %q: %s", secret, execRecord.InputJSON)
		}
	}
	for _, header := range []string{"authorization", "cookie", "x-goflow-webhook-secret", "x-api-key"} {
		if strings.Contains(lowerInput, header) {
			t.Fatalf("execution input leaked sensitive header name %q: %s", header, execRecord.InputJSON)
		}
	}
}

func contextWithRoute(req *http.Request, routeCtx *chi.Context) context.Context {
	return context.WithValue(req.Context(), chi.RouteCtxKey, routeCtx)
}
