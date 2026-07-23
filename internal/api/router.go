package api

import (
	"encoding/json"
	"io/fs"
	"net/http"
	"strings"

	"goflow/internal/engine"
	"goflow/internal/nodes"
	"goflow/internal/storage"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
)

func NewRouter(
	wfStore *storage.WorkflowStore,
	execStore *storage.ExecutionStore,
	credStore *storage.CredentialStore,
	registry *nodes.PluginRegistry,
	eng *engine.Engine,
	eventBus *engine.EventBus,
	uiFS fs.FS,
	apiKey string,
) *chi.Mux {
	r := chi.NewRouter()

	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(middleware.RequestSize(10 << 20))

	allowedOrigins := []string{
		"http://localhost:5173",
		"http://localhost:8080",
		"http://127.0.0.1:5173",
		"http://127.0.0.1:8080",
	}
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   allowedOrigins,
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type", "X-CSRF-Token"},
		ExposedHeaders:   []string{"Link"},
		AllowCredentials: true,
		MaxAge:           300,
	}))

	wfHandler := NewWorkflowHandler(wfStore, eng)
	execHandler := NewExecutionHandler(execStore)
	credHandler := NewCredentialHandler(credStore)
	nodeHandler := NewNodeHandler(registry)
	oauth2Handler := NewOAuth2Handler(credStore)
	wsHandler := NewWSHandler(eventBus, apiKey)
	aiHandler := NewAIHandler(credStore, registry)

	r.Route("/api/v1", func(r chi.Router) {
		r.Use(authMiddleware(apiKey, true))

		r.Get("/oauth2/authorize", oauth2Handler.Authorize)
		r.Get("/oauth2/callback", oauth2Handler.Callback)

		r.Get("/workflows", wfHandler.ListWorkflows)
		r.Post("/workflows", wfHandler.CreateWorkflow)
		r.Get("/workflows/{id}", wfHandler.GetWorkflow)
		r.Put("/workflows/{id}", wfHandler.UpdateWorkflow)
		r.Delete("/workflows/{id}", wfHandler.DeleteWorkflow)
		r.Put("/workflows/{id}/toggle", wfHandler.ToggleActive)
		r.Post("/workflows/{id}/trigger", wfHandler.TriggerWorkflow)

		r.Get("/executions/{id}", execHandler.GetExecution)
		r.Get("/workflows/{workflowId}/executions", execHandler.ListWorkflowExecutions)

		r.Get("/credentials", credHandler.ListCredentials)
		r.Post("/credentials", credHandler.CreateCredential)
		r.Delete("/credentials/{id}", credHandler.DeleteCredential)

		r.Get("/nodes/definitions", nodeHandler.ListDefinitions)

		r.Post("/ai/generate", aiHandler.GenerateWorkflow)
		r.Post("/ai/configure-node", aiHandler.ConfigureNode)
	})

	r.Post("/webhook/{workflowId}", wfHandler.TriggerWebhook)
	r.Get("/ws", wsHandler.ServeHTTP)

	if uiFS != nil {
		fileServer := http.FileServer(http.FS(uiFS))
		r.Get("/*", func(w http.ResponseWriter, r *http.Request) {
			if strings.HasPrefix(r.URL.Path, "/api") || strings.HasPrefix(r.URL.Path, "/ws") || strings.HasPrefix(r.URL.Path, "/webhook") {
				http.NotFound(w, r)
				return
			}
			w.Header().Set("Cache-Control", "no-store, no-cache, must-revalidate, max-age=0")
			w.Header().Set("Pragma", "no-cache")
			w.Header().Set("Expires", "0")
			fileServer.ServeHTTP(w, r)
		})
	}

	return r
}

func renderJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
