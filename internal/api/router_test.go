package api

import (
	"bytes"
	"encoding/json"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"

	"goflow/internal/crypto"
	"goflow/internal/pack"
	"goflow/internal/storage"
)

func TestRouterAllowsConfiguredMCPOriginInGlobalCORS(t *testing.T) {
	router := NewRouter(nil, nil, nil, nil, nil, nil, nil, nil, nil, "", 60, "http://127.0.0.1:8080", []string{"https://agent.example.com"}, 2, 30)
	req := httptest.NewRequest(http.MethodOptions, "/mcp", nil)
	req.Header.Set("Origin", "https://agent.example.com")
	req.Header.Set("Access-Control-Request-Method", "POST")
	req.Header.Set("Access-Control-Request-Headers", "Authorization, Content-Type")
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)
	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "https://agent.example.com" {
		t.Fatalf("expected custom MCP CORS origin, got %q; status=%d body=%s", got, rec.Code, rec.Body.String())
	}
}

func TestApplianceRoutesAreAbsentInGenericMode(t *testing.T) {
	router := NewRouter(nil, nil, nil, nil, nil, nil, nil, nil, nil, "", 60, "http://127.0.0.1:8080", nil, 2, 30)
	req := httptest.NewRequest(http.MethodGet, "/api/appliance/bootstrap", nil)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected generic appliance route 404, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestApplianceBootstrapAndMutationGuard(t *testing.T) {
	appliance := &ApplianceContext{
		Enabled:      true,
		Origin:       "http://example.com",
		SessionToken: "test-session-token",
		PackID:       "example.appliance",
		PackName:     "Example Appliance",
		PackVersion:  "0.1.0",
		Description:  "Example",
		WorkflowID:   "wf-1",
	}
	router := NewRouter(nil, nil, nil, nil, nil, nil, nil, nil, nil, "", 60, "http://127.0.0.1:8080", nil, 2, 30, appliance)

	req := httptest.NewRequest(http.MethodGet, "/api/appliance/bootstrap", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected bootstrap 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var bootstrap map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &bootstrap); err != nil {
		t.Fatalf("decode bootstrap: %v", err)
	}
	if bootstrap["token"] != "test-session-token" {
		t.Fatalf("unexpected bootstrap response: %#v", bootstrap)
	}

	mutation := func(origin, token, contentType, host string) int {
		req := httptest.NewRequest(http.MethodPost, "/api/appliance/setup/complete", bytes.NewBufferString(`{}`))
		req.Host = host
		if origin != "" {
			req.Header.Set("Origin", origin)
		}
		if token != "" {
			req.Header.Set(applianceTokenHeader, token)
		}
		if contentType != "" {
			req.Header.Set("Content-Type", contentType)
		}
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		return rec.Code
	}
	if got := mutation("http://example.com", "test-session-token", "application/json", "example.com"); got != http.StatusNotImplemented {
		t.Fatalf("expected guarded mutation to reach handler, got %d", got)
	}
	if got := mutation("http://evil.example", "test-session-token", "application/json", "example.com"); got != http.StatusForbidden {
		t.Fatalf("expected bad origin 403, got %d", got)
	}
	if got := mutation("http://example.com", "wrong", "application/json", "example.com"); got != http.StatusForbidden {
		t.Fatalf("expected bad token 403, got %d", got)
	}
	if got := mutation("http://example.com", "test-session-token", "text/plain", "example.com"); got != http.StatusUnsupportedMediaType {
		t.Fatalf("expected bad content type 415, got %d", got)
	}
	if got := mutation("http://example.com", "test-session-token", "application/json", "evil.example"); got != http.StatusNotFound {
		t.Fatalf("expected bad host 404, got %d", got)
	}
}

func TestRouterServesSPAIndexForFrontendRoutes(t *testing.T) {
	uiFS := fstest.MapFS{
		"index.html": &fstest.MapFile{Data: []byte(`<div id="app"></div>`)},
		"NODES.md":   &fstest.MapFile{Data: []byte(`# Nodes`)},
	}
	router := NewRouter(nil, nil, nil, nil, nil, nil, nil, nil, fs.FS(uiFS), "", 60, "http://127.0.0.1:8080", nil, 2, 30)

	req := httptest.NewRequest(http.MethodGet, "/workflows/wf-1", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected frontend route to serve index, got %d: %s", rec.Code, rec.Body.String())
	}
	if rec.Body.String() != `<div id="app"></div>` {
		t.Fatalf("expected index content, got %q", rec.Body.String())
	}

	req = httptest.NewRequest(http.MethodGet, "/NODES.md", nil)
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Body.String() != `# Nodes` {
		t.Fatalf("expected real static asset to be served, got %q", rec.Body.String())
	}
}

func TestApplianceSetupReadinessAndRedaction(t *testing.T) {
	db, err := storage.NewDB(filepath.Join(t.TempDir(), "goflow.db"))
	if err != nil {
		t.Fatalf("NewDB: %v", err)
	}
	defer db.Close()
	credStore := storage.NewCredentialStore(db, crypto.NewCryptoManager("test-master-key"))
	cred, err := credStore.Create("Telegram", "TELEGRAM_BOT", "123:secret-token")
	if err != nil {
		t.Fatalf("create credential: %v", err)
	}
	appliance := &ApplianceContext{
		Enabled:      true,
		Origin:       "http://example.com",
		SessionToken: "test-session-token",
		PackID:       "example.appliance",
		PackName:     "Example Appliance",
		PackVersion:  "0.1.0",
		Description:  "Example",
		WorkflowID:   "wf-1",
		DataDir:      t.TempDir(),
		ConfigSchema: []pack.ConfigField{
			{Key: "source_url", Label: "Source URL", Type: "url", Required: true},
		},
		CredentialRequirements: []pack.CredentialRequirement{
			{Key: "telegram", Label: "Telegram", Type: "TELEGRAM_BOT", Required: true},
		},
	}
	router := NewRouter(nil, nil, credStore, nil, nil, nil, nil, nil, nil, "", 60, "http://127.0.0.1:8080", nil, 2, 30, appliance)

	statusBody := applianceRequest(t, router, http.MethodGet, "/api/appliance/status", nil, nil)
	if !strings.Contains(statusBody, "NEEDS_SETUP") || !strings.Contains(statusBody, "config.source_url") || !strings.Contains(statusBody, "credential.telegram") {
		t.Fatalf("expected missing setup status, got %s", statusBody)
	}

	applianceRequest(t, router, http.MethodPost, "/api/appliance/setup/config", []byte(`{"values":{"source_url":"https://example.test/data.json"}}`), map[string]string{
		"Origin":             "http://example.com",
		applianceTokenHeader: "test-session-token",
		"Content-Type":       "application/json",
	})
	credentialBody := applianceRequest(t, router, http.MethodPost, "/api/appliance/setup/credentials", []byte(`{"slots":{"telegram":"`+cred.ID+`"}}`), map[string]string{
		"Origin":             "http://example.com",
		applianceTokenHeader: "test-session-token",
		"Content-Type":       "application/json",
	})
	if strings.Contains(credentialBody, cred.ID) || strings.Contains(credentialBody, "secret-token") {
		t.Fatalf("credential response leaked secret material: %s", credentialBody)
	}
	statusBody = applianceRequest(t, router, http.MethodGet, "/api/appliance/status", nil, nil)
	if !strings.Contains(statusBody, "READY") {
		t.Fatalf("expected ready status, got %s", statusBody)
	}
	setupBody := applianceRequest(t, router, http.MethodGet, "/api/appliance/setup", nil, nil)
	if !strings.Contains(setupBody, `"assigned":true`) || strings.Contains(setupBody, cred.ID) || strings.Contains(setupBody, "secret-token") {
		t.Fatalf("setup response was not redacted: %s", setupBody)
	}
}

func applianceRequest(t *testing.T, router http.Handler, method, path string, body []byte, headers map[string]string) string {
	t.Helper()
	reader := bytes.NewReader(body)
	req := httptest.NewRequest(method, path, reader)
	req.Host = "example.com"
	for key, value := range headers {
		req.Header.Set(key, value)
	}
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code < 200 || rec.Code >= 300 {
		t.Fatalf("%s %s returned %d: %s", method, path, rec.Code, rec.Body.String())
	}
	return rec.Body.String()
}
