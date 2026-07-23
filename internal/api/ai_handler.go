package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"goflow/internal/nodes"
	"goflow/internal/storage"
)

type AIHandler struct {
	credStore *storage.CredentialStore
	registry  *nodes.PluginRegistry
}

func NewAIHandler(cs *storage.CredentialStore, r *nodes.PluginRegistry) *AIHandler {
	return &AIHandler{credStore: cs, registry: r}
}

func (h *AIHandler) GenerateWorkflow(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Messages     []map[string]string `json:"messages"`
		CredentialID string              `json:"credential_id"`
		CurrentNodes json.RawMessage     `json:"current_nodes,omitempty"`
		CurrentEdges json.RawMessage     `json:"current_edges,omitempty"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if len(req.Messages) == 0 || req.CredentialID == "" {
		http.Error(w, "Messages and CredentialID are required", http.StatusBadRequest)
		return
	}

	// 1. Fetch and Decrypt the Credential
	cred, err := h.credStore.GetByID(req.CredentialID)
	if err != nil {
		http.Error(w, "Credential not found", http.StatusBadRequest)
		return
	}

	apiKey, err := h.credStore.GetDecryptedData(req.CredentialID)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to decrypt API Key: %v", err), http.StatusInternalServerError)
		return
	}

	// 2. Prepare dynamic system prompt based on registered Node definitions (compact markdown format)
	defs := h.registry.ListDefinitions()
	var defsSB strings.Builder
	for _, def := range defs {
		defsSB.WriteString(fmt.Sprintf("* Node Type: '%s' (%s) - %s\n", def.Type, def.Name, def.Description))
		if len(def.Params) > 0 {
			defsSB.WriteString("  Parameters:\n")
			for _, p := range def.Params {
				reqStr := "optional"
				if p.Required {
					reqStr = "required"
				}
				defaultStr := ""
				if p.Default != nil && fmt.Sprintf("%v", p.Default) != "" {
					defaultStr = fmt.Sprintf(", default: %v", p.Default)
				}
				defsSB.WriteString(fmt.Sprintf("    - %s (%s, %s%s) - %s\n", p.Name, p.Type, reqStr, defaultStr, p.Description))
			}
		}
		defsSB.WriteString("\n")
	}
	defsFormatted := defsSB.String()

	var currentFlowContext string
	if len(req.CurrentNodes) > 0 && string(req.CurrentNodes) != "null" && string(req.CurrentNodes) != "[]" {
		currentFlowContext = fmt.Sprintf("\n\nCRITICAL CONTEXT: The user is currently editing a workflow. Here is the current workflow structure:\nNodes:\n%s\nEdges:\n%s\n\nIf the user asks to modify, add, delete, or connect nodes, analyze this current workflow and return the updated complete workflow JSON structure inside a markdown block. If they only ask a question, request an explanation, or comment, simply reply in plain text without any JSON code block.", string(req.CurrentNodes), string(req.CurrentEdges))
	}

	systemPrompt := fmt.Sprintf(`You are Goflow AI Assistant, an expert workflow automation engineer.
Your task is to help the user build, modify, explain, or troubleshoot Goflow workflows based on their prompt.
Reply in natural language (Vietnamese by default).

RESPONSE FORMAT RULES:
1. If you propose a new workflow or suggest changes/fixes to the current workflow structure, write your explanation first, and then include the complete, updated workflow JSON structure inside a markdown code block starting with ` + "`" + "``json" + "`" + ` and ending with ` + "`" + "```" + "`" + `.
2. If you are just answering a general question, explaining how a node works, or chatting with the user without modifying the canvas structure, simply reply in plain text. Do NOT include any JSON code block in this case.

Example Goflow JSON Workflow Reference:
{
  "name": "GitHub Webhook to Telegram Alert",
  "nodes": [
    {
      "id": "webhookTrigger",
      "type": "webhookTrigger",
      "name": "GitHub Push Webhook",
      "position": { "x": 100, "y": 200 },
      "params": {
        "path": "github-push"
      }
    },
    {
      "id": "httpRequest",
      "type": "httpRequest",
      "name": "Fetch Commit Details",
      "position": { "x": 400, "y": 200 },
      "params": {
        "method": "GET",
        "url": "https://api.github.com/repos/{{webhookTrigger.body.repository.full_name}}/commits/{{webhookTrigger.body.after}}",
        "headers": "{\"User-Agent\": \"Goflow-App\"}",
        "body": ""
      }
    },
    {
      "id": "telegramBot",
      "type": "telegramBot",
      "name": "Send Dev Alert Telegram",
      "position": { "x": 700, "y": 200 },
      "params": {
        "chat_id": "12345678",
        "message": "Status Code: {{httpRequest.status_code}}\nData: {{httpRequest.data}}"
      }
    }
  ],
  "edges": [
    {
      "id": "edge_1",
      "source": "webhookTrigger",
      "target": "httpRequest"
    },
    {
      "id": "edge_2",
      "source": "httpRequest",
      "target": "telegramBot"
    }
  ]
}

Available node definitions and their parameters that you MUST use:
%s
%s

CRITICAL: Goflow Template Interpolation Syntax Rules
Preceding nodes expose outputs that subsequent nodes reference using placeholders: {{node_id.property}}. 
You MUST reference the outputs of preceding nodes using their EXACT property names listed below:
1. 'cronTrigger' Node: Exposes "triggered_at" (RFC3339 timestamp) and "schedule". (e.g. {{cronTriggerId.triggered_at}})
2. 'httpRequest' Node: Exposes "status_code" (int, e.g. 200), "data" (JSON or string body payload), and "headers". (e.g. {{httpRequestId.status_code}} or {{httpRequestId.data.repositories.0.name}})
3. 'openAIGPT' & 'deepseekAI' Nodes: Expose "ai_response" (AI generated string answer) and "model_used". (e.g. {{deepseekAIId.ai_response}})
4. 'jsonTransform' Node: Exposes "transformed" (the parsed JSON object structure based on the 'json_template' parameter). (e.g. {{jsonTransformId.transformed.message}})
5. 'postgresQuery' & 'mysqlQuery' Nodes: Expose "rows" (array of rows) and "affected_rows". (e.g. {{dbNodeId.rows}})

Guidelines for Workflow JSON:
1. Arrange nodes visually on the 2D canvas by setting logical "position" {x, y} coordinates (e.g., arrange them horizontally left-to-right, space them out by dx=300, dy=0 to avoid overlapping).
2. Connect nodes with edges. If connecting from a 'conditionIf' node, you MUST specify "sourceHandle" as either "true" or "false" to connect the branches. For other nodes, sourceHandle should be null or omitted.
3. For SaaS nodes (like 'openAIGPT', 'telegramBot', 'postgresQuery'), if a credential parameter is needed, do NOT include the actual API key in 'params'. Instead, leave the credential param empty or omit it, as the user will select their credential from the dropdown in the UI.
4. Cron expression for 'cronTrigger' node MUST be a valid 5-field standard cron string (e.g., "0 9 * * *" for daily 9am, "*/5 * * * *" for every 5 mins). NEVER output partial expressions like "0 9 ".
5. Begin with "{" and end with "}". No markdown wrapping.`, defsFormatted, currentFlowContext)

	// 3. Select API Endpoint based on Credential Type or heuristics
	var endpoint string
	var model string

	isOpenAI := false
	isDeepSeek := false

	if cred.Type == "OpenAI" {
		isOpenAI = true
	} else if cred.Type == "DeepSeek" {
		isDeepSeek = true
	} else if cred.Type == "API_KEY" {
		// Heuristics for general API_KEY credentials: check name keywords first
		lowerName := strings.ToLower(cred.Name)
		if strings.Contains(lowerName, "deepseek") {
			isDeepSeek = true
		} else if strings.Contains(lowerName, "openai") || strings.Contains(lowerName, "gpt") {
			isOpenAI = true
		} else if strings.HasPrefix(apiKey, "sk-") {
			// Fallback if no keywords found but has sk- prefix (default to OpenAI)
			isOpenAI = true
		} else {
			isOpenAI = true
		}
	}

	if isOpenAI {
		endpoint = "https://api.openai.com/v1/chat/completions"
		model = "gpt-4o"
	} else if isDeepSeek {
		endpoint = "https://api.deepseek.com/v1/chat/completions"
		model = "deepseek-chat"
	} else {
		http.Error(w, "Unsupported credential type for AI Assistant. Please use OpenAI, DeepSeek, or API_KEY credentials.", http.StatusBadRequest)
		return
	}

	// 4. Construct API Payload with entire chat history
	var llmMessages []map[string]string
	llmMessages = append(llmMessages, map[string]string{"role": "system", "content": systemPrompt})
	for _, m := range req.Messages {
		role := m["role"]
		content := m["content"]
		if role != "" && content != "" {
			llmMessages = append(llmMessages, map[string]string{"role": role, "content": content})
		}
	}

	apiReqBody := map[string]interface{}{
		"model":       model,
		"messages":    llmMessages,
		"temperature": 0.2,
	}

	apiReqJSON, err := json.Marshal(apiReqBody)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// 5. Send Request to LLM
	client := &http.Client{Timeout: 45 * time.Second}
	httpReq, err := http.NewRequest("POST", endpoint, bytes.NewBuffer(apiReqJSON))
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+apiKey)

	resp, err := client.Do(httpReq)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to call LLM API: %v", err), http.StatusInternalServerError)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		var errResp map[string]interface{}
		_ = json.NewDecoder(resp.Body).Decode(&errResp)
		errBytes, _ := json.Marshal(errResp)
		http.Error(w, fmt.Sprintf("LLM API returned error (HTTP %d): %s", resp.StatusCode, string(errBytes)), http.StatusInternalServerError)
		return
	}

	var apiResp struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&apiResp); err != nil {
		http.Error(w, "Failed to parse LLM API response", http.StatusInternalServerError)
		return
	}

	if len(apiResp.Choices) == 0 {
		http.Error(w, "LLM returned empty choices", http.StatusInternalServerError)
		return
	}

	content := apiResp.Choices[0].Message.Content
	content = strings.TrimSpace(content)

	jsonStr, textStr := extractWorkflowJSON(content)

	responsePayload := map[string]interface{}{}

	if jsonStr != "" {
		sanitized := sanitizeJSONString(jsonStr)
		var workflowData map[string]interface{}
		if err := json.Unmarshal([]byte(sanitized), &workflowData); err == nil {
			responsePayload["type"] = "workflow"
			responsePayload["workflow"] = workflowData
			responsePayload["text"] = textStr
			
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(responsePayload)
			return
		}
	}

	// Fallback to text message
	responsePayload["type"] = "text"
	responsePayload["text"] = strings.TrimSpace(strings.TrimPrefix(content, "TEXT:"))

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(responsePayload)
}

func (h *AIHandler) ConfigureNode(w http.ResponseWriter, r *http.Request) {
	var req struct {
		NodeType      string                 `json:"node_type"`
		Prompt        string                 `json:"prompt"`
		CurrentParams map[string]interface{} `json:"current_params"`
		CredentialID  string                 `json:"credential_id"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if req.NodeType == "" || req.Prompt == "" || req.CredentialID == "" {
		http.Error(w, "NodeType, Prompt and CredentialID are required", http.StatusBadRequest)
		return
	}

	// 1. Fetch and Decrypt the Credential
	cred, err := h.credStore.GetByID(req.CredentialID)
	if err != nil {
		http.Error(w, "Credential not found", http.StatusBadRequest)
		return
	}

	apiKey, err := h.credStore.GetDecryptedData(req.CredentialID)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to decrypt API Key: %v", err), http.StatusInternalServerError)
		return
	}

	// 2. Locate node definition to get parameters schema
	var nodeDef *nodes.NodeDefinition
	for _, def := range h.registry.ListDefinitions() {
		if string(def.Type) == req.NodeType {
			nodeDef = &def
			break
		}
	}

	if nodeDef == nil {
		http.Error(w, "Node type not found in registry", http.StatusBadRequest)
		return
	}

	defJSONBytes, _ := json.Marshal(nodeDef)
	currParamsJSONBytes, _ := json.Marshal(req.CurrentParams)

	systemPrompt := fmt.Sprintf(`You are Goflow Node Configurer.
Your task is to configure the parameters for a single node of type: "%s" based on the user's prompt.

Node Parameter Schema Definition:
%s

Current Parameter values (before modification):
%s

Instructions:
Modify the parameter values according to the user's request. You must respect the Node Parameter Schema.
Return ONLY a valid JSON object containing the updated parameter key-value pairs. Do not output markdown code fences, backticks, or explanations. Begin with "{" and end with "}".`, req.NodeType, string(defJSONBytes), string(currParamsJSONBytes))

	// 3. Select API Endpoint based on Credential Type or heuristics
	var endpoint string
	var model string

	isOpenAI := false
	isDeepSeek := false

	if cred.Type == "OpenAI" {
		isOpenAI = true
	} else if cred.Type == "DeepSeek" {
		isDeepSeek = true
	} else if cred.Type == "API_KEY" {
		lowerName := strings.ToLower(cred.Name)
		if strings.Contains(lowerName, "deepseek") {
			isDeepSeek = true
		} else if strings.Contains(lowerName, "openai") || strings.Contains(lowerName, "gpt") {
			isOpenAI = true
		} else if strings.HasPrefix(apiKey, "sk-") {
			isOpenAI = true
		} else {
			isOpenAI = true
		}
	}

	if isOpenAI {
		endpoint = "https://api.openai.com/v1/chat/completions"
		model = "gpt-4o"
	} else if isDeepSeek {
		endpoint = "https://api.deepseek.com/v1/chat/completions"
		model = "deepseek-chat"
	} else {
		http.Error(w, "Unsupported credential type for AI node configuration.", http.StatusBadRequest)
		return
	}

	// 4. Construct API Payload
	apiReqBody := map[string]interface{}{
		"model": model,
		"messages": []map[string]string{
			{"role": "system", "content": systemPrompt},
			{"role": "user", "content": req.Prompt},
		},
		"temperature": 0.2,
	}

	apiReqJSON, err := json.Marshal(apiReqBody)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// 5. Send Request to LLM
	client := &http.Client{Timeout: 30 * time.Second}
	httpReq, err := http.NewRequest("POST", endpoint, bytes.NewBuffer(apiReqJSON))
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+apiKey)

	resp, err := client.Do(httpReq)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to call LLM API: %v", err), http.StatusInternalServerError)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		var errResp map[string]interface{}
		_ = json.NewDecoder(resp.Body).Decode(&errResp)
		errBytes, _ := json.Marshal(errResp)
		http.Error(w, fmt.Sprintf("LLM API returned error (HTTP %d): %s", resp.StatusCode, string(errBytes)), http.StatusInternalServerError)
		return
	}

	var apiResp struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&apiResp); err != nil {
		http.Error(w, "Failed to parse LLM API response", http.StatusInternalServerError)
		return
	}

	if len(apiResp.Choices) == 0 {
		http.Error(w, "LLM returned empty choices", http.StatusInternalServerError)
		return
	}

	content := apiResp.Choices[0].Message.Content
	content = strings.TrimSpace(content)
	if strings.HasPrefix(content, "```") {
		lines := strings.Split(content, "\n")
		var bodyLines []string
		for _, line := range lines {
			trimmed := strings.TrimSpace(line)
			if !strings.HasPrefix(trimmed, "```") {
				bodyLines = append(bodyLines, line)
			}
		}
		content = strings.Join(bodyLines, "\n")
	}
	content = strings.TrimSpace(content)

	// Validate if it is valid JSON
	var updatedParams map[string]interface{}
	if err := json.Unmarshal([]byte(content), &updatedParams); err != nil {
		http.Error(w, fmt.Sprintf("LLM generated invalid params JSON: %v. Raw: %s", err, content), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Write([]byte(content))
}

func extractWorkflowJSON(content string) (string, string) {
	// 1. Try to find markdown code block ```json ... ```
	startFence := "```json"
	endFence := "```"
	
	idxStart := strings.Index(content, startFence)
	if idxStart != -1 {
		idxEnd := strings.Index(content[idxStart+len(startFence):], endFence)
		if idxEnd != -1 {
			actualEnd := idxStart + len(startFence) + idxEnd
			jsonStr := content[idxStart+len(startFence) : actualEnd]
			textStr := content[:idxStart] + "\n" + content[actualEnd+len(endFence):]
			return strings.TrimSpace(jsonStr), strings.TrimSpace(textStr)
		}
	}
	
	// Fallback to generic ``` code fence
	startFence = "```"
	idxStart = strings.Index(content, startFence)
	if idxStart != -1 {
		idxEnd := strings.Index(content[idxStart+len(startFence):], endFence)
		if idxEnd != -1 {
			actualEnd := idxStart + len(startFence) + idxEnd
			jsonStr := content[idxStart+len(startFence) : actualEnd]
			textStr := content[:idxStart] + "\n" + content[actualEnd+len(endFence):]
			return strings.TrimSpace(jsonStr), strings.TrimSpace(textStr)
		}
	}

	// 2. If no code fences, check if there is an outer JSON object
	firstBrace := strings.Index(content, "{")
	lastBrace := strings.LastIndex(content, "}")
	if firstBrace != -1 && lastBrace != -1 && lastBrace > firstBrace {
		jsonStr := content[firstBrace : lastBrace+1]
		if strings.Contains(jsonStr, `"nodes"`) || strings.Contains(jsonStr, `"edges"`) {
			textStr := content[:firstBrace] + "\n" + content[lastBrace+1:]
			return strings.TrimSpace(jsonStr), strings.TrimSpace(textStr)
		}
	}

	return "", content
}

func sanitizeJSONString(s string) string {
	var sb strings.Builder
	inQuote := false
	escaped := false
	
	runes := []rune(s)
	for i := 0; i < len(runes); i++ {
		r := runes[i]
		if escaped {
			sb.WriteRune(r)
			escaped = false
			continue
		}
		if r == '\\' {
			sb.WriteRune(r)
			escaped = true
			continue
		}
		if r == '"' {
			inQuote = !inQuote
			sb.WriteRune(r)
			continue
		}
		if inQuote {
			if r == '\n' {
				sb.WriteString("\\n")
			} else if r == '\r' {
				sb.WriteString("\\r")
			} else if r == '\t' {
				sb.WriteString("\\t")
			} else {
				sb.WriteRune(r)
			}
		} else {
			sb.WriteRune(r)
		}
	}
	return sb.String()
}
