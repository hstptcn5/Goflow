package api

import (
	"crypto/subtle"
	"net"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
)

const applianceTokenHeader = "X-Goflow-Appliance-Token"

type ApplianceContext struct {
	Enabled      bool
	Origin       string
	SessionToken string
	PackID       string
	PackName     string
	PackVersion  string
	Description  string
	WorkflowID   string
}

func mountApplianceRoutes(r chi.Router, appliance *ApplianceContext) {
	if appliance == nil || !appliance.Enabled {
		return
	}
	r.Route("/api/appliance", func(r chi.Router) {
		r.Use(applianceHostMiddleware(appliance))
		r.Get("/bootstrap", applianceBootstrapHandler(appliance))
		r.Get("/status", applianceStatusHandler(appliance))
		r.Get("/diagnostics", applianceDiagnosticsHandler(appliance))
		r.Group(func(r chi.Router) {
			r.Use(applianceMutationMiddleware(appliance))
			r.Post("/setup/complete", applianceNotImplementedMutation)
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

func applianceStatusHandler(appliance *ApplianceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		renderJSON(w, http.StatusOK, map[string]interface{}{
			"pack":        applianceIdentity(appliance),
			"workflow_id": appliance.WorkflowID,
			"state":       "NEEDS_SETUP",
		})
	}
}

func applianceDiagnosticsHandler(appliance *ApplianceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		renderJSON(w, http.StatusOK, map[string]interface{}{
			"pack_id":      appliance.PackID,
			"pack_version": appliance.PackVersion,
			"workflow_id":  appliance.WorkflowID,
			"state":        "NEEDS_SETUP",
		})
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
			if r.Header.Get("Content-Type") != "application/json" {
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
