package nodes

import (
	"os/exec"
	"strings"
	"testing"
	"time"
)

func availablePythonForTest(t *testing.T) string {
	t.Helper()
	for _, name := range []string{"python3", "python", "py"} {
		if path, err := exec.LookPath(name); err == nil {
			return path
		}
	}
	t.Skip("Python interpreter is not available on this runner")
	return ""
}

func TestPythonCodeNodeExecutesJSONProtocol(t *testing.T) {
	python := availablePythonForTest(t)
	ctx := NewExecutionContext("wf", "exec")
	ctx.SetOutput("source", map[string]interface{}{"value": 3})
	ctx.SetOutput("$trigger", map[string]interface{}{"name": "demo"})
	executor := NewPythonCodeExecutor()
	out, err := executor.Execute(ctx, &Node{Params: map[string]interface{}{
		"interpreter": python,
		"input":       map[string]interface{}{"n": 4},
		"code": `print("debug line")
output = {"sum": input["n"] + outputs["source"]["value"], "trigger": trigger["name"]}`,
		"timeout": 5,
	}})
	if err != nil {
		t.Fatal(err)
	}
	result := out.(map[string]interface{})
	if result["sum"] != float64(7) || result["trigger"] != "demo" {
		t.Fatalf("unexpected Python result %#v", result)
	}
}

func TestPythonCodeNodeDoesNotExposeCredentials(t *testing.T) {
	python := availablePythonForTest(t)
	ctx := NewExecutionContext("wf", "exec")
	ctx.Credentials["secret"] = "do-not-expose"
	out, err := NewPythonCodeExecutor().Execute(ctx, &Node{Params: map[string]interface{}{
		"interpreter": python,
		"code":        `output = {"has_credentials": "credentials" in globals()}`,
		"timeout":     5,
	}})
	if err != nil {
		t.Fatal(err)
	}
	if out.(map[string]interface{})["has_credentials"] != false {
		t.Fatalf("credentials leaked into Python namespace: %#v", out)
	}
}

func TestPythonCodeNodeTimeout(t *testing.T) {
	python := availablePythonForTest(t)
	start := time.Now()
	_, err := NewPythonCodeExecutor().Execute(NewExecutionContext("wf", "exec"), &Node{Params: map[string]interface{}{
		"interpreter": python,
		"code": `import time
time.sleep(10)
output = 1`,
		"timeout": 1,
	}})
	if err == nil || !strings.Contains(strings.ToLower(err.Error()), "timed out") {
		t.Fatalf("expected timeout error, got %v", err)
	}
	if elapsed := time.Since(start); elapsed > 5*time.Second {
		t.Fatalf("Python timeout took too long: %v", elapsed)
	}
}

func TestPythonProfilesFromEnvironment(t *testing.T) {
	t.Setenv("GOFLOW_PYTHON_PROFILES_JSON", `{"data":"/opt/data-python"}`)
	profiles, err := pythonProfiles()
	if err != nil {
		t.Fatal(err)
	}
	if profiles["data"] != "/opt/data-python" {
		t.Fatalf("profiles = %#v", profiles)
	}
	if _, err := resolvePythonInterpreter(&Node{Params: map[string]interface{}{"environment": "missing"}}); err == nil {
		t.Fatal("missing named Python environment was accepted")
	}
}
