package api

import (
	"bytes"
	"encoding/json"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"testing"
	"testing/fstest"
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
