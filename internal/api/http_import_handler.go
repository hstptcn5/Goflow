package api

import (
	"encoding/json"
	"net/http"
	"strings"

	"goflow/internal/nodes"
)

type HTTPImportHandler struct{}

func NewHTTPImportHandler() *HTTPImportHandler { return &HTTPImportHandler{} }

func (h *HTTPImportHandler) ImportCURL(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Command string `json:"command"`
	}
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 256<<10))
	if err := decoder.Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}
	if strings.TrimSpace(req.Command) == "" {
		http.Error(w, "cURL command is required", http.StatusBadRequest)
		return
	}
	result, err := nodes.ParseCURLCommand(req.Command)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	renderJSON(w, http.StatusOK, map[string]interface{}{
		"params":            result.Params,
		"credential_secret": result.CredentialSecret,
		"credential_hint":   result.CredentialHint,
		"secret_persisted":  false,
	})
}
