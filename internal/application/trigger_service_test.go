package application

import (
	"context"
	"encoding/json"
	"errors"
	"path/filepath"
	"sync"
	"testing"

	"goflow/internal/engine"
	"goflow/internal/nodes"
	"goflow/internal/storage"
)

type testAction struct{}

func (a *testAction) Execute(ctx *nodes.ExecutionContext, node *nodes.Node) (interface{}, error) {
	return map[string]interface{}{"ok": true}, nil
}

func TestTriggerServiceAsyncConcurrentIdempotency(t *testing.T) {
	service, execStore, wf := newTestTriggerService(t)

	const workers = 20
	var wg sync.WaitGroup
	wg.Add(workers)
	start := make(chan struct{})
	results := make(chan *TriggerResult, workers)
	errorsCh := make(chan error, workers)

	for i := 0; i < workers; i++ {
		go func() {
			defer wg.Done()
			<-start
			result, err := service.Trigger(context.Background(), TriggerRequest{
				WorkflowID:     wf.ID,
				Input:          map[string]interface{}{"source": "concurrent-idempotency"},
				Mode:           ModeAsync,
				Source:         SourceAPI,
				IdempotencyKey: "idem-concurrent",
			})
			if err != nil {
				errorsCh <- err
				return
			}
			results <- result
		}()
	}
	close(start)
	wg.Wait()
	close(results)
	close(errorsCh)

	for err := range errorsCh {
		t.Fatalf("concurrent trigger returned error: %v", err)
	}
	var executionID string
	count := 0
	deduplicated := 0
	for result := range results {
		count++
		if result.Deduplicated {
			deduplicated++
		}
		if executionID == "" {
			executionID = result.Execution.ID
		}
		if result.Execution.ID != executionID {
			t.Fatalf("expected same execution ID %s, got %s", executionID, result.Execution.ID)
		}
	}
	if count != workers {
		t.Fatalf("expected %d results, got %d", workers, count)
	}
	if deduplicated != workers-1 {
		t.Fatalf("expected %d deduplicated results, got %d", workers-1, deduplicated)
	}
	executions, err := execStore.ListByWorkflow(wf.ID, 50)
	if err != nil {
		t.Fatalf("list executions: %v", err)
	}
	matches := 0
	for _, exec := range executions {
		if exec.IdempotencyKey == "idem-concurrent" {
			matches++
		}
	}
	if matches != 1 {
		t.Fatalf("expected one execution with idempotency key, got %d: %#v", matches, executions)
	}
}

func (a *testAction) Validate(node *nodes.Node) error { return nil }

func (a *testAction) GetDefinition() nodes.NodeDefinition {
	return nodes.NodeDefinition{Type: "testAction"}
}

type blockingAction struct {
	release <-chan struct{}
}

func (a *blockingAction) Execute(ctx *nodes.ExecutionContext, node *nodes.Node) (interface{}, error) {
	select {
	case <-a.release:
		return map[string]interface{}{"ok": true}, nil
	case <-ctx.Context.Done():
		return nil, ctx.Context.Err()
	}
}

func (a *blockingAction) Validate(node *nodes.Node) error { return nil }

func (a *blockingAction) GetDefinition() nodes.NodeDefinition {
	return nodes.NodeDefinition{Type: "blockingAction"}
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
		Source:     SourceAPI,
	})
	if !errors.Is(err, ErrInvalidWorkflowInput) {
		t.Fatalf("expected ErrInvalidWorkflowInput, got %v", err)
	}
}

func TestTriggerServiceRejectsInactiveWorkflow(t *testing.T) {
	service, _, wf := newTestTriggerService(t)
	wf.IsActive = false
	if err := service.wfStore.Update(wf); err != nil {
		t.Fatalf("update workflow: %v", err)
	}

	_, err := service.Trigger(context.Background(), TriggerRequest{
		WorkflowID: wf.ID,
		Input:      map[string]interface{}{},
		Mode:       ModeAsync,
		Source:     SourceAPI,
	})
	if !errors.Is(err, ErrWorkflowInactive) {
		t.Fatalf("expected ErrWorkflowInactive, got %v", err)
	}
}

func TestTriggerServiceEnforcesCLIExposure(t *testing.T) {
	service, _, wf := newTestTriggerService(t)
	wf.ExposeCLI = false
	if err := service.wfStore.Update(wf); err != nil {
		t.Fatalf("update workflow: %v", err)
	}

	_, err := service.Trigger(context.Background(), TriggerRequest{
		WorkflowID: wf.ID,
		Input:      map[string]interface{}{},
		Mode:       ModeAsync,
		Source:     SourceCLI,
	})
	if !errors.Is(err, ErrWorkflowNotExposed) {
		t.Fatalf("expected ErrWorkflowNotExposed, got %v", err)
	}
}

func TestTriggerServiceEnforcesMCPExposureAndApproval(t *testing.T) {
	service, _, wf := newTestTriggerService(t)

	_, err := service.Trigger(context.Background(), TriggerRequest{
		WorkflowID: wf.ID,
		Input:      map[string]interface{}{},
		Mode:       ModeAsync,
		Source:     SourceMCPHTTP,
	})
	if !errors.Is(err, ErrWorkflowNotExposed) {
		t.Fatalf("expected ErrWorkflowNotExposed, got %v", err)
	}

	wf.ExposeMCP = true
	wf.RequiresApproval = true
	if err := service.wfStore.Update(wf); err != nil {
		t.Fatalf("update workflow: %v", err)
	}
	_, err = service.Trigger(context.Background(), TriggerRequest{
		WorkflowID: wf.ID,
		Input:      map[string]interface{}{},
		Mode:       ModeAsync,
		Source:     SourceMCPStdio,
	})
	if !errors.Is(err, ErrWorkflowRequiresApproval) {
		t.Fatalf("expected ErrWorkflowRequiresApproval, got %v", err)
	}
}

func TestTriggerServiceEnforcesPerWorkflowRejectConcurrency(t *testing.T) {
	release := make(chan struct{})
	service, _, wf := newTestTriggerServiceWithExecutor(t, &blockingAction{release: release}, "blockingAction")
	wf.MaxConcurrentRuns = 1
	wf.ConcurrencyPolicy = "reject"
	if err := service.wfStore.Update(wf); err != nil {
		t.Fatalf("update workflow: %v", err)
	}

	first, err := service.Trigger(context.Background(), TriggerRequest{
		WorkflowID: wf.ID,
		Input:      map[string]interface{}{},
		Mode:       ModeAsync,
		Source:     SourceAPI,
	})
	if err != nil {
		t.Fatalf("first trigger failed: %v", err)
	}
	if first.Execution.Status != "RUNNING" {
		t.Fatalf("expected first execution RUNNING, got %s", first.Execution.Status)
	}

	_, err = service.Trigger(context.Background(), TriggerRequest{
		WorkflowID: wf.ID,
		Input:      map[string]interface{}{},
		Mode:       ModeAsync,
		Source:     SourceAPI,
	})
	if !errors.Is(err, engine.ErrWorkflowConcurrencyLimit) {
		t.Fatalf("expected ErrWorkflowConcurrencyLimit, got %v", err)
	}
	close(release)
}

func TestTriggerServiceIdempotencyWinsOverPerWorkflowConcurrencyLimit(t *testing.T) {
	release := make(chan struct{})
	service, _, wf := newTestTriggerServiceWithExecutor(t, &blockingAction{release: release}, "blockingAction")
	wf.MaxConcurrentRuns = 1
	wf.ConcurrencyPolicy = "reject"
	if err := service.wfStore.Update(wf); err != nil {
		t.Fatalf("update workflow: %v", err)
	}

	first, err := service.Trigger(context.Background(), TriggerRequest{
		WorkflowID:     wf.ID,
		Input:          map[string]interface{}{},
		Mode:           ModeAsync,
		Source:         SourceAPI,
		IdempotencyKey: "idem-running",
	})
	if err != nil {
		t.Fatalf("first trigger failed: %v", err)
	}
	second, err := service.Trigger(context.Background(), TriggerRequest{
		WorkflowID:     wf.ID,
		Input:          map[string]interface{}{},
		Mode:           ModeAsync,
		Source:         SourceAPI,
		IdempotencyKey: "idem-running",
	})
	if err != nil {
		t.Fatalf("duplicate trigger should not hit workflow concurrency limit: %v", err)
	}
	if !second.Deduplicated || second.Execution.ID != first.Execution.ID {
		t.Fatalf("expected deduplicated existing execution, first=%#v second=%#v", first, second)
	}
	_, err = service.Trigger(context.Background(), TriggerRequest{
		WorkflowID:     wf.ID,
		Input:          map[string]interface{}{},
		Mode:           ModeAsync,
		Source:         SourceAPI,
		IdempotencyKey: "different-key",
	})
	if !errors.Is(err, engine.ErrWorkflowConcurrencyLimit) {
		t.Fatalf("expected different key to hit workflow concurrency limit, got %v", err)
	}
	close(release)
}

func TestTriggerServiceIdempotencyWinsOverGlobalConcurrencyLimit(t *testing.T) {
	release := make(chan struct{})
	service, _, wf := newTestTriggerServiceWithExecutorAndGlobalLimit(t, &blockingAction{release: release}, "blockingAction", 1)

	first, err := service.Trigger(context.Background(), TriggerRequest{
		WorkflowID:     wf.ID,
		Input:          map[string]interface{}{},
		Mode:           ModeAsync,
		Source:         SourceAPI,
		IdempotencyKey: "idem-global",
	})
	if err != nil {
		t.Fatalf("first trigger failed: %v", err)
	}
	second, err := service.Trigger(context.Background(), TriggerRequest{
		WorkflowID:     wf.ID,
		Input:          map[string]interface{}{},
		Mode:           ModeAsync,
		Source:         SourceAPI,
		IdempotencyKey: "idem-global",
	})
	if err != nil {
		t.Fatalf("duplicate trigger should not hit global concurrency limit: %v", err)
	}
	if !second.Deduplicated || second.Execution.ID != first.Execution.ID {
		t.Fatalf("expected deduplicated existing execution, first=%#v second=%#v", first, second)
	}
	_, err = service.Trigger(context.Background(), TriggerRequest{
		WorkflowID:     wf.ID,
		Input:          map[string]interface{}{},
		Mode:           ModeAsync,
		Source:         SourceAPI,
		IdempotencyKey: "different-global-key",
	})
	if !errors.Is(err, engine.ErrConcurrencyLimit) {
		t.Fatalf("expected different key to hit global concurrency limit, got %v", err)
	}
	close(release)
}

func newTestTriggerService(t *testing.T) (*TriggerService, *storage.ExecutionStore, *storage.Workflow) {
	return newTestTriggerServiceWithExecutor(t, &testAction{}, "testAction")
}

func newTestTriggerServiceWithExecutor(t *testing.T, executor nodes.NodeExecutor, nodeType string) (*TriggerService, *storage.ExecutionStore, *storage.Workflow) {
	return newTestTriggerServiceWithExecutorAndGlobalLimit(t, executor, nodeType, 0)
}

func newTestTriggerServiceWithExecutorAndGlobalLimit(t *testing.T, executor nodes.NodeExecutor, nodeType string, maxConcurrent int) (*TriggerService, *storage.ExecutionStore, *storage.Workflow) {
	t.Helper()
	db, err := storage.NewDB(filepath.Join(t.TempDir(), "goflow.db"))
	if err != nil {
		t.Fatalf("NewDB failed: %v", err)
	}
	t.Cleanup(db.Close)

	registry := nodes.NewPluginRegistry()
	if err := registry.Register(executor); err != nil {
		t.Fatalf("register test action failed: %v", err)
	}
	wfStore := storage.NewWorkflowStore(db)
	execStore := storage.NewExecutionStore(db)
	eng := engine.NewEngine(registry, execStore, storage.NewCredentialStore(db, nil), engine.NewEventBus(), wfStore, maxConcurrent)

	nodeList := []nodes.Node{{ID: "a", Type: nodes.NodeType(nodeType), Name: "Action", Params: map[string]interface{}{}}}
	nodesJSON, _ := json.Marshal(nodeList)
	wf := &storage.Workflow{
		ID:        "wf-test",
		Name:      "Test Workflow",
		IsActive:  true,
		NodesJSON: string(nodesJSON),
		EdgesJSON: "[]",
		ExposeCLI: true,
	}
	if err := wfStore.Create(wf); err != nil {
		t.Fatalf("Create workflow failed: %v", err)
	}

	return NewTriggerService(wfStore, eng), execStore, wf
}
