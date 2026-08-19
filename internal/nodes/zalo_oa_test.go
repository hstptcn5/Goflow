package nodes

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestZaloOASendsTextMessageWithEncryptedCredential(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("method=%s", r.Method)
		}
		if r.Header.Get("access_token") != "secret-token" {
			t.Fatalf("access token header missing")
		}
		var payload map[string]interface{}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("decode payload: %v", err)
		}
		recipient := payload["recipient"].(map[string]interface{})
		message := payload["message"].(map[string]interface{})
		if recipient["user_id"] != "user-123" || message["text"] != "Tin sáng nay" {
			t.Fatalf("unexpected payload: %#v", payload)
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"error":0,"message":"Success","data":{"message_id":"msg-42"}}`)
	}))
	defer server.Close()

	executor := NewZaloOAExecutorWithClient(server.Client(), server.URL+"/message")
	ctx := NewExecutionContext("wf", "exec")
	ctx.Credentials["zalo"] = "secret-token"
	node := &Node{Params: map[string]interface{}{
		"credential_id": "zalo",
		"user_id":       "user-123",
		"message":       "Tin sáng nay",
	}}
	result, err := executor.Execute(ctx, node)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	output := result.(map[string]interface{})
	if output["ok"] != true || output["message_id"] != "msg-42" {
		t.Fatalf("unexpected output: %#v", output)
	}
}

func TestZaloOARejectsProviderErrorWithoutEchoingCredential(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"error":-201,"message":"Recipient is not eligible"}`)
	}))
	defer server.Close()

	executor := NewZaloOAExecutorWithClient(server.Client(), server.URL)
	node := &Node{Params: map[string]interface{}{
		"access_token": "top-secret-token",
		"user_id":      "user-123",
		"message":      "Hello",
	}}
	_, err := executor.Execute(NewExecutionContext("wf", "exec"), node)
	if err == nil {
		t.Fatal("expected provider error")
	}
	if strings.Contains(err.Error(), "top-secret-token") {
		t.Fatalf("credential leaked in error: %v", err)
	}
	if !strings.Contains(err.Error(), "error -201") {
		t.Fatalf("provider error code missing: %v", err)
	}
}

func TestZaloOAValidateRequiresRecipientMessageAndCredential(t *testing.T) {
	executor := NewZaloOAExecutor()
	for name, params := range map[string]map[string]interface{}{
		"missing credential": {"user_id": "u", "message": "m"},
		"missing user":       {"access_token": "t", "message": "m"},
		"missing message":    {"access_token": "t", "user_id": "u"},
	} {
		t.Run(name, func(t *testing.T) {
			if err := executor.Validate(&Node{Params: params}); err == nil {
				t.Fatal("expected validation error")
			}
		})
	}
}

func TestZaloOANodeIsNotRetryable(t *testing.T) {
	definition := NewZaloOAExecutor().GetDefinition()
	if definition.Retryable {
		t.Fatal("Zalo OA send must not auto-retry because duplicate delivery is a side effect")
	}
}
