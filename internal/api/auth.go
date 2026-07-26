package api

import (
	"context"
	"encoding/base64"
	"net/http"
	"strings"

	"goflow/internal/storage"

	"github.com/go-chi/chi/v5"
)

type authContextKey struct{}

type AuthInfo struct {
	Subject string
	Admin   bool
	Token   *storage.AccessToken
	Scope   string
}

func authMiddleware(apiKey string, tokenStore *storage.AccessTokenStore, auditStore *storage.AuditStore, allowOAuthCallback bool) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if allowOAuthCallback && strings.HasPrefix(r.URL.Path, "/api/v1/oauth2/callback") {
				next.ServeHTTP(w, r)
				return
			}
			auth, ok := authenticateRequest(r, apiKey, tokenStore)
			if !ok {
				unauthorized(w)
				return
			}
			scope, adminOnly := requiredScope(r)
			auth.Scope = scope
			if !auth.Admin {
				if adminOnly || !auth.Token.HasScope(scope) || !auth.Token.AllowsWorkflow(workflowIDFromRequest(r)) {
					forbidden(w)
					recordAudit(auditStore, r, auth, false, "forbidden")
					return
				}
			}

			rec := &responseRecorder{ResponseWriter: w, statusCode: http.StatusOK}
			ctx := context.WithValue(r.Context(), authContextKey{}, auth)
			next.ServeHTTP(rec, r.WithContext(ctx))
			recordAudit(auditStore, r, auth, rec.statusCode < 400, http.StatusText(rec.statusCode))
		})
	}
}

func requireAPIKey(w http.ResponseWriter, r *http.Request, apiKey string) bool {
	if apiKey == "" || requestHasAPIKey(r, apiKey) {
		return true
	}
	unauthorized(w)
	return false
}

func requestHasAPIKey(r *http.Request, apiKey string) bool {
	if apiKey == "" {
		return false
	}
	authHeader := r.Header.Get("Authorization")
	if strings.HasPrefix(authHeader, "Bearer ") {
		return strings.TrimSpace(strings.TrimPrefix(authHeader, "Bearer ")) == apiKey
	}

	for _, protocol := range websocketProtocols(r) {
		if token, ok := strings.CutPrefix(protocol, "goflow."); ok {
			decoded, err := base64.RawURLEncoding.DecodeString(token)
			return err == nil && string(decoded) == apiKey
		}
	}
	return false
}

func authenticateRequest(r *http.Request, apiKey string, tokenStore *storage.AccessTokenStore) (AuthInfo, bool) {
	if apiKey == "" {
		return AuthInfo{Subject: "local", Admin: true}, true
	}
	authHeader := r.Header.Get("Authorization")
	if strings.HasPrefix(authHeader, "Bearer ") {
		rawToken := strings.TrimSpace(strings.TrimPrefix(authHeader, "Bearer "))
		if rawToken == apiKey {
			return AuthInfo{Subject: "api_key", Admin: true}, true
		}
		if tokenStore != nil {
			token, err := tokenStore.Authenticate(rawToken)
			if err == nil {
				return AuthInfo{Subject: "token:" + token.ID + ":" + token.Name, Token: token}, true
			}
		}
	}

	for _, protocol := range websocketProtocols(r) {
		if token, ok := strings.CutPrefix(protocol, "goflow."); ok {
			decoded, err := base64.RawURLEncoding.DecodeString(token)
			if err == nil && string(decoded) == apiKey {
				return AuthInfo{Subject: "api_key", Admin: true}, true
			}
		}
	}
	return AuthInfo{}, false
}

func AuthFromContext(ctx context.Context) (AuthInfo, bool) {
	auth, ok := ctx.Value(authContextKey{}).(AuthInfo)
	return auth, ok
}

func requiredScope(r *http.Request) (string, bool) {
	path := r.URL.Path
	method := r.Method
	switch {
	case strings.HasPrefix(path, "/api/v1/audit-events"):
		return "admin:audit", true
	case strings.HasPrefix(path, "/api/v1/tokens"):
		return "admin:tokens", true
	case strings.HasPrefix(path, "/api/v1/credentials"), strings.HasPrefix(path, "/api/v1/ai"), strings.HasPrefix(path, "/api/v1/oauth2"):
		return "admin:settings", true
	case path == "/api/v1/nodes/definitions":
		return "workflow:read", false
	case path == "/api/v1/workflows" && method == http.MethodGet:
		return "workflow:list", false
	case path == "/api/v1/workflows" && method == http.MethodPost:
		return "workflow:write", false
	case strings.HasPrefix(path, "/api/v1/executions/") && strings.HasSuffix(path, "/cancel"):
		return "execution:cancel", false
	case strings.Contains(path, "/executions") && method == http.MethodPost:
		return "workflow:run", false
	case strings.Contains(path, "/trigger") && method == http.MethodPost:
		return "workflow:run", false
	case strings.HasPrefix(path, "/api/v1/executions/") || strings.Contains(path, "/executions"):
		return "execution:read", false
	case strings.HasPrefix(path, "/api/v1/workflows/") && method == http.MethodGet:
		return "workflow:read", false
	case strings.HasPrefix(path, "/api/v1/workflows/"):
		return "workflow:write", false
	default:
		return "admin:api", true
	}
}

func workflowIDFromRequest(r *http.Request) string {
	if id := chi.URLParam(r, "id"); id != "" && strings.HasPrefix(r.URL.Path, "/api/v1/workflows/") {
		return id
	}
	if id := chi.URLParam(r, "workflowId"); id != "" {
		return id
	}
	parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
	if len(parts) >= 4 && parts[0] == "api" && parts[1] == "v1" && parts[2] == "workflows" {
		return parts[3]
	}
	if len(parts) >= 2 && parts[0] == "webhook" {
		return parts[1]
	}
	return ""
}

func executionIDFromRequest(r *http.Request) string {
	if id := chi.URLParam(r, "id"); id != "" && strings.HasPrefix(r.URL.Path, "/api/v1/executions/") {
		return id
	}
	return ""
}

func recordAudit(auditStore *storage.AuditStore, r *http.Request, auth AuthInfo, success bool, message string) {
	if auditStore == nil {
		return
	}
	_ = auditStore.Record(storage.AuditEvent{
		EventType:   r.Method + " " + r.URL.Path,
		Subject:     auth.Subject,
		Scope:       auth.Scope,
		WorkflowID:  workflowIDFromRequest(r),
		ExecutionID: executionIDFromRequest(r),
		Success:     success,
		Message:     message,
	})
}

type responseRecorder struct {
	http.ResponseWriter
	statusCode int
}

func (r *responseRecorder) WriteHeader(statusCode int) {
	r.statusCode = statusCode
	r.ResponseWriter.WriteHeader(statusCode)
}

func websocketProtocols(r *http.Request) []string {
	header := r.Header.Get("Sec-WebSocket-Protocol")
	if header == "" {
		return nil
	}
	parts := strings.Split(header, ",")
	protocols := make([]string, 0, len(parts))
	for _, part := range parts {
		if protocol := strings.TrimSpace(part); protocol != "" {
			protocols = append(protocols, protocol)
		}
	}
	return protocols
}

func unauthorized(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusUnauthorized)
	_, _ = w.Write([]byte(`{"error":"Unauthorized: invalid or missing API key"}`))
}

func forbidden(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusForbidden)
	_, _ = w.Write([]byte(`{"error":"Forbidden: token scope does not allow this action"}`))
}
