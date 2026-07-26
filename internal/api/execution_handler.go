package api

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"goflow/internal/engine"
	"goflow/internal/storage"

	"github.com/go-chi/chi/v5"
)

type ExecutionHandler struct {
	execStore *storage.ExecutionStore
	engine    *engine.Engine
}

func NewExecutionHandler(es *storage.ExecutionStore, eng *engine.Engine) *ExecutionHandler {
	return &ExecutionHandler{execStore: es, engine: eng}
}

func (h *ExecutionHandler) GetExecution(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	exec, err := h.execStore.GetByID(id)
	if err != nil {
		http.Error(w, "Execution log not found", http.StatusNotFound)
		return
	}
	if !requestAllowsWorkflow(r, exec.WorkflowID) {
		forbidden(w)
		return
	}
	renderJSON(w, http.StatusOK, executionInspectorDTOFromExecution(*exec))
}

func (h *ExecutionHandler) ListWorkflowExecutions(w http.ResponseWriter, r *http.Request) {
	workflowID := chi.URLParam(r, "workflowId")
	list, err := h.execStore.ListByWorkflow(workflowID, 50)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if list == nil {
		list = []storage.Execution{}
	}
	result := make([]ExecutionInspectorDTO, 0, len(list))
	for _, exec := range list {
		if !requestAllowsWorkflow(r, exec.WorkflowID) {
			continue
		}
		result = append(result, executionInspectorDTOFromExecution(exec))
	}
	renderJSON(w, http.StatusOK, result)
}

func (h *ExecutionHandler) CancelExecution(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	exec, err := h.execStore.GetByID(id)
	if err != nil {
		http.Error(w, "Execution log not found", http.StatusNotFound)
		return
	}
	if !requestAllowsWorkflow(r, exec.WorkflowID) {
		forbidden(w)
		return
	}
	if isExecutionTerminal(exec.Status) {
		renderJSON(w, http.StatusOK, map[string]interface{}{
			"id":      exec.ID,
			"status":  exec.Status,
			"message": "execution is already terminal",
		})
		return
	}
	if h.engine == nil || !h.engine.CancelExecution(id) {
		http.Error(w, "Execution is not currently cancellable", http.StatusConflict)
		return
	}
	renderJSON(w, http.StatusAccepted, map[string]interface{}{
		"id":      exec.ID,
		"status":  "CANCEL_REQUESTED",
		"message": "cancellation requested",
	})
}

type ExecutionInspectorDTO struct {
	ID               string        `json:"id"`
	WorkflowID       string        `json:"workflow_id"`
	Status           string        `json:"status"`
	DurationMs       int64         `json:"duration_ms"`
	LogsJSON         string        `json:"logs_json"`
	NodeLogs         []interface{} `json:"node_logs"`
	StartedAt        time.Time     `json:"started_at"`
	FinishedAt       *time.Time    `json:"finished_at,omitempty"`
	TriggerSource    string        `json:"trigger_source,omitempty"`
	TriggerPrincipal string        `json:"trigger_principal,omitempty"`
	RequestID        string        `json:"request_id,omitempty"`
	IdempotencyKey   string        `json:"idempotency_key,omitempty"`
	InputJSON        string        `json:"input_json,omitempty"`
	Input            interface{}   `json:"input,omitempty"`
	ErrorMessage     string        `json:"error_message,omitempty"`
	CancelledAt      *time.Time    `json:"cancelled_at,omitempty"`
}

func executionInspectorDTOFromExecution(exec storage.Execution) ExecutionInspectorDTO {
	redactedLogs := redactJSONPayload(exec.LogsJSON, []interface{}{})
	nodeLogs, _ := redactedLogs.([]interface{})
	if nodeLogs == nil {
		nodeLogs = []interface{}{}
	}
	redactedInput := redactJSONPayload(exec.InputJSON, nil)
	logsJSONBytes, _ := json.Marshal(nodeLogs)
	inputJSONBytes, _ := json.Marshal(redactedInput)
	inputJSON := ""
	if redactedInput != nil {
		inputJSON = string(inputJSONBytes)
	}
	return ExecutionInspectorDTO{
		ID:               exec.ID,
		WorkflowID:       exec.WorkflowID,
		Status:           exec.Status,
		DurationMs:       exec.DurationMs,
		LogsJSON:         string(logsJSONBytes),
		NodeLogs:         nodeLogs,
		StartedAt:        exec.StartedAt,
		FinishedAt:       exec.FinishedAt,
		TriggerSource:    exec.TriggerSource,
		TriggerPrincipal: exec.TriggerPrincipal,
		RequestID:        exec.RequestID,
		IdempotencyKey:   engine.RedactSensitiveString(exec.IdempotencyKey),
		InputJSON:        inputJSON,
		Input:            redactedInput,
		ErrorMessage:     engine.RedactSensitiveString(exec.ErrorMessage),
		CancelledAt:      exec.CancelledAt,
	}
}

func redactJSONPayload(raw string, fallback interface{}) interface{} {
	if strings.TrimSpace(raw) == "" {
		return fallback
	}
	var decoded interface{}
	if err := json.Unmarshal([]byte(raw), &decoded); err != nil {
		return engine.RedactSensitiveString(raw)
	}
	return engine.RedactSensitive(decoded)
}

func requestAllowsWorkflow(r *http.Request, workflowID string) bool {
	auth, ok := AuthFromContext(r.Context())
	if !ok || auth.Admin || auth.Token == nil {
		return true
	}
	return auth.Token.AllowsWorkflow(workflowID)
}

func isExecutionTerminal(status string) bool {
	switch strings.ToUpper(status) {
	case "SUCCESS", "FAILED", "CANCELLED", "INTERRUPTED", "REJECTED":
		return true
	default:
		return false
	}
}
