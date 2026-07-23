package nodes

import (
	"reflect"
	"testing"
)

func TestJSCodeRunnerExecutor(t *testing.T) {
	executor := NewJSCodeRunnerExecutor()
	ctx := NewExecutionContext("wf-1", "exec-1")
	ctx.SetOutput("node1", map[string]interface{}{
		"val": 42,
	})

	// Test 1: JSON fallback
	nodeJSON := &Node{
		Params: map[string]interface{}{
			"code": `{"status": "ok", "count": 10}`,
		},
	}
	res1, err := executor.Execute(ctx, nodeJSON)
	if err != nil {
		t.Fatalf("Test 1 failed with error: %v", err)
	}
	// JSON unmarshals numbers to float64, wait! jsonResult map[string]interface{} will have float64 for 10
	// Let's expect count to be float64(10)
	expectedJSON := map[string]interface{}{"status": "ok", "count": float64(10)}
	if !reflect.DeepEqual(res1, expectedJSON) {
		t.Errorf("Expected res1 to be %v, got %v", expectedJSON, res1)
	}

	// Test 2: Actual Javascript execution (no return statement, just expression)
	nodeJS1 := &Node{
		Params: map[string]interface{}{
			"code": `var x = outputs.node1.val; x * 2;`,
		},
	}
	res2, err := executor.Execute(ctx, nodeJS1)
	if err != nil {
		t.Fatalf("Test 2 failed with error: %v", err)
	}
	if res2 != int64(84) {
		t.Errorf("Expected res2 to be 84, got %v (%T)", res2, res2)
	}

	// Test 3: Actual Javascript execution (with return statement)
	nodeJS2 := &Node{
		Params: map[string]interface{}{
			"code": `
				var val = outputs.node1.val;
				if (val === 42) {
					return { matched: true, calculated: val + 8 };
				}
				return { matched: false };
			`,
		},
	}
	res3, err := executor.Execute(ctx, nodeJS2)
	if err != nil {
		t.Fatalf("Test 3 failed with error: %v", err)
	}
	expected3 := map[string]interface{}{
		"matched":    true,
		"calculated": int64(50),
	}
	if !reflect.DeepEqual(res3, expected3) {
		t.Errorf("Expected res3 to be %v, got %v", expected3, res3)
	}

	// Test 4: JS execution with timeout (infinite loop)
	nodeJS3 := &Node{
		Params: map[string]interface{}{
			"code":    `while(true) {}`,
			"timeout": "1", // 1 second timeout
		},
	}
	_, err = executor.Execute(ctx, nodeJS3)
	if err == nil {
		t.Fatalf("Expected timeout error, but got nil")
	}
	if !reflect.ValueOf(err).IsValid() || !reflect.ValueOf(err.Error()).IsValid() {
		t.Fatalf("Invalid error returned")
	}
	if !reflect.ValueOf(err.Error()).String() != "" && reflect.ValueOf(err.Error()).String() != "JS evaluation error: timeout" && !reflect.ValueOf(err.Error()).String().Contains("timeout") {
		// Wait, let's keep it simple: just check if error string contains "timeout"
	}
	// Let's do standard error check:
	errStr := err.Error()
	if !reflect.ValueOf(errStr).String().Contains("timeout") && !reflect.ValueOf(errStr).String().Contains("interrupted") {
		t.Errorf("Expected error to contain 'timeout' or 'interrupted', got: %v", errStr)
	}
}

