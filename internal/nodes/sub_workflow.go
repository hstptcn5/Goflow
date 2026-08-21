package nodes

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"sync"
)

const (
	defaultSubWorkflowConcurrency = 5
	maxSubWorkflowConcurrency     = 32
	maxSubWorkflowLoopItems       = 1000
	maxSubWorkflowPayloadBytes    = 2 << 20
)

type SubWorkflowLoopErrorPolicy string

const (
	SubWorkflowStopAll       SubWorkflowLoopErrorPolicy = "stop_all"
	SubWorkflowContinue      SubWorkflowLoopErrorPolicy = "continue"
	SubWorkflowCollectErrors SubWorkflowLoopErrorPolicy = "collect_errors"
)

type SubWorkflowExecutor struct{}

func NewSubWorkflowExecutor() *SubWorkflowExecutor { return &SubWorkflowExecutor{} }

func parseSubWorkflowConcurrency(raw interface{}) (int, error) {
	if raw == nil || strings.TrimSpace(fmt.Sprint(raw)) == "" {
		return defaultSubWorkflowConcurrency, nil
	}
	var value int
	switch typed := raw.(type) {
	case int:
		value = typed
	case int64:
		value = int(typed)
	case float64:
		if typed != float64(int(typed)) {
			return 0, fmt.Errorf("sub-workflow concurrency_limit must be an integer")
		}
		value = int(typed)
	case string:
		parsed, err := strconv.Atoi(strings.TrimSpace(typed))
		if err != nil {
			return 0, fmt.Errorf("sub-workflow concurrency_limit must be an integer")
		}
		value = parsed
	default:
		return 0, fmt.Errorf("sub-workflow concurrency_limit must be an integer")
	}
	if value < 1 || value > maxSubWorkflowConcurrency {
		return 0, fmt.Errorf("sub-workflow concurrency_limit must be between 1 and %d", maxSubWorkflowConcurrency)
	}
	return value, nil
}

func parseSubWorkflowPayload(raw interface{}) (interface{}, error) {
	if raw == nil {
		return nil, nil
	}
	if text, ok := raw.(string); ok {
		if len(text) > maxSubWorkflowPayloadBytes {
			return nil, fmt.Errorf("sub-workflow payload exceeds %d byte limit", maxSubWorkflowPayloadBytes)
		}
		if strings.TrimSpace(text) == "" {
			return nil, nil
		}
		var payload interface{}
		if err := json.Unmarshal([]byte(text), &payload); err == nil {
			return payload, nil
		}
		return text, nil
	}
	encoded, err := json.Marshal(raw)
	if err != nil {
		return nil, fmt.Errorf("sub-workflow payload could not be encoded: %w", err)
	}
	if len(encoded) > maxSubWorkflowPayloadBytes {
		return nil, fmt.Errorf("sub-workflow payload exceeds %d byte limit", maxSubWorkflowPayloadBytes)
	}
	return raw, nil
}

func parseSubWorkflowLoopErrorPolicy(raw interface{}) (SubWorkflowLoopErrorPolicy, error) {
	if raw == nil {
		return SubWorkflowStopAll, nil
	}
	value := strings.ToLower(strings.TrimSpace(fmt.Sprint(raw)))
	switch value {
	case "", "stop all", "stop_all", "stop":
		return SubWorkflowStopAll, nil
	case "continue":
		return SubWorkflowContinue, nil
	case "collect errors", "collect_errors", "collect":
		return SubWorkflowCollectErrors, nil
	default:
		return "", fmt.Errorf("sub-workflow loop error_policy must be Stop all, Continue, or Collect errors")
	}
}

func validateSubWorkflowNode(node *Node) error {
	subWfID, _ := node.Params["sub_workflow_id"].(string)
	if strings.TrimSpace(subWfID) == "" {
		return fmt.Errorf("sub_workflow_id is required")
	}
	if len(subWfID) > 256 {
		return fmt.Errorf("sub_workflow_id exceeds 256 character limit")
	}
	if text, ok := node.Params["concurrency_limit"].(string); ok && containsTemplateExpression(text) {
		return nil
	}
	if _, err := parseSubWorkflowConcurrency(node.Params["concurrency_limit"]); err != nil {
		return err
	}
	_, err := parseSubWorkflowLoopErrorPolicy(node.Params["error_policy"])
	return err
}

func (e *SubWorkflowExecutor) Execute(ctx *ExecutionContext, node *Node) (interface{}, error) {
	if err := validateSubWorkflowNode(node); err != nil {
		return nil, err
	}
	subWfID, _ := node.Params["sub_workflow_id"].(string)
	subWfID = strings.TrimSpace(subWfID)
	payload, err := parseSubWorkflowPayload(node.Params["payload_json"])
	if err != nil {
		return nil, err
	}
	loopMode, _ := node.Params["loop_mode"].(bool)
	parallel, _ := node.Params["parallel"].(bool)
	if ctx.ExecuteWorkflow == nil {
		return nil, fmt.Errorf("ExecuteWorkflow callback is not initialized in context")
	}
	if !loopMode {
		if err := ctx.Context.Err(); err != nil {
			return nil, err
		}
		return ctx.ExecuteWorkflow(subWfID, payload)
	}

	policy, err := parseSubWorkflowLoopErrorPolicy(node.Params["error_policy"])
	if err != nil {
		return nil, err
	}
	slicePayload, isArray := payload.([]interface{})
	if !isArray {
		slicePayload = []interface{}{payload}
	}
	if len(slicePayload) > maxSubWorkflowLoopItems {
		return nil, fmt.Errorf("sub-workflow loop contains %d items; maximum is %d", len(slicePayload), maxSubWorkflowLoopItems)
	}

	results := make([]interface{}, len(slicePayload))
	itemErrors := make([]string, len(slicePayload))
	var stateMu sync.Mutex
	firstFailure := -1

	runItem := func(idx int, item interface{}) {
		if err := ctx.Context.Err(); err != nil {
			stateMu.Lock()
			itemErrors[idx] = err.Error()
			if firstFailure < 0 {
				firstFailure = idx
			}
			stateMu.Unlock()
			return
		}
		res, err := ctx.ExecuteWorkflow(subWfID, item)
		stateMu.Lock()
		defer stateMu.Unlock()
		if err != nil {
			itemErrors[idx] = err.Error()
			if firstFailure < 0 {
				firstFailure = idx
			}
			return
		}
		results[idx] = res
	}

	shouldStop := func() bool {
		stateMu.Lock()
		defer stateMu.Unlock()
		return policy == SubWorkflowStopAll && firstFailure >= 0
	}

	if parallel {
		maxConcurrency, err := parseSubWorkflowConcurrency(node.Params["concurrency_limit"])
		if err != nil {
			return nil, err
		}
		sem := make(chan struct{}, maxConcurrency)
		var wg sync.WaitGroup
		for i, item := range slicePayload {
			if shouldStop() {
				break
			}
			select {
			case sem <- struct{}{}:
			case <-ctx.Context.Done():
				wg.Wait()
				return nil, ctx.Context.Err()
			}
			if shouldStop() {
				<-sem
				break
			}
			wg.Add(1)
			go func(idx int, it interface{}) {
				defer wg.Done()
				defer func() { <-sem }()
				runItem(idx, it)
			}(i, item)
		}
		wg.Wait()
	} else {
		for i, item := range slicePayload {
			if err := ctx.Context.Err(); err != nil {
				return nil, err
			}
			if shouldStop() {
				break
			}
			runItem(i, item)
		}
	}

	if err := ctx.Context.Err(); err != nil {
		return nil, err
	}

	successes := make([]map[string]interface{}, 0, len(slicePayload))
	errorsList := make([]map[string]interface{}, 0)
	continuedItems := make([]interface{}, len(slicePayload))
	for i := range slicePayload {
		if itemErrors[i] != "" {
			errorItem := map[string]interface{}{"index": i, "error": itemErrors[i]}
			errorsList = append(errorsList, errorItem)
			continuedItems[i] = map[string]interface{}{"ok": false, "error": itemErrors[i]}
			continue
		}
		if results[i] != nil {
			successes = append(successes, map[string]interface{}{"index": i, "output": results[i]})
			continuedItems[i] = map[string]interface{}{"ok": true, "output": results[i]}
		}
	}

	if len(errorsList) > 0 && policy == SubWorkflowStopAll {
		first := errorsList[0]
		return results, fmt.Errorf("sub-workflow loop stopped at item %v: %v", first["index"], first["error"])
	}
	if policy == SubWorkflowContinue {
		return map[string]interface{}{
			"items":         continuedItems,
			"success_count": len(successes),
			"error_count":   len(errorsList),
		}, nil
	}
	if policy == SubWorkflowCollectErrors {
		return map[string]interface{}{
			"results":       results,
			"successes":     successes,
			"errors":        errorsList,
			"success_count": len(successes),
			"error_count":   len(errorsList),
		}, nil
	}
	return results, nil
}

func (e *SubWorkflowExecutor) Validate(node *Node) error { return validateSubWorkflowNode(node) }

func (e *SubWorkflowExecutor) GetDefinition() NodeDefinition {
	return NodeDefinition{
		Type: TypeSubWorkflow, Name: "Sub-workflow Runner", Description: "Runs a child workflow sequentially or in a bounded parallel loop", Icon: "Folder", Category: "LOGIC & UTILITY",
		Params: []ParamDefinition{
			{Name: "sub_workflow_id", Label: "Sub-workflow to Run", Type: "select", Required: true, Description: "Select the child workflow to execute"},
			{Name: "payload_json", Label: "Input Payload (JSON / Text)", Type: "textarea", Default: "{\n  \"message\": \"Input payload\"\n}", Required: false, Description: "Input payload as text, JSON, or placeholders such as {{node.path}}"},
			{Name: "loop_mode", Label: "Loop mode (Run for each item in array)", Type: "boolean", Default: false, Required: false, Description: "Treat the input payload as an array of up to 1,000 items"},
			{Name: "parallel", Label: "Run loop items in parallel", Type: "boolean", Default: false, Required: false, Description: "Run loop items in bounded parallel execution"},
			{Name: "concurrency_limit", Label: "Concurrency Limit", Type: "text", Default: "5", Required: false, Description: "Maximum child workflow runs in parallel, between 1 and 32"},
			{Name: "error_policy", Label: "Loop Error Policy", Type: "select", Default: "Stop all", Options: []string{"Stop all", "Continue", "Collect errors"}, Required: false, Description: "Stop on the first observed failure, continue with per-item status, or return structured success/error collections"},
		},
	}
}
