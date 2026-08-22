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
	"os"
	"reflect"
	"runtime"
	"strconv"
	"strings"
	"time"

	"goflow/internal/apperror"
	"goflow/internal/application"
	"goflow/internal/client"
	"goflow/internal/engine"
	"goflow/internal/pack"
	"goflow/internal/packsetup"
	"goflow/internal/scheduler"
	"goflow/internal/storage"

	"github.com/go-chi/chi/v5"
)

const applianceTokenHeader = "X-Goflow-Appliance-Token"
const applianceMaxJSONBody int64 = 64 << 10

type ApplianceContext struct {
	Enabled                bool
	AppVersion             string
	IntegrityState         string
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
	Bindings               []pack.Binding
	RunUI                  *pack.RunUI
	Branding               *pack.Branding
	TelegramAPIBaseURL     string
	ConnectionTestClient   *http.Client
	ScheduleStore          *storage.WorkflowScheduleStore
	ScheduleClock          applianceClock
}

type applianceClock interface {
	Now() time.Time
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
	sourceTestLimiter := newFixedWindowRateLimiter(10, time.Minute)
	sourceTestSlots := make(chan struct{}, 1)
	r.Route("/api/appliance", func(r chi.Router) {
		r.Use(applianceHostMiddleware(appliance))
		r.Get("/bootstrap", applianceBootstrapHandler(appliance))
		r.Get("/status", applianceStatusHandler(appliance, wfStore, execStore, credStore))
		r.Get("/setup", applianceSetupHandler(appliance, credStore))
		r.Get("/diagnostics", applianceDiagnosticsHandler(appliance, wfStore, execStore, credStore))
		r.Get("/workflow/status", applianceWorkflowStatusHandler(appliance, wfStore, execStore, credStore))
		r.Get("/executions/latest", applianceLatestExecutionHandler(appliance, execStore))
		r.Get("/executions", applianceRecentExecutionsHandler(appliance, execStore))
		r.Get("/schedule", applianceScheduleHandler(appliance, execStore))
		r.Group(func(r chi.Router) {
			r.Use(applianceMutationMiddleware(appliance))
			r.Post("/setup/config", applianceSaveConfigHandler(appliance))
			r.Post("/setup/source/test", applianceTestSourceHandler(appliance, wfStore, sourceTestLimiter, sourceTestSlots))
			r.Post("/setup/credentials", applianceSaveCredentialsHandler(appliance, credStore))
			r.Post("/setup/credentials/create", applianceCreateCredentialHandler(appliance, credStore))
			r.Post("/setup/credentials/test", applianceTestCredentialHandler(appliance, credStore, credentialTestLimiter, credentialTestSlots))
			r.Post("/setup/complete", applianceCompleteHandler(appliance, wfStore, credStore))
			r.Post("/setup/reopen", applianceReopenHandler(appliance))
			r.Post("/workflow/run", applianceRunWorkflowHandler(appliance, credStore, triggerService))
			r.Put("/schedule", applianceSaveScheduleHandler(appliance, execStore))
		})
	})
}

func applianceBootstrapHandler(appliance *ApplianceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		renderJSON(w, http.StatusOK, map[string]interface{}{
			"token": appliance.SessionToken,
			"pack":  applianceIdentity(appliance),
			"app": map[string]string{
				"name":     "Goflow",
				"version":  appliancePublicAppVersion(appliance),
				"platform": runtime.GOOS + "/" + runtime.GOARCH,
				"channel":  "UNSIGNED-PILOT-BETA",
			},
		})
	}
}

func appliancePublicAppVersion(appliance *ApplianceContext) string {
	version := strings.TrimSpace(appliance.AppVersion)
	if version == "" {
		return "development"
	}
	return version
}

func applianceStatusHandler(appliance *ApplianceContext, wfStore *storage.WorkflowStore, execStore *storage.ExecutionStore, credStore *storage.CredentialStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		state, missing, completed := applianceRuntimeState(appliance, wfStore, execStore, applianceCredentialResolver(credStore))
		response := map[string]interface{}{
			"pack":           applianceIdentity(appliance),
			"workflow_id":    appliance.WorkflowID,
			"server":         "ok",
			"state":          state,
			"missing":        missing,
			"setup_complete": completed,
			"can_complete":   len(missing) == 0,
		}
		if migration, err := packsetup.LoadMigrationState(appliance.DataDir, applianceManifest(appliance)); err == nil && !completed {
			response["attention_category"] = migration.Category
		}
		renderJSON(w, http.StatusOK, response)
	}
}

type applianceScheduleView struct {
	Configured       bool                       `json:"configured"`
	Revision         int64                      `json:"revision"`
	Enabled          bool                       `json:"enabled"`
	Kind             string                     `json:"kind"`
	LocalTime        string                     `json:"local_time"`
	Timezone         string                     `json:"timezone"`
	State            string                     `json:"state"`
	ErrorCategory    string                     `json:"error_category,omitempty"`
	LastScheduledFor *time.Time                 `json:"last_scheduled_for,omitempty"`
	NextRunAt        *time.Time                 `json:"next_run_at,omitempty"`
	LastResult       *applianceExecutionSummary `json:"last_result,omitempty"`
}

func applianceScheduleHandler(appliance *ApplianceContext, execStore *storage.ExecutionStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		view, status, err := applianceScheduleViewFor(appliance, execStore)
		if err != nil {
			http.Error(w, "schedule status is not available", status)
			return
		}
		renderJSON(w, http.StatusOK, view)
	}
}

func applianceSaveScheduleHandler(appliance *ApplianceContext, execStore *storage.ExecutionStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			ExpectedRevision int64  `json:"expected_revision"`
			Enabled          bool   `json:"enabled"`
			LocalTime        string `json:"local_time"`
			Timezone         string `json:"timezone"`
		}
		if err := decodeApplianceJSON(w, r, &req); err != nil {
			return
		}
		if appliance.ScheduleStore == nil {
			http.Error(w, "schedule service is not available", http.StatusServiceUnavailable)
			return
		}
		now := applianceScheduleNow(appliance)
		schedule := &storage.WorkflowSchedule{
			WorkflowID:      appliance.WorkflowID,
			PackID:          appliance.PackID,
			SchemaVersion:   storage.WorkflowScheduleSchemaVersion,
			Enabled:         req.Enabled,
			Kind:            storage.ScheduleKindDaily,
			LocalTime:       req.LocalTime,
			Timezone:        req.Timezone,
			MissedRunPolicy: storage.ScheduleMissedRunSkip,
			State:           storage.ScheduleStateDisabled,
		}
		existing, err := appliance.ScheduleStore.GetByWorkflow(appliance.WorkflowID)
		switch {
		case err == nil:
			if existing.PackID != appliance.PackID || req.ExpectedRevision != existing.Revision {
				writeScheduleConflict(w)
				return
			}
			schedule.LastScheduledFor = existing.LastScheduledFor
			schedule.LastExecutionID = existing.LastExecutionID
		case errors.Is(err, storage.ErrWorkflowScheduleNotFound):
			if req.ExpectedRevision != 0 {
				writeScheduleConflict(w)
				return
			}
		case errors.Is(err, storage.ErrInvalidWorkflowSchedule):
			renderJSON(w, http.StatusConflict, map[string]interface{}{
				"category": storage.ScheduleErrorInvalid,
				"error":    "The saved schedule needs attention before it can be changed.",
			})
			return
		default:
			http.Error(w, "schedule could not be loaded", http.StatusInternalServerError)
			return
		}
		if req.Enabled {
			schedule.State = storage.ScheduleStateOK
			after := now
			if schedule.LastScheduledFor != nil && schedule.LastScheduledFor.After(after) {
				after = *schedule.LastScheduledFor
			}
			next, err := scheduler.NextDailyAfter(schedule.LocalTime, schedule.Timezone, after)
			if err != nil {
				renderJSON(w, http.StatusBadRequest, map[string]interface{}{
					"category": storage.ScheduleErrorInvalid,
					"error":    "Use a valid daily time and IANA timezone.",
				})
				return
			}
			schedule.NextRunAt = &next
		}
		if err := appliance.ScheduleStore.Configure(schedule, req.ExpectedRevision, now); err != nil {
			if errors.Is(err, storage.ErrWorkflowScheduleConflict) {
				writeScheduleConflict(w)
				return
			}
			if errors.Is(err, storage.ErrInvalidWorkflowSchedule) {
				renderJSON(w, http.StatusBadRequest, map[string]interface{}{
					"category": storage.ScheduleErrorInvalid,
					"error":    "Use a valid daily time and IANA timezone.",
				})
				return
			}
			http.Error(w, "schedule could not be saved", http.StatusInternalServerError)
			return
		}
		view, status, err := applianceScheduleViewFor(appliance, execStore)
		if err != nil {
			http.Error(w, "schedule was saved but status is not available", status)
			return
		}
		renderJSON(w, http.StatusOK, view)
	}
}

func writeScheduleConflict(w http.ResponseWriter) {
	renderJSON(w, http.StatusConflict, map[string]interface{}{
		"category": "revision_conflict",
		"error":    "The schedule changed in another session. Refresh and try again.",
	})
}

func applianceScheduleViewFor(appliance *ApplianceContext, execStore *storage.ExecutionStore) (applianceScheduleView, int, error) {
	view := applianceScheduleView{
		Revision:  0,
		Enabled:   false,
		Kind:      storage.ScheduleKindDaily,
		LocalTime: "09:00",
		Timezone:  "UTC",
		State:     storage.ScheduleStateDisabled,
	}
	if appliance.ScheduleStore == nil {
		return view, http.StatusServiceUnavailable, errors.New("schedule store unavailable")
	}
	schedule, err := appliance.ScheduleStore.GetByWorkflow(appliance.WorkflowID)
	if errors.Is(err, storage.ErrWorkflowScheduleNotFound) {
		return view, http.StatusOK, nil
	}
	if errors.Is(err, storage.ErrInvalidWorkflowSchedule) {
		view.Configured = true
		view.State = storage.ScheduleStateNeedsAttention
		view.ErrorCategory = storage.ScheduleErrorInvalid
		return view, http.StatusOK, nil
	}
	if err != nil {
		return view, http.StatusInternalServerError, err
	}
	view.Configured = true
	view.Revision = schedule.Revision
	view.Enabled = schedule.Enabled
	view.Kind = schedule.Kind
	view.LocalTime = schedule.LocalTime
	view.Timezone = schedule.Timezone
	view.State = schedule.State
	view.ErrorCategory = schedule.ErrorCategory
	view.LastScheduledFor = schedule.LastScheduledFor
	view.NextRunAt = schedule.NextRunAt
	if schedule.LastExecutionID != "" && execStore != nil {
		if execution, err := execStore.GetByID(schedule.LastExecutionID); err == nil && execution.WorkflowID == appliance.WorkflowID {
			summary := applianceExecutionSummaryFromExecution(*execution)
			view.LastResult = &summary
		}
	}
	return view, http.StatusOK, nil
}

func applianceScheduleNow(appliance *ApplianceContext) time.Time {
	if appliance.ScheduleClock != nil {
		return appliance.ScheduleClock.Now().UTC()
	}
	return time.Now().UTC()
}

func applianceDiagnosticsHandler(appliance *ApplianceContext, wfStore *storage.WorkflowStore, execStore *storage.ExecutionStore, credStore *storage.CredentialStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		renderJSON(w, http.StatusOK, buildApplianceDiagnostics(appliance, wfStore, execStore, credStore))
	}
}

type applianceDiagnostics struct {
	SchemaVersion int                             `json:"schema_version"`
	App           applianceDiagnosticsApp         `json:"app"`
	Pack          applianceDiagnosticsPack        `json:"pack"`
	Setup         applianceDiagnosticsSetup       `json:"setup"`
	Schedule      applianceDiagnosticsSchedule    `json:"schedule"`
	Executions    []applianceDiagnosticsExecution `json:"recent_executions"`
	Integrity     applianceDiagnosticsIntegrity   `json:"integrity"`
	Privacy       applianceDiagnosticsPrivacy     `json:"privacy"`
}

type applianceDiagnosticsApp struct {
	Name     string `json:"name"`
	Version  string `json:"version"`
	Platform string `json:"platform"`
}

type applianceDiagnosticsPack struct {
	ID      string `json:"id"`
	Version string `json:"version"`
}

type applianceDiagnosticsSetup struct {
	State    string `json:"state"`
	Category string `json:"category"`
}

type applianceDiagnosticsSchedule struct {
	Configured    bool   `json:"configured"`
	Enabled       bool   `json:"enabled"`
	State         string `json:"state"`
	ErrorCategory string `json:"error_category,omitempty"`
}

type applianceDiagnosticsExecution struct {
	Status        string `json:"status"`
	DurationMs    int64  `json:"duration_ms"`
	ErrorCategory string `json:"error_category,omitempty"`
}

type applianceDiagnosticsIntegrity struct {
	State string `json:"state"`
}

type applianceDiagnosticsPrivacy struct {
	LocalOnly           bool `json:"local_only"`
	CredentialIDsHidden bool `json:"credential_ids_hidden"`
	SecretsHidden       bool `json:"secrets_hidden"`
}

func buildApplianceDiagnostics(appliance *ApplianceContext, wfStore *storage.WorkflowStore, execStore *storage.ExecutionStore, credStore *storage.CredentialStore) applianceDiagnostics {
	state, _, completed := applianceReadiness(appliance, applianceCredentialResolver(credStore))
	setupCategory := apperror.CategorySetupIncomplete
	if state == "READY" {
		setupCategory = "ready"
	} else if migration, err := packsetup.LoadMigrationState(appliance.DataDir, applianceManifest(appliance)); err == nil && !completed {
		if category, _, ok := apperror.Public(migration.Category); ok {
			setupCategory = category
		}
	}
	schedule := applianceDiagnosticsSchedule{State: storage.ScheduleStateDisabled}
	if view, _, err := applianceScheduleViewFor(appliance, execStore); err == nil {
		schedule.Configured = view.Configured
		schedule.Enabled = view.Enabled
		schedule.State = view.State
		if category, _, ok := apperror.Public(view.ErrorCategory); ok {
			schedule.ErrorCategory = category
		}
	} else {
		schedule.State = storage.ScheduleStateNeedsAttention
		schedule.ErrorCategory = apperror.CategoryInternal
	}
	executions := []applianceDiagnosticsExecution{}
	if list, err := applianceRecentExecutions(execStore, appliance.WorkflowID, 10); err == nil {
		for _, execution := range list {
			executions = append(executions, applianceDiagnosticsExecution{
				Status: execution.Status, DurationMs: execution.DurationMs, ErrorCategory: execution.ErrorCategory,
			})
		}
	}
	integrity := appliance.IntegrityState
	if integrity != "verified" && integrity != "source_validated" && integrity != "embedded_verified" {
		integrity = "unknown"
	}
	return applianceDiagnostics{
		SchemaVersion: 1,
		App:           applianceDiagnosticsApp{Name: "Goflow", Version: appliancePublicAppVersion(appliance), Platform: runtime.GOOS + "/" + runtime.GOARCH},
		Pack:          applianceDiagnosticsPack{ID: appliance.PackID, Version: appliance.PackVersion},
		Setup:         applianceDiagnosticsSetup{State: state, Category: setupCategory},
		Schedule:      schedule,
		Executions:    executions,
		Integrity:     applianceDiagnosticsIntegrity{State: integrity},
		Privacy:       applianceDiagnosticsPrivacy{LocalOnly: true, CredentialIDsHidden: true, SecretsHidden: true},
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
		response := map[string]interface{}{
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
		}
		if migration, err := packsetup.LoadMigrationState(appliance.DataDir, applianceManifest(appliance)); err == nil && !completed {
			response["attention_category"] = migration.Category
		}
		renderJSON(w, http.StatusOK, response)
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
		var previous map[string]interface{}
		if loaded, err := packsetup.LoadConfig(appliance.DataDir, applianceManifest(appliance)); err == nil {
			previous = loaded.Config.Values
		}
		cfg, err := packsetup.SaveConfig(appliance.DataDir, applianceManifest(appliance), req.Values)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if previous != nil && !reflect.DeepEqual(previous, cfg.Values) {
			if state, err := packsetup.LoadState(appliance.DataDir, applianceManifest(appliance)); err == nil && state.Completed {
				if _, err := packsetup.SaveState(appliance.DataDir, applianceManifest(appliance), false, time.Now()); err != nil {
					http.Error(w, "configuration was saved but setup could not be reopened", http.StatusInternalServerError)
					return
				}
			}
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
		status := http.StatusCreated
		credentialID := ""
		if loaded, err := packsetup.LoadCredentialBindings(appliance.DataDir, applianceManifest(appliance), applianceCredentialResolver(credStore)); err == nil {
			if slot, assigned := loaded.Credentials.Slots[req.Key]; assigned {
				if err := credStore.UpdateData(slot.CredentialID, req.Value); err != nil {
					http.Error(w, "credential could not be saved", http.StatusInternalServerError)
					return
				}
				credentialID = slot.CredentialID
				status = http.StatusOK
			}
		}
		if credentialID == "" {
			cred, err := credStore.Create(req.Name, requirement.Type, req.Value)
			if err != nil {
				http.Error(w, "credential could not be saved", http.StatusInternalServerError)
				return
			}
			credentialID = cred.ID
		}
		creds, err := packsetup.SaveCredentialBindings(appliance.DataDir, applianceManifest(appliance), map[string]string{req.Key: credentialID}, applianceCredentialResolver(credStore))
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		renderJSON(w, status, map[string]interface{}{"credentials": redactedCredentialSlots(creds.Slots)})
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
		if err := applianceValidateCredentialDestination(r.Context(), appliance, credStore, req.Key); err != nil {
			writeApplianceValidationError(w, http.StatusBadGateway, err)
			return
		}
		renderJSON(w, http.StatusOK, applianceValidationResult{
			Status:  "VALID",
			Message: "Bot token is valid and the configured chat is accessible.",
			Summary: map[string]interface{}{"bot": "valid", "chat": "accessible"},
		})
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
				latest, _ := applianceLatestExecution(execStore, appliance.WorkflowID, applianceOutputNodeID(appliance))
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
			if errors.Is(err, engine.ErrConcurrencyLimit) || errors.Is(err, engine.ErrWorkflowConcurrencyLimit) {
				renderJSON(w, http.StatusConflict, map[string]interface{}{
					"category": "already_running",
					"error":    "This workflow is already running. The dashboard will keep tracking the active execution.",
				})
				return
			}
			writeExecutionError(w, err)
			return
		}
		renderJSON(w, http.StatusAccepted, executionAcceptedResponse(result))
	}
}

func applianceLatestExecutionHandler(appliance *ApplianceContext, execStore *storage.ExecutionStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		latest, err := applianceLatestExecution(execStore, appliance.WorkflowID, applianceOutputNodeID(appliance))
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
		executions, err := applianceRecentExecutions(execStore, appliance.WorkflowID, limit, applianceOutputNodeID(appliance))
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		renderJSON(w, http.StatusOK, map[string]interface{}{"executions": executions, "limit": limit})
	}
}

func applianceCompleteHandler(appliance *ApplianceContext, wfStore *storage.WorkflowStore, credStore *storage.CredentialStore) http.HandlerFunc {
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
		if err := applianceValidateCompletion(r.Context(), appliance, wfStore, credStore); err != nil {
			writeApplianceValidationError(w, http.StatusBadRequest, err)
			return
		}
		prepared, err := appliancePrepareCompletion(appliance, wfStore, credStore)
		if err != nil {
			http.Error(w, "setup could not be completed", http.StatusBadRequest)
			return
		}
		state, err := appliancePersistCompletion(prepared, applianceDefaultCompletionStore(appliance, wfStore))
		if err != nil {
			http.Error(w, "managed workflow could not be activated", http.StatusInternalServerError)
			return
		}
		renderJSON(w, http.StatusOK, map[string]interface{}{"state": "READY", "setup_complete": state.Completed})
	}
}

type appliancePreparedCompletion struct {
	appliance *ApplianceContext
	manifest  pack.Manifest
	original  *storage.Workflow
	updated   *storage.Workflow
}

type applianceCompletionStore struct {
	saveState      func(completed bool, now time.Time) (*packsetup.StateFile, error)
	updateWorkflow func(*storage.Workflow) error
}

func appliancePrepareCompletion(appliance *ApplianceContext, wfStore *storage.WorkflowStore, credStore *storage.CredentialStore) (*appliancePreparedCompletion, error) {
	prepared := &appliancePreparedCompletion{
		appliance: appliance,
		manifest:  applianceManifest(appliance),
	}
	if wfStore == nil {
		return prepared, nil
	}
	wf, err := wfStore.GetByID(appliance.WorkflowID)
	if err != nil {
		return nil, err
	}
	updated := *wf
	manifest := prepared.manifest
	bound, err := packsetup.PrepareBoundWorkflow(workflowClientValue(&updated), manifest, appliance.DataDir, applianceCredentialResolver(credStore))
	if err != nil {
		return nil, err
	}
	updated.IsActive = bound.IsActive
	updated.NodesJSON = bound.NodesJSON
	updated.EdgesJSON = bound.EdgesJSON
	prepared.original = wf
	prepared.updated = &updated
	return prepared, nil
}

func workflowClientValue(workflow *storage.Workflow) client.Workflow {
	return client.Workflow{
		ID:                workflow.ID,
		Name:              workflow.Name,
		Description:       workflow.Description,
		IsActive:          workflow.IsActive,
		NodesJSON:         workflow.NodesJSON,
		EdgesJSON:         workflow.EdgesJSON,
		Slug:              workflow.Slug,
		InputSchemaJSON:   workflow.InputSchemaJSON,
		OutputSchemaJSON:  workflow.OutputSchemaJSON,
		ExposeCLI:         workflow.ExposeCLI,
		ExposeMCP:         workflow.ExposeMCP,
		MCPToolName:       workflow.MCPToolName,
		MCPDescription:    workflow.MCPDescription,
		RiskLevel:         workflow.RiskLevel,
		RequiresApproval:  workflow.RequiresApproval,
		MaxConcurrentRuns: workflow.MaxConcurrentRuns,
		ConcurrencyPolicy: workflow.ConcurrencyPolicy,
	}
}

func applianceDefaultCompletionStore(appliance *ApplianceContext, wfStore *storage.WorkflowStore) applianceCompletionStore {
	return applianceCompletionStore{
		saveState: func(completed bool, now time.Time) (*packsetup.StateFile, error) {
			return packsetup.SaveState(appliance.DataDir, applianceManifest(appliance), completed, now)
		},
		updateWorkflow: func(wf *storage.Workflow) error {
			if wfStore == nil {
				return nil
			}
			return wfStore.Update(wf)
		},
	}
}

func appliancePersistCompletion(prepared *appliancePreparedCompletion, store applianceCompletionStore) (*packsetup.StateFile, error) {
	// Safe order: all setup inputs and the fully bound workflow are prepared before
	// mutation. The setup state is persisted first, then the active workflow. If
	// workflow persistence fails, Goflow compensates by restoring the original
	// workflow snapshot and rolling setup state back to incomplete before
	// returning a generic failure to the HTTP caller.
	state, err := store.saveState(true, time.Now())
	if err != nil {
		return nil, err
	}
	if prepared.updated == nil {
		return state, nil
	}
	if err := store.updateWorkflow(prepared.updated); err != nil {
		restoreErr := store.updateWorkflow(prepared.original)
		_, rollbackErr := store.saveState(false, time.Now())
		if restoreErr != nil {
			return nil, restoreErr
		}
		if rollbackErr != nil {
			return nil, rollbackErr
		}
		return nil, err
	}
	return state, nil
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

func applianceIdentity(appliance *ApplianceContext) map[string]interface{} {
	return map[string]interface{}{
		"id":          appliance.PackID,
		"name":        appliance.PackName,
		"version":     appliance.PackVersion,
		"description": appliance.Description,
		"run_ui":      appliance.RunUI,
		"branding":    appliance.Branding,
	}
}

func applianceOutputNodeID(appliance *ApplianceContext) string {
	if appliance == nil || appliance.RunUI == nil {
		return ""
	}
	return appliance.RunUI.OutputNodeID
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
		Bindings:               appliance.Bindings,
		RunUI:                  appliance.RunUI,
		Branding:               appliance.Branding,
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

// ApplianceScheduleReadiness exposes only the bounded readiness category needed
// by the managed scheduler. It never returns setup values or storage errors.
func ApplianceScheduleReadiness(appliance *ApplianceContext, credStore *storage.CredentialStore) (bool, string) {
	if appliance == nil || !appliance.Enabled {
		return false, "setup_incomplete"
	}
	if len(applianceMissingRequirements(appliance, applianceCredentialResolver(credStore))) > 0 {
		return false, "setup_incomplete"
	}
	state, err := packsetup.LoadState(appliance.DataDir, applianceManifest(appliance))
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return false, "setup_incomplete"
		}
		return false, "revalidation_required"
	}
	if !state.Completed {
		return false, "setup_incomplete"
	}
	return true, ""
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
		if loaded, err := packsetup.LoadCredentialBindings(appliance.DataDir, manifest, resolver); err != nil {
			if errors.Is(err, os.ErrNotExist) {
				for _, req := range manifest.CredentialRequirements {
					if req.Required {
						missing = append(missing, "credential."+req.Key)
					}
				}
			} else {
				missing = append(missing, "credential")
			}
		} else {
			for _, req := range manifest.CredentialRequirements {
				if !req.Required {
					continue
				}
				if _, ok := loaded.Credentials.Slots[req.Key]; !ok {
					missing = append(missing, "credential."+req.Key)
				}
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
	Output           interface{} `json:"output,omitempty"`
	ErrorMessage     string      `json:"error_message,omitempty"`
	ErrorCategory    string      `json:"error_category,omitempty"`
}

func applianceLatestExecution(execStore *storage.ExecutionStore, workflowID string, outputNodeID ...string) (*applianceExecutionSummary, error) {
	executions, err := applianceRecentExecutions(execStore, workflowID, 1, outputNodeID...)
	if err != nil || len(executions) == 0 {
		return nil, err
	}
	return &executions[0], nil
}

func applianceRecentExecutions(execStore *storage.ExecutionStore, workflowID string, limit int, outputNodeID ...string) ([]applianceExecutionSummary, error) {
	if execStore == nil {
		return []applianceExecutionSummary{}, nil
	}
	list, err := execStore.ListByWorkflow(workflowID, limit)
	if err != nil {
		return nil, err
	}
	result := make([]applianceExecutionSummary, 0, len(list))
	for _, exec := range list {
		result = append(result, applianceExecutionSummaryFromExecution(exec, outputNodeID...))
	}
	return result, nil
}

func applianceExecutionSummaryFromExecution(exec storage.Execution, outputNodeID ...string) applianceExecutionSummary {
	dto := executionInspectorDTOFromExecution(exec)
	errorCategory, errorMessage := appliancePublicExecutionError(dto.Status, dto.ErrorMessage)
	return applianceExecutionSummary{
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
		Output:           applianceOutput(dto.NodeLogs, firstString(outputNodeID)),
		ErrorMessage:     errorMessage,
		ErrorCategory:    errorCategory,
	}
}

func firstString(values []string) string {
	if len(values) == 0 {
		return ""
	}
	return values[0]
}

func applianceOutput(logs []interface{}, outputNodeID string) interface{} {
	var last interface{}
	for _, raw := range logs {
		log, ok := raw.(map[string]interface{})
		if !ok || !strings.EqualFold(fmt.Sprint(log["status"]), "SUCCESS") {
			continue
		}
		if output, exists := log["output"]; exists {
			last = output
			if outputNodeID != "" && fmt.Sprint(log["node_id"]) == outputNodeID {
				return output
			}
		}
	}
	if outputNodeID != "" {
		return nil
	}
	return last
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
