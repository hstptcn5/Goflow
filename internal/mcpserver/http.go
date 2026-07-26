package mcpserver

import (
	"net/http"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type HTTPOptions struct {
	BaseURL        string
	MaxInflight    int
	AllowedOrigins []string
}

func NewHTTPHandler(opts HTTPOptions) http.Handler {
	handler := mcp.NewStreamableHTTPHandler(func(r *http.Request) *mcp.Server {
		server := New(Options{
			BaseURL:     opts.BaseURL,
			APIKey:      bearerToken(r),
			MaxInflight: opts.MaxInflight,
		})
		mcpServer := mcp.NewServer(&mcp.Implementation{
			Name:    "goflow",
			Version: readVersion(),
		}, nil)
		server.registerTools(mcpServer)
		server.registerDynamicWorkflowTools(mcpServer)
		return mcpServer
	}, &mcp.StreamableHTTPOptions{
		Stateless:    true,
		JSONResponse: true,
	})
	return originMiddleware(normalizeOrigins(opts.AllowedOrigins), handler)
}

func bearerToken(r *http.Request) string {
	authHeader := r.Header.Get("Authorization")
	if strings.HasPrefix(authHeader, "Bearer ") {
		return strings.TrimSpace(strings.TrimPrefix(authHeader, "Bearer "))
	}
	return ""
}

func originMiddleware(allowed map[string]bool, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := strings.TrimSpace(r.Header.Get("Origin"))
		if origin != "" && len(allowed) > 0 && !allowed[origin] {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusForbidden)
			_, _ = w.Write([]byte(`{"error":"Forbidden: origin is not allowed for MCP HTTP"}`))
			return
		}
		next.ServeHTTP(w, r)
	})
}

func normalizeOrigins(origins []string) map[string]bool {
	result := map[string]bool{}
	for _, origin := range origins {
		origin = strings.TrimSpace(origin)
		if origin == "" {
			continue
		}
		result[origin] = true
	}
	return result
}
