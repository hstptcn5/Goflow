package api

import (
	"crypto/subtle"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"net"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	"goflow/internal/application"
	"goflow/internal/engine"
	"goflow/internal/pack"
	"goflow/internal/packsetup"
	"goflow/internal/storage"

	"github.com/go-chi/chi/v5"
)

const applianceTokenHeader = "X-Goflow-Appliance-Token"
const applianceMaxJSONBody int64 = 64 << 10

type ApplianceContext struct {
	Enabled                bool
	Origin                 string
	SessionToken           string
	PackID                 string
	PackName               string
	PackVersion            string
	Description            string
	WorkflowID             string
	DataDir                string
	ConfigSchema           []pack.ConfigField
	CredentialRequirements []pack.CredentialRequirement
	LegacyRequiredCreds    []string
	TelegramAPIBaseURL     string
	ConnectionTestClient   *http.Client
}

func mountApplianceRoutes(
	r chi.Router,
	appliance *ApplianceContext,
	wfStore *storage.WorkflowStore,
	execStore *storage.ExecutionStore,
	credStore *storage.CredentialStore,
	triggerService *application.TriggerService,
) {
	if appliance == nil || !appliance.Enabled {
		return
	}
	credentialTestLimiter := newFixedWindowRateLimiter(10, time.Minute)
	credentialTestSlots := make(chan struct{}, 1)
	r.Route("/api/appliance", func(r chi.Router) {
		r.Use(applianceHostMiddleware(appliance))
		r.Get("/bootstrap", applianceBootstrapHandler(appliance))
		r.Get("/status", applianceStatusHandler(appliance, wfStore, execStore, credStore))
		r.Get("/setup", applianceSetupHandler(appliance, credStore))
		r.Get("/diagnostics", applianceDiagnosticsHandler(appliance, wfStore, execStore, credStore))
		r.Get("/workflow/status", applianceWorkflowStatusHandler(appliance, wfStore, execStore, credStore))
		r.Get("/executions/latest", applianceLatestExecutionHandler(appliance, execStore))
		r.Get("/executions", applianceRecentExecutionsHandler(appliance, execStore))
		r.Group(func(r chi.Router) {
			r.Use(applianceMutationMiddleware(appliance))
			r.Post("/setup/config", applianceSaveConfigHandler(appliance))
			r.Post("/setup/credentials", applianceSaveCredentialsHandler(appliance, credStore))
			r.Post("/setup/credentials/create", applianceCreateCredentialHandler(appliance, credStore))
			r.Post("/setup/credentials/test", applianceTestCredentialHandler(appliance, credStore, credentialTestLimiter, credentialTestSlots))
			r.Post("/setup/complete", applianceCompleteHandler(appliance, credStore))
			r.Post("/setup/reopen", applianceReopenHandler(appliance))
			r.Post("/workflow/run", applianceRunWorkflowHandler(appliance, credStore, triggerService))
		})
	})
}

func applianceBootstrapHandler(appliance *ApplianceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		renderJSON(w, http.StatusOK, map[string]interface{}{
			"token": appliance.SessionToken,
			"pack":  applianceIdentity(appliance),
		})
	}
}

func applianceStatusHandler(appliance *ApplianceContext, wfStore *storage.WorkflowStore, execStore *storage.ExecutionStore, credStore *storage.CredentialStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		state, missing, completed := applianceRuntimeState(appliance, wfStore, execStore, applianceCredentialResolver(credStore))
		renderJSON(w, http.StatusOK, map[string]interface{}{
			"pack":           applianceIdentity(appliance),
			"workflow_id":    appliance.WorkflowID,
			"server":         "ok",
			"state":          state,
			"missing":        missing,
			"setup_complete": completed,
			"can_complete":   len(missing) == 0,
		})
	}
}

func applianceDiagnosticsHandler(appliance *ApplianceContext, wfStore *storage.WorkflowStore, execStore *storage.ExecutionStore, credStore *storage.CredentialStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		state, missing, completed := applianceRuntimeState(appliance, wfStore, execStore, applianceCredentialResolver(credStore))
		latest, _ := applianceLatestExecution(execStore, appliance.WorkflowID)
		renderJSON(w, http.StatusOK, map[string]interface{}{
			"pack":                  applianceIdentity(appliance),
			"workflow_id":           appliance.WorkflowID,
			"server":                "ok",
			"state":                 state,
			"missing":               missing,
			"setup_complete":        completed,
			"latest_execution":      latest,
			"credential_ids_hidden": true,
			"secrets_hidden":        true,
			"generated_at":          time.Now().UTC(),
		})
	}
}

func applianceSetupHandler(appliance *ApplianceContext, credStore *storage.CredentialStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		resolver := applianceCredentialResolver(credStore)
		state, missing, completed := applianceReadiness(appliance, resolver)
		configValues := map[string]interface{}{}
		if loaded, err := packsetup.LoadConfig(appliance.DataDir, applianceManifest(appliance)); err == nil {
			configValues = loaded.Config.Values
		}
		renderJSON(w, http.StatusOK, map[string]interface{}{
			"state":                     state,
			"missing":                   missing,
			"config_schema":             appliance.ConfigSchema,
			"credential_requirements":   redactedCredentialRequirements(appliance, credStore),
			"legacy_required_creds":     appliance.LegacyRequiredCreds,
			"current_config_values":     configValues,
			"credential_values_hidden":  true,
			"credential_ids_redacted":   true,
			"decrypted_values_returned": false,
			"setup_complete":            completed,
			"can_complete":              len(missing) == 0,
		})
	}
}

func applianceSaveConfigHandler(appliance *ApplianceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Values map[string]interface{} `json:"values"`
		}
		if err := decodeApplianceJSON(w, r, &req); err != nil {
			return
		}
		if req.Values == nil {
			req.Values = map[string]interface{}{}
		}
		cfg, err := packsetup.SaveConfig(appliance.DataDir, applianceManifest(appliance), req.Values)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		renderJSON(w, http.StatusOK, map[string]interface{}{"values": cfg.Values})
	}
}

func applianceSaveCredentialsHandler(appliance *ApplianceContext, credStore *storage.CredentialStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Slots map[string]string `json:"slots"`
		}
		if err := decodeApplianceJSON(w, r, &req); err != nil {
			return
		}
		if req.Slots == nil {
			req.Slots = map[string]string{}
		}
		creds, err := packsetup.SaveCredentialBindings(appliance.DataDir, applianceManifest(appliance), req.Slots, applianceCredentialResolver(credStore))
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		renderJSON(w, http.StatusOK, map[string]interface{}{"credentials": redactedCredentialSlots(creds.Slots)})
	}
}

func applianceCreateCredentialHandler(appliance *ApplianceContext, credStore *storage.CredentialStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if credStore == nil {
			http.Error(w, "credential store is not available", http.StatusInternalServerError)
			return
		}
		var req struct {
			Key   string `json:"key"`
			Name  string `json:"name"`
			Value string `json:"value"`
		}
		if err := decodeApplianceJSON(w, r, &req); err != nil {
			return
		}
		requirement, ok := credentialRequirement(appliance, req.Key)
		if !ok {
			http.Error(w, "credential slot is not declared", http.StatusBadRequest)
			return
		}
		if strings.TrimSpace(req.Name) == "" || strings.TrimSpace(req.Value) == "" {
			http.Error(w, "credential name and value are required", http.StatusBadRequest)
			return
		}
		cred, err := credStore.Create(req.Name, requirement.Type, req.Value)
		if err != nil {
			http.Error(w, "credential could not be saved", http.StatusInternalServerError)
			return
		}
		creds, err := packsetup.SaveCredentialBindings(appliance.DataDir, applianceManifest(appliance), map[string]string{req.Key: cred.ID}, applianceCredentialResolver(credStore))
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		renderJSON(w, http.StatusCreated, map[string]interface{}{"credentials": redactedCredentialSlots(creds.Slots)})
	}
}

func applianceTestCredentialHandler(appliance *ApplianceContext, credStore *storage.CredentialStore, limiter *fixedWindowRateLimiter, slots chan struct{}) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !limiter.Allow(rateLimitKey(r, appliance.PackID+":credential-test")) {
			http.Error(w, "credential test rate limit exceeded", http.StatusTooManyRequests)
			return
		}
		select {
		case slots <- struct{}{}:
			defer func() { <-slots }()
		default:
			http.Error(w, "credential test already running", http.StatusTooManyRequests)
			return
		}
		if credStore == nil {
			http.Error(w, "credential store is not available", http.StatusInternalServerError)
			return
		}
		var req struct {
			Key string `json:"key"`
		}
		if err := decodeApplianceJSON(w, r, &req); err != nil {
			return
		}
		requirement, ok := credentialRequirement(appliance, req.Key)
		if !ok {
			http.Error(w, "credential slot is not declared", http.StatusBadRequest)
			return
		}
		if requirement.TestKind == "" {
			renderJSON(w, http.StatusOK, map[string]interface{}{"status": "SKIPPED", "reason": "no connection test is declared"})
			return
		}
		loaded, err := packsetup.LoadCredentialBindings(appliance.DataDir, applianceManifest(appliance), applianceCredentialResolver(credStore))
		if err != nil {
			http.Error(w, "credential slot is not ready", http.StatusBadRequest)
			return
		}
		slot, ok := loaded.Credentials.Slots[req.Key]
		if !ok {
			http.Error(w, "credential slot is not ready", http.StatusBadRequest)
			return
		}
		secret, err := credStore.GetDecryptedData(slot.CredentialID)
		if err != nil {
			http.Error(w, "credential value is not available", http.StatusBadRequest)
			return
		}
		switch requirement.TestKind {
		case "telegram_get_me":
			if err := applianceTelegramGetMe(r, appliance, secret); err != nil {
				renderJSON(w, http.StatusBadGateway, map[string]interface{}{"status": "FAILED", "error": err.Error()})
				return
			}
			renderJSON(w, http.StatusOK, map[string]interface{}{"status": "OK"})
		default:
			renderJSON(w, http.StatusOK, map[string]interface{}{"status": "SKIPPED", "reason": "connection test is not implemented"})
		}
	}
}

func applianceWorkflowStatusHandler(appliance *ApplianceContext, wfStore *storage.WorkflowStore, execStore *storage.ExecutionStore, credStore *storage.CredentialStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		state, missing, completed := applianceRuntimeState(appliance, wfStore, execStore, applianceCredentialResolver(credStore))
		response := map[string]interface{}{
			"workflow_id":    appliance.WorkflowID,
			"server":         "ok",
			"state":          state,
			"missing":        missing,
			"setup_complete": completed,
			"latest_execution": func() interface{} {
				latest, _ := applianceLatestExecution(execStore, appliance.WorkflowID)
				return latest
			}(),
		}
		if wfStore != nil {
			if wf, err := wfStore.GetByID(appliance.WorkflowID); err == nil {
				response["workflow"] = map[string]interface{}{
					"id":        wf.ID,
					"name":      wf.Name,
					"is_active": wf.IsActive,
					"risk":      wf.RiskLevel,
				}
			} else {
				response["state"] = "ERROR"
				response["error"] = "managed workflow is not available"
			}
		}
		renderJSON(w, http.StatusOK, response)
	}
}

func applianceRunWorkflowHandler(appliance *ApplianceContext, credStore *storage.CredentialStore, triggerService *application.TriggerService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Input          interface{} `json:"input"`
			IdempotencyKey string      `json:"idempotency_key"`
		}
		if err := decodeApplianceJSON(w, r, &req); err != nil {
			return
		}
		if triggerService == nil {
			http.Error(w, "workflow runner is not available", http.StatusServiceUnavailable)
			return
		}
		state, missing, _ := applianceReadiness(appliance, applianceCredentialResolver(credStore))
		if state != "READY" {
			renderJSON(w, http.StatusConflict, map[string]interface{}{"error": "setup requirements are missing", "missing": missing, "state": state})
			return
		}
		result, err := triggerService.Trigger(r.Context(), application.TriggerRequest{
			WorkflowID:     appliance.WorkflowID,
			Input:          req.Input,
			Mode:           application.ModeAsync,
			Source:         application.SourceUI,
			Principal:      "appliance:" + appliance.PackID,
			RequestID:      r.Header.Get("X-Request-ID"),
			IdempotencyKey: req.IdempotencyKey,
		})
		if err != nil {
			writeExecutionError(w, err)
			return
		}
		renderJSON(w, http.StatusAccepted, executionAcceptedResponse(result))
	}
}

func applianceLatestExecutionHandler(appliance *ApplianceContext, execStore *storage.ExecutionStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		latest, err := applianceLatestExecution(execStore, appliance.WorkflowID)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		renderJSON(w, http.StatusOK, map[string]interface{}{"execution": latest})
	}
}

func applianceRecentExecutionsHandler(appliance *ApplianceContext, execStore *storage.ExecutionStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		limit := 10
		if raw := strings.TrimSpace(r.URL.Query().Get("limit")); raw != "" {
			parsed, err := strconv.Atoi(raw)
			if err != nil || parsed <= 0 {
				http.Error(w, "limit must be a positive integer", http.StatusBadRequest)
				return
			}
			limit = parsed
		}
		if limit > 50 {
			limit = 50
		}
		executions, err := applianceRecentExecutions(execStore, appliance.WorkflowID, limit)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		renderJSON(w, http.StatusOK, map[string]interface{}{"executions": executions, "limit": limit})
	}
}

func applianceCompleteHandler(appliance *ApplianceContext, credStore *storage.CredentialStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req struct{}
		if err := decodeApplianceJSON(w, r, &req); err != nil {
			return
		}
		missing := applianceMissingRequirements(appliance, applianceCredentialResolver(credStore))
		if len(missing) > 0 {
			renderJSON(w, http.StatusBadRequest, map[string]interface{}{"error": "setup requirements are missing", "missing": missing})
			return
		}
		state, err := packsetup.SaveState(appliance.DataDir, applianceManifest(appliance), true, time.Now())
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		renderJSON(w, http.StatusOK, map[string]interface{}{"state": "READY", "setup_complete": state.Completed})
	}
}

func applianceReopenHandler(appliance *ApplianceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req struct{}
		if err := decodeApplianceJSON(w, r, &req); err != nil {
			return
		}
		if _, err := packsetup.SaveState(appliance.DataDir, applianceManifest(appliance), false, time.Now()); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		renderJSON(w, http.StatusOK, map[string]interface{}{"state": "NEEDS_SETUP", "setup_complete": false})
	}
}

func applianceNotImplementedMutation(w http.ResponseWriter, r *http.Request) {
	renderJSON(w, http.StatusNotImplemented, map[string]string{"error": "appliance mutation is not implemented yet"})
}

func applianceIdentity(appliance *ApplianceContext) map[string]string {
	return map[string]string{
		"id":          appliance.PackID,
		"name":        appliance.PackName,
		"version":     appliance.PackVersion,
		"description": appliance.Description,
	}
}

func applianceHostMiddleware(appliance *ApplianceContext) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !sameHost(r.Host, appliance.Origin) {
				http.NotFound(w, r)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

func applianceMutationMiddleware(appliance *ApplianceContext) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			mediaType, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
			if err != nil || mediaType != "application/json" {
				http.Error(w, "content type must be application/json", http.StatusUnsupportedMediaType)
				return
			}
			if r.Header.Get("Origin") != appliance.Origin {
				http.Error(w, "origin is not allowed", http.StatusForbidden)
				return
			}
			if subtle.ConstantTimeCompare([]byte(r.Header.Get(applianceTokenHeader)), []byte(appliance.SessionToken)) != 1 {
				http.Error(w, "appliance token is required", http.StatusForbidden)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

func sameHost(hostHeader, origin string) bool {
	originHost := strings.TrimPrefix(strings.TrimPrefix(origin, "http://"), "https://")
	requestHost := hostHeader
	if h, p, err := net.SplitHostPort(hostHeader); err == nil {
		requestHost = net.JoinHostPort(normalizeLoopbackHost(h), p)
	}
	if h, p, err := net.SplitHostPort(originHost); err == nil {
		originHost = net.JoinHostPort(normalizeLoopbackHost(h), p)
	}
	return strings.EqualFold(requestHost, originHost)
}

func normalizeLoopbackHost(host string) string {
	if strings.EqualFold(host, "localhost") {
		return "127.0.0.1"
	}
	return host
}

func decodeApplianceJSON(w http.ResponseWriter, r *http.Request, dest interface{}) error {
	r.Body = http.MaxBytesReader(w, r.Body, applianceMaxJSONBody)
	defer r.Body.Close()
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(dest); err != nil {
		http.Error(w, "invalid JSON body", http.StatusBadRequest)
		return err
	}
	if err := dec.Decode(&struct{}{}); err != io.EOF {
		http.Error(w, "invalid JSON body", http.StatusBadRequest)
		return fmt.Errorf("extra JSON content")
	}
	return nil
}

func applianceManifest(appliance *ApplianceContext) pack.Manifest {
	return pack.Manifest{
		ID:                     appliance.PackID,
		Name:                   appliance.PackName,
		Version:                appliance.PackVersion,
		Description:            appliance.Description,
		ConfigSchema:           appliance.ConfigSchema,
		CredentialRequirements: appliance.CredentialRequirements,
		RequiredCredentials:    appliance.LegacyRequiredCreds,
	}
}

func applianceReadiness(appliance *ApplianceContext, resolver packsetup.CredentialResolver) (string, []string, bool) {
	missing := applianceMissingRequirements(appliance, resolver)
	completed := false
	if state, err := packsetup.LoadState(appliance.DataDir, applianceManifest(appliance)); err == nil {
		completed = state.Completed
	}
	if len(missing) > 0 || !completed {
		return "NEEDS_SETUP", missing, completed
	}
	return "READY", missing, completed
}

func applianceRuntimeState(appliance *ApplianceContext, wfStore *storage.WorkflowStore, execStore *storage.ExecutionStore, resolver packsetup.CredentialResolver) (string, []string, bool) {
	state, missing, completed := applianceReadiness(appliance, resolver)
	if state != "READY" {
		return state, missing, completed
	}
	if wfStore != nil {
		wf, err := wfStore.GetByID(appliance.WorkflowID)
		if err != nil || !wf.IsActive {
			return "ERROR", missing, completed
		}
	}
	executions, err := applianceRecentExecutions(execStore, appliance.WorkflowID, 10)
	if err != nil {
		return "DEGRADED", missing, completed
	}
	for _, exec := range executions {
		if strings.EqualFold(exec.Status, "RUNNING") {
			return "RUNNING", missing, completed
		}
		if strings.EqualFold(exec.Status, "FAILED") {
			return "ERROR", missing, completed
		}
	}
	return "READY", missing, completed
}

func applianceMissingRequirements(appliance *ApplianceContext, resolver packsetup.CredentialResolver) []string {
	missing := []string{}
	manifest := applianceManifest(appliance)
	if len(manifest.ConfigSchema) > 0 {
		if _, err := packsetup.LoadConfig(appliance.DataDir, manifest); err != nil {
			if errors.Is(err, os.ErrNotExist) {
				for _, field := range manifest.ConfigSchema {
					if field.Required {
						missing = append(missing, "config."+field.Key)
					}
				}
			} else {
				missing = append(missing, "config")
			}
		}
	}
	if len(manifest.CredentialRequirements) > 0 {
		if _, err := packsetup.LoadCredentialBindings(appliance.DataDir, manifest, resolver); err != nil {
			if errors.Is(err, os.ErrNotExist) {
				for _, req := range manifest.CredentialRequirements {
					if req.Required {
						missing = append(missing, "credential."+req.Key)
					}
				}
			} else {
				missing = append(missing, "credential")
			}
		}
	}
	return missing
}

type applianceExecutionSummary struct {
	ID               string      `json:"id"`
	WorkflowID       string      `json:"workflow_id"`
	Status           string      `json:"status"`
	DurationMs       int64       `json:"duration_ms"`
	StartedAt        time.Time   `json:"started_at"`
	FinishedAt       *time.Time  `json:"finished_at,omitempty"`
	TriggerSource    string      `json:"trigger_source,omitempty"`
	TriggerPrincipal string      `json:"trigger_principal,omitempty"`
	RequestID        string      `json:"request_id,omitempty"`
	Input            interface{} `json:"input,omitempty"`
	ErrorMessage     string      `json:"error_message,omitempty"`
}

func applianceLatestExecution(execStore *storage.ExecutionStore, workflowID string) (*applianceExecutionSummary, error) {
	executions, err := applianceRecentExecutions(execStore, workflowID, 1)
	if err != nil || len(executions) == 0 {
		return nil, err
	}
	return &executions[0], nil
}

func applianceRecentExecutions(execStore *storage.ExecutionStore, workflowID string, limit int) ([]applianceExecutionSummary, error) {
	if execStore == nil {
		return []applianceExecutionSummary{}, nil
	}
	list, err := execStore.ListByWorkflow(workflowID, limit)
	if err != nil {
		return nil, err
	}
	result := make([]applianceExecutionSummary, 0, len(list))
	for _, exec := range list {
		dto := executionInspectorDTOFromExecution(exec)
		result = append(result, applianceExecutionSummary{
			ID:               dto.ID,
			WorkflowID:       dto.WorkflowID,
			Status:           dto.Status,
			DurationMs:       dto.DurationMs,
			StartedAt:        dto.StartedAt,
			FinishedAt:       dto.FinishedAt,
			TriggerSource:    dto.TriggerSource,
			TriggerPrincipal: engine.RedactSensitiveString(dto.TriggerPrincipal),
			RequestID:        engine.RedactSensitiveString(dto.RequestID),
			Input:            dto.Input,
			ErrorMessage:     dto.ErrorMessage,
		})
	}
	return result, nil
}

func applianceCredentialResolver(credStore *storage.CredentialStore) packsetup.CredentialResolver {
	if credStore == nil {
		return nil
	}
	return packsetup.CredentialLookupFunc(func(id string) (packsetup.CredentialIdentity, error) {
		cred, err := credStore.GetByID(id)
		if err != nil {
			return packsetup.CredentialIdentity{}, errors.New("credential not found")
		}
		return packsetup.CredentialIdentity{ID: cred.ID, Type: cred.Type}, nil
	})
}

func redactedCredentialRequirements(appliance *ApplianceContext, credStore *storage.CredentialStore) []map[string]interface{} {
	assigned := map[string]bool{}
	if loaded, err := packsetup.LoadCredentialBindings(appliance.DataDir, applianceManifest(appliance), applianceCredentialResolver(credStore)); err == nil {
		for key := range loaded.Credentials.Slots {
			assigned[key] = true
		}
	}
	result := make([]map[string]interface{}, 0, len(appliance.CredentialRequirements))
	for _, req := range appliance.CredentialRequirements {
		result = append(result, map[string]interface{}{
			"key":          req.Key,
			"label":        req.Label,
			"description":  req.Description,
			"type":         req.Type,
			"required":     req.Required,
			"test_kind":    req.TestKind,
			"display_only": req.DisplayOnly,
			"assigned":     assigned[req.Key],
		})
	}
	return result
}

func redactedCredentialSlots(slots map[string]packsetup.CredentialSlot) map[string]map[string]interface{} {
	result := map[string]map[string]interface{}{}
	for key, slot := range slots {
		result[key] = map[string]interface{}{
			"credential_type": slot.CredentialType,
			"assigned":        slot.CredentialID != "",
		}
	}
	return result
}

func credentialRequirement(appliance *ApplianceContext, key string) (pack.CredentialRequirement, bool) {
	for _, req := range appliance.CredentialRequirements {
		if req.Key == key {
			return req, true
		}
	}
	return pack.CredentialRequirement{}, false
}

func applianceTelegramGetMe(r *http.Request, appliance *ApplianceContext, token string) error {
	base := strings.TrimRight(appliance.TelegramAPIBaseURL, "/")
	if base == "" {
		base = "https://api.telegram.org"
	}
	parsed, err := url.Parse(base)
	if err != nil || !parsed.IsAbs() || parsed.Host == "" || (parsed.Scheme != "http" && parsed.Scheme != "https") {
		return fmt.Errorf("telegram API base URL is invalid")
	}
	testURL := fmt.Sprintf("%s/bot%s/getMe", base, token)
	req, err := http.NewRequestWithContext(r.Context(), http.MethodGet, testURL, nil)
	if err != nil {
		return fmt.Errorf("telegram connection test could not be created")
	}
	client := appliance.ConnectionTestClient
	if client == nil {
		client = &http.Client{Timeout: 10 * time.Second}
	}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("telegram connection test failed: %s", redactConnectionTestText(err.Error()))
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(io.LimitReader(resp.Body, (64<<10)+1))
	if err != nil {
		return fmt.Errorf("telegram connection test response could not be read")
	}
	if len(data) > (64 << 10) {
		return fmt.Errorf("telegram connection test response exceeds limit")
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("telegram connection test returned status %d: %s", resp.StatusCode, redactConnectionTestText(string(data)))
	}
	var decoded struct {
		OK bool `json:"ok"`
	}
	if err := json.Unmarshal(data, &decoded); err != nil || !decoded.OK {
		return fmt.Errorf("telegram connection test did not return ok")
	}
	return nil
}

func redactConnectionTestText(text string) string {
	if len(text) > 4096 {
		text = text[:4096]
	}
	for _, pattern := range []*regexp.Regexp{
		regexp.MustCompile(`(?i)bot[0-9]+:[A-Za-z0-9_-]+`),
		regexp.MustCompile(`(?i)(token|authorization|password|secret)["'=:\s]+[^"',}\s]+`),
	} {
		text = pattern.ReplaceAllString(text, "[REDACTED]")
	}
	return text
}
