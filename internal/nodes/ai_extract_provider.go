package nodes

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

const defaultDeepSeekExtractBaseURL = "https://api.deepseek.com"

type ProviderAIExtractExecutor struct {
	openai          *AIExtractExecutor
	client          *http.Client
	deepseekBaseURL string
}

func NewProviderAIExtractExecutor() *ProviderAIExtractExecutor {
	return &ProviderAIExtractExecutor{
		openai:          NewAIExtractExecutor(),
		client:          &http.Client{Timeout: 90 * time.Second},
		deepseekBaseURL: defaultDeepSeekExtractBaseURL,
	}
}

func NewProviderAIExtractExecutorWithClients(openai *AIExtractExecutor, client *http.Client, deepseekBaseURL string) *ProviderAIExtractExecutor {
	if openai == nil {
		openai = NewAIExtractExecutor()
	}
	if client == nil {
		client = &http.Client{Timeout: 90 * time.Second}
	}
	if strings.TrimSpace(deepseekBaseURL) == "" {
		deepseekBaseURL = defaultDeepSeekExtractBaseURL
	}
	return &ProviderAIExtractExecutor{
		openai:          openai,
		client:          client,
		deepseekBaseURL: strings.TrimRight(deepseekBaseURL, "/"),
	}
}

func aiExtractProvider(node *Node) string {
	provider, _ := node.Params["provider"].(string)
	provider = strings.ToLower(strings.TrimSpace(provider))
	if provider == "" {
		return "openai"
	}
	return provider
}

func cloneAIExtractNode(node *Node) *Node {
	clone := *node
	clone.Params = make(map[string]interface{}, len(node.Params))
	for key, value := range node.Params {
		clone.Params[key] = value
	}
	return &clone
}

func normalizeAIExtractModel(node *Node, provider string) *Node {
	clone := cloneAIExtractNode(node)
	model, _ := clone.Params["model"].(string)
	model = strings.TrimSpace(model)
	switch provider {
	case "deepseek":
		if model == "" || model == "auto" || model == "gpt-5" {
			clone.Params["model"] = "deepseek-v4-flash"
		}
	default:
		if model == "" || model == "auto" {
			clone.Params["model"] = "gpt-5"
		}
	}
	return clone
}

func (e *ProviderAIExtractExecutor) Execute(ctx *ExecutionContext, node *Node) (interface{}, error) {
	provider := aiExtractProvider(node)
	switch provider {
	case "openai":
		result, err := e.openai.Execute(ctx, normalizeAIExtractModel(node, provider))
		if err != nil {
			return nil, err
		}
		if output, ok := result.(map[string]interface{}); ok {
			output["provider"] = provider
		}
		return result, nil
	case "deepseek":
		return e.executeDeepSeek(ctx, normalizeAIExtractModel(node, provider))
	default:
		return nil, fmt.Errorf("AI Extract provider must be openai or deepseek")
	}
}

func (e *ProviderAIExtractExecutor) Validate(node *Node) error {
	provider := aiExtractProvider(node)
	if provider != "openai" && provider != "deepseek" {
		return fmt.Errorf("AI Extract provider must be openai or deepseek")
	}
	if provider == "deepseek" {
		inputType, _ := node.Params["input_type"].(string)
		if strings.TrimSpace(inputType) == "" {
			inputType = "text"
		}
		if inputType != "text" {
			return fmt.Errorf("AI Extract DeepSeek provider currently supports input_type=text only")
		}
	}
	_, err := parseAIExtractRequest(normalizeAIExtractModel(node, provider))
	return err
}

func aiExtractProviderAPIKey(ctx *ExecutionContext, node *Node, provider string) (string, error) {
	apiKey, _ := node.Params["api_key"].(string)
	credentialID, _ := node.Params["credential_id"].(string)
	if strings.TrimSpace(credentialID) != "" {
		secret, ok := ctx.Credentials[strings.TrimSpace(credentialID)]
		if !ok || strings.TrimSpace(secret) == "" {
			return "", fmt.Errorf("AI Extract %s credential is not available", provider)
		}
		apiKey = secret
	}
	if strings.TrimSpace(apiKey) == "" {
		return "", fmt.Errorf("AI Extract requires a %s API key or credential", provider)
	}
	return strings.TrimSpace(apiKey), nil
}

func (e *ProviderAIExtractExecutor) executeDeepSeek(ctx *ExecutionContext, node *Node) (interface{}, error) {
	apiKey, err := aiExtractProviderAPIKey(ctx, node, "DeepSeek")
	if err != nil {
		return nil, err
	}
	request, err := parseAIExtractRequest(node)
	if err != nil {
		return nil, err
	}
	if request.InputType != "text" {
		return nil, fmt.Errorf("AI Extract DeepSeek provider currently supports text input only")
	}
	policy, err := aiExtractSourcePolicy(ctx, request.SourcePolicyNodeID)
	if err != nil {
		return nil, err
	}

	schemaJSON, err := json.Marshal(request.Schema)
	if err != nil {
		return nil, fmt.Errorf("AI Extract could not encode JSON schema: %w", err)
	}
	systemPrompt := "Extract only facts supported by the supplied input. Do not invent missing values. Return JSON only. The JSON must follow this JSON Schema exactly: " + string(schemaJSON)
	userPrompt := request.Instructions + "\n\nINPUT:\n" + request.Input
	payload := map[string]interface{}{
		"model": request.Model,
		"messages": []map[string]string{
			{"role": "system", "content": systemPrompt},
			{"role": "user", "content": userPrompt},
		},
		"response_format": map[string]string{"type": "json_object"},
		"temperature":     0,
	}

	result, rawText, err := e.callDeepSeek(ctx.Context, apiKey, payload)
	if err != nil {
		return nil, err
	}
	var structured interface{}
	if err := json.Unmarshal([]byte(rawText), &structured); err != nil {
		return nil, fmt.Errorf("AI Extract DeepSeek returned invalid JSON: %w", err)
	}
	if err := validateAIExtractSchemaValue(request.Schema, structured, "$", true); err != nil {
		return nil, fmt.Errorf("AI Extract DeepSeek output failed schema validation: %w", err)
	}

	return map[string]interface{}{
		"data":          structured,
		"raw_text":      rawText,
		"model_used":    request.Model,
		"provider":      "deepseek",
		"input_type":    request.InputType,
		"response_id":   result["id"],
		"source_policy": policy,
	}, nil
}

func (e *ProviderAIExtractExecutor) callDeepSeek(ctx context.Context, apiKey string, payload map[string]interface{}) (map[string]interface{}, string, error) {
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, "", err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, e.deepseekBaseURL+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return nil, "", fmt.Errorf("AI Extract DeepSeek request could not be created")
	}
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("Content-Type", "application/json")
	resp, err := e.client.Do(req)
	if err != nil {
		return nil, "", fmt.Errorf("AI Extract could not connect to DeepSeek: %w", err)
	}
	defer resp.Body.Close()
	respBody, err := readBounded(resp.Body, maxAIExtractResponse)
	if err != nil {
		return nil, "", err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, "", fmt.Errorf("AI Extract DeepSeek response failed with HTTP %d: %s", resp.StatusCode, boundedErrorText(respBody))
	}
	var result struct {
		ID      string `json:"id"`
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, "", fmt.Errorf("AI Extract could not parse DeepSeek response: %w", err)
	}
	if len(result.Choices) == 0 || strings.TrimSpace(result.Choices[0].Message.Content) == "" {
		return nil, "", fmt.Errorf("AI Extract DeepSeek response did not contain message content")
	}
	return map[string]interface{}{"id": result.ID}, strings.TrimSpace(result.Choices[0].Message.Content), nil
}

func validateAIExtractSchemaValue(schema map[string]interface{}, value interface{}, path string, topLevel bool) error {
	typeName, _ := schema["type"].(string)
	switch typeName {
	case "object":
		object, ok := value.(map[string]interface{})
		if !ok {
			return fmt.Errorf("%s must be an object", path)
		}
		if required, ok := schema["required"].([]interface{}); ok {
			for _, item := range required {
				name, _ := item.(string)
				if name != "" {
					if _, exists := object[name]; !exists {
						return fmt.Errorf("%s.%s is required", path, name)
					}
				}
			}
		}
		properties, _ := schema["properties"].(map[string]interface{})
		for name, rawProperty := range properties {
			child, exists := object[name]
			if !exists {
				continue
			}
			propertySchema, ok := rawProperty.(map[string]interface{})
			if !ok {
				continue
			}
			if err := validateAIExtractSchemaValue(propertySchema, child, path+"."+name, false); err != nil {
				return err
			}
		}
		if additional, exists := schema["additionalProperties"]; exists && additional == false {
			for name := range object {
				if _, known := properties[name]; !known {
					return fmt.Errorf("%s.%s is not allowed", path, name)
				}
			}
		}
	case "array":
		array, ok := value.([]interface{})
		if !ok {
			return fmt.Errorf("%s must be an array", path)
		}
		if itemSchema, ok := schema["items"].(map[string]interface{}); ok {
			for index, item := range array {
				if err := validateAIExtractSchemaValue(itemSchema, item, fmt.Sprintf("%s[%d]", path, index), false); err != nil {
					return err
				}
			}
		}
	case "string":
		if _, ok := value.(string); !ok {
			return fmt.Errorf("%s must be a string", path)
		}
	case "integer":
		number, ok := value.(float64)
		if !ok || number != float64(int64(number)) {
			return fmt.Errorf("%s must be an integer", path)
		}
	case "number":
		if _, ok := value.(float64); !ok {
			return fmt.Errorf("%s must be a number", path)
		}
	case "boolean":
		if _, ok := value.(bool); !ok {
			return fmt.Errorf("%s must be a boolean", path)
		}
	case "":
		if topLevel {
			return fmt.Errorf("top-level schema must declare type=object")
		}
	}
	return nil
}

func (e *ProviderAIExtractExecutor) GetDefinition() NodeDefinition {
	def := e.openai.GetDefinition()
	def.Description = "Extracts structured JSON with OpenAI or DeepSeek. OpenAI supports text, images, documents and audio/video speech; DeepSeek currently supports text extraction."

	params := make([]ParamDefinition, 0, len(def.Params)+1)
	params = append(params, ParamDefinition{
		Name:        "provider",
		Label:       "AI Provider",
		Type:        "select",
		Default:     "openai",
		Options:     []string{"openai", "deepseek"},
		Required:    true,
		Description: "Select which provider executes the extraction. Existing workflows default to OpenAI.",
	})
	for _, param := range def.Params {
		switch param.Name {
		case "model":
			param.Label = "Model"
			param.Default = "auto"
			param.Description = "Use auto for gpt-5 with OpenAI or deepseek-v4-flash with DeepSeek, or enter an explicit provider model ID."
		case "input_type":
			param.Description = "OpenAI supports all listed input types. DeepSeek currently supports text only."
		case "api_key":
			param.Label = "Provider API Key"
			param.Description = "Optional direct key. Prefer an encrypted credential."
		case "credential_id":
			param.Label = "AI Provider Credential"
			param.CredentialKinds = []string{"API_KEY"}
			param.CredentialProviders = []string{"openai", "deepseek"}
			param.Description = "Choose an API-key credential matching the selected provider."
		}
		params = append(params, param)
	}
	def.Params = params
	return def
}
