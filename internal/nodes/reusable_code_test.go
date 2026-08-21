package nodes

import (
	"fmt"
	"os"
	"testing"
)

func TestReusableJSCodePreservesSingleInputContract(t *testing.T) {
	manifest := ReusableCodeManifest{
		SchemaVersion: 1,
		Type:          "user.double_value",
		Name:          "Double value",
		Version:       "1.0.0",
		Runtime:       "js",
		Code:          `return input.value * 2`,
		Inputs:        []ParamDefinition{{Name: "input", Label: "Input", Type: "json"}},
	}
	executor, err := NewReusableCodeExecutor(manifest)
	if err != nil {
		t.Fatal(err)
	}
	out, err := executor.Execute(NewExecutionContext("wf", "exec"), &Node{Params: map[string]interface{}{
		"input": map[string]interface{}{"value": 4},
	}})
	if err != nil {
		t.Fatal(err)
	}
	if fmt.Sprint(out) != "8" {
		t.Fatalf("output = %#v", out)
	}
}

func TestSaveAndDiscoverReusableCodeManifest(t *testing.T) {
	dir := t.TempDir()
	manifest := ReusableCodeManifest{
		SchemaVersion: 1,
		Type:          "user.echo",
		Name:          "Echo",
		Version:       "2.1.0",
		Runtime:       "js",
		Code:          `return input`,
		Inputs:        []ParamDefinition{{Name: "input", Label: "Input", Type: "json"}},
	}
	path, err := SaveReusableCodeManifest(dir, manifest)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatal(err)
	}
	executors, errs := DiscoverReusableCodeExecutors(dir)
	if len(errs) != 0 || len(executors) != 1 {
		t.Fatalf("executors=%d errs=%v", len(executors), errs)
	}
	if executors[0].GetDefinition().Type != "user.echo" || executors[0].GetDefinition().Version != "2.1.0" {
		t.Fatalf("definition = %#v", executors[0].GetDefinition())
	}
}
