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
}

func NewEngine(
	r *nodes.PluginRegistry,
	es *storage.ExecutionStore,
	cs *storage.CredentialStore,
	eb *EventBus,
) *Engine {
	return &Engine{
		registry:       r,
		executionStore: es,
		credStore:      cs,
		eventBus:       eb,
	}
}

func (e *Engine) ExecuteWorkflow(wf *storage.Workflow, triggerPayload interface{}) (*storage.Execution, error) {
	var nodeList []nodes.Node
	if err := json.Unmarshal([]byte(wf.NodesJSON), &nodeList); err != nil {
		return nil, fmt.Errorf("invalid workflow nodes_json: %w", err)
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
	var logsMu sync.Mutex
	hasFailed := false

	// Thực thi từng lớp Node (Layer-by-Layer parallel execution)
	for _, layer := range plan.ExecutionLayers {
		if hasFailed {
			break
		}

		var wg sync.WaitGroup
		wg.Add(len(layer))

		for _, nodeID := range layer {
			go func(nid string) {
				defer wg.Done()

				nodeObj := plan.Nodes[nid]
				executor, ok := e.registry.Get(nodeObj.Type)

				nodeStart := time.Now()

				// Emit Start Event
				e.eventBus.Publish(ExecutionEvent{
					WorkflowID:  wf.ID,
					ExecutionID: executionID,
					NodeID:      nid,
					Status:      "RUNNING",
					Timestamp:   nodeStart,
				})

				if !ok {
					errStr := fmt.Sprintf("unregistered node executor type: %s", nodeObj.Type)
					durationMs := time.Since(nodeStart).Milliseconds()

					logsMu.Lock()
					nodeLogs = append(nodeLogs, NodeLog{
						NodeID:     nid,
						Status:     "FAILED",
						DurationMs: durationMs,
						Attempts:   1,
						Error:      errStr,
					})
					hasFailed = true
					logsMu.Unlock()

					e.eventBus.Publish(ExecutionEvent{
						WorkflowID:  wf.ID,
						ExecutionID: executionID,
						NodeID:      nid,
						Status:      "FAILED",
						Timestamp:   time.Now(),
						Error:       errStr,
						DurationMs:  durationMs,
					})
					return
				}

				// Auto-retry Loop (Tối đa 3 lần thử khi gặp lỗi)
				maxRetries := 3
				var lastErr error
				var output interface{}
				attemptsUsed := 0

				for attempt := 1; attempt <= maxRetries; attempt++ {
					attemptsUsed = attempt
					output, lastErr = executor.Execute(ctx, nodeObj)
					if lastErr == nil {
						break
					}

					if attempt < maxRetries {
						log.Printf("[Engine] Node %s (%s) attempt %d failed: %v. Retrying in 500ms...", nid, nodeObj.Name, attempt, lastErr)
						time.Sleep(500 * time.Millisecond)
					}
				}

				durationMs := time.Since(nodeStart).Milliseconds()

				logsMu.Lock()
				defer logsMu.Unlock()

				if lastErr != nil {
					log.Printf("[Engine] Node %s (%s) FAILED after %d attempts: %v", nid, nodeObj.Name, attemptsUsed, lastErr)
					nodeLogs = append(nodeLogs, NodeLog{
						NodeID:     nid,
						Status:     "FAILED",
						DurationMs: durationMs,
						Attempts:   attemptsUsed,
						Error:      lastErr.Error(),
					})
					hasFailed = true

					e.eventBus.Publish(ExecutionEvent{
						WorkflowID:  wf.ID,
						ExecutionID: executionID,
						NodeID:      nid,
						Status:      "FAILED",
						Timestamp:   time.Now(),
						Error:       lastErr.Error(),
						DurationMs:  durationMs,
					})
				} else {
					ctx.SetOutput(nid, output)
					nodeLogs = append(nodeLogs, NodeLog{
						NodeID:     nid,
						Status:     "SUCCESS",
						DurationMs: durationMs,
						Attempts:   attemptsUsed,
						Output:     output,
					})

					e.eventBus.Publish(ExecutionEvent{
						WorkflowID:  wf.ID,
						ExecutionID: executionID,
						NodeID:      nid,
						Status:      "SUCCESS",
						Timestamp:   time.Now(),
						Payload:     output,
						DurationMs:  durationMs,
					})
				}
			}(nodeID)
		}

		wg.Wait()
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
