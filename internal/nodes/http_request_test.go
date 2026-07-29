package nodes

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestHTTPRequestRejectsOversizedResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(strings.Repeat("x", int(maxHTTPResponseBytes)+1)))
	}))
	defer server.Close()

	executor := NewHTTPRequestExecutor()
	ctx := NewExecutionContext("wf-1", "exec-1")
	node := &Node{Params: map[string]interface{}{
		"url":    server.URL,
		"method": http.MethodGet,
	}}

	_, err := executor.Execute(ctx, node)
	if err == nil || !strings.Contains(err.Error(), "response exceeds") {
		t.Fatalf("expected response size error, got %v", err)
	}
}

func TestHTTPRequestAllowsResponseAtLimit(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(strings.Repeat("x", int(maxHTTPResponseBytes))))
	}))
	defer server.Close()

	executor := NewHTTPRequestExecutor()
	ctx := NewExecutionContext("wf-1", "exec-1")
	node := &Node{Params: map[string]interface{}{
		"url":    server.URL,
		"method": http.MethodGet,
	}}

	if _, err := executor.Execute(ctx, node); err != nil {
		t.Fatalf("expected response at limit to succeed, got %v", err)
	}
}
