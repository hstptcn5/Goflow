package engine

import (
	"encoding/json"
	"errors"
	"os"
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

	if err := eng.ExecuteWorkflowAsync(wf, nil); err != nil {
		t.Fatalf("first async execution should start: %v", err)
	}

	_, err = eng.ExecuteWorkflow(wf, nil)
	if !errors.Is(err, ErrConcurrencyLimit) {
		t.Fatalf("expected ErrConcurrencyLimit, got %v", err)
	}
	time.Sleep(250 * time.Millisecond)
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
	eng := NewEngine(registry, execStore, credStore, eventBus, wfStore)

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

	// We expect 5 logs (since all 5 nodes are processed and logged, but some as skipped)
	// Actually, wait! Did we append skipped nodes to logs?
	// Let's check in engine.go: when a node is StateSkipped:
	// We decrement remainingCount and continue. We don't append to nodeLogs for skipped nodes!
	// Wait, is that true? Let's check:
	// Yes! In engine.go, when state is StateSkipped, it just continues:
	// `remainingCount--`
	// `continue`
	// It doesn't append to nodeLogs!
	// So skipped nodes are not listed in NodeLog, meaning they are skipped from logs too!
	// That is fine, or if they are listed? The executed nodes list will only contain executed ones.
	// Let's check the number of logged nodes:
	// trigger_1: success (executed)
	// condition_1: success (executed)
	// node_true: skipped (not logged)
	// node_false: success (executed)
	// node_merge: success (executed)
	// So we expect exactly 4 logs!
	expectedLogCount := 4
	var loggedNodeIDs []string
	for _, l := range logs {
		loggedNodeIDs = append(loggedNodeIDs, l.NodeID)
	}

	if len(logs) != expectedLogCount {
		t.Errorf("Expected %d logged steps, got %d. Logged: %s", expectedLogCount, len(logs), strings.Join(loggedNodeIDs, ", "))
	}
}

func TestSubWorkflowExecution(t *testing.T) {
	registry := nodes.NewPluginRegistry()
	_ = registry.Register(&mockTrigger{})
	_ = registry.Register(nodes.NewSubWorkflowExecutor())

	executedMap := make(map[string]bool)
	actionExec := &mockAction{executed: executedMap}
	_ = registry.Register(actionExec)

	dbFile := "test_sub_wf.db"
	_ = os.Remove(dbFile)
	defer os.Remove(dbFile)

	db, err := storage.NewDB(dbFile)
	if err != nil {
		t.Fatalf("failed to open test db: %v", err)
	}
	defer db.Close()

	execStore := storage.NewExecutionStore(db)
	credStore := storage.NewCredentialStore(db, nil)
	wfStore := storage.NewWorkflowStore(db)
	eventBus := NewEventBus()
	eng := NewEngine(registry, execStore, credStore, eventBus, wfStore)

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
