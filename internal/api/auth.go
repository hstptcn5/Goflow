package api

import (
	"encoding/base64"
	"net/http"
	"strings"
)

func authMiddleware(apiKey string, allowOAuthCallback bool) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		if apiKey == "" {
			return next
		}
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if allowOAuthCallback && strings.HasPrefix(r.URL.Path, "/api/v1/oauth2/callback") {
				next.ServeHTTP(w, r)
				return
			}
			if !requestHasAPIKey(r, apiKey) {
				unauthorized(w)
				return
			}
			next.ServeHTTP(w, r)
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
