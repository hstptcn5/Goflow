package nodes

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

type DeepSeekAIExecutor struct {
	client *http.Client
}

func NewDeepSeekAIExecutor() *DeepSeekAIExecutor {
	return &DeepSeekAIExecutor{
		client: &http.Client{
			Timeout: 90 * time.Second,
		},
	}
}

func (e *DeepSeekAIExecutor) Execute(ctx *ExecutionContext, node *Node) (interface{}, error) {
	apiKey, _ := node.Params["api_key"].(string)
	credID, _ := node.Params["credential_id"].(string)
	if credID != "" {
		if secret, ok := ctx.Credentials[credID]; ok {
			apiKey = secret
		}
	}

	model, _ := node.Params["model"].(string)
	if model == "" {
		model = "deepseek-chat"
	}

	prompt, _ := node.Params["prompt"].(string)
	systemMsg, _ := node.Params["system_message"].(string)
	if systemMsg == "" {
		systemMsg = "You are a helpful DeepSeek AI assistant."
	}

	if apiKey == "" {
		return nil, fmt.Errorf("DeepSeek API Key or Credential is required")
	}
	if prompt == "" {
		return nil, fmt.Errorf("Prompt is required for DeepSeek AI Node")
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

	jsonBytes, _ := json.Marshal(payloadMap)
	req, err := http.NewRequest("POST", "https://api.deepseek.com/chat/completions", bytes.NewBuffer(jsonBytes))
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

	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("DeepSeek API error (%d): %s", resp.StatusCode, string(respBody))
	}

	var result map[string]interface{}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("failed to parse DeepSeek response: %w", err)
	}

	var aiReply string
	if choices, ok := result["choices"].([]interface{}); ok && len(choices) > 0 {
		if choiceMap, ok := choices[0].(map[string]interface{}); ok {
			if msgMap, ok := choiceMap["message"].(map[string]interface{}); ok {
				aiReply, _ = msgMap["content"].(string)
			}
		}
	}

	return map[string]interface{}{
		"ai_response": aiReply,
		"raw_result":  result,
		"model_used":  model,
		"provider":    "DeepSeek AI",
	}, nil
}

func (e *DeepSeekAIExecutor) Validate(node *Node) error {
	return nil
}

func (e *DeepSeekAIExecutor) GetDefinition() NodeDefinition {
	return NodeDefinition{
		Type:        TypeDeepSeekAI,
		Name:        "DeepSeek AI",
		Description: "Calls DeepSeek chat or reasoner models to generate AI responses",
		Icon:        "Brain",
		Category:    "AI & LLM",
		Retryable:   true,
		Params: []ParamDefinition{
			{
				Name:        "model",
				Label:       "DeepSeek Model",
				Type:        "select",
				Default:     "deepseek-chat",
				Options:     []string{"deepseek-chat", "deepseek-reasoner", "deepseek-coder"},
				Required:    true,
				Description: "Choose the DeepSeek model: deepseek-chat or deepseek-reasoner",
			},
			{
				Name:        "prompt",
				Label:       "User Prompt",
				Type:        "textarea",
				Default:     "Explain quantum computing in simple terms",
				Required:    true,
				Description: "Prompt sent to DeepSeek AI",
			},
			{
				Name:        "system_message",
				Label:       "System Message",
				Type:        "text",
				Default:     "You are a helpful AI assistant.",
				Required:    false,
				Description: "Defines the assistant role and behavior",
			},
			{
				Name:        "api_key",
				Label:       "DeepSeek API Key (sk-...)",
				Type:        "text",
				Default:     "",
				Required:    false,
				Description: "DeepSeek API key (sk-...)",
			},
			{
				Name:        "credential_id",
				Label:       "Credential Secret",
				Type:        "credential",
				Default:     "",
				Required:    false,
				Description: "Or select a saved credential",
			},
		},
	}
}
