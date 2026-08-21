package nodes

import (
	"reflect"
	"testing"
)

func TestWorkflowStateNodeUsesInjectedPersistentBackend(t *testing.T) {
	values := map[string]interface{}{}
	ctx := NewExecutionContext("wf-a", "exec")
	ctx.StateGet = func(scope, key string) (interface{}, bool, error) {
		value, ok := values[scope+":"+key]
		return value, ok, nil
	}
	ctx.StateSet = func(scope, key string, value interface{}) error {
		values[scope+":"+key] = value
		return nil
	}
	ctx.StateDelete = func(scope, key string) (bool, error) {
		lookup := scope + ":" + key
		_, ok := values[lookup]
		delete(values, lookup)
		return ok, nil
	}
	ctx.StateIncrement = func(scope, key string, delta float64) (float64, error) {
		lookup := scope + ":" + key
		current, _ := values[lookup].(float64)
		current += delta
		values[lookup] = current
		return current, nil
	}

	executor := NewWorkflowStateExecutor()
	setOut, err := executor.Execute(ctx, &Node{Params: map[string]interface{}{
		"operation": "SET", "scope": "Workflow", "key": "cursor", "value": `{"page":2}`,
	}})
	if err != nil {
		t.Fatal(err)
	}
	if got := setOut.(map[string]interface{})["value"]; !reflect.DeepEqual(got, map[string]interface{}{"page": float64(2)}) {
		t.Fatalf("set value = %#v", got)
	}

	getOut, err := executor.Execute(ctx, &Node{Params: map[string]interface{}{
		"operation": "GET", "scope": "Workflow", "key": "cursor",
	}})
	if err != nil {
		t.Fatal(err)
	}
	if getOut.(map[string]interface{})["found"] != true {
		t.Fatalf("get output = %#v", getOut)
	}

	incOut, err := executor.Execute(ctx, &Node{Params: map[string]interface{}{
		"operation": "INCREMENT", "scope": "Global", "key": "count", "delta": 2,
	}})
	if err != nil {
		t.Fatal(err)
	}
	if incOut.(map[string]interface{})["value"] != float64(2) {
		t.Fatalf("increment output = %#v", incOut)
	}

	deleteOut, err := executor.Execute(ctx, &Node{Params: map[string]interface{}{
		"operation": "DELETE", "scope": "Workflow", "key": "cursor",
	}})
	if err != nil {
		t.Fatal(err)
	}
	if deleteOut.(map[string]interface{})["deleted"] != true {
		t.Fatalf("delete output = %#v", deleteOut)
	}
}

func TestWorkflowStateDefinitionIsRegistered(t *testing.T) {
	registry := NewBuiltinRegistry()
	if _, ok := registry.Get(TypeWorkflowState); !ok {
		t.Fatal("Workflow State node is not registered")
	}
}
