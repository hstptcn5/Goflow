package nodes

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestNormalizedHTTPSourceExecutorUsesVaultCredentialAndReturnsOnlyNormalizedData(t *testing.T) {
	var apiKey string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		apiKey = r.Header.Get("X-Store-Key")
		_, _ = w.Write([]byte(`{"report_date":"2026-08-09","revenue":48250.75,"vendor_extra":"discardable-by-workflow"}`))
	}))
	defer server.Close()
	node := &Node{ID: "source", Type: TypeNormalizedHTTPSource, Params: map[string]interface{}{
		"url": server.URL, "auth_mode": "api_key", "api_key_header": "X-Store-Key", "credential_id": "store-credential", "pagination": "none",
		"response_contract": map[string]interface{}{"required": map[string]interface{}{"report_date": map[string]interface{}{"type": "string", "non_empty": true}, "revenue": map[string]interface{}{"type": "number"}}},
	}}
	ctx := NewExecutionContextWithContext(context.Background(), "workflow", "execution")
	ctx.Credentials["store-credential"] = "adapter-secret-canary"
	result, err := NewNormalizedHTTPSourceExecutorWithClient(server.Client(), nil).Execute(ctx, node)
	if err != nil {
		t.Fatal(err)
	}
	if apiKey != "adapter-secret-canary" {
		t.Fatalf("api key = %q", apiKey)
	}
	if text := strings.ToLower(strings.TrimSpace(toJSON(t, result))); strings.Contains(text, "adapter-secret-canary") || strings.Contains(text, strings.ToLower(server.URL)) || strings.Contains(text, "vendor_extra") || strings.Contains(text, "discardable") {
		t.Fatalf("output leaked secret or URL: %s", text)
	}
}

func TestNormalizedHTTPSourceExecutorValidationAndCredentialFailures(t *testing.T) {
	valid := &Node{ID: "source", Type: TypeNormalizedHTTPSource, Params: map[string]interface{}{
		"url": "https://example.test/data", "auth_mode": "none", "pagination": "cursor", "cursor_query_param": "cursor", "items_field": "items", "next_cursor_field": "next_cursor", "max_pages": 2.0, "max_items": 10.0,
		"response_contract": map[string]interface{}{"required": map[string]interface{}{"id": map[string]interface{}{"type": "integer"}}},
	}}
	if err := NewNormalizedHTTPSourceExecutor().Validate(valid); err != nil {
		t.Fatalf("valid node: %v", err)
	}
	tests := []struct {
		name   string
		mutate func(*Node)
		want   string
	}{
		{name: "relative URL", mutate: func(n *Node) { n.Params["url"] = "/relative" }, want: "absolute"},
		{name: "noninteger pages", mutate: func(n *Node) { n.Params["max_pages"] = 1.5 }, want: "integer"},
		{name: "credential with none", mutate: func(n *Node) { n.Params["credential_id"] = "secret" }, want: "only with auth"},
		{name: "missing auth credential", mutate: func(n *Node) { n.Params["auth_mode"] = "bearer" }, want: "credential_id"},
		{name: "missing contract", mutate: func(n *Node) { delete(n.Params, "response_contract") }, want: "response_contract"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			copyNode := *valid
			copyNode.Params = map[string]interface{}{}
			for key, value := range valid.Params {
				copyNode.Params[key] = value
			}
			tt.mutate(&copyNode)
			err := NewNormalizedHTTPSourceExecutor().Validate(&copyNode)
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("want %q, got %v", tt.want, err)
			}
		})
	}
	authNode := *valid
	authNode.Params = map[string]interface{}{}
	for key, value := range valid.Params {
		authNode.Params[key] = value
	}
	authNode.Params["auth_mode"] = "bearer"
	authNode.Params["credential_id"] = "missing"
	_, err := NewNormalizedHTTPSourceExecutor().Execute(NewExecutionContext("workflow", "execution"), &authNode)
	if err == nil || !strings.Contains(err.Error(), "not available") {
		t.Fatalf("missing credential accepted: %v", err)
	}
}

func toJSON(t *testing.T, value interface{}) string {
	t.Helper()
	data, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}
