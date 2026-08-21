package engine

import (
	"encoding/json"
	"errors"
	"sync"
	"testing"

	"goflow/internal/nodes"
	"goflow/internal/storage"
)

type policyFailAction struct{}

func (a *policyFailAction) Execute(ctx *nodes.ExecutionContext, node *nodes.Node) (interface{}, error) {
	return nil, errors.New("policy test failure")
}
func (a *policyFailAction) Validate(node *nodes.Node) error { return nil }
func (a *policyFailAction) GetDefinition() nodes.NodeDefinition {
	return nodes.NodeDefinition{Type: "policyFailAction", Category: "ACTION", Retryable: false}
}

type policyPassAction struct{}

func (a *policyPassAction) Execute(ctx *nodes.ExecutionContext, node *nodes.Node) (interface{}, error) {
	return map[string]interface{}{"ok": true}, nil
}
func (a *policyPassAction) Validate(node *nodes.Node) error { return nil }
func (a *policyPassAction) GetDefinition() nodes.NodeDefinition {
	return nodes.NodeDefinition{Type: "policyPassAction", Category: "ACTION", Retryable: false}
}

type policyRecordAction struct {
	mu     sync.Mutex
	calls  map[string]int
	params map[string]map[string]interface{}
}

func newPolicyRecordAction() *policyRecordAction {
	return &policyRecordAction{calls: map[string]int{}, params: map[string]map[string]interface{}{}}
}

func (a *policyRecordAction) Execute(ctx *nodes.ExecutionContext, node *nodes.Node) (interface{}, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.calls[node.ID]++
	copyParams := make(map[string]interface{}, len(node.Params))
	for key, value := range node.Params {
		copyParams[key] = value
	}
	a.params[node.ID] = copyParams
	return map[string]interface{}{"recorded": node.ID}, nil
}
func (a *policyRecordAction) Validate(node *nodes.Node) error { return nil }
func (a *policyRecordAction) GetDefinition() nodes.NodeDefinition {
	return nodes.NodeDefinition{Type: "policyRecordAction", Category: "ACTION", Retryable: false}
}

func (a *policyRecordAction) callCount(nodeID string) int {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.calls[nodeID]
}

func (a *policyRecordAction) param(nodeID, name string) interface{} {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.params[nodeID][name]
}

func newErrorPolicyEngine(t *testing.T) (*Engine, *storage.WorkflowStore, *policyRecordAction) {
	t.Helper()
	registry := nodes.NewPluginRegistry()
	recorder := newPolicyRecordAction()
	for _, executor := range []nodes.NodeExecutor{&policyFailAction{}, &policyPassAction{}, recorder} {
		if err := registry.Register(executor); err != nil {
			t.Fatalf("register executor: %v", err)
		}
	}

	db, err := storage.NewDB(":memory:")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	wfStore := storage.NewWorkflowStore(db)
	eng := NewEngine(registry, storage.NewExecutionStore(db), storage.NewCredentialStore(db, nil), NewEventBus(), wfStore)
	return eng, wfStore, recorder
}

func executeErrorPolicyWorkflow(t *testing.T, eng *Engine, wfStore *storage.WorkflowStore, id string, nodeList []nodes.Node, edgeList []nodes.Edge) *storage.Execution {
	t.Helper()
	nodesJSON, err := json.Marshal(nodeList)
	if err != nil {
		t.Fatalf("marshal nodes: %v", err)
	}
	edgesJSON, err := json.Marshal(edgeList)
	if err != nil {
		t.Fatalf("marshal edges: %v", err)
	}
	wf := &storage.Workflow{ID: id, Name: id, NodesJSON: string(nodesJSON), EdgesJSON: string(edgesJSON)}
	if err := wfStore.Create(wf); err != nil {
		t.Fatalf("create workflow: %v", err)
	}
	exec, err := eng.ExecuteWorkflow(wf, nil)
	if err != nil {
		t.Fatalf("execute workflow: %v", err)
	}
	return exec
}

func logForNode(t *testing.T, exec *storage.Execution, nodeID string) NodeLog {
	t.Helper()
	var logs []NodeLog
	if err := json.Unmarshal([]byte(exec.LogsJSON), &logs); err != nil {
		t.Fatalf("decode logs: %v", err)
	}
	for _, item := range logs {
		if item.NodeID == nodeID {
			return item
		}
	}
	t.Fatalf("missing log for node %s", nodeID)
	return NodeLog{}
}

func TestNodeErrorPolicyDefaultsToStopWorkflow(t *testing.T) {
	eng, wfStore, recorder := newErrorPolicyEngine(t)
	exec := executeErrorPolicyWorkflow(t, eng, wfStore, "wf-policy-stop", []nodes.Node{
		{ID: "fail", Type: "policyFailAction", Params: map[string]interface{}{}},
		{ID: "normal", Type: "policyRecordAction", Params: map[string]interface{}{}},
	}, []nodes.Edge{{ID: "normal-edge", Source: "fail", Target: "normal"}})

	if exec.Status != "FAILED" {
		t.Fatalf("status = %s, want FAILED", exec.Status)
	}
	if recorder.callCount("normal") != 0 {
		t.Fatalf("normal branch calls = %d, want 0", recorder.callCount("normal"))
	}
	if got := logForNode(t, exec, "fail"); got.Status != "FAILED" {
		t.Fatalf("failure log status = %s, want FAILED", got.Status)
	}
}

func TestNodeErrorPolicyContinueRunsOnlyNormalOutput(t *testing.T) {
	eng, wfStore, recorder := newErrorPolicyEngine(t)
	exec := executeErrorPolicyWorkflow(t, eng, wfStore, "wf-policy-continue", []nodes.Node{
		{ID: "fail", Type: "policyFailAction", Params: map[string]interface{}{"on_error": nodes.ErrorPolicyContinueLabel}},
		{ID: "normal", Type: "policyRecordAction", Params: map[string]interface{}{"failed": "{{fail.failed}}"}},
		{ID: "error", Type: "policyRecordAction", Params: map[string]interface{}{}},
	}, []nodes.Edge{
		{ID: "normal-edge", Source: "fail", Target: "normal"},
		{ID: "error-edge", Source: "fail", SourceHandle: "error", Target: "error"},
	})

	if exec.Status != "SUCCESS" {
		t.Fatalf("status = %s, want SUCCESS", exec.Status)
	}
	if exec.ErrorMessage != "" {
		t.Fatalf("handled failure leaked execution error: %q", exec.ErrorMessage)
	}
	if recorder.callCount("normal") != 1 || recorder.callCount("error") != 0 {
		t.Fatalf("branch calls normal=%d error=%d, want 1/0", recorder.callCount("normal"), recorder.callCount("error"))
	}
	if recorder.param("normal", "failed") != true {
		t.Fatalf("continued node did not receive failure envelope: %#v", recorder.param("normal", "failed"))
	}
	failedLog := logForNode(t, exec, "fail")
	if failedLog.Status != "FAILED" || failedLog.Error == "" || failedLog.Output == nil {
		t.Fatalf("handled failure log lost visibility: %#v", failedLog)
	}
}

func TestNodeErrorPolicyErrorOutputRoutesOnlyErrorEdge(t *testing.T) {
	eng, wfStore, recorder := newErrorPolicyEngine(t)
	exec := executeErrorPolicyWorkflow(t, eng, wfStore, "wf-policy-error-output", []nodes.Node{
		{ID: "fail", Type: "policyFailAction", Params: map[string]interface{}{"on_error": nodes.ErrorPolicyErrorOutputLabel}},
		{ID: "normal", Type: "policyRecordAction", Params: map[string]interface{}{}},
		{ID: "error", Type: "policyRecordAction", Params: map[string]interface{}{"message": "{{fail.error}}", "policy": "{{fail.policy}}"}},
	}, []nodes.Edge{
		{ID: "normal-edge", Source: "fail", Target: "normal"},
		{ID: "error-edge", Source: "fail", SourceHandle: "error", Target: "error"},
	})

	if exec.Status != "SUCCESS" {
		t.Fatalf("status = %s, want SUCCESS", exec.Status)
	}
	if recorder.callCount("normal") != 0 || recorder.callCount("error") != 1 {
		t.Fatalf("branch calls normal=%d error=%d, want 0/1", recorder.callCount("normal"), recorder.callCount("error"))
	}
	if recorder.param("error", "message") != "policy test failure" {
		t.Fatalf("error message param = %#v", recorder.param("error", "message"))
	}
	if recorder.param("error", "policy") != string(nodes.ErrorPolicyErrorOutput) {
		t.Fatalf("error policy param = %#v", recorder.param("error", "policy"))
	}
}

func TestNodeErrorPolicyErrorOutputWithoutErrorEdgeFailsClosed(t *testing.T) {
	eng, wfStore, recorder := newErrorPolicyEngine(t)
	exec := executeErrorPolicyWorkflow(t, eng, wfStore, "wf-policy-error-output-missing", []nodes.Node{
		{ID: "fail", Type: "policyFailAction", Params: map[string]interface{}{"on_error": nodes.ErrorPolicyErrorOutputLabel}},
		{ID: "normal", Type: "policyRecordAction", Params: map[string]interface{}{}},
	}, []nodes.Edge{{ID: "normal-edge", Source: "fail", Target: "normal"}})

	if exec.Status != "FAILED" {
		t.Fatalf("status = %s, want FAILED when error route is missing", exec.Status)
	}
	if recorder.callCount("normal") != 0 {
		t.Fatalf("normal branch calls = %d, want 0", recorder.callCount("normal"))
	}
}

func TestSuccessfulNodeNeverActivatesReservedErrorOutput(t *testing.T) {
	eng, wfStore, recorder := newErrorPolicyEngine(t)
	exec := executeErrorPolicyWorkflow(t, eng, wfStore, "wf-policy-success", []nodes.Node{
		{ID: "pass", Type: "policyPassAction", Params: map[string]interface{}{"on_error": nodes.ErrorPolicyErrorOutputLabel}},
		{ID: "normal", Type: "policyRecordAction", Params: map[string]interface{}{}},
		{ID: "error", Type: "policyRecordAction", Params: map[string]interface{}{}},
	}, []nodes.Edge{
		{ID: "normal-edge", Source: "pass", Target: "normal"},
		{ID: "error-edge", Source: "pass", SourceHandle: "error", Target: "error"},
	})

	if exec.Status != "SUCCESS" {
		t.Fatalf("status = %s, want SUCCESS", exec.Status)
	}
	if recorder.callCount("normal") != 1 || recorder.callCount("error") != 0 {
		t.Fatalf("branch calls normal=%d error=%d, want 1/0", recorder.callCount("normal"), recorder.callCount("error"))
	}
}
