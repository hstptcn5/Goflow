package api

import (
	"encoding/json"
	"io/fs"
	"net/http"
	"os"
	"strings"

	"goflow/internal/application"
	"goflow/internal/engine"
	"goflow/internal/mcpserver"
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
	tokenStore *storage.AccessTokenStore,
	auditStore *storage.AuditStore,
	registry *nodes.PluginRegistry,
	eng *engine.Engine,
	eventBus *engine.EventBus,
	uiFS fs.FS,
	apiKey string,
	webhookRateLimitPerMinute int,
	mcpBaseURL string,
	mcpAllowedOrigins []string,
	mcpMaxInflight int,
	mcpRateLimitPerMinute int,
	applianceOptions ...*ApplianceContext,
) *chi.Mux {
	r := chi.NewRouter()
	var appliance *ApplianceContext
	if len(applianceOptions) > 0 {
		appliance = applianceOptions[0]
	}

	if os.Getenv("GOFLOW_HTTP_LOGS") == "1" {
		r.Use(middleware.Logger)
	}
	r.Use(middleware.Recoverer)
	r.Use(middleware.RequestSize(10 << 20))

	allowedOrigins := []string{
		"http://localhost:5173",
		"http://localhost:8080",
		"http://127.0.0.1:5173",
		"http://127.0.0.1:8080",
	}
	allowedOrigins = append(allowedOrigins, mcpAllowedOrigins...)
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   allowedOrigins,
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type", "X-CSRF-Token", "X-Goflow-Trigger-Source", "MCP-Protocol-Version", "Mcp-Session-Id", "Last-Event-ID"},
		ExposedHeaders:   []string{"Link", "Mcp-Session-Id"},
		AllowCredentials: true,
		MaxAge:           300,
	}))

	r.Get("/healthz", func(w http.ResponseWriter, r *http.Request) {
		renderJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})

	triggerService := application.NewTriggerService(wfStore, eng)
	wfHandler := NewWorkflowHandler(wfStore, triggerService, webhookRateLimitPerMinute)
	appBuilderHandler := NewAppBuilderHandler(wfStore)
	execHandler := NewExecutionHandler(execStore, eng, triggerService)
	credHandler := NewCredentialHandler(credStore)
	tokenHandler := NewTokenHandler(tokenStore, auditStore)
	auditHandler := NewAuditHandler(auditStore)
	nodeHandler := NewNodeHandler(registry)
	oauth2Handler := NewOAuth2Handler(credStore)
	wsHandler := NewWSHandler(eventBus, apiKey)
	aiHandler := NewAIHandler(credStore, registry)
	agentHandler := NewAIAgentHandler(credStore, registry, wfStore, eng)
	httpImportHandler := NewHTTPImportHandler()
	customNodeHandler := NewCustomNodeHandler(registry)

	r.Route("/api/v1", func(r chi.Router) {
		r.Use(authMiddleware(apiKey, tokenStore, auditStore, true))

		r.Get("/oauth2/authorize", oauth2Handler.Authorize)
		r.Get("/oauth2/callback", oauth2Handler.Callback)

		r.Get("/workflows", wfHandler.ListWorkflows)
		r.Post("/workflows", wfHandler.CreateWorkflow)
		r.Get("/workflows/{id}", wfHandler.GetWorkflow)
		r.Put("/workflows/{id}", wfHandler.UpdateWorkflow)
		r.Get("/workflows/{id}/interface", wfHandler.GetWorkflowInterface)
		r.Put("/workflows/{id}/interface", wfHandler.UpdateWorkflowInterface)
		r.Delete("/workflows/{id}", wfHandler.DeleteWorkflow)
		r.Put("/workflows/{id}/toggle", wfHandler.ToggleActive)
		r.Post("/workflows/{id}/trigger", wfHandler.TriggerWorkflow)
		r.Post("/workflows/{id}/executions", wfHandler.CreateExecution)
		r.Post("/workflows/{id}/app/analyze", appBuilderHandler.Analyze)
		r.Post("/workflows/{id}/app/build", appBuilderHandler.Build)

		r.Get("/executions/{id}", execHandler.GetExecution)
		r.Post("/executions/{id}/cancel", execHandler.CancelExecution)
		r.Post("/executions/{id}/replay", execHandler.ReplayExecution)
		r.Get("/workflows/{workflowId}/executions", execHandler.ListWorkflowExecutions)

		r.Get("/credentials", credHandler.ListCredentials)
		r.Post("/credentials", credHandler.CreateCredential)
		r.Delete("/credentials/{id}", credHandler.DeleteCredential)

		r.Get("/tokens", tokenHandler.ListTokens)
		r.Post("/tokens", tokenHandler.CreateToken)
		r.Delete("/tokens/{id}", tokenHandler.DeleteToken)

		r.Get("/audit-events", auditHandler.ListAuditEvents)

		r.Get("/nodes/definitions", nodeHandler.ListDefinitions)
		r.Post("/http/import-curl", httpImportHandler.ImportCURL)
		r.Post("/custom-nodes/promote", customNodeHandler.PromoteCode)

		r.Post("/ai/generate", aiHandler.GenerateWorkflow)
		r.Post("/ai/configure-node", aiHandler.ConfigureNode)
		r.Post("/ai/code", aiHandler.AssistCode)
		r.Post("/ai/review", aiHandler.ReviewWorkflow)
		r.Post("/ai/agent/iterate", agentHandler.Iterate)
	})

	r.Post("/webhook/{workflowId}", wfHandler.TriggerWebhook)
	r.Get("/ws", wsHandler.ServeHTTP)
	mountApplianceRoutes(r, appliance, wfStore, execStore, credStore, triggerService)
	r.With(authMiddleware(apiKey, tokenStore, auditStore, false)).Mount("/mcp", mcpserver.NewHTTPHandler(mcpserver.HTTPOptions{
		BaseURL:            mcpBaseURL,
		MaxInflight:        mcpMaxInflight,
		RateLimitPerMinute: mcpRateLimitPerMinute,
		AllowedOrigins:     mcpAllowedOrigins,
	}))

	if uiFS != nil {
		fileServer := http.FileServer(http.FS(uiFS))
		r.Get("/*", func(w http.ResponseWriter, r *http.Request) {
			if strings.HasPrefix(r.URL.Path, "/api") || strings.HasPrefix(r.URL.Path, "/ws") || strings.HasPrefix(r.URL.Path, "/webhook") || strings.HasPrefix(r.URL.Path, "/mcp") {
				http.NotFound(w, r)
				return
			}
			if path := strings.TrimPrefix(r.URL.Path, "/"); path != "" {
				if file, err := uiFS.Open(path); err == nil {
					stat, statErr := file.Stat()
					_ = file.Close()
					if statErr == nil && !stat.IsDir() {
						fileServer.ServeHTTP(w, r)
						return
					}
				}
			}
			w.Header().Set("Cache-Control", "no-store, no-cache, must-revalidate, max-age=0")
			w.Header().Set("Pragma", "no-cache")
			w.Header().Set("Expires", "0")
			indexReq := r.Clone(r.Context())
			indexReq.URL.Path = "/"
			fileServer.ServeHTTP(w, indexReq)
		})
	}

	return r
}

func renderJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
