package engine

import (
	"encoding/json"
	"errors"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"goflow/internal/nodes"
	"goflow/internal/storage"
)

// Mock Node Executors for Testing Skip Logic
type mockTrigger struct{}

func (m *mockTrigger) Execute(ctx *nodes.ExecutionContext, node *nodes.Node) (interface{}, error) {
	val, _ := node.Params["val"].(string)
	return map[string]interface{}{"result": val}, nil
}
func (m *mockTrigger) Validate(node *nodes.Node) error { return nil }
func (m *mockTrigger) GetDefinition() nodes.NodeDefinition {
	return nodes.NodeDefinition{Type: "mockTrigger"}
}

type mockAction struct {
	executed map[string]bool
}

func (m *mockAction) Execute(ctx *nodes.ExecutionContext, node *nodes.Node) (interface{}, error) {
	m.executed[node.ID] = true
	return map[string]interface{}{"status": "executed", "id": node.ID}, nil
}
func (m *mockAction) Validate(node *nodes.Node) error { return nil }
func (m *mockAction) GetDefinition() nodes.NodeDefinition {
	return nodes.NodeDefinition{Type: "mockAction"}
}

type slowAction struct{}

func (m *slowAction) Execute(ctx *nodes.ExecutionContext, node *nodes.Node) (interface{}, error) {
	time.Sleep(200 * time.Millisecond)
	return map[string]interface{}{"status": "done"}, nil
}
func (m *slowAction) Validate(node *nodes.Node) error { return nil }
func (m *slowAction) GetDefinition() nodes.NodeDefinition {
	return nodes.NodeDefinition{Type: "slowAction"}
}

type countingAction struct {
	current int
	max     int
}

func (m *countingAction) Execute(ctx *nodes.ExecutionContext, node *nodes.Node) (interface{}, error) {
	m.current++
	if m.current > m.max {
		m.max = m.current
	}
	time.Sleep(50 * time.Millisecond)
	m.current--
	return map[string]interface{}{"status": "done"}, nil
}
func (m *countingAction) Validate(node *nodes.Node) error { return nil }
func (m *countingAction) GetDefinition() nodes.NodeDefinition {
	return nodes.NodeDefinition{Type: "countingAction", Retryable: false}
}

type panicAction struct{}

func (m *panicAction) Execute(ctx *nodes.ExecutionContext, node *nodes.Node) (interface{}, error) {
	panic("boom")
}
func (m *panicAction) Validate(node *nodes.Node) error { return nil }
func (m *panicAction) GetDefinition() nodes.NodeDefinition {
	return nodes.NodeDefinition{Type: "panicAction", Retryable: false}
}

type typedSourceAction struct{}

func (m *typedSourceAction) Execute(ctx *nodes.ExecutionContext, node *nodes.Node) (interface{}, error) {
	return map[string]interface{}{
		"str":           "hello",
		"num":           float64(42),
		"bool":          true,
		"profile":       map[string]interface{}{"role": "admin"},
		"items":         []interface{}{float64(1), "two"},
		"delay_seconds": float64(1),
	}, nil
}
func (m *typedSourceAction) Validate(node *nodes.Node) error { return nil }
func (m *typedSourceAction) GetDefinition() nodes.NodeDefinition {
	return nodes.NodeDefinition{Type: "typedSourceAction"}
}

type echoParamsAction struct{}

func (m *echoParamsAction) Execute(ctx *nodes.ExecutionContext, node *nodes.Node) (interface{}, error) {
	return map[string]interface{}{"params": node.Params}, nil
}
func (m *echoParamsAction) Validate(node *nodes.Node) error { return nil }
func (m *echoParamsAction) GetDefinition() nodes.NodeDefinition {
	return nodes.NodeDefinition{Type: "echoParamsAction"}
}

func TestExecuteWorkflowConcurrencyLimit(t *testing.T) {
	registry := nodes.NewPluginRegistry()
	_ = registry.Register(&slowAction{})

	db, err := storage.NewDB(":memory:")
	if err != nil {
		t.Fatalf("failed to open memory db: %v", err)
	}
	defer db.Close()

	execStore := storage.NewExecutionStore(db)
	credStore := storage.NewCredentialStore(db, nil)
	wfStore := storage.NewWorkflowStore(db)
	eventBus := NewEventBus()
	eng := NewEngine(registry, execStore, credStore, eventBus, wfStore, 1)

	nodeList := []nodes.Node{
		{ID: "slow_1", Type: "slowAction", Name: "Slow", Params: map[string]interface{}{}},
	}
	nodesJSON, _ := json.Marshal(nodeList)
	wf := &storage.Workflow{
		ID:        "wf-limit",
		Name:      "Concurrency limit",
		NodesJSON: string(nodesJSON),
		EdgesJSON: "[]",
	}
	if err := wfStore.Create(wf); err != nil {
		t.Fatalf("failed to create workflow in DB: %v", err)
	}

	if err := eng.ExecuteWorkflowAsync(wf, nil); err != nil {
		t.Fatalf("first async execution should start: %v", err)
	}

	_, err = eng.ExecuteWorkflow(wf, nil)
	if !errors.Is(err, ErrConcurrencyLimit) {
		t.Fatalf("expected ErrConcurrencyLimit, got %v", err)
	}
	time.Sleep(250 * time.Millisecond)
}

func TestNodePanicMarksExecutionFailed(t *testing.T) {
	registry := nodes.NewPluginRegistry()
	_ = registry.Register(&panicAction{})

	db, err := storage.NewDB(filepath.Join(t.TempDir(), "goflow.db"))
	if err != nil {
		t.Fatalf("failed to open db: %v", err)
	}
	defer db.Close()

	execStore := storage.NewExecutionStore(db)
	wfStore := storage.NewWorkflowStore(db)
	eng := NewEngine(registry, execStore, storage.NewCredentialStore(db, nil), NewEventBus(), wfStore)

	nodeList := []nodes.Node{{ID: "panic", Type: "panicAction", Name: "Panic", Params: map[string]interface{}{}}}
	nodesJSON, _ := json.Marshal(nodeList)
	wf := &storage.Workflow{
		ID:        "wf-panic",
		Name:      "Panic Workflow",
		NodesJSON: string(nodesJSON),
		EdgesJSON: "[]",
	}
	if err := wfStore.Create(wf); err != nil {
		t.Fatalf("create workflow: %v", err)
	}

	exec, err := eng.ExecuteWorkflow(wf, nil)
	if err != nil {
		t.Fatalf("execute workflow returned error: %v", err)
	}
	if exec.Status != "FAILED" {
		t.Fatalf("expected FAILED, got %s", exec.Status)
	}
	if !strings.Contains(exec.ErrorMessage, "node panic recovered") {
		t.Fatalf("expected recovered panic error, got %q", exec.ErrorMessage)
	}
}

func TestUnknownNodeTypeMarksExecutionFailedWithoutHanging(t *testing.T) {
	registry := nodes.NewPluginRegistry()

	db, err := storage.NewDB(filepath.Join(t.TempDir(), "goflow.db"))
	if err != nil {
		t.Fatalf("failed to open db: %v", err)
	}
	defer db.Close()

	execStore := storage.NewExecutionStore(db)
	wfStore := storage.NewWorkflowStore(db)
	eng := NewEngine(registry, execStore, storage.NewCredentialStore(db, nil), NewEventBus(), wfStore)

	nodeList := []nodes.Node{{ID: "missing", Type: "missingNodeType", Name: "Missing", Params: map[string]interface{}{}}}
	nodesJSON, _ := json.Marshal(nodeList)
	wf := &storage.Workflow{
		ID:        "wf-missing-node",
		Name:      "Missing Node",
		NodesJSON: string(nodesJSON),
		EdgesJSON: "[]",
	}
	if err := wfStore.Create(wf); err != nil {
		t.Fatalf("create workflow: %v", err)
	}

	done := make(chan *storage.Execution, 1)
	errs := make(chan error, 1)
	go func() {
		exec, err := eng.ExecuteWorkflow(wf, nil)
		if err != nil {
			errs <- err
			return
		}
		done <- exec
	}()

	select {
	case err := <-errs:
		t.Fatalf("execute workflow returned error: %v", err)
	case exec := <-done:
		if exec.Status != "FAILED" {
			t.Fatalf("expected FAILED, got %s", exec.Status)
		}
		if !strings.Contains(exec.ErrorMessage, "unregistered node executor type") {
			t.Fatalf("expected missing executor error, got %q", exec.ErrorMessage)
		}
	case <-time.After(2 * time.Second):
		t.Fatalf("execution hung for unknown node type")
	}
}

func TestRuntimeCompleteExpressionsPreserveTypes(t *testing.T) {
	registry := nodes.NewPluginRegistry()
	_ = registry.Register(&typedSourceAction{})
	_ = registry.Register(&echoParamsAction{})

	db, err := storage.NewDB(filepath.Join(t.TempDir(), "goflow.db"))
	if err != nil {
		t.Fatalf("failed to open db: %v", err)
	}
	defer db.Close()

	execStore := storage.NewExecutionStore(db)
	wfStore := storage.NewWorkflowStore(db)
	eng := NewEngine(registry, execStore, storage.NewCredentialStore(db, nil), NewEventBus(), wfStore)

	nodeList := []nodes.Node{
		{ID: "source", Type: "typedSourceAction", Name: "Typed Source", Params: map[string]interface{}{}},
		{ID: "capture", Type: "echoParamsAction", Name: "Capture", Params: map[string]interface{}{
			"str":        "{{source.str}}",
			"num":        "{{source.num}}",
			"bool":       "{{source.bool}}",
			"profile":    "{{source.profile}}",
			"items":      "{{source.items}}",
			"triggerObj": "{{$trigger.profile}}",
			"triggerNum": "{{$trigger.count}}",
			"mixed":      "count={{$trigger.count}} profile={{$trigger.profile}}",
		}},
	}
	edgeList := []nodes.Edge{{ID: "e1", Source: "source", Target: "capture"}}
	nodesJSON, _ := json.Marshal(nodeList)
	edgesJSON, _ := json.Marshal(edgeList)
	wf := &storage.Workflow{
		ID:        "wf-runtime-types",
		Name:      "Runtime Types",
		NodesJSON: string(nodesJSON),
		EdgesJSON: string(edgesJSON),
	}
	if err := wfStore.Create(wf); err != nil {
		t.Fatalf("create workflow: %v", err)
	}

	exec, err := eng.ExecuteWorkflow(wf, map[string]interface{}{
		"count":   float64(3),
		"profile": map[string]interface{}{"role": "trigger"},
	})
	if err != nil {
		t.Fatalf("execute workflow: %v", err)
	}
	if exec.Status != "SUCCESS" {
		t.Fatalf("expected SUCCESS, got %s: %s", exec.Status, exec.ErrorMessage)
	}

	var logs []NodeLog
	if err := json.Unmarshal([]byte(exec.LogsJSON), &logs); err != nil {
		t.Fatalf("parse logs: %v", err)
	}
	var capture NodeLog
	for _, log := range logs {
		if log.NodeID == "capture" {
			capture = log
			break
		}
	}
	output, ok := capture.Output.(map[string]interface{})
	if !ok {
		t.Fatalf("capture output has type %T", capture.Output)
	}
	params, ok := output["params"].(map[string]interface{})
	if !ok {
		t.Fatalf("capture params has type %T", output["params"])
	}
	if params["str"] != "hello" {
		t.Fatalf("expected string value, got %#v", params["str"])
	}
	if _, ok := params["num"].(float64); !ok || params["num"] != float64(42) {
		t.Fatalf("number expression lost type: %#v (%T)", params["num"], params["num"])
	}
	if _, ok := params["bool"].(bool); !ok || params["bool"] != true {
		t.Fatalf("boolean expression lost type: %#v (%T)", params["bool"], params["bool"])
	}
	if profile, ok := params["profile"].(map[string]interface{}); !ok || profile["role"] != "admin" {
		t.Fatalf("object expression lost type: %#v (%T)", params["profile"], params["profile"])
	}
	if items, ok := params["items"].([]interface{}); !ok || len(items) != 2 || items[0] != float64(1) {
		t.Fatalf("array expression lost type: %#v (%T)", params["items"], params["items"])
	}
	if triggerObj, ok := params["triggerObj"].(map[string]interface{}); !ok || triggerObj["role"] != "trigger" {
		t.Fatalf("$trigger object expression lost type: %#v (%T)", params["triggerObj"], params["triggerObj"])
	}
	if _, ok := params["triggerNum"].(float64); !ok || params["triggerNum"] != float64(3) {
		t.Fatalf("$trigger number expression lost type: %#v (%T)", params["triggerNum"], params["triggerNum"])
	}
	if params["mixed"] != `count=3 profile={"role":"trigger"}` {
		t.Fatalf("mixed interpolation should stringify values, got %#v", params["mixed"])
	}
}

func TestDelaySleepAcceptsNumericExpressionRuntimeValue(t *testing.T) {
	registry := nodes.NewPluginRegistry()
	_ = registry.Register(&typedSourceAction{})
	_ = registry.Register(nodes.NewDelaySleepExecutor())

	db, err := storage.NewDB(filepath.Join(t.TempDir(), "goflow.db"))
	if err != nil {
		t.Fatalf("failed to open db: %v", err)
	}
	defer db.Close()

	execStore := storage.NewExecutionStore(db)
	wfStore := storage.NewWorkflowStore(db)
	eng := NewEngine(registry, execStore, storage.NewCredentialStore(db, nil), NewEventBus(), wfStore)

	nodeList := []nodes.Node{
		{ID: "source", Type: "typedSourceAction", Name: "Typed Source", Params: map[string]interface{}{}},
		{ID: "delay", Type: nodes.TypeDelaySleep, Name: "Delay", Params: map[string]interface{}{
			"seconds": "{{source.delay_seconds}}",
		}},
	}
	edgeList := []nodes.Edge{{ID: "e1", Source: "source", Target: "delay"}}
	nodesJSON, _ := json.Marshal(nodeList)
	edgesJSON, _ := json.Marshal(edgeList)
	wf := &storage.Workflow{
		ID:        "wf-delay-expression",
		Name:      "Delay Expression",
		NodesJSON: string(nodesJSON),
		EdgesJSON: string(edgesJSON),
	}
	if err := wfStore.Create(wf); err != nil {
		t.Fatalf("create workflow: %v", err)
	}

	start := time.Now()
	exec, err := eng.ExecuteWorkflow(wf, nil)
	if err != nil {
		t.Fatalf("execute workflow: %v", err)
	}
	elapsed := time.Since(start)
	if exec.Status != "SUCCESS" {
		t.Fatalf("expected SUCCESS, got %s: %s", exec.Status, exec.ErrorMessage)
	}
	if elapsed < 900*time.Millisecond {
		t.Fatalf("delay expression did not wait for numeric value, elapsed %s", elapsed)
	}

	var logs []NodeLog
	if err := json.Unmarshal([]byte(exec.LogsJSON), &logs); err != nil {
		t.Fatalf("parse logs: %v", err)
	}
	for _, log := range logs {
		if log.NodeID != "delay" {
			continue
		}
		output, _ := log.Output.(map[string]interface{})
		if output["delayed_seconds"] != float64(1) {
			t.Fatalf("expected delayed_seconds 1, got %#v", output["delayed_seconds"])
		}
		return
	}
	t.Fatalf("delay node log not found")
}

func TestCancelAsyncWorkflow(t *testing.T) {
	registry := nodes.NewPluginRegistry()
	_ = registry.Register(nodes.NewDelaySleepExecutor())

	db, err := storage.NewDB(filepath.Join(t.TempDir(), "goflow.db"))
	if err != nil {
		t.Fatalf("failed to open db: %v", err)
	}
	defer db.Close()

	execStore := storage.NewExecutionStore(db)
	wfStore := storage.NewWorkflowStore(db)
	eng := NewEngine(registry, execStore, storage.NewCredentialStore(db, nil), NewEventBus(), wfStore)

	nodeList := []nodes.Node{
		{ID: "delay", Type: nodes.TypeDelaySleep, Name: "Delay", Params: map[string]interface{}{"seconds": "5"}},
	}
	nodesJSON, _ := json.Marshal(nodeList)
	wf := &storage.Workflow{
		ID:        "wf-cancel",
		Name:      "Cancellation",
		NodesJSON: string(nodesJSON),
		EdgesJSON: "[]",
	}
	if err := wfStore.Create(wf); err != nil {
		t.Fatalf("failed to create workflow: %v", err)
	}

	exec, _, err := eng.StartWorkflowAsync(wf, nil, TriggerOptions{})
	if err != nil {
		t.Fatalf("start workflow async failed: %v", err)
	}
	if !eng.CancelExecution(exec.ID) {
		t.Fatalf("expected active execution to be cancellable")
	}

	var got *storage.Execution
	for i := 0; i < 30; i++ {
		got, err = execStore.GetByID(exec.ID)
		if err != nil {
			t.Fatalf("get execution failed: %v", err)
		}
		if got.Status == "CANCELLED" {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}
	if got.Status != "CANCELLED" {
		t.Fatalf("expected CANCELLED, got %s", got.Status)
	}
	if got.CancelledAt == nil {
		t.Fatalf("expected cancelled_at to be set")
	}
}

func TestMaxParallelNodesPerExecution(t *testing.T) {
	counter := &countingAction{}
	registry := nodes.NewPluginRegistry()
	_ = registry.Register(counter)

	db, err := storage.NewDB(filepath.Join(t.TempDir(), "goflow.db"))
	if err != nil {
		t.Fatalf("failed to open db: %v", err)
	}
	defer db.Close()

	execStore := storage.NewExecutionStore(db)
	wfStore := storage.NewWorkflowStore(db)
	eng := NewEngine(registry, execStore, storage.NewCredentialStore(db, nil), NewEventBus(), wfStore, 0, 1)

	nodeList := []nodes.Node{
		{ID: "a", Type: "countingAction", Name: "A", Params: map[string]interface{}{}},
		{ID: "b", Type: "countingAction", Name: "B", Params: map[string]interface{}{}},
		{ID: "c", Type: "countingAction", Name: "C", Params: map[string]interface{}{}},
	}
	nodesJSON, _ := json.Marshal(nodeList)
	wf := &storage.Workflow{
		ID:        "wf-node-limit",
		Name:      "Node limit",
		NodesJSON: string(nodesJSON),
		EdgesJSON: "[]",
	}
	if err := wfStore.Create(wf); err != nil {
		t.Fatalf("failed to create workflow: %v", err)
	}

	exec, err := eng.ExecuteWorkflow(wf, nil)
	if err != nil {
		t.Fatalf("execute workflow failed: %v", err)
	}
	if exec.Status != "SUCCESS" {
		t.Fatalf("expected SUCCESS, got %s", exec.Status)
	}
	if counter.max != 1 {
		t.Fatalf("expected max node concurrency 1, got %d", counter.max)
	}
}

func TestExecuteWorkflowWithSkipLogic(t *testing.T) {
	// Setup registry and register executors
	registry := nodes.NewPluginRegistry()
	_ = registry.Register(&mockTrigger{})
	_ = registry.Register(nodes.NewConditionIFExecutor())

	executedMap := make(map[string]bool)
	actionExec := &mockAction{executed: executedMap}
	_ = registry.Register(actionExec)

	// Mock Stores
	// We don't need real DB for store if we mock or use temporary SQLite in memory
	db, err := storage.NewDB(":memory:")
	if err != nil {
		t.Fatalf("failed to open memory db: %v", err)
	}
	defer db.Close()

	execStore := storage.NewExecutionStore(db)
	credStore := storage.NewCredentialStore(db, nil)
	wfStore := storage.NewWorkflowStore(db)
	eventBus := NewEventBus()
	eng := NewEngine(registry, execStore, credStore, eventBus, wfStore, 1)

	// Define nodes:
	// trigger_1 (val = "ALERT")
	// condition_1 (field = "{{trigger_1.result}}", operator = "equals", value = "ALERT")
	// node_true (mockAction, runs on true branch)
	// node_false (mockAction, runs on false branch)
	// node_merge (mockAction, connected to both node_true and node_false)
	nodeList := []nodes.Node{
		{ID: "trigger_1", Type: "mockTrigger", Name: "Trigger", Params: map[string]interface{}{"val": "ALERT"}},
		{
			ID:   "condition_1",
			Type: nodes.TypeConditionIF,
			Name: "Condition",
			Params: map[string]interface{}{
				"field":    "{{trigger_1.result}}",
				"operator": "equals",
				"value":    "ALERT",
			},
		},
		{ID: "node_true", Type: "mockAction", Name: "True Branch Action"},
		{ID: "node_false", Type: "mockAction", Name: "False Branch Action"},
		{ID: "node_merge", Type: "mockAction", Name: "Merge Action"},
	}

	edgeList := []nodes.Edge{
		{ID: "e1", Source: "trigger_1", Target: "condition_1"},
		{ID: "e2", Source: "condition_1", SourceHandle: "true", Target: "node_true"},
		{ID: "e3", Source: "condition_1", SourceHandle: "false", Target: "node_false"},
		{ID: "e4", Source: "node_true", Target: "node_merge"},
		{ID: "e5", Source: "node_false", Target: "node_merge"},
	}

	nodesJSON, _ := json.Marshal(nodeList)
	edgesJSON, _ := json.Marshal(edgeList)

	wf := &storage.Workflow{
		ID:        "wf-1",
		Name:      "Skip Logic Test",
		NodesJSON: string(nodesJSON),
		EdgesJSON: string(edgesJSON),
	}

	if err := wfStore.Create(wf); err != nil {
		t.Fatalf("failed to create workflow in DB: %v", err)
	}

	// 1. Run with condition evaluating to TRUE
	exec, err := eng.ExecuteWorkflow(wf, nil)
	if err != nil {
		t.Fatalf("workflow execution failed: %v", err)
	}

	if exec.Status != "SUCCESS" {
		t.Errorf("Expected execution status SUCCESS, got %s", exec.Status)
	}

	// Check which nodes were executed
	if !executedMap["node_true"] {
		t.Errorf("Expected node_true to be executed, but it was not")
	}
	if executedMap["node_false"] {
		t.Errorf("Expected node_false to be skipped, but it was executed")
	}
	if !executedMap["node_merge"] {
		t.Errorf("Expected node_merge to be executed (as it joins an active branch), but it was not")
	}

	// Reset executedMap
	for k := range executedMap {
		delete(executedMap, k)
	}

	// 2. Run with condition evaluating to FALSE (trigger_1.val = "NOT_ALERT")
	nodeList[0].Params["val"] = "NOT_ALERT"
	nodesJSON2, _ := json.Marshal(nodeList)
	wf.NodesJSON = string(nodesJSON2)

	exec2, err := eng.ExecuteWorkflow(wf, nil)
	if err != nil {
		t.Fatalf("workflow execution 2 failed: %v", err)
	}

	if exec2.Status != "SUCCESS" {
		t.Errorf("Expected execution status SUCCESS, got %s", exec2.Status)
	}

	// Check which nodes were executed
	if executedMap["node_true"] {
		t.Errorf("Expected node_true to be skipped, but it was executed")
	}
	if !executedMap["node_false"] {
		t.Errorf("Expected node_false to be executed, but it was not")
	}
	if !executedMap["node_merge"] {
		t.Errorf("Expected node_merge to be executed (as it joins an active branch), but it was not")
	}

	// Verify that logs contain the correct states
	var logs []NodeLog
	if err := json.Unmarshal([]byte(exec2.LogsJSON), &logs); err != nil {
		t.Fatalf("failed to unmarshal logs: %v", err)
	}

	expectedLogCount := 5
	var loggedNodeIDs []string
	statusByNode := map[string]string{}
	for _, l := range logs {
		loggedNodeIDs = append(loggedNodeIDs, l.NodeID)
		statusByNode[l.NodeID] = l.Status
	}

	if len(logs) != expectedLogCount {
		t.Errorf("Expected %d logged steps, got %d. Logged: %s", expectedLogCount, len(logs), strings.Join(loggedNodeIDs, ", "))
	}
	if statusByNode["node_true"] != "SKIPPED" {
		t.Errorf("Expected node_true to be logged as SKIPPED, got %q. Logs: %s", statusByNode["node_true"], strings.Join(loggedNodeIDs, ", "))
	}
}

func TestSubWorkflowExecution(t *testing.T) {
	registry := nodes.NewPluginRegistry()
	_ = registry.Register(&mockTrigger{})
	_ = registry.Register(nodes.NewSubWorkflowExecutor())

	executedMap := make(map[string]bool)
	actionExec := &mockAction{executed: executedMap}
	_ = registry.Register(actionExec)

	dbFile := filepath.Join(t.TempDir(), "test_sub_wf.db")

	db, err := storage.NewDB(dbFile)
	if err != nil {
		t.Fatalf("failed to open test db: %v", err)
	}
	defer db.Close()

	execStore := storage.NewExecutionStore(db)
	credStore := storage.NewCredentialStore(db, nil)
	wfStore := storage.NewWorkflowStore(db)
	eventBus := NewEventBus()
	eng := NewEngine(registry, execStore, credStore, eventBus, wfStore, 1)

	// 1. Create Sub-workflow
	subNodeList := []nodes.Node{
		{ID: "sub_trigger", Type: "mockTrigger", Name: "Sub Trigger"},
		{ID: "sub_action", Type: "mockAction", Name: "Sub Action"},
	}
	subEdgeList := []nodes.Edge{
		{ID: "sub_e1", Source: "sub_trigger", Target: "sub_action"},
	}
	subNodesJSON, _ := json.Marshal(subNodeList)
	subEdgesJSON, _ := json.Marshal(subEdgeList)

	subWf := &storage.Workflow{
		ID:        "sub-wf-id",
		Name:      "Sub Workflow",
		NodesJSON: string(subNodesJSON),
		EdgesJSON: string(subEdgesJSON),
	}
	if err := wfStore.Create(subWf); err != nil {
		t.Fatalf("failed to create sub-workflow: %v", err)
	}

	// 2. Create Main Workflow
	mainNodeList := []nodes.Node{
		{ID: "main_trigger", Type: "mockTrigger", Name: "Main Trigger"},
		{
			ID:   "sub_runner",
			Type: nodes.TypeSubWorkflow,
			Name: "Run Sub-workflow",
			Params: map[string]interface{}{
				"sub_workflow_id": "sub-wf-id",
				"payload_json":    `{"test":"value"}`,
			},
		},
	}
	mainEdgeList := []nodes.Edge{
		{ID: "main_e1", Source: "main_trigger", Target: "sub_runner"},
	}
	mainNodesJSON, _ := json.Marshal(mainNodeList)
	mainEdgesJSON, _ := json.Marshal(mainEdgeList)

	mainWf := &storage.Workflow{
		ID:        "main-wf-id",
		Name:      "Main Workflow",
		NodesJSON: string(mainNodesJSON),
		EdgesJSON: string(mainEdgesJSON),
	}
	if err := wfStore.Create(mainWf); err != nil {
		t.Fatalf("failed to create main workflow: %v", err)
	}

	// 3. Execute Main Workflow
	exec, err := eng.ExecuteWorkflow(mainWf, nil)
	if err != nil {
		t.Fatalf("failed to execute main workflow: %v", err)
	}

	if exec.Status != "SUCCESS" {
		t.Errorf("Expected status SUCCESS, got %s", exec.Status)
	}

	// Check if sub-workflow's action was executed
	if !executedMap["sub_action"] {
		t.Errorf("Expected sub-workflow action sub_action to be executed, but it was not")
	}

	// Verify that the output of sub_runner contains sub_action's output
	var logs []NodeLog
	_ = json.Unmarshal([]byte(exec.LogsJSON), &logs)
	var subRunnerLog *NodeLog
	for _, l := range logs {
		if l.NodeID == "sub_runner" {
			subRunnerLog = &l
			break
		}
	}
	if subRunnerLog == nil {
		t.Fatalf("sub_runner node log not found")
	}
	if subRunnerLog.Status != "SUCCESS" {
		t.Errorf("Expected sub_runner status SUCCESS, got %s", subRunnerLog.Status)
	}

	outMap, ok := subRunnerLog.Output.(map[string]interface{})
	if !ok {
		t.Fatalf("Expected output of sub_runner to be a map, got %T", subRunnerLog.Output)
	}
	subActionOut, exists := outMap["sub_action"]
	if !exists {
		t.Errorf("Expected sub_action output to exist in sub_runner results, but it did not")
	} else {
		subActionOutMap, ok := subActionOut.(map[string]interface{})
		if !ok || subActionOutMap["id"] != "sub_action" {
			t.Errorf("Invalid sub_action output: %v", subActionOut)
		}
	}
}

func TestSubWorkflowCycleFailsExecution(t *testing.T) {
	registry := nodes.NewPluginRegistry()
	_ = registry.Register(nodes.NewSubWorkflowExecutor())

	db, err := storage.NewDB(filepath.Join(t.TempDir(), "goflow.db"))
	if err != nil {
		t.Fatalf("failed to open db: %v", err)
	}
	defer db.Close()

	execStore := storage.NewExecutionStore(db)
	wfStore := storage.NewWorkflowStore(db)
	eng := NewEngine(registry, execStore, storage.NewCredentialStore(db, nil), NewEventBus(), wfStore)

	nodeList := []nodes.Node{{
		ID:   "self",
		Type: nodes.TypeSubWorkflow,
		Name: "Self",
		Params: map[string]interface{}{
			"sub_workflow_id": "wf-cycle",
		},
	}}
	nodesJSON, _ := json.Marshal(nodeList)
	wf := &storage.Workflow{
		ID:        "wf-cycle",
		Name:      "Cycle",
		NodesJSON: string(nodesJSON),
		EdgesJSON: "[]",
	}
	if err := wfStore.Create(wf); err != nil {
		t.Fatalf("create workflow: %v", err)
	}

	exec, err := eng.ExecuteWorkflow(wf, nil)
	if err != nil {
		t.Fatalf("execute workflow returned protocol error: %v", err)
	}
	if exec.Status != "FAILED" {
		t.Fatalf("expected FAILED, got %s", exec.Status)
	}
	if !strings.Contains(exec.ErrorMessage, "cycle detected") {
		t.Fatalf("expected cycle error, got %q", exec.ErrorMessage)
	}
}

func TestSubWorkflowDepthLimitFailsExecution(t *testing.T) {
	t.Setenv("GOFLOW_MAX_SUBWORKFLOW_DEPTH", "1")
	registry := nodes.NewPluginRegistry()
	_ = registry.Register(nodes.NewSubWorkflowExecutor())
	_ = registry.Register(&mockAction{executed: make(map[string]bool)})

	db, err := storage.NewDB(filepath.Join(t.TempDir(), "goflow.db"))
	if err != nil {
		t.Fatalf("failed to open db: %v", err)
	}
	defer db.Close()

	execStore := storage.NewExecutionStore(db)
	wfStore := storage.NewWorkflowStore(db)
	eng := NewEngine(registry, execStore, storage.NewCredentialStore(db, nil), NewEventBus(), wfStore)

	createSubWorkflowTestWorkflow(t, wfStore, "wf-c", []nodes.Node{{ID: "done", Type: "mockAction", Name: "Done", Params: map[string]interface{}{}}})
	createSubWorkflowTestWorkflow(t, wfStore, "wf-b", []nodes.Node{{
		ID:   "to_c",
		Type: nodes.TypeSubWorkflow,
		Name: "To C",
		Params: map[string]interface{}{
			"sub_workflow_id": "wf-c",
		},
	}})
	wfA := createSubWorkflowTestWorkflow(t, wfStore, "wf-a", []nodes.Node{{
		ID:   "to_b",
		Type: nodes.TypeSubWorkflow,
		Name: "To B",
		Params: map[string]interface{}{
			"sub_workflow_id": "wf-b",
		},
	}})

	exec, err := eng.ExecuteWorkflow(wfA, nil)
	if err != nil {
		t.Fatalf("execute workflow returned protocol error: %v", err)
	}
	if exec.Status != "FAILED" {
		t.Fatalf("expected FAILED, got %s", exec.Status)
	}
	if !strings.Contains(exec.ErrorMessage, "depth limit exceeded") {
		t.Fatalf("expected depth limit error, got %q", exec.ErrorMessage)
	}
}

func createSubWorkflowTestWorkflow(t *testing.T, wfStore *storage.WorkflowStore, id string, nodeList []nodes.Node) *storage.Workflow {
	t.Helper()
	nodesJSON, _ := json.Marshal(nodeList)
	wf := &storage.Workflow{
		ID:        id,
		Name:      id,
		NodesJSON: string(nodesJSON),
		EdgesJSON: "[]",
	}
	if err := wfStore.Create(wf); err != nil {
		t.Fatalf("create workflow %s: %v", id, err)
	}
	return wf
}

func BenchmarkEngineParallel(b *testing.B) {
	registry := nodes.NewPluginRegistry()
	_ = registry.Register(&mockTrigger{})
	_ = registry.Register(&mockAction{executed: make(map[string]bool)})

	db, err := storage.NewDB(":memory:")
	if err != nil {
		b.Fatalf("failed to open memory db: %v", err)
	}
	defer db.Close()

	execStore := storage.NewExecutionStore(db)
	credStore := storage.NewCredentialStore(db, nil)
	wfStore := storage.NewWorkflowStore(db)
	eventBus := NewEventBus()
	eng := NewEngine(registry, execStore, credStore, eventBus, wfStore)

	nodeList := []nodes.Node{
		{ID: "trigger_1", Type: "mockTrigger", Name: "Trigger", Params: map[string]interface{}{"val": "BENCH"}},
		{ID: "action_1", Type: "mockAction", Name: "Action 1"},
		{ID: "action_2", Type: "mockAction", Name: "Action 2"},
	}
	edgeList := []nodes.Edge{
		{ID: "e1", Source: "trigger_1", Target: "action_1"},
		{ID: "e2", Source: "action_1", Target: "action_2"},
	}
	nodesJSON, _ := json.Marshal(nodeList)
	edgesJSON, _ := json.Marshal(edgeList)

	wf := &storage.Workflow{
		ID:        "wf-bench",
		Name:      "Bench Workflow",
		NodesJSON: string(nodesJSON),
		EdgesJSON: string(edgesJSON),
	}

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_, err := eng.ExecuteWorkflow(wf, nil)
			if err != nil {
				b.Errorf("ExecuteWorkflow failed: %v", err)
			}
		}
	})
}
