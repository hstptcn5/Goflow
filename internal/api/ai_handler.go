package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"goflow/internal/engine"
	"goflow/internal/nodes"
	"goflow/internal/storage"
)

type AIHandler struct {
	credStore *storage.CredentialStore
	registry  *nodes.PluginRegistry
}

type workflowDraft struct {
	Name  string       `json:"name"`
	Nodes []nodes.Node `json:"nodes"`
	Edges []nodes.Edge `json:"edges"`
}

func NewAIHandler(cs *storage.CredentialStore, r *nodes.PluginRegistry) *AIHandler {
	return &AIHandler{credStore: cs, registry: r}
}

func resolveAIProvider(cred *storage.Credential, apiKey string) (string, string, bool) {
	credType := strings.ToLower(strings.TrimSpace(cred.Type))
	lowerName := strings.ToLower(cred.Name)

	switch credType {
	case "openai":
		return "https://api.openai.com/v1/chat/completions", "gpt-4o", true
	case "deepseek":
		return "https://api.deepseek.com/v1/chat/completions", "deepseek-chat", true
	case "api_key":
		if strings.Contains(lowerName, "deepseek") {
			return "https://api.deepseek.com/v1/chat/completions", "deepseek-chat", true
		}
		if strings.Contains(lowerName, "openai") || strings.Contains(lowerName, "gpt") || strings.HasPrefix(apiKey, "sk-") {
			return "https://api.openai.com/v1/chat/completions", "gpt-4o", true
		}
		return "https://api.openai.com/v1/chat/completions", "gpt-4o", true
	default:
		return "", "", false
	}
}

func (h *AIHandler) validateWorkflowDraft(draft workflowDraft) []string {
	var issues []string
	if strings.TrimSpace(draft.Name) == "" {
		issues = append(issues, "workflow.name is required")
	}
	if len(draft.Nodes) == 0 {
		issues = append(issues, "workflow.nodes must contain at least one node")
		return issues
	}

	seenNodeIDs := make(map[string]bool, len(draft.Nodes))
	for i := range draft.Nodes {
		node := &draft.Nodes[i]
		node.ID = strings.TrimSpace(node.ID)
		node.Name = strings.TrimSpace(node.Name)
		if node.Params == nil {
			node.Params = map[string]interface{}{}
		}

		if node.ID == "" {
			issues = append(issues, fmt.Sprintf("nodes[%d].id is required", i))
			continue
		}
		if seenNodeIDs[node.ID] {
			issues = append(issues, fmt.Sprintf("duplicate node id: %s", node.ID))
		}
		seenNodeIDs[node.ID] = true

		executor, ok := h.registry.Get(node.Type)
		if !ok {
			issues = append(issues, fmt.Sprintf("node %q uses unknown type %q", node.ID, node.Type))
			continue
		}

		def := executor.GetDefinition()
		allowedParams := make(map[string]nodes.ParamDefinition, len(def.Params))
		for _, param := range def.Params {
			allowedParams[param.Name] = param
		}
		for paramName := range node.Params {
			if _, ok := allowedParams[paramName]; !ok {
				issues = append(issues, fmt.Sprintf("node %q has unsupported parameter %q for type %q", node.ID, paramName, node.Type))
			}
		}
		for _, param := range def.Params {
			if !param.Required {
				continue
			}
			value, ok := node.Params[param.Name]
			if !ok || isBlankParam(value) {
				issues = append(issues, fmt.Sprintf("node %q is missing required parameter %q", node.ID, param.Name))
			}
		}
		if err := executor.Validate(node); err != nil {
			issues = append(issues, fmt.Sprintf("node %q validation failed: %v", node.ID, err))
		}
	}

	edgeIDs := make(map[string]bool, len(draft.Edges))
	for i, edge := range draft.Edges {
		if strings.TrimSpace(edge.ID) == "" {
			issues = append(issues, fmt.Sprintf("edges[%d].id is required", i))
		} else if edgeIDs[edge.ID] {
			issues = append(issues, fmt.Sprintf("duplicate edge id: %s", edge.ID))
		}
		edgeIDs[edge.ID] = true
		if strings.TrimSpace(edge.Source) == "" || strings.TrimSpace(edge.Target) == "" {
			issues = append(issues, fmt.Sprintf("edge %q must include source and target", edge.ID))
			continue
		}
		if !seenNodeIDs[edge.Source] {
			issues = append(issues, fmt.Sprintf("edge %q references unknown source node %q", edge.ID, edge.Source))
		}
		if !seenNodeIDs[edge.Target] {
			issues = append(issues, fmt.Sprintf("edge %q references unknown target node %q", edge.ID, edge.Target))
		}
	}

	if len(issues) == 0 {
		if _, err := engine.BuildDAGPlan(draft.Nodes, draft.Edges); err != nil {
			issues = append(issues, err.Error())
		}
	}
	return issues
}

func (h *AIHandler) parseAndValidateWorkflowJSON(jsonStr string) (map[string]interface{}, []string, bool) {
	var draft workflowDraft
	if err := json.Unmarshal([]byte(jsonStr), &draft); err != nil {
		return nil, []string{fmt.Sprintf("workflow JSON is invalid: %v", err)}, false
	}
	issues := h.validateWorkflowDraft(draft)
	var workflowData map[string]interface{}
	if err := json.Unmarshal([]byte(jsonStr), &workflowData); err != nil {
		return nil, []string{fmt.Sprintf("workflow JSON object is invalid: %v", err)}, false
	}
	return workflowData, issues, true
}

func isBlankParam(value interface{}) bool {
	switch v := value.(type) {
	case nil:
		return true
	case string:
		return strings.TrimSpace(v) == ""
	case []interface{}:
		return len(v) == 0
	case map[string]interface{}:
		return len(v) == 0
	default:
		return false
	}
}

func callChatCompletion(endpoint, apiKey, model string, messages []map[string]string, timeout time.Duration) (string, error) {
	apiReqBody := map[string]interface{}{
		"model":       model,
		"messages":    messages,
		"temperature": 0.2,
	}
	apiReqJSON, err := json.Marshal(apiReqBody)
	if err != nil {
		return "", err
	}

	client := &http.Client{Timeout: timeout}
	httpReq, err := http.NewRequest("POST", endpoint, bytes.NewBuffer(apiReqJSON))
	if err != nil {
		return "", err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+apiKey)

	resp, err := client.Do(httpReq)
	if err != nil {
		return "", fmt.Errorf("failed to call LLM API: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		var errResp map[string]interface{}
		_ = json.NewDecoder(resp.Body).Decode(&errResp)
		errBytes, _ := json.Marshal(errResp)
		return "", fmt.Errorf("LLM API returned error (HTTP %d): %s", resp.StatusCode, string(errBytes))
	}

	var apiResp struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&apiResp); err != nil {
		return "", fmt.Errorf("failed to parse LLM API response")
	}
	if len(apiResp.Choices) == 0 {
		return "", fmt.Errorf("LLM returned empty choices")
	}
	return strings.TrimSpace(apiResp.Choices[0].Message.Content), nil
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
1. If you propose a new workflow or suggest changes/fixes to the current workflow structure, write your explanation first, and then include the complete, updated workflow JSON structure inside a markdown code block starting with `+"`"+"``json"+"`"+` and ending with `+"`"+"```"+"`"+`.
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
5. The workflow JSON inside the markdown code block must begin with "{" and end with "}".`, defsFormatted, currentFlowContext)

	// 3. Select API Endpoint based on Credential Type or heuristics
	endpoint, model, ok := resolveAIProvider(cred, apiKey)
	if !ok {
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

	content, err := callChatCompletion(endpoint, apiKey, model, llmMessages, 45*time.Second)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	jsonStr, textStr := extractWorkflowJSON(content)

	responsePayload := map[string]interface{}{}

	if jsonStr != "" {
		sanitized := sanitizeJSONString(jsonStr)
		workflowData, validationIssues, parseOK := h.parseAndValidateWorkflowJSON(sanitized)
		if parseOK && len(validationIssues) > 0 {
			repairMessages := append([]map[string]string{}, llmMessages...)
			repairMessages = append(repairMessages,
				map[string]string{"role": "assistant", "content": content},
				map[string]string{
					"role": "user",
					"content": fmt.Sprintf(`The workflow JSON you returned is invalid for Goflow.
Fix every validation issue below and return the complete corrected workflow JSON in a single markdown json code block. Do not change the user's intent.

Validation issues:
- %s`, strings.Join(validationIssues, "\n- ")),
				},
			)
			if repairedContent, err := callChatCompletion(endpoint, apiKey, model, repairMessages, 45*time.Second); err == nil {
				repairedJSON, repairedText := extractWorkflowJSON(repairedContent)
				if repairedJSON != "" {
					workflowData, validationIssues, parseOK = h.parseAndValidateWorkflowJSON(sanitizeJSONString(repairedJSON))
					if parseOK {
						textStr = strings.TrimSpace(textStr + "\n" + repairedText)
					}
				}
			}
		}
		if parseOK && len(validationIssues) == 0 {
			responsePayload["type"] = "workflow"
			responsePayload["workflow"] = workflowData
			responsePayload["text"] = textStr
			responsePayload["validated"] = true

			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(responsePayload)
			return
		}
		if parseOK && len(validationIssues) > 0 {
			responsePayload["type"] = "text"
			responsePayload["text"] = fmt.Sprintf("AI generated a workflow, but it failed validation:\n- %s\n\nPlease refine your request or include the missing required values.", strings.Join(validationIssues, "\n- "))
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(responsePayload)
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
	endpoint, model, ok := resolveAIProvider(cred, apiKey)
	if !ok {
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
