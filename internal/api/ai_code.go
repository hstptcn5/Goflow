package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

const (
	maxAICodePromptBytes = 64 << 10
	maxAICodeBytes       = 256 << 10
	maxAISampleBytes     = 256 << 10
)

type aiCodeRequest struct {
	CredentialID string          `json:"credential_id"`
	Language     string          `json:"language"`
	Action       string          `json:"action"`
	Prompt       string          `json:"prompt"`
	ExistingCode string          `json:"existing_code,omitempty"`
	SampleInput  json.RawMessage `json:"sample_input,omitempty"`
}

func normalizeAICodeRequest(req *aiCodeRequest) error {
	req.Language = strings.ToLower(strings.TrimSpace(req.Language))
	if req.Language != "js" && req.Language != "javascript" && req.Language != "python" {
		return fmt.Errorf("language must be js or python")
	}
	if req.Language == "javascript" {
		req.Language = "js"
	}
	req.Action = strings.ToLower(strings.TrimSpace(req.Action))
	if req.Action == "" {
		req.Action = "generate"
	}
	if req.Action != "generate" && req.Action != "fix" {
		return fmt.Errorf("action must be generate or fix")
	}
	if strings.TrimSpace(req.CredentialID) == "" {
		return fmt.Errorf("credential_id is required")
	}
	if strings.TrimSpace(req.Prompt) == "" {
		return fmt.Errorf("prompt is required")
	}
	if len(req.Prompt) > maxAICodePromptBytes || len(req.ExistingCode) > maxAICodeBytes || len(req.SampleInput) > maxAISampleBytes {
		return fmt.Errorf("AI code assistance request exceeds size limits")
	}
	return nil
}

func aiCodeMessages(req aiCodeRequest) []map[string]string {
	runtimeNote := "JavaScript runs in Goflow's bounded goja runtime. Use input, outputs and trigger; return a JSON-compatible value."
	if req.Language == "python" {
		runtimeNote = "Python runs as trusted external CPython. Use input, outputs and trigger; assign the JSON-compatible result to output. Do not assume credentials are available."
	}
	sample := strings.TrimSpace(string(req.SampleInput))
	if sample == "" {
		sample = "null"
	}
	user := fmt.Sprintf("Action: %s\nUser request: %s\nSample input: %s", req.Action, req.Prompt, sample)
	if strings.TrimSpace(req.ExistingCode) != "" {
		user += "\nExisting code:\n" + req.ExistingCode
	}
	return []map[string]string{
		{"role": "system", "content": "You generate small Goflow code-node snippets. " + runtimeNote + " Return only the final code, without Markdown fences or explanation. Never claim the code was executed."},
		{"role": "user", "content": user},
	}
}

func stripCodeFence(text string) string {
	text = strings.TrimSpace(text)
	if !strings.HasPrefix(text, "```") {
		return text
	}
	lines := strings.Split(text, "\n")
	if len(lines) >= 2 {
		lines = lines[1:]
	}
	if len(lines) > 0 && strings.TrimSpace(lines[len(lines)-1]) == "```" {
		lines = lines[:len(lines)-1]
	}
	return strings.TrimSpace(strings.Join(lines, "\n"))
}

func (h *AIHandler) AssistCode(w http.ResponseWriter, r *http.Request) {
	var req aiCodeRequest
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20))
	if err := decoder.Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}
	if err := normalizeAICodeRequest(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	cred, err := h.credStore.GetByID(req.CredentialID)
	if err != nil {
		http.Error(w, "Credential not found", http.StatusBadRequest)
		return
	}
	apiKey, err := h.credStore.GetDecryptedData(req.CredentialID)
	if err != nil {
		http.Error(w, "Credential could not be decrypted", http.StatusInternalServerError)
		return
	}
	endpoint, model, ok := resolveAIProvider(cred, apiKey)
	if !ok {
		http.Error(w, "Credential is not a supported AI provider", http.StatusBadRequest)
		return
	}
	code, err := callChatCompletion(endpoint, apiKey, model, aiCodeMessages(req), 45*time.Second)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}
	code = stripCodeFence(code)
	if code == "" || len(code) > maxAICodeBytes {
		http.Error(w, "AI returned empty or oversized code", http.StatusBadGateway)
		return
	}
	renderJSON(w, http.StatusOK, map[string]interface{}{
		"language": req.Language,
		"action":   req.Action,
		"code":     code,
		"executed": false,
	})
}
