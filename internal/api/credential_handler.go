package api

import (
	"encoding/json"
	"net/http"

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
		Name string `json:"name"`
		Type string `json:"type"`
		Data string `json:"data"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if req.Name == "" || req.Data == "" {
		http.Error(w, "Name and Data are required", http.StatusBadRequest)
		return
	}

	cred, err := h.credStore.Create(req.Name, req.Type, req.Data)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Đảm bảo không trả về cipher data ở response
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
