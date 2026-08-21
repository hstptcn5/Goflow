package nodes

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/dop251/goja"
)

const (
	defaultJSTimeoutSeconds = 5
	maxJSTimeoutSeconds     = 30
	maxJSCodeBytes          = 256 << 10
)

type JSCodeRunnerExecutor struct{}

func NewJSCodeRunnerExecutor() *JSCodeRunnerExecutor { return &JSCodeRunnerExecutor{} }

func parseJSTimeout(raw interface{}) (int, error) {
	if raw == nil || strings.TrimSpace(fmt.Sprint(raw)) == "" {
		return defaultJSTimeoutSeconds, nil
	}
	var value int
	switch typed := raw.(type) {
	case int:
		value = typed
	case int64:
		value = int(typed)
	case float64:
		if typed != float64(int(typed)) {
			return 0, fmt.Errorf("JavaScript timeout must be an integer")
		}
		value = int(typed)
	case string:
		parsed, err := strconv.Atoi(strings.TrimSpace(typed))
		if err != nil {
			return 0, fmt.Errorf("JavaScript timeout must be an integer")
		}
		value = parsed
	default:
		return 0, fmt.Errorf("JavaScript timeout must be an integer")
	}
	if value < 1 || value > maxJSTimeoutSeconds {
		return 0, fmt.Errorf("JavaScript timeout must be between 1 and %d seconds", maxJSTimeoutSeconds)
	}
	return value, nil
}

func validateJSCodeRunner(node *Node) error {
	codeStr, _ := node.Params["code"].(string)
	if strings.TrimSpace(codeStr) == "" {
		return fmt.Errorf("JavaScript code is required")
	}
	if len(codeStr) > maxJSCodeBytes {
		return fmt.Errorf("JavaScript code exceeds %d byte limit", maxJSCodeBytes)
	}
	if timeoutText, ok := node.Params["timeout"].(string); ok && containsTemplateExpression(timeoutText) {
		return nil
	}
	_, err := parseJSTimeout(node.Params["timeout"])
	return err
}

func (e *JSCodeRunnerExecutor) Execute(ctx *ExecutionContext, node *Node) (interface{}, error) {
	codeStr, _ := node.Params["code"].(string)
	if strings.TrimSpace(codeStr) == "" {
		return nil, fmt.Errorf("JavaScript code is required")
	}
	if len(codeStr) > maxJSCodeBytes {
		return nil, fmt.Errorf("JavaScript code exceeds %d byte limit", maxJSCodeBytes)
	}
	var jsonResult interface{}
	if err := json.Unmarshal([]byte(codeStr), &jsonResult); err == nil {
		return jsonResult, nil
	}
	timeoutSeconds, err := parseJSTimeout(node.Params["timeout"])
	if err != nil {
		return nil, err
	}

	vm := goja.New()
	outputsCopy := ctx.GetOutputs()
	_ = vm.Set("outputs", outputsCopy)
	_ = vm.Set("input", node.Params["input"])
	if trigger, ok := outputsCopy["$trigger"]; ok {
		_ = vm.Set("trigger", trigger)
	}

	scriptToRun := codeStr
	if strings.Contains(codeStr, "return") {
		scriptToRun = fmt.Sprintf("(function(){\n%s\n})()", codeStr)
	}

	done := make(chan struct{})
	defer close(done)
	timer := time.AfterFunc(time.Duration(timeoutSeconds)*time.Second, func() { vm.Interrupt("timeout") })
	defer timer.Stop()
	go func() {
		select {
		case <-ctx.Context.Done():
			vm.Interrupt("cancelled")
		case <-done:
		}
	}()

	val, err := vm.RunString(scriptToRun)
	if err != nil {
		if ctx.Context.Err() != nil {
			return nil, ctx.Context.Err()
		}
		return nil, fmt.Errorf("JS evaluation error: %w", err)
	}
	if val == nil || goja.IsUndefined(val) || goja.IsNull(val) {
		return nil, nil
	}
	return val.Export(), nil
}

func (e *JSCodeRunnerExecutor) Validate(node *Node) error { return validateJSCodeRunner(node) }

func (e *JSCodeRunnerExecutor) GetDefinition() NodeDefinition {
	return NodeDefinition{
		Type: TypeJSCodeRunner, Name: "JS Code Runner", Description: "Runs bounded custom JavaScript or JSON expressions to transform data", Icon: "Code", Category: "LOGIC & UTILITY", Retryable: true,
		Params: []ParamDefinition{
			{Name: "input", Label: "Input Value", Type: "text", Default: "", Required: false, Description: "Optional setup-bound value exposed to code as input"},
			{Name: "code", Label: "JavaScript Code / JSON Expression", Type: "textarea", Control: "code", Language: "javascript", Default: "{\n  \"status\": \"processed\",\n  \"message\": \"Custom Code Execution\"\n}", Required: true, Description: "JavaScript code or JSON expression to execute"},
			{Name: "timeout", Label: "Execution Timeout (Seconds)", Type: "text", Default: "5", Required: false, Advanced: true, Description: "Maximum script runtime, between 1 and 30 seconds"},
		},
	}
}
