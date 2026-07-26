package api

import (
	"encoding/json"
	"net/http"
	"strings"

	"goflow/internal/storage"

	"github.com/go-chi/chi/v5"
)

type TokenHandler struct {
	tokenStore *storage.AccessTokenStore
	auditStore *storage.AuditStore
}

func NewTokenHandler(tokenStore *storage.AccessTokenStore, auditStore *storage.AuditStore) *TokenHandler {
	return &TokenHandler{tokenStore: tokenStore, auditStore: auditStore}
}

type createTokenRequest struct {
	Name             string   `json:"name"`
	Scopes           []string `json:"scopes"`
	AllowedWorkflows []string `json:"allowed_workflows"`
}

type createTokenResponse struct {
	*storage.AccessToken
	Token string `json:"token"`
}

func (h *TokenHandler) ListTokens(w http.ResponseWriter, r *http.Request) {
	tokens, err := h.tokenStore.List()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if tokens == nil {
		tokens = []storage.AccessToken{}
	}
	renderJSON(w, http.StatusOK, tokens)
}

func (h *TokenHandler) CreateToken(w http.ResponseWriter, r *http.Request) {
	var req createTokenRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}
	if len(req.Scopes) == 0 {
		http.Error(w, "At least one scope is required", http.StatusBadRequest)
		return
	}
	token, rawToken, err := h.tokenStore.Create(req.Name, req.Scopes, req.AllowedWorkflows)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	h.recordTokenAudit(r, true, "created token "+token.ID)
	renderJSON(w, http.StatusCreated, createTokenResponse{AccessToken: token, Token: rawToken})
}

func (h *TokenHandler) DeleteToken(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimSpace(chi.URLParam(r, "id"))
	deleted, err := h.tokenStore.Delete(id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if !deleted {
		http.Error(w, "Token not found", http.StatusNotFound)
		return
	}
	h.recordTokenAudit(r, true, "deleted token "+id)
	renderJSON(w, http.StatusOK, map[string]string{"message": "Token deleted"})
}

func (h *TokenHandler) recordTokenAudit(r *http.Request, success bool, message string) {
	if h.auditStore == nil {
		return
	}
	subject := requestPrincipal(r)
	if auth, ok := AuthFromContext(r.Context()); ok && auth.Subject != "" {
		subject = auth.Subject
	}
	_ = h.auditStore.Record(storage.AuditEvent{
		EventType: "token_management",
		Subject:   subject,
		Scope:     "admin:tokens",
		Success:   success,
		Message:   message,
	})
}
