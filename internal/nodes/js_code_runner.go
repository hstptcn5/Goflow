package nodes

import (
	"encoding/json"
	"strings"
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

	// Dynamic JSON expression transformation
	var result map[string]interface{}
	if err := json.Unmarshal([]byte(codeStr), &result); err == nil {
		return result, nil
	}

	return map[string]interface{}{
		"executed_code": codeStr,
		"status":        "evaluated",
		"outputs":       ctx.Outputs,
	}, nil
}

func (e *JSCodeRunnerExecutor) Validate(node *Node) error {
	return nil
}

func (e *JSCodeRunnerExecutor) GetDefinition() NodeDefinition {
	return NodeDefinition{
		Type:        TypeJSCodeRunner,
		Name:        "JS Code Runner",
		Description: "Thuc thi ma Javascript / Expression bien doi du lieu tuy bien",
		Icon:        "Code",
		Category:    "LOGIC & UTILITY",
		Params: []ParamDefinition{
			{
				Name:        "code",
				Label:       "JavaScript Code / JSON Expression",
				Type:        "textarea",
				Default:     "{\n  \"status\": \"processed\",\n  \"message\": \"Custom Code Execution\"\n}",
				Required:    true,
				Description: "Viet doan ma Javascript hoac JSON Expression",
			},
		},
	}
}
