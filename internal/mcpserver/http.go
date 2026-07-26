package mcpserver

import (
	"net/http"
	"strings"
	"sync"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type HTTPOptions struct {
	BaseURL        string
	MaxInflight    int
	AllowedOrigins []string
}

func NewHTTPHandler(opts HTTPOptions) http.Handler {
	limiters := newHTTPClientLimiters(opts.MaxInflight)
	handler := mcp.NewStreamableHTTPHandler(func(r *http.Request) *mcp.Server {
		server := New(Options{
			BaseURL:       opts.BaseURL,
			APIKey:        bearerToken(r),
			MaxInflight:   opts.MaxInflight,
			TriggerSource: "mcp_http",
			RunInflight:   limiters.limiterFor(r),
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

type httpClientLimiters struct {
	maxInflight int
	mu          sync.Mutex
	byPrincipal map[string]chan struct{}
}

func newHTTPClientLimiters(maxInflight int) *httpClientLimiters {
	if maxInflight <= 0 {
		maxInflight = 2
	}
	return &httpClientLimiters{
		maxInflight: maxInflight,
		byPrincipal: map[string]chan struct{}{},
	}
}

func (l *httpClientLimiters) limiterFor(r *http.Request) chan struct{} {
	principal := bearerToken(r)
	if principal == "" {
		principal = r.RemoteAddr
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	limiter, ok := l.byPrincipal[principal]
	if !ok {
		limiter = make(chan struct{}, l.maxInflight)
		l.byPrincipal[principal] = limiter
	}
	return limiter
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
