package api

import (
	"crypto/subtle"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"strings"

	"goflow/internal/pack"
	"goflow/internal/packsetup"
	"goflow/internal/storage"

	"github.com/go-chi/chi/v5"
)

const applianceTokenHeader = "X-Goflow-Appliance-Token"
const applianceMaxJSONBody int64 = 64 << 10

type ApplianceContext struct {
	Enabled                bool
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
}

func mountApplianceRoutes(r chi.Router, appliance *ApplianceContext, credStore *storage.CredentialStore) {
	if appliance == nil || !appliance.Enabled {
		return
	}
	r.Route("/api/appliance", func(r chi.Router) {
		r.Use(applianceHostMiddleware(appliance))
		r.Get("/bootstrap", applianceBootstrapHandler(appliance))
		r.Get("/status", applianceStatusHandler(appliance))
		r.Get("/setup", applianceSetupHandler(appliance, credStore))
		r.Get("/diagnostics", applianceDiagnosticsHandler(appliance))
		r.Group(func(r chi.Router) {
			r.Use(applianceMutationMiddleware(appliance))
			r.Post("/setup/config", applianceSaveConfigHandler(appliance))
			r.Post("/setup/credentials", applianceSaveCredentialsHandler(appliance, credStore))
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
		state, missing := applianceReadiness(appliance, nil)
		renderJSON(w, http.StatusOK, map[string]interface{}{
			"pack":        applianceIdentity(appliance),
			"workflow_id": appliance.WorkflowID,
			"state":       state,
			"missing":     missing,
		})
	}
}

func applianceDiagnosticsHandler(appliance *ApplianceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		state, _ := applianceReadiness(appliance, nil)
		renderJSON(w, http.StatusOK, map[string]interface{}{
			"pack_id":      appliance.PackID,
			"pack_version": appliance.PackVersion,
			"workflow_id":  appliance.WorkflowID,
			"state":        state,
		})
	}
}

func applianceSetupHandler(appliance *ApplianceContext, credStore *storage.CredentialStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		resolver := applianceCredentialResolver(credStore)
		state, missing := applianceReadiness(appliance, resolver)
		configValues := map[string]interface{}{}
		if loaded, err := packsetup.LoadConfig(appliance.DataDir, applianceManifest(appliance)); err == nil {
			configValues = loaded.Config.Values
		}
		renderJSON(w, http.StatusOK, map[string]interface{}{
			"state":                     state,
			"missing":                   missing,
			"config_schema":             appliance.ConfigSchema,
			"credential_requirements":   redactedCredentialRequirements(appliance, credStore),
			"legacy_required_creds":     appliance.LegacyRequiredCreds,
			"current_config_values":     configValues,
			"credential_values_hidden":  true,
			"credential_ids_redacted":   true,
			"decrypted_values_returned": false,
		})
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
		cfg, err := packsetup.SaveConfig(appliance.DataDir, applianceManifest(appliance), req.Values)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
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
	}
}

func applianceReadiness(appliance *ApplianceContext, resolver packsetup.CredentialResolver) (string, []string) {
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
		if _, err := packsetup.LoadCredentialBindings(appliance.DataDir, manifest, resolver); err != nil {
			if errors.Is(err, os.ErrNotExist) {
				for _, req := range manifest.CredentialRequirements {
					if req.Required {
						missing = append(missing, "credential."+req.Key)
					}
				}
			} else {
				missing = append(missing, "credential")
			}
		}
	}
	if len(missing) > 0 {
		return "NEEDS_SETUP", missing
	}
	return "READY", missing
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
