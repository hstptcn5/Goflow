package engine

import (
	"encoding/json"
	"fmt"
	"log"
	"sync"
	"time"

	"goflow/internal/nodes"
	"goflow/internal/storage"

	"github.com/google/uuid"
)

type NodeLog struct {
	NodeID     string      `json:"node_id"`
	Status     string      `json:"status"` // 'RUNNING', 'SUCCESS', 'FAILED'
	DurationMs int64       `json:"duration_ms"`
	Attempts   int         `json:"attempts"`
	Output     interface{} `json:"output,omitempty"`
	Error      string      `json:"error,omitempty"`
}

type Engine struct {
	registry       *nodes.PluginRegistry
	executionStore *storage.ExecutionStore
	credStore      *storage.CredentialStore
	eventBus       *EventBus
	wfStore        *storage.WorkflowStore
}

func NewEngine(
	r *nodes.PluginRegistry,
	es *storage.ExecutionStore,
	cs *storage.CredentialStore,
	eb *EventBus,
	ws *storage.WorkflowStore,
) *Engine {
	return &Engine{
		registry:       r,
		executionStore: es,
		credStore:      cs,
		eventBus:       eb,
		wfStore:        ws,
	}
}

type NodeState string

const (
	StatePending NodeState = "PENDING"
	StateRunning NodeState = "RUNNING"
	StateSuccess NodeState = "SUCCESS"
	StateSkipped NodeState = "SKIPPED"
	StateFailed  NodeState = "FAILED"
)

func (e *Engine) ExecuteWorkflow(wf *storage.Workflow, triggerPayload interface{}) (*storage.Execution, error) {
	var nodeList []nodes.Node
	if err := json.Unmarshal([]byte(wf.NodesJSON), &nodeList); err != nil {
		return nil, fmt.Errorf("invalid workflow nodes_json: %w", err)
	}

	if len(nodeList) == 0 {
		return nil, fmt.Errorf("cannot execute empty workflow: workflow contains no nodes in DB")
	}

	var edgeList []nodes.Edge
	if err := json.Unmarshal([]byte(wf.EdgesJSON), &edgeList); err != nil {
		return nil, fmt.Errorf("invalid workflow edges_json: %w", err)
	}

	plan, err := BuildDAGPlan(nodeList, edgeList)
	if err != nil {
		return nil, fmt.Errorf("failed to build DAG plan: %w", err)
	}

	executionID := uuid.New().String()
	execRecord := &storage.Execution{
		ID:         executionID,
		WorkflowID: wf.ID,
		Status:     "RUNNING",
		StartedAt:  time.Now(),
		LogsJSON:   "[]",
	}

	if err := e.executionStore.Create(execRecord); err != nil {
		return nil, fmt.Errorf("failed to record execution in DB: %w", err)
	}

	ctx := nodes.NewExecutionContext(wf.ID, executionID)
	ctx.ExecuteWorkflow = func(subWfID string, payload interface{}) (interface{}, error) {
		subWf, err := e.wfStore.GetByID(subWfID)
		if err != nil {
			return nil, fmt.Errorf("sub-workflow %s not found: %w", subWfID, err)
		}

		execRecord, err := e.ExecuteWorkflow(subWf, payload)
		if err != nil {
			return nil, fmt.Errorf("sub-workflow execution failed: %w", err)
		}

		if execRecord.Status == "FAILED" {
			return nil, fmt.Errorf("sub-workflow execution status returned FAILED")
		}

		var logs []NodeLog
		_ = json.Unmarshal([]byte(execRecord.LogsJSON), &logs)

		results := make(map[string]interface{})
		for _, logItem := range logs {
			if logItem.Status == "SUCCESS" && logItem.Output != nil {
				results[logItem.NodeID] = logItem.Output
			}
		}
		return results, nil
	}

	if triggerPayload != nil {
		ctx.SetOutput("$trigger", triggerPayload)
	}

	// Load decrypted credentials nếu có trong workflow
	allCreds, _ := e.credStore.ListAll()
	for _, c := range allCreds {
		decrypted, err := e.credStore.GetDecryptedData(c.ID)
		if err == nil {
			ctx.Credentials[c.ID] = decrypted
		}
	}

	startTime := time.Now()
	nodeLogs := make([]NodeLog, 0, len(nodeList))
	var stateMu sync.Mutex
	hasFailed := false

	// Initialize execution states, dynamic in-degrees, and incoming path flags
	nodeStates := make(map[string]NodeState)
	inDegrees := make(map[string]int)
	hasActiveIncomingPath := make(map[string]bool)

	for _, node := range nodeList {
		nodeStates[node.ID] = StatePending
		inDegrees[node.ID] = plan.InDegree[node.ID]
		hasActiveIncomingPath[node.ID] = false
	}

	// Triggers (nodes with in-degree 0) have active incoming path by default
	for nodeID, deg := range plan.InDegree {
		if deg == 0 {
			hasActiveIncomingPath[nodeID] = true
		}
	}

	// Channel to queue nodes that are ready to run
	readyChan := make(chan string, len(nodeList))
	doneChan := make(chan struct{})
	for nodeID, deg := range inDegrees {
		if deg == 0 {
			readyChan <- nodeID
		}
	}

	remainingCount := len(nodeList)

schedulerLoop:
	for remainingCount > 0 {
		select {
		case nid := <-readyChan:
			stateMu.Lock()
			state := nodeStates[nid]

			// If workflow already failed, skip any pending nodes to wind down execution quickly
			if hasFailed && state == StatePending {
				nodeStates[nid] = StateSkipped
				state = StateSkipped
			}

			if state == StateSkipped {
				nodeStates[nid] = StateSkipped
				remainingCount--
				if remainingCount == 0 {
					close(doneChan)
				}

				// Propagate skip to dependents
				for _, edge := range plan.EdgesFrom[nid] {
					childID := edge.Target
					inDegrees[childID]--
					if inDegrees[childID] == 0 {
						if !hasActiveIncomingPath[childID] {
							nodeStates[childID] = StateSkipped
						}
						readyChan <- childID
					}
				}
				stateMu.Unlock()
				continue
			}

			// Run the pending node
			nodeStates[nid] = StateRunning
			stateMu.Unlock()

			go func(nodeID string) {
				nodeObj := plan.Nodes[nodeID]
				executor, ok := e.registry.Get(nodeObj.Type)
				nodeStart := time.Now()

				// Emit Start Event
				e.eventBus.Publish(ExecutionEvent{
					WorkflowID:  wf.ID,
					ExecutionID: executionID,
					NodeID:      nodeID,
					Status:      "RUNNING",
					Timestamp:   time.Now(),
				})

				if !ok {
					errStr := fmt.Sprintf("unregistered node executor type: %s", nodeObj.Type)
					durationMs := time.Since(nodeStart).Milliseconds()

					stateMu.Lock()
					nodeLogs = append(nodeLogs, NodeLog{
						NodeID:     nodeID,
						Status:     "FAILED",
						DurationMs: durationMs,
						Attempts:   1,
						Error:      errStr,
					})
					nodeStates[nodeID] = StateFailed
					hasFailed = true
					remainingCount--
					if remainingCount == 0 {
						close(doneChan)
					}

					// Propagate failure/skip to children
					for _, edge := range plan.EdgesFrom[nodeID] {
						childID := edge.Target
						inDegrees[childID]--
						if inDegrees[childID] == 0 {
							if !hasActiveIncomingPath[childID] {
								nodeStates[childID] = StateSkipped
							}
							readyChan <- childID
						}
					}
					stateMu.Unlock()

					e.eventBus.Publish(ExecutionEvent{
						WorkflowID:  wf.ID,
						ExecutionID: executionID,
						NodeID:      nodeID,
						Status:      "FAILED",
						Timestamp:   time.Now(),
						Error:       errStr,
						DurationMs:  durationMs,
					})
					return
				}

				// Resolve parameters dynamically using ctx before execution
				resolvedParams := nodes.ResolveParams(ctx, nodeObj.Params)
				evaluatedNode := &nodes.Node{
					ID:       nodeObj.ID,
					Type:     nodeObj.Type,
					Name:     nodeObj.Name,
					Position: nodeObj.Position,
					Params:   resolvedParams,
				}

				// Auto-retry Loop (Tối đa 3 lần thử khi gặp lỗi)
				maxRetries := 3
				var lastErr error
				var output interface{}
				attemptsUsed := 0

				for attempt := 1; attempt <= maxRetries; attempt++ {
					attemptsUsed = attempt
					output, lastErr = executor.Execute(ctx, evaluatedNode)
					if lastErr == nil {
						break
					}

					if attempt < maxRetries {
						log.Printf("[Engine] Node %s (%s) attempt %d failed: %v. Retrying in 500ms...", nodeID, nodeObj.Name, attempt, lastErr)
						time.Sleep(500 * time.Millisecond)
					}
				}

				durationMs := time.Since(nodeStart).Milliseconds()

				stateMu.Lock()
				defer stateMu.Unlock()

				remainingCount--
				if remainingCount == 0 {
					close(doneChan)
				}

				if lastErr != nil {
					log.Printf("[Engine] Node %s (%s) FAILED after %d attempts: %v", nodeID, nodeObj.Name, attemptsUsed, lastErr)
					nodeLogs = append(nodeLogs, NodeLog{
						NodeID:     nodeID,
						Status:     "FAILED",
						DurationMs: durationMs,
						Attempts:   attemptsUsed,
						Error:      lastErr.Error(),
					})
					nodeStates[nodeID] = StateFailed
					hasFailed = true

					// Propagate failure/skip to children
					for _, edge := range plan.EdgesFrom[nodeID] {
						childID := edge.Target
						inDegrees[childID]--
						if inDegrees[childID] == 0 {
							if !hasActiveIncomingPath[childID] {
								nodeStates[childID] = StateSkipped
							}
							readyChan <- childID
						}
					}

					e.eventBus.Publish(ExecutionEvent{
						WorkflowID:  wf.ID,
						ExecutionID: executionID,
						NodeID:      nodeID,
						Status:      "FAILED",
						Timestamp:   time.Now(),
						Error:       lastErr.Error(),
						DurationMs:  durationMs,
					})
				} else {
					ctx.SetOutput(nodeID, output)
					nodeLogs = append(nodeLogs, NodeLog{
						NodeID:     nodeID,
						Status:     "SUCCESS",
						DurationMs: durationMs,
						Attempts:   attemptsUsed,
						Output:     output,
					})
					nodeStates[nodeID] = StateSuccess

					// Branching Analysis (Skip Logic)
					// Check if executor output specifies a target branch handle
					var targetHandle string
					if outMap, ok := output.(map[string]interface{}); ok {
						if th, ok := outMap["target_handle"].(string); ok {
							targetHandle = th
						}
					}

					// Update and propagate active paths to dependents
					for _, edge := range plan.EdgesFrom[nodeID] {
						childID := edge.Target

						// If targetHandle is specified, we check if edge SourceHandle matches it
						edgeFollowed := true
						if targetHandle != "" {
							if edge.SourceHandle != "" && edge.SourceHandle != targetHandle {
								edgeFollowed = false
							}
						}

						if edgeFollowed {
							hasActiveIncomingPath[childID] = true
						}

						inDegrees[childID]--
						if inDegrees[childID] == 0 {
							// When inDegree becomes 0, if the node has no active incoming path, mark it as skipped
							if !hasActiveIncomingPath[childID] {
								nodeStates[childID] = StateSkipped
							}
							readyChan <- childID
						}
					}

					e.eventBus.Publish(ExecutionEvent{
						WorkflowID:  wf.ID,
						ExecutionID: executionID,
						NodeID:      nodeID,
						Status:      "SUCCESS",
						Timestamp:   time.Now(),
						Payload:     output,
						DurationMs:  durationMs,
					})
				}
			}(nid)
		case <-doneChan:
			break schedulerLoop
		}
	}

	totalDuration := time.Since(startTime).Milliseconds()
	finalStatus := "SUCCESS"
	if hasFailed {
		finalStatus = "FAILED"
	}

	logsJSONBytes, _ := json.Marshal(nodeLogs)
	_ = e.executionStore.UpdateStatus(executionID, finalStatus, totalDuration, string(logsJSONBytes))

	execRecord.Status = finalStatus
	execRecord.DurationMs = totalDuration
	execRecord.LogsJSON = string(logsJSONBytes)

	return execRecord, nil
}
