package nodes

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/dop251/goja"
)

type JSCodeRunnerExecutor struct{}

func NewJSCodeRunnerExecutor() *JSCodeRunnerExecutor { return &JSCodeRunnerExecutor{} }

func (e *JSCodeRunnerExecutor) Execute(ctx *ExecutionContext, node *Node) (interface{}, error) {
	codeStr, _ := node.Params["code"].(string)
	if strings.TrimSpace(codeStr) == "" {
		codeStr = "return { status: 'processed', timestamp: new Date() };"
	}
	var jsonResult map[string]interface{}
	if err := json.Unmarshal([]byte(codeStr), &jsonResult); err == nil {
		return jsonResult, nil
	}

	timeoutSeconds := 5
	if timeoutVal, ok := node.Params["timeout"]; ok {
		switch value := timeoutVal.(type) {
		case string:
			if parsed, err := strconv.Atoi(value); err == nil && parsed > 0 { timeoutSeconds = parsed }
		case float64:
			if value > 0 { timeoutSeconds = int(value) }
		case int:
			if value > 0 { timeoutSeconds = value }
		}
	}

	vm := goja.New()
	ctx.mu.RLock()
	outputsCopy := make(map[string]interface{}, len(ctx.Outputs))
	for key, value := range ctx.Outputs { outputsCopy[key] = value }
	ctx.mu.RUnlock()
	_ = vm.Set("outputs", outputsCopy)
	_ = vm.Set("input", node.Params["input"])
	if trigger, ok := outputsCopy["$trigger"]; ok { _ = vm.Set("trigger", trigger) }

	scriptToRun := codeStr
	if strings.Contains(codeStr, "return") { scriptToRun = fmt.Sprintf("(function(){\n%s\n})()", codeStr) }
	timer := time.AfterFunc(time.Duration(timeoutSeconds)*time.Second, func() { vm.Interrupt("timeout") })
	defer timer.Stop()
	val, err := vm.RunString(scriptToRun)
	if err != nil { return nil, fmt.Errorf("JS evaluation error: %w", err) }
	if val == nil { return nil, nil }
	return val.Export(), nil
}

func (e *JSCodeRunnerExecutor) Validate(node *Node) error { return nil }

func (e *JSCodeRunnerExecutor) GetDefinition() NodeDefinition {
	return NodeDefinition{
		Type: TypeJSCodeRunner, Name: "JS Code Runner", Description: "Runs custom JavaScript or JSON expressions to transform data", Icon: "Code", Category: "LOGIC & UTILITY", Retryable: true,
		Params: []ParamDefinition{
			{Name: "input", Label: "Input Value", Type: "text", Default: "", Required: false, Description: "Optional setup-bound value exposed to code as input"},
			{Name: "code", Label: "JavaScript Code / JSON Expression", Type: "textarea", Default: "{\n  \"status\": \"processed\",\n  \"message\": \"Custom Code Execution\"\n}", Required: true, Description: "JavaScript code or JSON expression to execute"},
			{Name: "timeout", Label: "Execution Timeout (Seconds)", Type: "text", Default: "5", Required: false, Description: "Maximum script runtime in seconds"},
		},
	}
}
