package api

import (
	"encoding/json"
	"net/http"
	"strings"

	"goflow/internal/engine"
	"goflow/internal/storage"

	"github.com/go-chi/chi/v5"
)

type WorkflowHandler struct {
	wfStore *storage.WorkflowStore
	engine  *engine.Engine
}

func NewWorkflowHandler(ws *storage.WorkflowStore, eng *engine.Engine) *WorkflowHandler {
	return &WorkflowHandler{
		wfStore: ws,
		engine:  eng,
	}
}

func (h *WorkflowHandler) ListWorkflows(w http.ResponseWriter, r *http.Request) {
	list, err := h.wfStore.ListAll()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if list == nil {
		list = []storage.Workflow{}
	}
	renderJSON(w, http.StatusOK, list)
}

func (h *WorkflowHandler) GetWorkflow(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	wf, err := h.wfStore.GetByID(id)
	if err != nil {
		http.Error(w, "Workflow not found", http.StatusNotFound)
		return
	}
	renderJSON(w, http.StatusOK, wf)
}

func (h *WorkflowHandler) CreateWorkflow(w http.ResponseWriter, r *http.Request) {
	var wf storage.Workflow
	if err := json.NewDecoder(r.Body).Decode(&wf); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if wf.Name == "" {
		http.Error(w, "Name is required", http.StatusBadRequest)
		return
	}

	if wf.NodesJSON == "" {
		wf.NodesJSON = "[]"
	}
	if wf.EdgesJSON == "" {
		wf.EdgesJSON = "[]"
	}

	if err := h.wfStore.Create(&wf); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	renderJSON(w, http.StatusCreated, wf)
}

func (h *WorkflowHandler) UpdateWorkflow(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var wf storage.Workflow
	if err := json.NewDecoder(r.Body).Decode(&wf); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}
	wf.ID = id

	if err := h.wfStore.Update(&wf); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	updated, _ := h.wfStore.GetByID(id)
	renderJSON(w, http.StatusOK, updated)
}

func (h *WorkflowHandler) ToggleActive(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var req struct {
		IsActive bool `json:"is_active"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request payload", http.StatusBadRequest)
		return
	}

	if err := h.wfStore.ToggleActive(id, req.IsActive); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	renderJSON(w, http.StatusOK, map[string]interface{}{"status": "ok", "is_active": req.IsActive})
}

func (h *WorkflowHandler) DeleteWorkflow(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if err := h.wfStore.Delete(id); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	renderJSON(w, http.StatusOK, map[string]string{"message": "Workflow deleted"})
}

func (h *WorkflowHandler) TriggerWorkflow(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		id = chi.URLParam(r, "workflowId")
	}
	wf, err := h.wfStore.GetByID(id)
	if err != nil {
		http.Error(w, "Workflow not found", http.StatusNotFound)
		return
	}

	var payload interface{}
	if strings.HasPrefix(r.Header.Get("Content-Type"), "application/json") {
		_ = json.NewDecoder(r.Body).Decode(&payload)
	}

	// Thực thi async hoặc sync dựa trên query param `async=true`
	if r.URL.Query().Get("async") == "true" {
		go func() {
			_, _ = h.engine.ExecuteWorkflow(wf, payload)
		}()
		renderJSON(w, http.StatusAccepted, map[string]string{"message": "Workflow triggered in background"})
		return
	}

	exec, err := h.engine.ExecuteWorkflow(wf, payload)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	renderJSON(w, http.StatusOK, exec)
}
