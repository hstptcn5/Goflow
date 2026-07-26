package api

import (
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
