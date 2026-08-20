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
	maxOpenAIResponseBytes int64 = 2 << 20
	maxOpenAIPromptChars         = 250000
	maxOpenAISystemChars         = 50000
)

type OpenAIGPTExecutor struct {
	client *http.Client
}

func NewOpenAIGPTExecutor() *OpenAIGPTExecutor {
	return &OpenAIGPTExecutor{client: &http.Client{Timeout: 60 * time.Second}}
}

func (e *OpenAIGPTExecutor) Execute(ctx *ExecutionContext, node *Node) (interface{}, error) {
	if err := validateOpenAINode(node); err != nil {
		return nil, err
	}
	apiKey, err := resolveNodeCredential(ctx, node, "api_key", "OpenAI API key", "openai")
	if err != nil {
		return nil, err
	}

	model, _ := node.Params["model"].(string)
	model = strings.TrimSpace(model)
	if model == "" {
		model = "gpt-4o-mini"
	}
	prompt, _ := node.Params["prompt"].(string)
	systemMsg, _ := node.Params["system_message"].(string)
	if strings.TrimSpace(systemMsg) == "" {
		systemMsg = "You are a helpful AI assistant."
	}

	payloadMap := map[string]interface{}{
		"model": model,
		"messages": []map[string]string{
			{"role": "system", "content": systemMsg},
			{"role": "user", "content": prompt},
		},
		"temperature": 0.7,
	}
	jsonBytes, err := json.Marshal(payloadMap)
	if err != nil {
		return nil, fmt.Errorf("OpenAI request could not be encoded: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx.Context, http.MethodPost, "https://api.openai.com/v1/chat/completions", bytes.NewReader(jsonBytes))
	if err != nil {
		return nil, fmt.Errorf("failed to create OpenAI request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+apiKey)

	resp, err := e.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("OpenAI API request failed: %w", err)
	}
	defer resp.Body.Close()
	respBody, err := readNodeResponseBody(resp, maxOpenAIResponseBytes)
	if err != nil {
		return nil, fmt.Errorf("OpenAI API %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("OpenAI API error (%d): %s", resp.StatusCode, boundedNodeErrorText(respBody))
	}

	var result map[string]interface{}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("failed to parse OpenAI response: %w", err)
	}
	aiReply := extractChatCompletionContent(result)
	if strings.TrimSpace(aiReply) == "" {
		return nil, fmt.Errorf("OpenAI returned an empty assistant response")
	}
	return map[string]interface{}{
		"ai_response": aiReply,
		"raw_result":  result,
		"model_used":  model,
		"provider":    "openai",
	}, nil
}

func extractChatCompletionContent(result map[string]interface{}) string {
	choices, _ := result["choices"].([]interface{})
	if len(choices) == 0 {
		return ""
	}
	choiceMap, _ := choices[0].(map[string]interface{})
	msgMap, _ := choiceMap["message"].(map[string]interface{})
	content, _ := msgMap["content"].(string)
	return content
}

func validateOpenAINode(node *Node) error {
	prompt, _ := node.Params["prompt"].(string)
	if strings.TrimSpace(prompt) == "" {
		return fmt.Errorf("Prompt is required for OpenAI Node")
	}
	if len([]rune(prompt)) > maxOpenAIPromptChars {
		return fmt.Errorf("OpenAI prompt exceeds %d character limit", maxOpenAIPromptChars)
	}
	systemMsg, _ := node.Params["system_message"].(string)
	if len([]rune(systemMsg)) > maxOpenAISystemChars {
		return fmt.Errorf("OpenAI system message exceeds %d character limit", maxOpenAISystemChars)
	}
	credentialID, _ := node.Params["credential_id"].(string)
	apiKey, _ := node.Params["api_key"].(string)
	if strings.TrimSpace(credentialID) == "" && strings.TrimSpace(apiKey) == "" {
		return fmt.Errorf("OpenAI API Key or Credential is required")
	}
	return nil
}

func (e *OpenAIGPTExecutor) Validate(node *Node) error { return validateOpenAINode(node) }

func (e *OpenAIGPTExecutor) GetDefinition() NodeDefinition {
	return NodeDefinition{
		Type:        TypeOpenAIGPT,
		Name:        "OpenAI ChatGPT",
		Description: "Calls OpenAI chat models to generate text responses",
		Icon:        "Bot",
		Category:    "AI & LLM",
		Retryable:   true,
		Params: []ParamDefinition{
			{Name: "model", Label: "AI Model", Type: "select", Default: "gpt-4o-mini", Options: []string{"gpt-4o-mini", "gpt-4o", "gpt-3.5-turbo"}, Required: true, Description: "Choose the OpenAI model"},
			{Name: "prompt", Label: "User Prompt", Type: "textarea", Default: "Summarize the latest status", Required: true, Description: "Prompt sent to the AI model"},
			{Name: "system_message", Label: "System Message", Type: "text", Default: "You are a helpful AI assistant.", Required: false, Description: "System role and behavior for the AI model"},
			{Name: "api_key", Label: "OpenAI API Key (legacy)", Type: "password", Default: "", Required: false, Description: "Legacy direct key. Prefer an encrypted OpenAI credential."},
			{Name: "credential_id", Label: "Credential Secret", Type: "credential", Default: "", Required: false, Description: "Encrypted OpenAI API key", CredentialKinds: []string{"API_KEY"}, CredentialProviders: []string{"openai"}},
		},
	}
}
