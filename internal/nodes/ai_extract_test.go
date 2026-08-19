package nodes

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestAIExtractTextUsesResponsesStructuredOutputAndSourcePolicy(t *testing.T) {
	var sawStructured bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/responses" {
			http.NotFound(w, r)
			return
		}
		if got := r.Header.Get("Authorization"); got != "Bearer test-key" {
			t.Fatalf("authorization=%q", got)
		}
		var payload map[string]interface{}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("decode payload: %v", err)
		}
		text, _ := payload["text"].(map[string]interface{})
		format, _ := text["format"].(map[string]interface{})
		sawStructured = format["type"] == "json_schema" && format["strict"] == true
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"id":"resp_test","output":[{"type":"message","content":[{"type":"output_text","text":"{\"title\":\"Hello\",\"facts\":[\"A\"]}"}]}]}`)
	}))
	defer server.Close()

	executor := NewAIExtractExecutorWithClients(server.Client(), server.Client(), server.URL, true)
	ctx := NewExecutionContext("wf", "exec")
	ctx.SetOutput("policy", map[string]interface{}{"allowed": true, "policy_enforced": true, "source_id": "source-a"})
	node := &Node{Params: map[string]interface{}{
		"api_key":              "test-key",
		"model":                "gpt-test",
		"input_type":           "text",
		"input":                "Hello source",
		"instructions":         "Extract title and facts",
		"source_policy_node_id": "policy",
		"json_schema": map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"title": map[string]interface{}{"type": "string"},
				"facts": map[string]interface{}{"type": "array", "items": map[string]interface{}{"type": "string"}},
			},
			"required":             []interface{}{"title", "facts"},
			"additionalProperties": false,
		},
	}}
	result, err := executor.Execute(ctx, node)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	if !sawStructured {
		t.Fatal("Responses request did not enable strict JSON schema output")
	}
	output := result.(map[string]interface{})
	data := output["data"].(map[string]interface{})
	if data["title"] != "Hello" || output["response_id"] != "resp_test" {
		t.Fatalf("unexpected extraction output: %#v", output)
	}
	policy := output["source_policy"].(map[string]interface{})
	if policy["source_id"] != "source-a" {
		t.Fatalf("source policy was not propagated: %#v", policy)
	}
}

func TestAIExtractMediaURLTranscribesBeforeStructuredExtraction(t *testing.T) {
	mediaServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "video/mp4")
		_, _ = w.Write([]byte("fake-mp4-with-audio"))
	}))
	defer mediaServer.Close()

	var transcriptionCalls, responseCalls int
	apiServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v1/audio/transcriptions":
			transcriptionCalls++
			if err := r.ParseMultipartForm(1 << 20); err != nil {
				t.Fatalf("parse multipart: %v", err)
			}
			file, header, err := r.FormFile("file")
			if err != nil {
				t.Fatalf("audio file missing: %v", err)
			}
			defer file.Close()
			body, _ := io.ReadAll(file)
			if header.Filename != "clip.mp4" || string(body) != "fake-mp4-with-audio" {
				t.Fatalf("unexpected transcription upload filename=%s body=%q", header.Filename, string(body))
			}
			if r.FormValue("model") != "gpt-4o-mini-transcribe" || r.FormValue("language") != "vi" {
				t.Fatalf("unexpected transcription fields model=%q language=%q", r.FormValue("model"), r.FormValue("language"))
			}
			w.Header().Set("Content-Type", "application/json")
			fmt.Fprint(w, `{"text":"Xin chào từ video"}`)
		case "/v1/responses":
			responseCalls++
			var payload map[string]interface{}
			_ = json.NewDecoder(r.Body).Decode(&payload)
			encoded, _ := json.Marshal(payload)
			if !strings.Contains(string(encoded), "Xin chào từ video") {
				t.Fatalf("transcript was not forwarded to extraction: %s", encoded)
			}
			w.Header().Set("Content-Type", "application/json")
			fmt.Fprint(w, `{"id":"resp_media","output":[{"type":"message","content":[{"type":"output_text","text":"{\"summary\":\"Video greeting\",\"facts\":[]}"}]}]}`)
		default:
			http.NotFound(w, r)
		}
	}))
	defer apiServer.Close()

	executor := NewAIExtractExecutorWithClients(apiServer.Client(), mediaServer.Client(), apiServer.URL, true)
	node := &Node{Params: map[string]interface{}{
		"api_key":      "test-key",
		"model":        "gpt-test",
		"input_type":   "media_url",
		"input":        mediaServer.URL + "/clip.mp4",
		"filename":     "clip.mp4",
		"language":     "vi",
		"max_media_bytes": 1024,
		"json_schema": `{"type":"object","properties":{"summary":{"type":"string"},"facts":{"type":"array","items":{"type":"string"}}},"required":["summary","facts"],"additionalProperties":false}`,
	}}
	result, err := executor.Execute(NewExecutionContext("wf", "exec"), node)
	if err != nil {
		t.Fatalf("Execute media failed: %v", err)
	}
	if transcriptionCalls != 1 || responseCalls != 1 {
		t.Fatalf("calls transcription=%d responses=%d", transcriptionCalls, responseCalls)
	}
	if result.(map[string]interface{})["transcript"] != "Xin chào từ video" {
		t.Fatalf("transcript missing: %#v", result)
	}
}

func TestAIExtractRequiresAllowedSourcePolicyWhenConfigured(t *testing.T) {
	executor := NewAIExtractExecutor()
	ctx := NewExecutionContext("wf", "exec")
	ctx.Credentials["openai"] = "secret"
	node := &Node{Params: map[string]interface{}{
		"credential_id":        "openai",
		"input_type":           "text",
		"input":                "hello",
		"source_policy_node_id": "missing",
		"json_schema":           `{"type":"object","properties":{},"additionalProperties":false}`,
	}}
	if _, err := executor.Execute(ctx, node); err == nil {
		t.Fatal("expected missing source policy to block extraction")
	}
}

func TestAIExtractRejectsInvalidSchemaAndPrivateMediaLiteral(t *testing.T) {
	executor := NewAIExtractExecutor()
	invalidSchema := &Node{Params: map[string]interface{}{
		"input_type": "text",
		"input":      "hello",
		"json_schema": `{"type":"array"}`,
	}}
	if err := executor.Validate(invalidSchema); err == nil {
		t.Fatal("expected non-object schema to fail")
	}

	privateMedia := &Node{Params: map[string]interface{}{
		"api_key":        "key",
		"input_type":     "media_url",
		"input":          "http://127.0.0.1/clip.mp4",
		"filename":       "clip.mp4",
		"max_media_bytes": 1024,
		"json_schema":     `{"type":"object","properties":{},"additionalProperties":false}`,
	}}
	if _, err := executor.Execute(NewExecutionContext("wf", "exec"), privateMedia); err == nil || !strings.Contains(err.Error(), "private or local") {
		t.Fatalf("expected private media URL rejection, got %v", err)
	}
}
