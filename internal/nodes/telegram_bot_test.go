package nodes

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestTelegramUsesCredentialIDAndMockBaseURL(t *testing.T) {
	var observedPath string
	var payload map[string]interface{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		observedPath = r.URL.Path
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("decode payload: %v", err)
		}
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"ok": true})
	}))
	defer server.Close()

	executor := NewTelegramBotExecutor()
	executor.baseURL = server.URL
	ctx := NewExecutionContext("wf-1", "exec-1")
	ctx.Credentials["cred-1"] = "123:credential-token"

	result, err := executor.Execute(ctx, &Node{Params: map[string]interface{}{
		"bot_token":     "literal-token",
		"credential_id": "cred-1",
		"chat_id":       "12345",
		"message":       "hello",
	}})
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	if observedPath != "/bot123:credential-token/sendMessage" {
		t.Fatalf("expected credential token in API path, got %q", observedPath)
	}
	if payload["chat_id"] != "12345" || payload["text"] != "hello" {
		t.Fatalf("unexpected payload: %#v", payload)
	}
	if result.(map[string]interface{})["ok"] != true {
		t.Fatalf("unexpected result: %#v", result)
	}
}

func TestTelegramCredentialIDDoesNotFallBackToLiteralToken(t *testing.T) {
	executor := NewTelegramBotExecutor()
	ctx := NewExecutionContext("wf-1", "exec-1")
	_, err := executor.Execute(ctx, &Node{Params: map[string]interface{}{
		"bot_token":     "literal-token",
		"credential_id": "missing",
		"chat_id":       "12345",
		"message":       "hello",
	}})
	if err == nil || !strings.Contains(err.Error(), "credential is not available") {
		t.Fatalf("expected missing credential error, got %v", err)
	}
}

func TestTelegramErrorBodyIsBoundedAndRedacted(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"ok":false,"description":"bad bot123:secret-token token"}` + strings.Repeat("x", 5000)))
	}))
	defer server.Close()

	executor := NewTelegramBotExecutor()
	executor.baseURL = server.URL
	_, err := executor.Execute(NewExecutionContext("wf-1", "exec-1"), &Node{Params: map[string]interface{}{
		"bot_token": "123:secret-token",
		"chat_id":   "12345",
		"message":   "hello",
	}})
	if err == nil {
		t.Fatalf("expected telegram API error")
	}
	errText := err.Error()
	if strings.Contains(errText, "secret-token") {
		t.Fatalf("telegram error leaked token: %v", err)
	}
	if len(errText) > 4300 {
		t.Fatalf("telegram error was not bounded: %d bytes", len(errText))
	}
}

func TestTelegramRequestUsesExecutionContext(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		<-r.Context().Done()
	}))
	defer server.Close()

	executor := NewTelegramBotExecutor()
	executor.baseURL = server.URL
	parent, cancel := context.WithCancel(context.Background())
	cancel()
	ctx := NewExecutionContextWithContext(parent, "wf-1", "exec-1")
	_, err := executor.Execute(ctx, &Node{Params: map[string]interface{}{
		"bot_token": "123:secret-token",
		"chat_id":   "12345",
		"message":   "hello",
	}})
	if err == nil {
		t.Fatalf("expected context cancellation error")
	}
	if strings.Contains(err.Error(), "secret-token") {
		t.Fatalf("cancellation error leaked token: %v", err)
	}
}
