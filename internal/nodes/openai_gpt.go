package nodes

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

type OpenAIGPTExecutor struct {
	client *http.Client
}

func NewOpenAIGPTExecutor() *OpenAIGPTExecutor {
	return &OpenAIGPTExecutor{
		client: &http.Client{
			Timeout: 60 * time.Second,
		},
	}
}

func (e *OpenAIGPTExecutor) Execute(ctx *ExecutionContext, node *Node) (interface{}, error) {
	apiKey, _ := node.Params["api_key"].(string)
	credID, _ := node.Params["credential_id"].(string)
	if credID != "" {
		if secret, ok := ctx.Credentials[credID]; ok {
			apiKey = secret
		}
	}

	model, _ := node.Params["model"].(string)
	if model == "" {
		model = "gpt-4o-mini"
	}

	prompt, _ := node.Params["prompt"].(string)
	systemMsg, _ := node.Params["system_message"].(string)
	if systemMsg == "" {
		systemMsg = "You are a helpful AI assistant."
	}

	if apiKey == "" {
		return nil, fmt.Errorf("OpenAI API Key or Credential is required")
	}
	if prompt == "" {
		return nil, fmt.Errorf("Prompt is required for OpenAI Node")
	}

	payloadMap := map[string]interface{}{
		"model": model,
		"messages": []map[string]string{
			{"role": "system", "content": systemMsg},
			{"role": "user", "content": prompt},
		},
		"temperature": 0.7,
	}

	jsonBytes, _ := json.Marshal(payloadMap)
	req, err := http.NewRequest("POST", "https://api.openai.com/v1/chat/completions", bytes.NewBuffer(jsonBytes))
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

	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("OpenAI API error (%d): %s", resp.StatusCode, string(respBody))
	}

	var result map[string]interface{}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("failed to parse OpenAI response: %w", err)
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
	}, nil
}

func (e *OpenAIGPTExecutor) Validate(node *Node) error {
	return nil
}

func (e *OpenAIGPTExecutor) GetDefinition() NodeDefinition {
	return NodeDefinition{
		Type:        TypeOpenAIGPT,
		Name:        "OpenAI ChatGPT",
		Description: "Calls OpenAI chat models to generate text responses",
		Icon:        "Bot",
		Category:    "AI & LLM",
		Retryable:   true,
		Params: []ParamDefinition{
			{
				Name:        "model",
				Label:       "AI Model",
				Type:        "select",
				Default:     "gpt-4o-mini",
				Options:     []string{"gpt-4o-mini", "gpt-4o", "gpt-3.5-turbo"},
				Required:    true,
				Description: "Choose the OpenAI model",
			},
			{
				Name:        "prompt",
				Label:       "User Prompt",
				Type:        "textarea",
				Default:     "Summarize the latest status",
				Required:    true,
				Description: "Prompt sent to the AI model",
			},
			{
				Name:        "system_message",
				Label:       "System Message",
				Type:        "text",
				Default:     "You are a helpful AI assistant.",
				Required:    false,
				Description: "System role and behavior for the AI model",
			},
			{
				Name:        "api_key",
				Label:       "OpenAI API Key (sk-...)",
				Type:        "text",
				Default:     "",
				Required:    false,
				Description: "OpenAI API key (sk-...)",
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
