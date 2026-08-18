package nodes

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHTTPRequestInjectsEncryptedCredentialAtRuntime(t *testing.T) {
	var authorization string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authorization = r.Header.Get("Authorization")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer server.Close()

	executor := NewHTTPRequestExecutor()
	ctx := NewExecutionContextWithContext(context.Background(), "wf", "exec")
	ctx.Credentials["cred-1"] = "private-token"
	node := &Node{ID: "request", Type: TypeHTTPRequest, Params: map[string]interface{}{
		"method":            "GET",
		"url":               server.URL,
		"headers":           "{}",
		"credential_id":     "cred-1",
		"credential_header": "Authorization",
		"credential_prefix": "Bearer ",
	}}
	if _, err := executor.Execute(ctx, node); err != nil {
		t.Fatal(err)
	}
	if authorization != "Bearer private-token" {
		t.Fatalf("unexpected authorization header %q", authorization)
	}
}

func TestHTTPRequestFailsClosedWhenCredentialIsMissing(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("request should not be sent without credential")
	}))
	defer server.Close()

	ctx := NewExecutionContextWithContext(context.Background(), "wf", "exec")
	node := &Node{ID: "request", Type: TypeHTTPRequest, Params: map[string]interface{}{
		"method": "GET", "url": server.URL, "headers": "{}", "credential_id": "missing",
	}}
	if _, err := NewHTTPRequestExecutor().Execute(ctx, node); err == nil {
		t.Fatal("expected missing credential error")
	}
}
