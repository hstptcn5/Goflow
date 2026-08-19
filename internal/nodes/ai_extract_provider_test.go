package nodes

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestProviderAIExtractDeepSeekText(t *testing.T) {
	var got map[string]interface{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/chat/completions" {
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
		if r.Header.Get("Authorization") != "Bearer ds-secret" {
			t.Fatalf("unexpected authorization header")
		}
		if err := json.NewDecoder(r.Body).Decode(&got); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"ds-1","choices":[{"message":{"content":"{\"company\":\"ABC\",\"revenue\":48500000}"}}]}`))
	}))
	defer server.Close()

	executor := NewProviderAIExtractExecutorWithClients(nil, server.Client(), server.URL)
	ctx := NewExecutionContext("wf", "exec")
	ctx.Credentials["deepseek-cred"] = "ds-secret"
	node := &Node{
		ID:   "extract",
		Type: NewAIExtractExecutor().GetDefinition().Type,
		Name: "Extract",
		Params: map[string]interface{}{
			"provider":      "deepseek",
			"model":         "auto",
			"input_type":    "text",
			"input":         "ABC đạt doanh thu 48.500.000 đồng.",
			"instructions":  "Extract company and revenue as JSON.",
			"json_schema":   `{"type":"object","properties":{"company":{"type":"string"},"revenue":{"type":"integer"}},"required":["company","revenue"],"additionalProperties":false}`,
			"schema_name":   "sales",
			"credential_id": "deepseek-cred",
		}
	}

	result, err := executor.Execute(ctx, node)
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	output := result.(map[string]interface{})
	if output["provider"] != "deepseek" {
		t.Fatalf("unexpected provider: %#v", output["provider"])
	}
	if output["model_used"] != "deepseek-v4-flash" {
		t.Fatalf("unexpected model: %#v", output["model_used"])
	}
	if got["model"] != "deepseek-v4-flash" {
		t.Fatalf("unexpected request model: %#v", got["model"])
	}
	format, _ := got["response_format"].(map[string]interface{})
	if format["type"] != "json_object" {
		t.Fatalf("expected json_object response format: %#v", got["response_format"])
	}
}

func TestProviderAIExtractDeepSeekRejectsNonText(t *testing.T) {
	executor := NewProviderAIExtractExecutor()
	node := &Node{
		ID:   "extract",
		Type: NewAIExtractExecutor().GetDefinition().Type,
		Params: map[string]interface{}{
			"provider":    "deepseek",
			"model":       "auto",
			"input_type":  "image_url",
			"input":       "https://example.com/image.png",
			"json_schema": `{"type":"object","properties":{},"required":[],"additionalProperties":false}`,
			"schema_name": "image",
		}
	}
	if err := executor.Validate(node); err == nil {
		t.Fatal("expected DeepSeek non-text input validation error")
	}
}

func TestProviderAIExtractDefinitionSupportsBothProviders(t *testing.T) {
	def := NewProviderAIExtractExecutor().GetDefinition()
	if len(def.Params) == 0 || def.Params[0].Name != "provider" {
		t.Fatalf("provider parameter must be first: %#v", def.Params)
	}
	var credential ParamDefinition
	for _, param := range def.Params {
		if param.Name == "credential_id" {
			credential = param
			break
		}
	}
	if len(credential.CredentialProviders) != 2 || credential.CredentialProviders[0] != "openai" || credential.CredentialProviders[1] != "deepseek" {
		t.Fatalf("unexpected providers: %#v", credential.CredentialProviders)
	}
}
