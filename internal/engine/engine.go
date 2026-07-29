package engine

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"goflow/internal/nodes"
	"goflow/internal/storage"

	"github.com/google/uuid"
	"golang.org/x/oauth2"
)

var ErrConcurrencyLimit = errors.New("execution concurrency limit reached")
var ErrWorkflowConcurrencyLimit = errors.New("workflow concurrency limit reached")

type subWorkflowStateKey struct{}

type subWorkflowState struct {
	Stack []string
	Depth int
}

type TriggerOptions struct {
	Source         string
	Principal      string
	RequestID      string
	IdempotencyKey string
}

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
	maxNodeSlots   int
	activeMu       sync.Mutex
	activeCancels  map[string]context.CancelFunc
	workflowMu     sync.Mutex
	workflowActive map[string]int
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
	maxNodeSlots := 0
	if len(maxConcurrent) > 1 && maxConcurrent[1] > 0 {
		maxNodeSlots = maxConcurrent[1]
	}
	return &Engine{
		registry:       r,
		executionStore: es,
		credStore:      cs,
		eventBus:       eb,
		wfStore:        ws,
		executionSlots: slots,
		maxNodeSlots:   maxNodeSlots,
		activeCancels:  make(map[string]context.CancelFunc),
		workflowActive: make(map[string]int),
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
	return e.ExecuteWorkflowWithOptions(wf, triggerPayload, TriggerOptions{})
}

func (e *Engine) ExecuteWorkflowWithOptions(wf *storage.Workflow, triggerPayload interface{}, opts TriggerOptions) (*storage.Execution, error) {
	releaseWorkflow, err := e.acquireWorkflowSlot(wf)
	if err != nil {
		return nil, err
	}
	defer releaseWorkflow()

	release, err := e.acquireExecutionSlot()
	if err != nil {
		return nil, err
	}
	defer release()

	return e.executeWorkflow(context.Background(), wf, triggerPayload, nil, opts)
}

func (e *Engine) ExecuteWorkflowAsync(wf *storage.Workflow, triggerPayload interface{}) error {
	_, _, err := e.StartWorkflowAsync(wf, triggerPayload, TriggerOptions{})
	return err
}

func (e *Engine) StartWorkflowAsync(wf *storage.Workflow, triggerPayload interface{}, opts TriggerOptions) (*storage.Execution, bool, error) {
	if opts.IdempotencyKey != "" {
		if existing, err := e.executionStore.GetByIdempotencyKey(wf.ID, opts.IdempotencyKey); err == nil {
			return existing, true, nil
		}
	}

	releaseWorkflow, err := e.acquireWorkflowSlot(wf)
	if err != nil {
		return nil, false, err
	}

	release, err := e.acquireExecutionSlot()
	if err != nil {
		releaseWorkflow()
		return nil, false, err
	}

	execRecord, err := e.createExecutionRecord(wf, triggerPayload, opts)
	if err != nil {
		release()
		releaseWorkflow()
		if opts.IdempotencyKey != "" && storage.IsExecutionIdempotencyConflict(err) {
			if existing, getErr := e.executionStore.GetByIdempotencyKey(wf.ID, opts.IdempotencyKey); getErr == nil {
				return existing, true, nil
			}
		}
		return nil, false, err
	}

	runCtx, cancel := context.WithCancel(context.Background())
	e.registerExecutionCancel(execRecord.ID, cancel)
	go func() {
		defer releaseWorkflow()
		defer release()
		defer e.unregisterExecutionCancel(execRecord.ID)
		if _, err := e.executeWorkflow(runCtx, wf, triggerPayload, execRecord, TriggerOptions{}); err != nil {
			_ = e.executionStore.UpdateStatusWithError(execRecord.ID, "FAILED", 0, execRecord.LogsJSON, err.Error())
		}
	}()
	return execRecord, false, nil
}

func (e *Engine) CancelExecution(id string) bool {
	e.activeMu.Lock()
	cancel, ok := e.activeCancels[id]
	e.activeMu.Unlock()
	if ok {
		cancel()
	}
	return ok
}

func (e *Engine) registerExecutionCancel(id string, cancel context.CancelFunc) {
	e.activeMu.Lock()
	defer e.activeMu.Unlock()
	e.activeCancels[id] = cancel
}

func (e *Engine) unregisterExecutionCancel(id string) {
	e.activeMu.Lock()
	defer e.activeMu.Unlock()
	delete(e.activeCancels, id)
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

func (e *Engine) acquireWorkflowSlot(wf *storage.Workflow) (func(), error) {
	if wf == nil || wf.MaxConcurrentRuns <= 0 || wf.ConcurrencyPolicy != "reject" {
		return func() {}, nil
	}
	e.workflowMu.Lock()
	defer e.workflowMu.Unlock()
	if e.workflowActive[wf.ID] >= wf.MaxConcurrentRuns {
		return nil, ErrWorkflowConcurrencyLimit
	}
	e.workflowActive[wf.ID]++
	return func() {
		e.workflowMu.Lock()
		defer e.workflowMu.Unlock()
		if e.workflowActive[wf.ID] > 0 {
			e.workflowActive[wf.ID]--
		}
	}, nil
}

func (e *Engine) createExecutionRecord(wf *storage.Workflow, triggerPayload interface{}, opts TriggerOptions) (*storage.Execution, error) {
	executionID := uuid.New().String()
	inputJSON := ""
	if triggerPayload != nil {
		if data, err := json.Marshal(triggerPayload); err == nil {
			inputJSON = string(data)
		}
	}
	execRecord := &storage.Execution{
		ID:               executionID,
		WorkflowID:       wf.ID,
		Status:           "RUNNING",
		StartedAt:        time.Now(),
		LogsJSON:         "[]",
		TriggerSource:    opts.Source,
		TriggerPrincipal: opts.Principal,
		RequestID:        opts.RequestID,
		IdempotencyKey:   opts.IdempotencyKey,
		InputJSON:        inputJSON,
	}

	if err := e.executionStore.Create(execRecord); err != nil {
		return nil, fmt.Errorf("failed to record execution in DB: %w", err)
	}
	return execRecord, nil
}

func (e *Engine) executeWorkflow(runCtx context.Context, wf *storage.Workflow, triggerPayload interface{}, execRecord *storage.Execution, opts TriggerOptions) (*storage.Execution, error) {
	if runCtx == nil {
		runCtx = context.Background()
	}
	state := subWorkflowStateFromContext(runCtx)
	if len(state.Stack) == 0 {
		state.Stack = []string{wf.ID}
		runCtx = context.WithValue(runCtx, subWorkflowStateKey{}, state)
	}
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

	if execRecord == nil {
		var err error
		execRecord, err = e.createExecutionRecord(wf, triggerPayload, opts)
		if err != nil {
			return nil, err
		}
	}
	executionID := execRecord.ID

	ctx := nodes.NewExecutionContextWithContext(runCtx, wf.ID, execRecord.ID)
	if triggerPayload != nil {
		ctx.SetOutput("$trigger", triggerPayload)
	}
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
		currentState := subWorkflowStateFromContext(runCtx)
		for _, item := range currentState.Stack {
			if item == subWfID {
				return nil, fmt.Errorf("sub-workflow cycle detected: %s already exists in execution stack", subWfID)
			}
		}
		maxDepth := maxSubWorkflowDepth()
		nextDepth := currentState.Depth + 1
		if nextDepth > maxDepth {
			return nil, fmt.Errorf("sub-workflow depth limit exceeded: depth %d is greater than max %d", nextDepth, maxDepth)
		}
		nextStack := append(append([]string{}, currentState.Stack...), subWfID)
		childRunCtx := context.WithValue(runCtx, subWorkflowStateKey{}, subWorkflowState{
			Stack: nextStack,
			Depth: nextDepth,
		})

		childExec, err := e.executeWorkflow(childRunCtx, subWf, payload, nil, TriggerOptions{
			Source:    "sub_workflow",
			Principal: execRecord.ID,
		})
		if err != nil {
			return nil, fmt.Errorf("sub-workflow execution failed: %w", err)
		}

		if childExec.Status == "FAILED" {
			if childExec.ErrorMessage != "" {
				return nil, fmt.Errorf("sub-workflow execution failed: %s", childExec.ErrorMessage)
			}
			return nil, fmt.Errorf("sub-workflow execution status returned FAILED")
		}

		var logs []NodeLog
		_ = json.Unmarshal([]byte(childExec.LogsJSON), &logs)

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
	nodeSlots := e.newNodeSlots()

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
	if remainingCount == 0 {
		close(doneChan)
	}

schedulerLoop:
	for {
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
				nodeLogs = append(nodeLogs, NodeLog{
					NodeID:     nid,
					Status:     "SKIPPED",
					DurationMs: 0,
					Attempts:   0,
				})
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
				e.eventBus.Publish(ExecutionEvent{
					WorkflowID:  wf.ID,
					ExecutionID: executionID,
					NodeID:      nid,
					Status:      "SKIPPED",
					Timestamp:   time.Now(),
				})
				continue
			}

			// Run the pending node
			nodeStates[nid] = StateRunning
			stateMu.Unlock()

			go func(nodeID string) {
				nodeObj := plan.Nodes[nodeID]
				executor, ok := e.registry.Get(nodeObj.Type)
				nodeStart := time.Now()
				defer func() {
					if recovered := recover(); recovered != nil {
						durationMs := time.Since(nodeStart).Milliseconds()
						errStr := redactSensitiveString(fmt.Sprintf("node panic recovered: %v", recovered))

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
						remainingCount--
						if remainingCount == 0 {
							close(doneChan)
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
					}
				}()

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
						Error:      redactSensitiveString(errStr),
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
					remainingCount--
					if remainingCount == 0 {
						close(doneChan)
					}
					stateMu.Unlock()

					e.eventBus.Publish(ExecutionEvent{
						WorkflowID:  wf.ID,
						ExecutionID: executionID,
						NodeID:      nodeID,
						Status:      "FAILED",
						Timestamp:   time.Now(),
						Error:       redactSensitiveString(errStr),
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
					if err := runCtx.Err(); err != nil {
						lastErr = err
						break
					}
					releaseNodeSlot, err := acquireNodeSlot(runCtx, nodeSlots)
					if err != nil {
						lastErr = err
						break
					}
					attemptsUsed = attempt
					output, lastErr = executor.Execute(ctx, evaluatedNode)
					releaseNodeSlot()
					if lastErr == nil {
						break
					}

					if attempt < maxRetries {
						log.Printf("[Engine] Node %s (%s) attempt %d failed: %v. Retrying in 500ms...", nodeID, nodeObj.Name, attempt, lastErr)
						select {
						case <-time.After(500 * time.Millisecond):
						case <-runCtx.Done():
							lastErr = runCtx.Err()
						}
					}
				}

				durationMs := time.Since(nodeStart).Milliseconds()

				stateMu.Lock()
				defer stateMu.Unlock()

				if lastErr != nil {
					log.Printf("[Engine] Node %s (%s) FAILED after %d attempts: %v", nodeID, nodeObj.Name, attemptsUsed, lastErr)
					redactedErr := redactError(lastErr)
					nodeLogs = append(nodeLogs, NodeLog{
						NodeID:     nodeID,
						Status:     "FAILED",
						DurationMs: durationMs,
						Attempts:   attemptsUsed,
						Error:      redactedErr,
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
						Error:       redactedErr,
						DurationMs:  durationMs,
					})
				} else {
					ctx.SetOutput(nodeID, output)
					redactedOutput := redactSensitive(output)
					nodeLogs = append(nodeLogs, NodeLog{
						NodeID:     nodeID,
						Status:     "SUCCESS",
						DurationMs: durationMs,
						Attempts:   attemptsUsed,
						Output:     redactedOutput,
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
						Payload:     redactedOutput,
						DurationMs:  durationMs,
					})
				}
				remainingCount--
				if remainingCount == 0 {
					close(doneChan)
				}
			}(nid)
		case <-doneChan:
			break schedulerLoop
		}
	}

	totalDuration := time.Since(startTime).Milliseconds()
	finalStatus := "SUCCESS"
	errorMessage := ""
	if runCtx.Err() != nil {
		finalStatus = "CANCELLED"
		errorMessage = runCtx.Err().Error()
	} else if hasFailed {
		finalStatus = "FAILED"
		for _, item := range nodeLogs {
			if item.Error != "" {
				errorMessage = item.Error
				break
			}
		}
	}

	logsJSONBytes, _ := json.Marshal(nodeLogs)
	_ = e.executionStore.UpdateStatusWithError(executionID, finalStatus, totalDuration, string(logsJSONBytes), errorMessage)

	execRecord.Status = finalStatus
	execRecord.DurationMs = totalDuration
	execRecord.LogsJSON = string(logsJSONBytes)
	execRecord.ErrorMessage = errorMessage

	return execRecord, nil
}

func subWorkflowStateFromContext(ctx context.Context) subWorkflowState {
	if ctx == nil {
		return subWorkflowState{}
	}
	state, _ := ctx.Value(subWorkflowStateKey{}).(subWorkflowState)
	return state
}

func maxSubWorkflowDepth() int {
	const fallback = 5
	raw := strings.TrimSpace(os.Getenv("GOFLOW_MAX_SUBWORKFLOW_DEPTH"))
	if raw == "" {
		return fallback
	}
	value, err := strconv.Atoi(raw)
	if err != nil || value < 1 {
		return fallback
	}
	return value
}

func (e *Engine) newNodeSlots() chan struct{} {
	if e.maxNodeSlots <= 0 {
		return nil
	}
	return make(chan struct{}, e.maxNodeSlots)
}

func acquireNodeSlot(ctx context.Context, slots chan struct{}) (func(), error) {
	if slots == nil {
		return func() {}, nil
	}
	select {
	case slots <- struct{}{}:
		return func() { <-slots }, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}
