package api

import (
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"goflow/internal/storage"

	"github.com/go-chi/chi/v5"
)

func TestAuthMiddlewareAllowsScopedWorkflowRun(t *testing.T) {
	db := newAuthTestDB(t)
	tokenStore := storage.NewAccessTokenStore(db)
	auditStore := storage.NewAuditStore(db)
	_, rawToken, err := tokenStore.Create("runner", []string{"workflow:run"}, []string{"wf-1"})
	if err != nil {
		t.Fatalf("create token: %v", err)
	}

	router := chi.NewRouter()
	router.Route("/api/v1", func(r chi.Router) {
		r.Use(authMiddleware("admin-key", tokenStore, auditStore, false))
		r.Post("/workflows/{id}/executions", func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusAccepted)
		})
	})

	req := httptest.NewRequest(http.MethodPost, "/api/v1/workflows/wf-1/executions", nil)
	req.Header.Set("Authorization", "Bearer "+rawToken)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusAccepted {
		t.Fatalf("expected 202, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestAuthMiddlewareRejectsMissingScope(t *testing.T) {
	db := newAuthTestDB(t)
	tokenStore := storage.NewAccessTokenStore(db)
	_, rawToken, err := tokenStore.Create("reader", []string{"workflow:list"}, []string{"wf-1"})
	if err != nil {
		t.Fatalf("create token: %v", err)
	}

	router := chi.NewRouter()
	router.Route("/api/v1", func(r chi.Router) {
		r.Use(authMiddleware("admin-key", tokenStore, nil, false))
		r.Post("/workflows/{id}/executions", func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusAccepted)
		})
	})

	req := httptest.NewRequest(http.MethodPost, "/api/v1/workflows/wf-1/executions", nil)
	req.Header.Set("Authorization", "Bearer "+rawToken)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestAuthMiddlewareRejectsWorkflowOutsideAllowlist(t *testing.T) {
	db := newAuthTestDB(t)
	tokenStore := storage.NewAccessTokenStore(db)
	_, rawToken, err := tokenStore.Create("runner", []string{"workflow:run"}, []string{"wf-1"})
	if err != nil {
		t.Fatalf("create token: %v", err)
	}

	router := chi.NewRouter()
	router.Route("/api/v1", func(r chi.Router) {
		r.Use(authMiddleware("admin-key", tokenStore, nil, false))
		r.Post("/workflows/{id}/executions", func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusAccepted)
		})
	})

	req := httptest.NewRequest(http.MethodPost, "/api/v1/workflows/wf-2/executions", nil)
	req.Header.Set("Authorization", "Bearer "+rawToken)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d: %s", rec.Code, rec.Body.String())
	}
}

func newAuthTestDB(t *testing.T) *storage.DB {
	t.Helper()
	db, err := storage.NewDB(filepath.Join(t.TempDir(), "goflow.db"))
	if err != nil {
		t.Fatalf("new db: %v", err)
	}
	t.Cleanup(db.Close)
	return db
}
