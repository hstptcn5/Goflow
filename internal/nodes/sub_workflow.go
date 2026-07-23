package nodes

import (
	"encoding/json"
	"fmt"
	"strconv"
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
			maxConcurrency := 5
			if limitVal, ok := node.Params["concurrency_limit"]; ok {
				switch l := limitVal.(type) {
				case string:
					if val, err := strconv.Atoi(l); err == nil && val > 0 {
						maxConcurrency = val
					}
				case float64:
					if l > 0 {
						maxConcurrency = int(l)
					}
				case int:
					if l > 0 {
						maxConcurrency = l
					}
				}
			}

			sem := make(chan struct{}, maxConcurrency)
			var wg sync.WaitGroup
			for i, item := range slicePayload {
				wg.Add(1)
				sem <- struct{}{} // Acquire slot
				go func(idx int, it interface{}) {
					defer wg.Done()
					defer func() { <-sem }() // Release slot
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
		Description: "Runs a child workflow sequentially or in parallel loop mode",
		Icon:        "Folder",
		Category:    "LOGIC & UTILITY",
		Params: []ParamDefinition{
			{
				Name:        "sub_workflow_id",
				Label:       "Sub-workflow to Run",
				Type:        "select",
				Required:    true,
				Description: "Select the child workflow to execute",
			},
			{
				Name:        "payload_json",
				Label:       "Input Payload (JSON / Text)",
				Type:        "textarea",
				Default:     "{\n  \"message\": \"Input payload\"\n}",
				Required:    false,
				Description: "Input payload as text, JSON, or placeholders such as {{node.path}}",
			},
			{
				Name:        "loop_mode",
				Label:       "Loop mode (Run for each item in array)",
				Type:        "boolean",
				Default:     false,
				Required:    false,
				Description: "Treat the input payload as an array of items to iterate over",
			},
			{
				Name:        "parallel",
				Label:       "Run loop items in parallel (Goroutines)",
				Type:        "boolean",
				Default:     false,
				Required:    false,
				Description: "Run loop items in parallel goroutines or sequentially",
			},
			{
				Name:        "concurrency_limit",
				Label:       "Concurrency Limit",
				Type:        "text",
				Default:     "5",
				Required:    false,
				Description: "Maximum number of child workflow runs in parallel",
			},
		},
	}
}
