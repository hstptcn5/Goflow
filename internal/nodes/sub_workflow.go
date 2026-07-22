package nodes

import (
	"encoding/json"
	"fmt"
	"sync"
)

type SubWorkflowExecutor struct{}

func NewSubWorkflowExecutor() *SubWorkflowExecutor {
	return &SubWorkflowExecutor{}
}

func (e *SubWorkflowExecutor) Execute(ctx *ExecutionContext, node *Node) (interface{}, error) {
	subWfID, _ := node.Params["sub_workflow_id"].(string)
	if subWfID == "" {
		return nil, fmt.Errorf("sub_workflow_id is required")
	}

	payloadJSON, _ := node.Params["payload_json"].(string)
	var payload interface{}
	if payloadJSON != "" {
		if err := json.Unmarshal([]byte(payloadJSON), &payload); err != nil {
			// Fallback: treat as raw string if it is not valid JSON
			payload = payloadJSON
		}
	}

	loopMode, _ := node.Params["loop_mode"].(bool)
	parallel, _ := node.Params["parallel"].(bool)

	if ctx.ExecuteWorkflow == nil {
		return nil, fmt.Errorf("ExecuteWorkflow callback is not initialized in context")
	}

	// 1. Loop Mode: Iterate over array payload
	if loopMode {
		slicePayload, isArray := payload.([]interface{})
		if !isArray {
			// If not array, wrap in an array of 1 element
			slicePayload = []interface{}{payload}
		}

		results := make([]interface{}, len(slicePayload))
		var errorsList []string
		var errMu sync.Mutex

		if parallel {
			var wg sync.WaitGroup
			for i, item := range slicePayload {
				wg.Add(1)
				go func(idx int, it interface{}) {
					defer wg.Done()
					res, err := ctx.ExecuteWorkflow(subWfID, it)
					errMu.Lock()
					defer errMu.Unlock()
					if err != nil {
						errorsList = append(errorsList, fmt.Sprintf("Item %d failed: %v", idx, err))
					} else {
						results[idx] = res
					}
				}(i, item)
			}
			wg.Wait()
		} else {
			for i, item := range slicePayload {
				res, err := ctx.ExecuteWorkflow(subWfID, item)
				if err != nil {
					errorsList = append(errorsList, fmt.Sprintf("Item %d failed: %v", i, err))
				} else {
					results[i] = res
				}
			}
		}

		if len(errorsList) > 0 {
			return results, fmt.Errorf("loop execution completed with errors: %s", errorsList)
		}
		return results, nil
	}

	// 2. Simple execution (Single sub-workflow run)
	return ctx.ExecuteWorkflow(subWfID, payload)
}

func (e *SubWorkflowExecutor) Validate(node *Node) error {
	subWfID, _ := node.Params["sub_workflow_id"].(string)
	if subWfID == "" {
		return fmt.Errorf("sub_workflow_id is required")
	}
	return nil
}

func (e *SubWorkflowExecutor) GetDefinition() NodeDefinition {
	return NodeDefinition{
		Type:        TypeSubWorkflow,
		Name:        "Sub-workflow Runner",
		Description: "Chạy workflow con theo cách tuần tự hoặc vòng lặp song song bằng Goroutine",
		Icon:        "Folder",
		Category:    "LOGIC & UTILITY",
		Params: []ParamDefinition{
			{
				Name:        "sub_workflow_id",
				Label:       "Sub-workflow to Run",
				Type:        "select",
				Required:    true,
				Description: "Chọn workflow con để thực thi",
			},
			{
				Name:        "payload_json",
				Label:       "Input Payload (JSON / Text)",
				Type:        "textarea",
				Default:     "{\n  \"message\": \"Input payload\"\n}",
				Required:    false,
				Description: "Điền input dưới dạng text, JSON hoặc biến số {{node.path}}",
			},
			{
				Name:        "loop_mode",
				Label:       "Loop mode (Run for each item in array)",
				Type:        "boolean",
				Default:     false,
				Required:    false,
				Description: "Biểu thị đầu vào payload là một mảng dữ liệu cần lặp qua",
			},
			{
				Name:        "parallel",
				Label:       "Run loop items in parallel (Goroutines)",
				Type:        "boolean",
				Default:     false,
				Required:    false,
				Description: "Thực thi vòng lặp song song sử dụng Goroutine hoặc chạy tuần tự",
			},
		},
	}
}
