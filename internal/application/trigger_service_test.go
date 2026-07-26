package application

import (
	"context"
	"encoding/json"
	"errors"
	"path/filepath"
	"testing"

	"goflow/internal/engine"
	"goflow/internal/nodes"
	"goflow/internal/storage"
)

type testAction struct{}

func (a *testAction) Execute(ctx *nodes.ExecutionContext, node *nodes.Node) (interface{}, error) {
	return map[string]interface{}{"ok": true}, nil
}

func (a *testAction) Validate(node *nodes.Node) error { return nil }

func (a *testAction) GetDefinition() nodes.NodeDefinition {
	return nodes.NodeDefinition{Type: "testAction"}
}

func TestTriggerServiceSyncStoresSourceMetadata(t *testing.T) {
	service, execStore, wf := newTestTriggerService(t)

	result, err := service.Trigger(context.Background(), TriggerRequest{
		WorkflowID: wf.ID,
		Input:      map[string]interface{}{"source": "test"},
		Mode:       ModeSync,
		Source:     SourceCLI,
		Principal:  "cli-user",
		RequestID:  "req-1",
	})
	if err != nil {
		t.Fatalf("Trigger failed: %v", err)
	}
	if result.Execution.Status != "SUCCESS" {
		t.Fatalf("expected SUCCESS, got %s", result.Execution.Status)
	}

	got, err := execStore.GetByID(result.Execution.ID)
	if err != nil {
		t.Fatalf("GetByID failed: %v", err)
	}
	if got.TriggerSource != string(SourceCLI) || got.TriggerPrincipal != "cli-user" || got.RequestID != "req-1" {
		t.Fatalf("trigger metadata was not stored: %+v", got)
	}
}

func TestTriggerServiceAsyncIdempotency(t *testing.T) {
	service, _, wf := newTestTriggerService(t)

	first, err := service.Trigger(context.Background(), TriggerRequest{
		WorkflowID:     wf.ID,
		Input:          map[string]interface{}{"source": "test"},
		Mode:           ModeAsync,
		Source:         SourceAPI,
		IdempotencyKey: "idem-1",
	})
	if err != nil {
		t.Fatalf("first Trigger failed: %v", err)
	}
	second, err := service.Trigger(context.Background(), TriggerRequest{
		WorkflowID:     wf.ID,
		Input:          map[string]interface{}{"source": "test"},
		Mode:           ModeAsync,
		Source:         SourceAPI,
		IdempotencyKey: "idem-1",
	})
	if err != nil {
		t.Fatalf("second Trigger failed: %v", err)
	}
	if !second.Deduplicated {
		t.Fatalf("expected second trigger to be deduplicated")
	}
	if first.Execution.ID != second.Execution.ID {
		t.Fatalf("expected same execution ID, got %s and %s", first.Execution.ID, second.Execution.ID)
	}
}

func TestTriggerServiceRejectsInvalidInputSchema(t *testing.T) {
	service, _, wf := newTestTriggerService(t)
	wf.InputSchemaJSON = `{"type":"object","required":["date"],"properties":{"date":{"type":"string"}}}`
	if err := service.wfStore.Update(wf); err != nil {
		t.Fatalf("update workflow schema failed: %v", err)
	}

	_, err := service.Trigger(context.Background(), TriggerRequest{
		WorkflowID: wf.ID,
		Input:      map[string]interface{}{"date": float64(1)},
		Mode:       ModeAsync,
		Source:     SourceMCP,
	})
	if !errors.Is(err, ErrInvalidWorkflowInput) {
		t.Fatalf("expected ErrInvalidWorkflowInput, got %v", err)
	}
}

func newTestTriggerService(t *testing.T) (*TriggerService, *storage.ExecutionStore, *storage.Workflow) {
	t.Helper()
	db, err := storage.NewDB(filepath.Join(t.TempDir(), "goflow.db"))
	if err != nil {
		t.Fatalf("NewDB failed: %v", err)
	}
	t.Cleanup(db.Close)

	registry := nodes.NewPluginRegistry()
	if err := registry.Register(&testAction{}); err != nil {
		t.Fatalf("register test action failed: %v", err)
	}
	wfStore := storage.NewWorkflowStore(db)
	execStore := storage.NewExecutionStore(db)
	eng := engine.NewEngine(registry, execStore, storage.NewCredentialStore(db, nil), engine.NewEventBus(), wfStore)

	nodeList := []nodes.Node{{ID: "a", Type: "testAction", Name: "Action", Params: map[string]interface{}{}}}
	nodesJSON, _ := json.Marshal(nodeList)
	wf := &storage.Workflow{
		ID:        "wf-test",
		Name:      "Test Workflow",
		NodesJSON: string(nodesJSON),
		EdgesJSON: "[]",
	}
	if err := wfStore.Create(wf); err != nil {
		t.Fatalf("Create workflow failed: %v", err)
	}

	return NewTriggerService(wfStore, eng), execStore, wf
}
