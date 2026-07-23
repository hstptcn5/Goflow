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

func NewJSCodeRunnerExecutor() *JSCodeRunnerExecutor {
	return &JSCodeRunnerExecutor{}
}

func (e *JSCodeRunnerExecutor) Execute(ctx *ExecutionContext, node *Node) (interface{}, error) {
	codeStr, _ := node.Params["code"].(string)
	if strings.TrimSpace(codeStr) == "" {
		codeStr = "return { status: 'processed', timestamp: new Date() };"
	}

	// 1. If the input is valid JSON, parse and return it immediately
	var jsonResult map[string]interface{}
	if err := json.Unmarshal([]byte(codeStr), &jsonResult); err == nil {
		return jsonResult, nil
	}

	// Resolve timeout parameter (default to 5 seconds)
	timeoutSeconds := 5
	if timeoutVal, ok := node.Params["timeout"]; ok {
		switch t := timeoutVal.(type) {
		case string:
			if val, err := strconv.Atoi(t); err == nil && val > 0 {
				timeoutSeconds = val
			}
		case float64:
			if t > 0 {
				timeoutSeconds = int(t)
			}
		case int:
			if t > 0 {
				timeoutSeconds = t
			}
		}
	}

	// 2. Otherwise execute actual JavaScript using Goja engine
	vm := goja.New()

	ctx.mu.RLock()
	outputsCopy := make(map[string]interface{})
	for k, v := range ctx.Outputs {
		outputsCopy[k] = v
	}
	ctx.mu.RUnlock()

	_ = vm.Set("outputs", outputsCopy)
	if trigger, ok := outputsCopy["$trigger"]; ok {
		_ = vm.Set("trigger", trigger)
	}

	var scriptToRun string
	if strings.Contains(codeStr, "return") {
		scriptToRun = fmt.Sprintf("(function(){\n%s\n})()", codeStr)
	} else {
		scriptToRun = codeStr
	}

	// Set interrupt timer
	timer := time.AfterFunc(time.Duration(timeoutSeconds)*time.Second, func() {
		vm.Interrupt("timeout")
	})
	defer timer.Stop()

	val, err := vm.RunString(scriptToRun)
	if err != nil {
		return nil, fmt.Errorf("JS evaluation error: %w", err)
	}

	if val == nil {
		return nil, nil
	}

	return val.Export(), nil
}

func (e *JSCodeRunnerExecutor) Validate(node *Node) error {
	return nil
}

func (e *JSCodeRunnerExecutor) GetDefinition() NodeDefinition {
	return NodeDefinition{
		Type:        TypeJSCodeRunner,
		Name:        "JS Code Runner",
		Description: "Runs custom JavaScript or JSON expressions to transform data",
		Icon:        "Code",
		Category:    "LOGIC & UTILITY",
		Retryable:   true,
		Params: []ParamDefinition{
			{
				Name:        "code",
				Label:       "JavaScript Code / JSON Expression",
				Type:        "textarea",
				Default:     "{\n  \"status\": \"processed\",\n  \"message\": \"Custom Code Execution\"\n}",
				Required:    true,
				Description: "JavaScript code or JSON expression to execute",
			},
			{
				Name:        "timeout",
				Label:       "Execution Timeout (Seconds)",
				Type:        "text",
				Default:     "5",
				Required:    false,
				Description: "Maximum script runtime in seconds",
			},
		},
	}
}
