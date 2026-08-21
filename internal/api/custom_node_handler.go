package api

import (
	"encoding/json"
	"net/http"
	"strings"

	"goflow/internal/nodes"
)

type CustomNodeHandler struct {
	registry *nodes.PluginRegistry
}

func NewCustomNodeHandler(registry *nodes.PluginRegistry) *CustomNodeHandler {
	return &CustomNodeHandler{registry: registry}
}

func (h *CustomNodeHandler) PromoteCode(w http.ResponseWriter, r *http.Request) {
	var manifest nodes.ReusableCodeManifest
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20))
	if err := decoder.Decode(&manifest); err != nil {
		http.Error(w, "Invalid reusable code manifest", http.StatusBadRequest)
		return
	}
	if manifest.SchemaVersion == 0 {
		manifest.SchemaVersion = 1
	}
	if strings.TrimSpace(manifest.Version) == "" {
		manifest.Version = "1.0.0"
	}
	executor, err := nodes.NewReusableCodeExecutor(manifest)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	path, err := nodes.SaveReusableCodeManifest(nodes.DefaultReusableCodeDir(), manifest)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if err := h.registry.RegisterOrReplaceCustom(executor); err != nil {
		http.Error(w, err.Error(), http.StatusConflict)
		return
	}
	renderJSON(w, http.StatusCreated, map[string]interface{}{
		"status":  "registered",
		"type":    manifest.Type,
		"version": manifest.Version,
		"path":    path,
	})
}
