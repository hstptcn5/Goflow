package api

import (
	"net/http"

	"goflow/internal/storage"

	"github.com/go-chi/chi/v5"
)

type ExecutionHandler struct {
	execStore *storage.ExecutionStore
}

func NewExecutionHandler(es *storage.ExecutionStore) *ExecutionHandler {
	return &ExecutionHandler{execStore: es}
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
