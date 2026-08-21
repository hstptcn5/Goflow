package nodes

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
)

const (
	defaultPythonTimeoutSeconds = 10
	maxPythonTimeoutSeconds     = 120
	maxPythonCodeBytes          = 256 << 10
	maxPythonInputBytes         = 2 << 20
	maxPythonOutputBytes        = 10 << 20
	maxPythonStderrBytes        = 1 << 20
)

const pythonBootstrap = `import contextlib
import io
import json
import sys
import traceback

payload = json.load(sys.stdin)
namespace = {
    "input": payload.get("input"),
    "outputs": payload.get("outputs", {}),
    "trigger": payload.get("trigger"),
}
stdout_capture = io.StringIO()
try:
    with contextlib.redirect_stdout(stdout_capture):
        exec(compile(payload.get("code", ""), "<goflow-python-node>", "exec"), namespace, namespace)
    envelope = {
        "ok": True,
        "output": namespace.get("output"),
        "stdout": stdout_capture.getvalue(),
    }
    sys.stdout.write(json.dumps(envelope, ensure_ascii=False, separators=(",", ":")))
except BaseException:
    sys.stderr.write(stdout_capture.getvalue())
    traceback.print_exc(file=sys.stderr)
    sys.exit(1)
`

type PythonCodeExecutor struct{}

func NewPythonCodeExecutor() *PythonCodeExecutor { return &PythonCodeExecutor{} }

type boundedCapture struct {
	buf      bytes.Buffer
	limit    int
	exceeded bool
}

func (b *boundedCapture) Write(p []byte) (int, error) {
	original := len(p)
	remaining := b.limit - b.buf.Len()
	if remaining > 0 {
		if len(p) > remaining {
			_, _ = b.buf.Write(p[:remaining])
			b.exceeded = true
		} else {
			_, _ = b.buf.Write(p)
		}
	} else if len(p) > 0 {
		b.exceeded = true
	}
	return original, nil
}

func (b *boundedCapture) String() string { return b.buf.String() }

func parsePythonTimeout(raw interface{}) (int, error) {
	if raw == nil || strings.TrimSpace(fmt.Sprint(raw)) == "" {
		return defaultPythonTimeoutSeconds, nil
	}
	var value int
	switch typed := raw.(type) {
	case int:
		value = typed
	case int64:
		value = int(typed)
	case float64:
		if typed != float64(int(typed)) {
			return 0, fmt.Errorf("Python timeout must be an integer")
		}
		value = int(typed)
	case string:
		parsed, err := strconv.Atoi(strings.TrimSpace(typed))
		if err != nil {
			return 0, fmt.Errorf("Python timeout must be an integer")
		}
		value = parsed
	default:
		return 0, fmt.Errorf("Python timeout must be an integer")
	}
	if value < 1 || value > maxPythonTimeoutSeconds {
		return 0, fmt.Errorf("Python timeout must be between 1 and %d seconds", maxPythonTimeoutSeconds)
	}
	return value, nil
}

func pythonProfiles() (map[string]string, error) {
	profiles := map[string]string{}
	raw := strings.TrimSpace(os.Getenv("GOFLOW_PYTHON_PROFILES_JSON"))
	if raw == "" {
		return profiles, nil
	}
	if err := json.Unmarshal([]byte(raw), &profiles); err != nil {
		return nil, fmt.Errorf("GOFLOW_PYTHON_PROFILES_JSON must be an object of profile names to interpreter paths: %w", err)
	}
	for name, path := range profiles {
		if strings.TrimSpace(name) == "" || strings.TrimSpace(path) == "" {
			return nil, fmt.Errorf("Python profile names and interpreter paths must be non-empty")
		}
	}
	return profiles, nil
}

func pythonEnvironmentOptions() []string {
	options := []string{"default"}
	profiles, err := pythonProfiles()
	if err != nil {
		return options
	}
	names := make([]string, 0, len(profiles))
	for name := range profiles {
		name = strings.TrimSpace(name)
		if name != "" && name != "default" {
			names = append(names, name)
		}
	}
	sort.Strings(names)
	return append(options, names...)
}

func resolvePythonInterpreter(node *Node) (string, error) {
	if explicit := strings.TrimSpace(conditionValueString(node.Params["interpreter"])); explicit != "" {
		return explicit, nil
	}
	profile := strings.TrimSpace(conditionValueString(node.Params["environment"]))
	if profile == "" {
		profile = "default"
	}
	profiles, err := pythonProfiles()
	if err != nil {
		return "", err
	}
	if configured := strings.TrimSpace(profiles[profile]); configured != "" {
		return configured, nil
	}
	if profile != "default" {
		return "", fmt.Errorf("Python environment %q is not configured in GOFLOW_PYTHON_PROFILES_JSON", profile)
	}
	for _, candidate := range []string{"python3", "python", "py"} {
		if path, err := exec.LookPath(candidate); err == nil {
			return path, nil
		}
	}
	return "", fmt.Errorf("no Python interpreter found; configure an interpreter path or GOFLOW_PYTHON_PROFILES_JSON")
}

func validatePythonCodeNode(node *Node) error {
	code, _ := node.Params["code"].(string)
	if strings.TrimSpace(code) == "" {
		return fmt.Errorf("Python code is required")
	}
	if len(code) > maxPythonCodeBytes {
		return fmt.Errorf("Python code exceeds %d byte limit", maxPythonCodeBytes)
	}
	if timeoutText, ok := node.Params["timeout"].(string); ok && containsTemplateExpression(timeoutText) {
		return nil
	}
	if _, err := parsePythonTimeout(node.Params["timeout"]); err != nil {
		return err
	}
	if workingDir := strings.TrimSpace(conditionValueString(node.Params["working_directory"])); workingDir != "" && !containsTemplateExpression(workingDir) {
		info, err := os.Stat(workingDir)
		if err != nil {
			return fmt.Errorf("Python working directory is not accessible: %w", err)
		}
		if !info.IsDir() {
			return fmt.Errorf("Python working directory must be a directory")
		}
	}
	return nil
}

func (e *PythonCodeExecutor) Execute(ctx *ExecutionContext, node *Node) (interface{}, error) {
	if err := validatePythonCodeNode(node); err != nil {
		return nil, err
	}
	interpreter, err := resolvePythonInterpreter(node)
	if err != nil {
		return nil, err
	}
	timeoutSeconds, err := parsePythonTimeout(node.Params["timeout"])
	if err != nil {
		return nil, err
	}

	outputs := ctx.GetOutputs()
	payload := map[string]interface{}{
		"input":   node.Params["input"],
		"outputs": outputs,
		"code":    node.Params["code"],
	}
	if trigger, ok := outputs["$trigger"]; ok {
		payload["trigger"] = trigger
	}
	encoded, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("encode Python input: %w", err)
	}
	if len(encoded) > maxPythonInputBytes {
		return nil, fmt.Errorf("Python input exceeds %d byte limit", maxPythonInputBytes)
	}

	runCtx, cancel := context.WithTimeout(ctx.Context, time.Duration(timeoutSeconds)*time.Second)
	defer cancel()
	cmd := exec.CommandContext(runCtx, interpreter, "-c", pythonBootstrap)
	if workingDir := strings.TrimSpace(conditionValueString(node.Params["working_directory"])); workingDir != "" {
		abs, err := filepath.Abs(workingDir)
		if err != nil {
			return nil, fmt.Errorf("resolve Python working directory: %w", err)
		}
		cmd.Dir = abs
	}
	cmd.Stdin = bytes.NewReader(encoded)
	stdout := &boundedCapture{limit: maxPythonOutputBytes}
	stderr := &boundedCapture{limit: maxPythonStderrBytes}
	cmd.Stdout = stdout
	cmd.Stderr = stderr

	err = cmd.Run()
	if runCtx.Err() != nil {
		if ctx.Context.Err() != nil {
			return nil, ctx.Context.Err()
		}
		return nil, fmt.Errorf("Python execution timed out after %d seconds", timeoutSeconds)
	}
	if stdout.exceeded {
		return nil, fmt.Errorf("Python protocol output exceeds %d byte limit", maxPythonOutputBytes)
	}
	if err != nil {
		message := strings.TrimSpace(stderr.String())
		if stderr.exceeded {
			message += "\n[stderr truncated]"
		}
		if message == "" {
			message = err.Error()
		}
		return nil, fmt.Errorf("Python execution failed: %s", message)
	}

	var envelope struct {
		OK     bool        `json:"ok"`
		Output interface{} `json:"output"`
		Stdout string      `json:"stdout"`
	}
	if err := json.Unmarshal(stdout.buf.Bytes(), &envelope); err != nil {
		return nil, fmt.Errorf("Python node returned an invalid protocol response: %w", err)
	}
	if !envelope.OK {
		return nil, fmt.Errorf("Python node returned an unsuccessful protocol response")
	}
	return envelope.Output, nil
}

func (e *PythonCodeExecutor) Validate(node *Node) error { return validatePythonCodeNode(node) }

func (e *PythonCodeExecutor) GetDefinition() NodeDefinition {
	return NodeDefinition{
		Type: TypePythonCode, Name: "Python Code", Description: "Runs trusted local Python in an external CPython environment for custom computation", Icon: "Code2", Category: "LOGIC & UTILITY", Retryable: false,
		Params: []ParamDefinition{
			{Name: "environment", Label: "Python Environment", Type: "select", Options: pythonEnvironmentOptions(), Default: "default", Required: false, Description: "Named profile from GOFLOW_PYTHON_PROFILES_JSON; default auto-discovers CPython"},
			{Name: "interpreter", Label: "Interpreter Path", Type: "text", Default: "", Required: false, Advanced: true, Description: "Optional direct python/python.exe path overriding the environment profile"},
			{Name: "input", Label: "Input Value", Type: "json", Default: "null", Required: false, Description: "Value exposed to code as input"},
			{Name: "code", Label: "Python Code", Type: "textarea", Default: "output = {\"status\": \"processed\"}", Required: true, Control: "code", Language: "python", Description: "Set the variable output to a JSON-compatible result. Trusted code runs with the Goflow OS account permissions."},
			{Name: "timeout", Label: "Execution Timeout (Seconds)", Type: "number", Default: 10, Required: false, Advanced: true, Description: "Maximum runtime, between 1 and 120 seconds"},
			{Name: "working_directory", Label: "Working Directory", Type: "text", Default: "", Required: false, Advanced: true, Description: "Optional working directory. Python v1 is trusted local code, not a security sandbox."},
		},
	}
}
