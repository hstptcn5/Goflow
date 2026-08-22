package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"

	"goflow/internal/appbuilder"
	"goflow/internal/pack"
	"goflow/internal/storage"

	"github.com/go-chi/chi/v5"
)

type AppBuilderHandler struct {
	wfStore *storage.WorkflowStore
}

type appBuildRequest struct {
	ID          string         `json:"id"`
	Name        string         `json:"name"`
	Version     string         `json:"version"`
	Description string         `json:"description"`
	RunUI       *pack.RunUI    `json:"run_ui"`
	Branding    *pack.Branding `json:"branding,omitempty"`
}

func NewAppBuilderHandler(store *storage.WorkflowStore) *AppBuilderHandler {
	return &AppBuilderHandler{wfStore: store}
}

func (h *AppBuilderHandler) Analyze(w http.ResponseWriter, r *http.Request) {
	wf, err := h.wfStore.GetByID(chi.URLParam(r, "id"))
	if err != nil {
		http.Error(w, "Workflow not found", http.StatusNotFound)
		return
	}
	report, err := appbuilder.Analyze(wf.NodesJSON)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	renderJSON(w, http.StatusOK, report)
}

func (h *AppBuilderHandler) Build(w http.ResponseWriter, r *http.Request) {
	wf, err := h.wfStore.GetByID(chi.URLParam(r, "id"))
	if err != nil {
		http.Error(w, "Workflow not found", http.StatusNotFound)
		return
	}
	var req appBuildRequest
	r.Body = http.MaxBytesReader(w, r.Body, 256<<10)
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(&req); err != nil {
		http.Error(w, "Invalid app build request", http.StatusBadRequest)
		return
	}
	report, err := appbuilder.Analyze(wf.NodesJSON)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if !report.CanBuild {
		renderJSON(w, http.StatusUnprocessableEntity, map[string]interface{}{"error": report.Summary, "portability": report})
		return
	}
	if strings.TrimSpace(req.Name) == "" {
		req.Name = wf.Name
	}
	if strings.TrimSpace(req.ID) == "" {
		req.ID = slugAppID(req.Name)
	}
	if strings.TrimSpace(req.Version) == "" {
		req.Version = "1.0.0"
	}
	if req.RunUI == nil {
		req.RunUI = &pack.RunUI{InputMode: "direct", OutputMode: "auto", SubmitLabel: "Chạy"}
	}

	tempDir, err := os.MkdirTemp("", "goflow-app-build-*")
	if err != nil {
		http.Error(w, "Could not prepare app build", http.StatusInternalServerError)
		return
	}
	defer os.RemoveAll(tempDir)
	packDir := filepath.Join(tempDir, "pack")
	if err := os.MkdirAll(filepath.Join(packDir, "workflows"), 0700); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	manifest := pack.Manifest{
		SchemaVersion: pack.SupportedSchema,
		ID:            req.ID, Name: req.Name, Version: req.Version, Description: req.Description,
		EntryWorkflow:        pack.DefaultWorkflowPath,
		RequiredCredentials:  []string{},
		SupportedPlatforms:   []string{pack.Platform(runtime.GOOS, runtime.GOARCH)},
		RequiredCapabilities: []string{pack.CapabilityPackV1, pack.CapabilityAppUIV1},
		RunUI:                req.RunUI, Branding: req.Branding,
	}
	appWorkflow := *wf
	appWorkflow.IsActive = true
	portableNodes, credentialRequirements, bindings, err := appbuilder.ExternalizeCredentials(appWorkflow.NodesJSON)
	if err != nil {
		http.Error(w, "Could not externalize app credentials", http.StatusUnprocessableEntity)
		return
	}
	appWorkflow.NodesJSON = portableNodes
	manifest.CredentialRequirements = credentialRequirements
	manifest.Bindings = bindings
	if len(bindings) > 0 {
		manifest.RequiredCapabilities = append(manifest.RequiredCapabilities, pack.CapabilitySetupBindingsV1)
	}
	manifestData, _ := json.MarshalIndent(manifest, "", "  ")
	workflowData, _ := json.MarshalIndent(&appWorkflow, "", "  ")
	if err := os.WriteFile(filepath.Join(packDir, pack.ManifestFile), append(manifestData, '\n'), 0600); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if err := os.WriteFile(filepath.Join(packDir, filepath.FromSlash(pack.DefaultWorkflowPath)), append(workflowData, '\n'), 0600); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	result, err := pack.BuildApp(pack.AppBuildOptions{PackDir: packDir, OutputDir: tempDir})
	if err != nil {
		http.Error(w, fmt.Sprintf("Build failed: %v", err), http.StatusUnprocessableEntity)
		return
	}
	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", result.AppName))
	http.ServeFile(w, r, result.AppPath)
}

var appIDInvalid = regexp.MustCompile(`[^a-z0-9.-]+`)

func slugAppID(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	value = appIDInvalid.ReplaceAllString(value, "-")
	value = strings.Trim(value, ".-")
	if value == "" {
		return "goflow-app"
	}
	return value
}
