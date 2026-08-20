package nodes

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

const (
	maxDeepSeekResponseBytes int64 = 2 << 20
	maxDeepSeekPromptChars         = 250000
	maxDeepSeekSystemChars         = 50000
)

type DeepSeekAIExecutor struct {
	client *http.Client
}

func NewDeepSeekAIExecutor() *DeepSeekAIExecutor {
	return &DeepSeekAIExecutor{client: &http.Client{Timeout: 90 * time.Second}}
}

func (e *DeepSeekAIExecutor) Execute(ctx *ExecutionContext, node *Node) (interface{}, error) {
	if err := validateDeepSeekNode(node); err != nil {
		return nil, err
	}
	apiKey, err := resolveNodeCredential(ctx, node, "api_key", "DeepSeek API key", "deepseek")
	if err != nil {
		return nil, err
	}

	model, _ := node.Params["model"].(string)
	model = strings.TrimSpace(model)
	if model == "" {
		model = "deepseek-v4-flash"
	}
	prompt, _ := node.Params["prompt"].(string)
	systemMsg, _ := node.Params["system_message"].(string)
	if strings.TrimSpace(systemMsg) == "" {
		systemMsg = "You are a helpful DeepSeek AI assistant."
	}

	payloadMap := map[string]interface{}{
		"model": model,
		"messages": []map[string]string{
			{"role": "system", "content": systemMsg},
			{"role": "user", "content": prompt},
		},
		"temperature": 0.7,
		"stream":      false,
	}
	jsonBytes, err := json.Marshal(payloadMap)
	if err != nil {
		return nil, fmt.Errorf("DeepSeek request could not be encoded: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx.Context, http.MethodPost, "https://api.deepseek.com/chat/completions", bytes.NewReader(jsonBytes))
	if err != nil {
		return nil, fmt.Errorf("failed to create DeepSeek request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+apiKey)

	resp, err := e.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("DeepSeek API request failed: %w", err)
	}
	defer resp.Body.Close()
	respBody, err := readNodeResponseBody(resp, maxDeepSeekResponseBytes)
	if err != nil {
		return nil, fmt.Errorf("DeepSeek API %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("DeepSeek API error (%d): %s", resp.StatusCode, boundedNodeErrorText(respBody))
	}

	var result map[string]interface{}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("failed to parse DeepSeek response: %w", err)
	}
	aiReply := extractChatCompletionContent(result)
	if strings.TrimSpace(aiReply) == "" {
		return nil, fmt.Errorf("DeepSeek returned an empty assistant response")
	}
	return map[string]interface{}{
		"ai_response": aiReply,
		"raw_result":  result,
		"model_used":  model,
		"provider":    "deepseek",
	}, nil
}

func validateDeepSeekNode(node *Node) error {
	prompt, _ := node.Params["prompt"].(string)
	if strings.TrimSpace(prompt) == "" {
		return fmt.Errorf("Prompt is required for DeepSeek AI Node")
	}
	if len([]rune(prompt)) > maxDeepSeekPromptChars {
		return fmt.Errorf("DeepSeek prompt exceeds %d character limit", maxDeepSeekPromptChars)
	}
	systemMsg, _ := node.Params["system_message"].(string)
	if len([]rune(systemMsg)) > maxDeepSeekSystemChars {
		return fmt.Errorf("DeepSeek system message exceeds %d character limit", maxDeepSeekSystemChars)
	}
	credentialID, _ := node.Params["credential_id"].(string)
	apiKey, _ := node.Params["api_key"].(string)
	if strings.TrimSpace(credentialID) == "" && strings.TrimSpace(apiKey) == "" {
		return fmt.Errorf("DeepSeek API Key or Credential is required")
	}
	return nil
}

func (e *DeepSeekAIExecutor) Validate(node *Node) error { return validateDeepSeekNode(node) }

func (e *DeepSeekAIExecutor) GetDefinition() NodeDefinition {
	return NodeDefinition{
		Type:        TypeDeepSeekAI,
		Name:        "DeepSeek AI",
		Description: "Calls DeepSeek chat or reasoner models to generate AI responses",
		Icon:        "Brain",
		Category:    "AI & LLM",
		Retryable:   true,
		Params: []ParamDefinition{
			{Name: "model", Label: "DeepSeek Model", Type: "select", Default: "deepseek-v4-flash", Options: []string{"deepseek-v4-flash", "deepseek-v4-pro"}, Required: true, Description: "Choose the DeepSeek model"},
			{Name: "prompt", Label: "User Prompt", Type: "textarea", Default: "Explain quantum computing in simple terms", Required: true, Description: "Prompt sent to DeepSeek AI"},
			{Name: "system_message", Label: "System Message", Type: "text", Default: "You are a helpful AI assistant.", Required: false, Description: "Defines the assistant role and behavior"},
			{Name: "api_key", Label: "DeepSeek API Key (legacy)", Type: "password", Default: "", Required: false, Description: "Legacy direct key. Prefer an encrypted DeepSeek credential."},
			{Name: "credential_id", Label: "Credential Secret", Type: "credential", Default: "", Required: false, Description: "Encrypted DeepSeek API key", CredentialKinds: []string{"API_KEY"}, CredentialProviders: []string{"deepseek"}},
		},
	}
}
