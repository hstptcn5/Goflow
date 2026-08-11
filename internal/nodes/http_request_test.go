package nodes

import (
	"context"
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

func TestHTTPRequestValidationRejectsUnsafeInputs(t *testing.T) {
	tests := []struct {
		name   string
		params map[string]interface{}
		want   string
	}{
		{name: "relative url", params: map[string]interface{}{"url": "/local", "method": "GET"}, want: "absolute http or https URL"},
		{name: "unsupported scheme", params: map[string]interface{}{"url": "file:///tmp/data.json", "method": "GET"}, want: "absolute http or https URL"},
		{name: "unsupported method", params: map[string]interface{}{"url": "https://example.test", "method": "TRACE"}, want: "not supported"},
		{name: "bad headers json", params: map[string]interface{}{"url": "https://example.test", "method": "GET", "headers": "not-json"}, want: "headers"},
		{name: "bad header name", params: map[string]interface{}{"url": "https://example.test", "method": "GET", "headers": `{"Bad Header":"x"}`}, want: "invalid"},
		{name: "oversized body", params: map[string]interface{}{"url": "https://example.test", "method": "POST", "body": strings.Repeat("x", int(maxHTTPRequestBytes)+1)}, want: "body exceeds"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			executor := NewHTTPRequestExecutor()
			_, err := executor.Execute(NewExecutionContext("wf-1", "exec-1"), &Node{Params: tt.params})
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("expected %q error, got %v", tt.want, err)
			}
		})
	}
}

func TestHTTPRequestDoesNotForwardAuthAcrossOriginRedirect(t *testing.T) {
	var redirectedAuth string
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		redirectedAuth = r.Header.Get("Authorization")
		w.WriteHeader(http.StatusOK)
	}))
	defer target.Close()
	source := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, target.URL, http.StatusFound)
	}))
	defer source.Close()

	executor := NewHTTPRequestExecutor()
	ctx := NewExecutionContextWithContext(context.Background(), "wf-1", "exec-1")
	result, err := executor.Execute(ctx, &Node{Params: map[string]interface{}{
		"url":     source.URL,
		"method":  http.MethodGet,
		"headers": `{"Authorization":"Bearer secret"}`,
	}})
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	if redirectedAuth != "" {
		t.Fatalf("authorization header was forwarded across origin")
	}
	output := result.(map[string]interface{})
	if output["status_code"] != http.StatusOK {
		t.Fatalf("expected sanitized redirect to complete, got %#v", output)
	}
}
