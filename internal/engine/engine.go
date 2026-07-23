package engine

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"sync"
	"time"

	"goflow/internal/nodes"
	"goflow/internal/storage"

	"github.com/google/uuid"
	"golang.org/x/oauth2"
)

var ErrConcurrencyLimit = errors.New("execution concurrency limit reached")

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
	executionSlots chan struct{}
}

func NewEngine(
	r *nodes.PluginRegistry,
	es *storage.ExecutionStore,
	cs *storage.CredentialStore,
	eb *EventBus,
	ws *storage.WorkflowStore,
	maxConcurrent ...int,
) *Engine {
	var slots chan struct{}
	if len(maxConcurrent) > 0 && maxConcurrent[0] > 0 {
		slots = make(chan struct{}, maxConcurrent[0])
	}
	return &Engine{
		registry:       r,
		executionStore: es,
		credStore:      cs,
		eventBus:       eb,
		wfStore:        ws,
		executionSlots: slots,
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
	release, err := e.acquireExecutionSlot()
	if err != nil {
		return nil, err
	}
	defer release()

	return e.executeWorkflow(wf, triggerPayload)
}

func (e *Engine) ExecuteWorkflowAsync(wf *storage.Workflow, triggerPayload interface{}) error {
	release, err := e.acquireExecutionSlot()
	if err != nil {
		return err
	}

	go func() {
		defer release()
		_, _ = e.executeWorkflow(wf, triggerPayload)
	}()
	return nil
}

func (e *Engine) acquireExecutionSlot() (func(), error) {
	if e.executionSlots == nil {
		return func() {}, nil
	}
	select {
	case e.executionSlots <- struct{}{}:
		return func() { <-e.executionSlots }, nil
	default:
		return nil, ErrConcurrencyLimit
	}
}

func (e *Engine) executeWorkflow(wf *storage.Workflow, triggerPayload interface{}) (*storage.Execution, error) {
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
	ctx.RefreshCredential = func(credID string) (string, error) {
		cred, err := e.credStore.GetByID(credID)
		if err != nil {
			return "", err
		}
		if cred.Type != "oauth2" {
			return e.credStore.GetDecryptedData(credID)
		}

		decryptedRaw, err := e.credStore.GetDecryptedData(credID)
		if err != nil {
			return "", err
		}

		var payload struct {
			Config struct {
				ClientID     string `json:"client_id"`
				ClientSecret string `json:"client_secret"`
				AuthURL      string `json:"auth_url"`
				TokenURL     string `json:"token_url"`
				Scopes       string `json:"scopes"`
			} `json:"config"`
			Token *oauth2.Token `json:"token"`
		}

		if err := json.Unmarshal([]byte(decryptedRaw), &payload); err != nil {
			return "", err
		}

		if payload.Token == nil {
			return "", fmt.Errorf("OAuth2 token not linked yet")
		}

		if payload.Token.Expiry.Before(time.Now().Add(60 * time.Second)) {
			conf := &oauth2.Config{
				ClientID:     payload.Config.ClientID,
				ClientSecret: payload.Config.ClientSecret,
				Endpoint: oauth2.Endpoint{
					AuthURL:  payload.Config.AuthURL,
					TokenURL: payload.Config.TokenURL,
				},
			}

			ts := conf.TokenSource(context.Background(), payload.Token)
			newToken, err := ts.Token()
			if err != nil {
				return "", fmt.Errorf("failed to refresh OAuth2 token: %w", err)
			}

			payload.Token = newToken
			updatedBytes, err := json.Marshal(payload)
			if err == nil {
				_ = e.credStore.UpdateData(credID, string(updatedBytes))
			}
		}

		return payload.Token.AccessToken, nil
	}

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

	// Chỉ nạp và giải mã những credentials thực sự được tham chiếu trong các node của workflow
	for _, nodeObj := range nodeList {
		if credID, ok := nodeObj.Params["credential_id"].(string); ok && credID != "" {
			if _, loaded := ctx.Credentials[credID]; !loaded {
				decrypted, err := e.credStore.GetDecryptedData(credID)
				if err == nil {
					ctx.Credentials[credID] = decrypted
				}
			}
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

				// Auto-retry Loop: chỉ retry cho các node có Retryable=true (an toàn khi thực thi lại)
				// Các node có side-effect (gửi email, tin nhắn) mặc định Retryable=false → chỉ chạy 1 lần
				maxRetries := 1
				if executor.GetDefinition().Retryable {
					maxRetries = 3
				}
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
