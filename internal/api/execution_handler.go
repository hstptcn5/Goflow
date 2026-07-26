package api

import (
	"net/http"
	"strings"

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
	renderJSON(w, http.StatusOK, exec)
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
	renderJSON(w, http.StatusOK, list)
}

func (h *ExecutionHandler) CancelExecution(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	exec, err := h.execStore.GetByID(id)
	if err != nil {
		http.Error(w, "Execution log not found", http.StatusNotFound)
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

func isExecutionTerminal(status string) bool {
	switch strings.ToUpper(status) {
	case "SUCCESS", "FAILED", "CANCELLED", "INTERRUPTED", "REJECTED":
		return true
	default:
		return false
	}
}
