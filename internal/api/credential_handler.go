package api

import (
	"encoding/json"
	"net/http"
	"strings"

	"goflow/internal/storage"

	"github.com/go-chi/chi/v5"
)

type CredentialHandler struct {
	credStore *storage.CredentialStore
}

func NewCredentialHandler(cs *storage.CredentialStore) *CredentialHandler {
	return &CredentialHandler{credStore: cs}
}

func (h *CredentialHandler) ListCredentials(w http.ResponseWriter, r *http.Request) {
	list, err := h.credStore.ListAll()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if list == nil {
		list = []storage.Credential{}
	}
	renderJSON(w, http.StatusOK, list)
}

func (h *CredentialHandler) CreateCredential(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name     string `json:"name"`
		Type     string `json:"type"` // legacy request field
		Kind     string `json:"kind"`
		Provider string `json:"provider"`
		Data     string `json:"data"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if strings.TrimSpace(req.Name) == "" || strings.TrimSpace(req.Data) == "" {
		http.Error(w, "Name and Data are required", http.StatusBadRequest)
		return
	}

	var (
		cred *storage.Credential
		err  error
	)
	if strings.TrimSpace(req.Kind) != "" || strings.TrimSpace(req.Provider) != "" {
		cred, err = h.credStore.CreateWithMetadata(strings.TrimSpace(req.Name), req.Kind, req.Provider, req.Data)
	} else {
		cred, err = h.credStore.Create(strings.TrimSpace(req.Name), req.Type, req.Data)
	}
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Never return encrypted secret material to the client.
	cred.DataEncrypted = ""
	renderJSON(w, http.StatusCreated, cred)
}

func (h *CredentialHandler) DeleteCredential(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if err := h.credStore.Delete(id); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	renderJSON(w, http.StatusOK, map[string]string{"message": "Credential deleted"})
}
