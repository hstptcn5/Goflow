package nodes

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
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
		_, _ = w.Write([]byte("{\"id\":\"ds-1\",\"choices\":[{\"message\":{\"content\":\"{\\\"company\\\":\\\"ABC\\\",\\\"revenue\\\":48500000}\"}}]}"))
	}))
	defer server.Close()

	executor := NewProviderAIExtractExecutorWithClients(nil, server.Client(), server.URL)
	ctx := NewExecutionContext("wf", "exec")
	ctx.Credentials["deepseek-cred"] = "ds-secret"
	ctx.CredentialMetadata["deepseek-cred"] = CredentialMetadata{Kind: "API_KEY", Provider: "deepseek", Type: "deepseek"}
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
			"json_schema":   "{\"type\":\"object\",\"properties\":{\"company\":{\"type\":\"string\"},\"revenue\":{\"type\":\"integer\"}},\"required\":[\"company\",\"revenue\"],\"additionalProperties\":false}",
			"schema_name":   "sales",
			"credential_id": "deepseek-cred",
		},
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
			"provider":      "deepseek",
			"model":         "auto",
			"input_type":    "image_url",
			"input":         "https://example.com/image.png",
			"json_schema":   "{\"type\":\"object\",\"properties\":{},\"required\":[],\"additionalProperties\":false}",
			"schema_name":   "image",
			"credential_id": "deepseek-cred",
		},
	}
	if err := executor.Validate(node); err == nil || !strings.Contains(err.Error(), "input_type=text") {
		t.Fatalf("expected DeepSeek non-text input validation error, got %v", err)
	}
}

func TestProviderAIExtractSerializesStructuredTextInput(t *testing.T) {
	node := &Node{
		ID: "extract",
		Params: map[string]interface{}{
			"provider":      "openai",
			"model":         "auto",
			"input_type":    "text",
			"input":         map[string]interface{}{"price": 2654.25, "symbol": "XAUUSD"},
			"credential_id": "openai-cred",
		},
	}
	prepared, err := prepareProviderAIExtractNode(node, "openai")
	if err != nil {
		t.Fatalf("prepare: %v", err)
	}
	input, ok := prepared.Params["input"].(string)
	if !ok {
		t.Fatalf("structured input was not serialized: %#v", prepared.Params["input"])
	}
	var decoded map[string]interface{}
	if err := json.Unmarshal([]byte(input), &decoded); err != nil {
		t.Fatalf("serialized input is not JSON: %v", err)
	}
	if decoded["symbol"] != "XAUUSD" || decoded["price"] != 2654.25 {
		t.Fatalf("unexpected serialized input: %#v", decoded)
	}
}

func TestProviderAIExtractRejectsPlaintextAPIKey(t *testing.T) {
	node := &Node{
		ID: "extract",
		Params: map[string]interface{}{
			"provider":      "openai",
			"input_type":    "text",
			"input":         "hello",
			"api_key":       "sk-plaintext-should-not-live-in-workflows",
			"credential_id": "openai-cred",
		},
	}
	_, err := prepareProviderAIExtractNode(node, "openai")
	if err == nil || !strings.Contains(err.Error(), "plaintext api_key") {
		t.Fatalf("expected plaintext api_key rejection, got %v", err)
	}
}

func TestProviderAIExtractRejectsMismatchedCredentialProvider(t *testing.T) {
	ctx := NewExecutionContext("wf", "exec")
	ctx.Credentials["wrong-cred"] = "secret"
	ctx.CredentialMetadata["wrong-cred"] = CredentialMetadata{Kind: "API_KEY", Provider: "openai", Type: "openai"}
	node := &Node{
		ID: "extract",
		Params: map[string]interface{}{
			"provider":      "deepseek",
			"credential_id": "wrong-cred",
		},
	}
	if err := validateAIExtractCredentialCompatibility(ctx, node, "deepseek"); err == nil || !strings.Contains(err.Error(), "registered for openai") {
		t.Fatalf("expected provider mismatch error, got %v", err)
	}
}

func TestProviderAIExtractRejectsWrongCredentialKind(t *testing.T) {
	ctx := NewExecutionContext("wf", "exec")
	ctx.Credentials["wrong-kind"] = "secret"
	ctx.CredentialMetadata["wrong-kind"] = CredentialMetadata{Kind: "BEARER_TOKEN", Provider: "deepseek", Type: "bearer_token"}
	node := &Node{
		ID: "extract",
		Params: map[string]interface{}{
			"provider":      "deepseek",
			"credential_id": "wrong-kind",
		},
	}
	if err := validateAIExtractCredentialCompatibility(ctx, node, "deepseek"); err == nil || !strings.Contains(err.Error(), "must use kind API_KEY") {
		t.Fatalf("expected API_KEY kind rejection, got %v", err)
	}
}

func TestProviderAIExtractRejectsCredentialWithoutMetadata(t *testing.T) {
	ctx := NewExecutionContext("wf", "exec")
	ctx.Credentials["unknown-cred"] = "secret"
	node := &Node{ID: "extract", Params: map[string]interface{}{"credential_id": "unknown-cred"}}
	if err := validateAIExtractCredentialCompatibility(ctx, node, "openai"); err == nil || !strings.Contains(err.Error(), "metadata is unavailable") {
		t.Fatalf("expected missing credential metadata error, got %v", err)
	}
}

func TestProviderAIExtractDefinitionSupportsBothProviders(t *testing.T) {
	def := NewProviderAIExtractExecutor().GetDefinition()
	if len(def.Params) == 0 || def.Params[0].Name != "provider" {
		t.Fatalf("provider parameter must be first: %#v", def.Params)
	}
	var credential ParamDefinition
	for _, param := range def.Params {
		if param.Name == "api_key" {
			t.Fatal("plaintext api_key must not be exposed by the provider-aware definition")
		}
		if param.Name == "credential_id" {
			credential = param
		}
	}
	if !credential.Required {
		t.Fatal("AI Extract credential must be required")
	}
	if len(credential.CredentialProviders) != 2 || credential.CredentialProviders[0] != "openai" || credential.CredentialProviders[1] != "deepseek" {
		t.Fatalf("unexpected providers: %#v", credential.CredentialProviders)
	}
}
