package api

import (
	"crypto/subtle"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"regexp"
	"strings"
	"time"

	"goflow/internal/application"
	"goflow/internal/engine"
	"goflow/internal/nodes"
	"goflow/internal/storage"

	"github.com/go-chi/chi/v5"
)

type WorkflowHandler struct {
	wfStore            *storage.WorkflowStore
	triggerService     *application.TriggerService
	webhookRateLimiter *fixedWindowRateLimiter
}

func NewWorkflowHandler(ws *storage.WorkflowStore, triggerService *application.TriggerService, webhookRateLimitPerMinute int) *WorkflowHandler {
	return &WorkflowHandler{
		wfStore:            ws,
		triggerService:     triggerService,
		webhookRateLimiter: newFixedWindowRateLimiter(webhookRateLimitPerMinute, time.Minute),
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
	if existing, err := h.wfStore.GetByID(id); err == nil {
		preserveWorkflowInterface(&wf, existing)
	}

	if err := h.wfStore.Update(&wf); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	updated, _ := h.wfStore.GetByID(id)
	renderJSON(w, http.StatusOK, updated)
}

func (h *WorkflowHandler) GetWorkflowInterface(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	wf, err := h.wfStore.GetByID(id)
	if err != nil {
		http.Error(w, "Workflow not found", http.StatusNotFound)
		return
	}
	renderJSON(w, http.StatusOK, workflowInterfaceFromWorkflow(wf))
}

func (h *WorkflowHandler) UpdateWorkflowInterface(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	wf, err := h.wfStore.GetByID(id)
	if err != nil {
		http.Error(w, "Workflow not found", http.StatusNotFound)
		return
	}

	var req workflowInterface
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}
	if err := validateWorkflowInterface(req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	applyWorkflowInterface(wf, req)
	if err := h.wfStore.Update(wf); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	updated, _ := h.wfStore.GetByID(id)
	renderJSON(w, http.StatusOK, workflowInterfaceFromWorkflow(updated))
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
	var payload interface{}
	if strings.HasPrefix(r.Header.Get("Content-Type"), "application/json") {
		_ = json.NewDecoder(r.Body).Decode(&payload)
	}

	if r.URL.Query().Get("async") == "true" {
		result, err := h.triggerService.Trigger(r.Context(), application.TriggerRequest{
			WorkflowID:     id,
			Input:          payload,
			Mode:           application.ModeAsync,
			Source:         "api",
			Principal:      requestPrincipal(r),
			RequestID:      r.Header.Get("X-Request-ID"),
			IdempotencyKey: r.Header.Get("Idempotency-Key"),
		})
		if err != nil {
			writeExecutionError(w, err)
			return
		}
		renderJSON(w, http.StatusAccepted, executionAcceptedResponse(result))
		return
	}

	result, err := h.triggerService.Trigger(r.Context(), application.TriggerRequest{
		WorkflowID: id,
		Input:      payload,
		Mode:       application.ModeSync,
		Source:     application.SourceAPI,
		Principal:  requestPrincipal(r),
		RequestID:  r.Header.Get("X-Request-ID"),
	})
	if err != nil {
		writeExecutionError(w, err)
		return
	}

	renderJSON(w, http.StatusOK, result.Execution)
}

func (h *WorkflowHandler) CreateExecution(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var req struct {
		Input          interface{} `json:"input"`
		Mode           string      `json:"mode"`
		IdempotencyKey string      `json:"idempotency_key"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil && !errors.Is(err, io.EOF) {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}
	if req.Mode == "" {
		req.Mode = "async"
	}
	if req.Mode != "async" {
		http.Error(w, "Only async mode is supported by this endpoint", http.StatusBadRequest)
		return
	}
	if req.IdempotencyKey == "" {
		req.IdempotencyKey = r.Header.Get("Idempotency-Key")
	}

	result, err := h.triggerService.Trigger(r.Context(), application.TriggerRequest{
		WorkflowID:     id,
		Input:          req.Input,
		Mode:           application.ModeAsync,
		Source:         application.SourceAPI,
		Principal:      requestPrincipal(r),
		RequestID:      r.Header.Get("X-Request-ID"),
		IdempotencyKey: req.IdempotencyKey,
	})
	if err != nil {
		writeExecutionError(w, err)
		return
	}
	renderJSON(w, http.StatusAccepted, executionAcceptedResponse(result))
}

func (h *WorkflowHandler) TriggerWebhook(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "workflowId")
	wf, err := h.wfStore.GetByID(id)
	if err != nil {
		http.Error(w, "Workflow not found", http.StatusNotFound)
		return
	}
	if !wf.IsActive {
		http.Error(w, "Workflow is inactive", http.StatusConflict)
		return
	}
	if !h.webhookRateLimiter.Allow(rateLimitKey(r, id)) {
		http.Error(w, "Webhook rate limit exceeded", http.StatusTooManyRequests)
		return
	}

	bodyBytes, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "Failed to read request body", http.StatusBadRequest)
		return
	}

	var nodeList []map[string]interface{}
	if err := json.Unmarshal([]byte(wf.NodesJSON), &nodeList); err == nil {
		for _, node := range nodeList {
			nodeType, _ := node["type"].(string)
			if nodeType != string(nodes.TypeWebhookTrigger) && nodeType != string(nodes.TypeGithubWebhook) {
				continue
			}
			params, _ := node["params"].(map[string]interface{})
			secret, _ := params["secret"].(string)
			if secret == "" {
				continue
			}
			gotSecret := r.Header.Get("X-Goflow-Webhook-Secret")
			if subtle.ConstantTimeCompare([]byte(gotSecret), []byte(secret)) != 1 {
				http.Error(w, "Invalid webhook secret", http.StatusUnauthorized)
				return
			}
		}
	}

	var body interface{}
	if strings.HasPrefix(r.Header.Get("Content-Type"), "application/json") && len(bodyBytes) > 0 {
		_ = json.Unmarshal(bodyBytes, &body)
	}
	if body == nil && len(bodyBytes) > 0 {
		body = string(bodyBytes)
	}

	headers := make(map[string]interface{}, len(r.Header))
	for key, values := range r.Header {
		headers[key] = values
	}
	query := make(map[string]interface{}, len(r.URL.Query()))
	for key, values := range r.URL.Query() {
		query[key] = values
	}

	payload := map[string]interface{}{
		"body":     body,
		"body_raw": string(bodyBytes),
		"headers":  headers,
		"method":   r.Method,
		"path":     r.URL.Path,
		"query":    query,
	}

	if r.URL.Query().Get("async") == "true" {
		result, err := h.triggerService.Trigger(r.Context(), application.TriggerRequest{
			WorkflowID:     id,
			Input:          payload,
			Mode:           application.ModeAsync,
			Source:         application.SourceWebhook,
			Principal:      r.RemoteAddr,
			RequestID:      r.Header.Get("X-Request-ID"),
			IdempotencyKey: r.Header.Get("Idempotency-Key"),
		})
		if err != nil {
			writeExecutionError(w, err)
			return
		}
		renderJSON(w, http.StatusAccepted, executionAcceptedResponse(result))
		return
	}

	result, err := h.triggerService.Trigger(r.Context(), application.TriggerRequest{
		WorkflowID: id,
		Input:      payload,
		Mode:       application.ModeSync,
		Source:     application.SourceWebhook,
		Principal:  r.RemoteAddr,
		RequestID:  r.Header.Get("X-Request-ID"),
	})
	if err != nil {
		writeExecutionError(w, err)
		return
	}
	renderJSON(w, http.StatusOK, result.Execution)
}

func executionAcceptedResponse(result *application.TriggerResult) map[string]interface{} {
	return map[string]interface{}{
		"execution_id": result.Execution.ID,
		"workflow_id":  result.Execution.WorkflowID,
		"status":       result.Execution.Status,
		"deduplicated": result.Deduplicated,
	}
}

func requestPrincipal(r *http.Request) string {
	if value := r.Header.Get("X-Goflow-Principal"); value != "" {
		return value
	}
	return r.RemoteAddr
}

func writeExecutionError(w http.ResponseWriter, err error) {
	if errors.Is(err, engine.ErrConcurrencyLimit) {
		http.Error(w, err.Error(), http.StatusTooManyRequests)
		return
	}
	if strings.Contains(strings.ToLower(err.Error()), "workflow not found") {
		http.Error(w, "Workflow not found", http.StatusNotFound)
		return
	}
	http.Error(w, err.Error(), http.StatusInternalServerError)
}

type workflowInterface struct {
	WorkflowID        string `json:"workflow_id,omitempty"`
	Slug              string `json:"slug"`
	InputSchemaJSON   string `json:"input_schema_json"`
	OutputSchemaJSON  string `json:"output_schema_json"`
	ExposeCLI         bool   `json:"expose_cli"`
	ExposeMCP         bool   `json:"expose_mcp"`
	MCPToolName       string `json:"mcp_tool_name"`
	MCPDescription    string `json:"mcp_description"`
	RiskLevel         string `json:"risk_level"`
	RequiresApproval  bool   `json:"requires_approval"`
	MaxConcurrentRuns int    `json:"max_concurrent_runs"`
	ConcurrencyPolicy string `json:"concurrency_policy"`
}

var interfaceNamePattern = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9_-]{0,63}$`)

func workflowInterfaceFromWorkflow(wf *storage.Workflow) workflowInterface {
	return workflowInterface{
		WorkflowID:        wf.ID,
		Slug:              wf.Slug,
		InputSchemaJSON:   wf.InputSchemaJSON,
		OutputSchemaJSON:  wf.OutputSchemaJSON,
		ExposeCLI:         wf.ExposeCLI,
		ExposeMCP:         wf.ExposeMCP,
		MCPToolName:       wf.MCPToolName,
		MCPDescription:    wf.MCPDescription,
		RiskLevel:         wf.RiskLevel,
		RequiresApproval:  wf.RequiresApproval,
		MaxConcurrentRuns: wf.MaxConcurrentRuns,
		ConcurrencyPolicy: wf.ConcurrencyPolicy,
	}
}

func applyWorkflowInterface(wf *storage.Workflow, req workflowInterface) {
	wf.Slug = strings.TrimSpace(req.Slug)
	wf.InputSchemaJSON = strings.TrimSpace(req.InputSchemaJSON)
	wf.OutputSchemaJSON = strings.TrimSpace(req.OutputSchemaJSON)
	wf.ExposeCLI = req.ExposeCLI
	wf.ExposeMCP = req.ExposeMCP
	wf.MCPToolName = strings.TrimSpace(req.MCPToolName)
	wf.MCPDescription = strings.TrimSpace(req.MCPDescription)
	wf.RiskLevel = strings.TrimSpace(req.RiskLevel)
	wf.RequiresApproval = req.RequiresApproval
	wf.MaxConcurrentRuns = req.MaxConcurrentRuns
	wf.ConcurrencyPolicy = strings.TrimSpace(req.ConcurrencyPolicy)
}

func preserveWorkflowInterface(wf *storage.Workflow, existing *storage.Workflow) {
	wf.Slug = existing.Slug
	wf.InputSchemaJSON = existing.InputSchemaJSON
	wf.OutputSchemaJSON = existing.OutputSchemaJSON
	wf.ExposeCLI = existing.ExposeCLI
	wf.ExposeMCP = existing.ExposeMCP
	wf.MCPToolName = existing.MCPToolName
	wf.MCPDescription = existing.MCPDescription
	wf.RiskLevel = existing.RiskLevel
	wf.RequiresApproval = existing.RequiresApproval
	wf.MaxConcurrentRuns = existing.MaxConcurrentRuns
	wf.ConcurrencyPolicy = existing.ConcurrencyPolicy
}

func validateWorkflowInterface(req workflowInterface) error {
	if req.Slug != "" && !interfaceNamePattern.MatchString(req.Slug) {
		return errors.New("slug must use letters, numbers, dash, or underscore and be 1-64 characters")
	}
	if req.MCPToolName != "" && !interfaceNamePattern.MatchString(req.MCPToolName) {
		return errors.New("MCP tool name must use letters, numbers, dash, or underscore and be 1-64 characters")
	}
	if err := validateJSONObject(req.InputSchemaJSON, "input_schema_json"); err != nil {
		return err
	}
	if err := validateJSONObject(req.OutputSchemaJSON, "output_schema_json"); err != nil {
		return err
	}
	switch req.RiskLevel {
	case "", "low", "medium", "high":
	default:
		return errors.New("risk_level must be low, medium, or high")
	}
	switch req.ConcurrencyPolicy {
	case "", "global", "allow", "reject", "queue":
	default:
		return errors.New("concurrency_policy must be global, allow, reject, or queue")
	}
	if req.MaxConcurrentRuns < 0 {
		return errors.New("max_concurrent_runs cannot be negative")
	}
	return nil
}

func validateJSONObject(raw string, field string) error {
	if strings.TrimSpace(raw) == "" {
		return nil
	}
	var decoded interface{}
	if err := json.Unmarshal([]byte(raw), &decoded); err != nil {
		return errors.New(field + " must be valid JSON")
	}
	if _, ok := decoded.(map[string]interface{}); !ok {
		return errors.New(field + " must be a JSON object")
	}
	return nil
}
