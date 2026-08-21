package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"goflow/internal/engine"
	"goflow/internal/nodes"
	"goflow/internal/storage"

	"github.com/google/uuid"
)

const (
	defaultAgentIterations = 2
	maxAgentIterations     = 3
)

type AIAgentHandler struct {
	credStore *storage.CredentialStore
	registry  *nodes.PluginRegistry
	wfStore   *storage.WorkflowStore
	engine    *engine.Engine
	validator *AIHandler
}

type aiAgentRequest struct {
	Goal         string                 `json:"goal"`
	CredentialID string                 `json:"credential_id"`
	Workflow     workflowDraft          `json:"workflow"`
	Execution    map[string]interface{} `json:"execution,omitempty"`
	MaxIterations int                  `json:"max_iterations,omitempty"`
}

type aiAgentModelProposal struct {
	Summary             string         `json:"summary"`
	ExpectedImprovement string         `json:"expected_improvement"`
	ProposedWorkflow    *workflowDraft `json:"proposed_workflow"`
}

type aiAgentResponse struct {
	Summary                  string         `json:"summary"`
	ExpectedImprovement      string         `json:"expected_improvement,omitempty"`
	ProposedWorkflow         *workflowDraft `json:"proposed_workflow,omitempty"`
	ProposalValidated        bool           `json:"proposal_validated"`
	ProposalValidationIssues []string       `json:"proposal_validation_issues,omitempty"`
	Iterations               int            `json:"iterations"`
	TestStatus               string         `json:"test_status"`
	TestBlockedReasons       []string       `json:"test_blocked_reasons,omitempty"`
	TestExecution            interface{}    `json:"test_execution,omitempty"`
	Provider                 string         `json:"provider"`
	Model                    string         `json:"model"`
}

func NewAIAgentHandler(cs *storage.CredentialStore, registry *nodes.PluginRegistry, wfStore *storage.WorkflowStore, eng *engine.Engine) *AIAgentHandler {
	return &AIAgentHandler{
		credStore: cs,
		registry:  registry,
		wfStore:   wfStore,
		engine:    eng,
		validator: NewAIHandler(cs, registry),
	}
}

func normalizeAgentIterations(value int) int {
	if value <= 0 {
		return defaultAgentIterations
	}
	if value > maxAgentIterations {
		return maxAgentIterations
	}
	return value
}

func (h *AIAgentHandler) compactAgentDefinitions() []map[string]interface{} {
	defs := h.registry.ListDefinitions()
	out := make([]map[string]interface{}, 0, len(defs))
	for _, def := range defs {
		params := make([]map[string]interface{}, 0, len(def.Params))
		for _, param := range def.Params {
			params = append(params, map[string]interface{}{
				"name":        param.Name,
				"type":        param.Type,
				"required":    param.Required,
				"default":     param.Default,
				"options":     param.Options,
				"description": param.Description,
			})
		}
		out = append(out, map[string]interface{}{
			"type":         def.Type,
			"name":         def.Name,
			"description":  def.Description,
			"params":       params,
			"capabilities": def.Capabilities,
		})
	}
	return out
}

var agentAutoTestAllowedTypes = map[nodes.NodeType]bool{
	nodes.TypeManualTrigger:  true,
	nodes.TypeCronTrigger:    true,
	nodes.TypeWebhookTrigger: true,
	nodes.TypeGithubWebhook:  true,
	nodes.TypeJSONTransform:  true,
	nodes.TypeConditionIF:    true,
	nodes.TypeSwitch:         true,
}

func agentAutoTestSafety(draft workflowDraft) (bool, []string) {
	var reasons []string
	seen := map[string]bool{}
	for _, node := range draft.Nodes {
		if agentAutoTestAllowedTypes[node.Type] {
			continue
		}
		reason := fmt.Sprintf("node %q (%s) cần quyền hoặc có thể tạo side effect nên Agent không tự chạy", node.ID, node.Type)
		if !seen[reason] {
			seen[reason] = true
			reasons = append(reasons, reason)
		}
	}
	return len(reasons) == 0, reasons
}

func (h *AIAgentHandler) buildAgentMessages(goal string, current workflowDraft, execution interface{}, feedback string) []map[string]string {
	feedback = strings.TrimSpace(feedback)
	if feedback == "" {
		feedback = "Chưa có feedback từ vòng trước."
	}
	systemPrompt := `Bạn là Goflow Workflow Agent Lab. Nhiệm vụ của bạn là cải thiện một workflow draft theo mục tiêu người dùng và bằng chứng execution có sẵn.

Bạn được cung cấp toàn bộ node registry hiện tại. Chỉ dùng node type và parameter name có trong registry. Không bịa node hoặc output contract. Không bao giờ đưa plaintext secret, API key, token, password, Authorization header, credential id hoặc private key vào workflow proposal. Giữ nguyên credential hiện có bằng cách bỏ credential parameter khỏi proposal khi không cần thay đổi.

Đây là Agent Lab có giới hạn, không phải quyền production. Goflow sẽ tự quyết định proposal nào đủ an toàn để test-run. Bạn không được tuyên bố rằng workflow đã được lưu, kích hoạt hoặc chạy production.

Ưu tiên:
- deterministic node trước AI khi hợp lý;
- workflow nhỏ, rõ, dễ debug;
- xử lý lỗi và output shape chính xác;
- giữ node id hiện tại khi node vẫn cùng vai trò;
- nếu execution cho thấy lỗi, sửa nguyên nhân cụ thể thay vì viết lại toàn bộ không cần thiết.

Trả đúng một JSON object:
{
  "summary": "đã thay đổi gì",
  "expected_improvement": "lợi ích dự kiến",
  "proposed_workflow": {
    "name": "tên workflow",
    "nodes": [],
    "edges": []
  }
}`
	userPrompt := fmt.Sprintf(`Mục tiêu người dùng:\n%s\n\nWorkflow draft hiện tại:\n%s\n\nExecution/bằng chứng gần nhất (đã redacted, có thể null):\n%s\n\nFeedback từ validation hoặc test vòng trước:\n%s\n\nToàn bộ node definitions hiện tại:\n%s\n\nHãy trả proposal hoàn chỉnh cho vòng tiếp theo.`,
		strings.TrimSpace(engine.RedactSensitiveString(goal)),
		boundedReviewJSON(current),
		boundedReviewJSON(execution),
		engine.RedactSensitiveString(feedback),
		boundedReviewJSON(h.compactAgentDefinitions()),
	)
	return []map[string]string{
		{"role": "system", "content": systemPrompt},
		{"role": "user", "content": userPrompt},
	}
}

func parseAgentProposal(content string) (aiAgentModelProposal, error) {
	var proposal aiAgentModelProposal
	cleaned := sanitizeJSONString(strings.TrimSpace(content))
	if err := json.Unmarshal([]byte(cleaned), &proposal); err != nil {
		return aiAgentModelProposal{}, fmt.Errorf("Agent trả JSON không hợp lệ: %w", err)
	}
	proposal.Summary = strings.TrimSpace(proposal.Summary)
	proposal.ExpectedImprovement = strings.TrimSpace(proposal.ExpectedImprovement)
	if proposal.Summary == "" {
		return aiAgentModelProposal{}, fmt.Errorf("Agent trả summary rỗng")
	}
	if proposal.ProposedWorkflow == nil {
		return aiAgentModelProposal{}, fmt.Errorf("Agent không trả proposed_workflow")
	}
	return proposal, nil
}

func (h *AIAgentHandler) runSafeDraftTest(draft workflowDraft) (interface{}, error) {
	nodesJSON, err := json.Marshal(draft.Nodes)
	if err != nil {
		return nil, err
	}
	edgesJSON, err := json.Marshal(draft.Edges)
	if err != nil {
		return nil, err
	}
	tempID := "agent-test-" + uuid.NewString()
	temp := &storage.Workflow{
		ID:               tempID,
		Name:             "Agent Lab test",
		Description:      "Ephemeral Agent Lab workflow",
		IsActive:         false,
		NodesJSON:        string(nodesJSON),
		EdgesJSON:        string(edgesJSON),
		ExposeCLI:        false,
		ExposeMCP:        false,
		RiskLevel:        "low",
		RequiresApproval: false,
	}
	if err := h.wfStore.Create(temp); err != nil {
		return nil, fmt.Errorf("không tạo được workflow test tạm: %w", err)
	}
	defer func() { _ = h.wfStore.Delete(tempID) }()

	execution, execErr := h.engine.ExecuteWorkflowWithOptions(temp, map[string]interface{}{"agent_test": true}, engine.TriggerOptions{
		Source:    "ai-agent-test",
		Principal: "goflow-agent-lab",
	})
	if execution == nil {
		return nil, execErr
	}
	redacted := reviewJSONValue(execution)
	if execErr != nil {
		return redacted, execErr
	}
	if strings.ToUpper(strings.TrimSpace(execution.Status)) != "SUCCESS" {
		return redacted, fmt.Errorf("agent test kết thúc với status %s", execution.Status)
	}
	return redacted, nil
}

func (h *AIAgentHandler) Iterate(w http.ResponseWriter, r *http.Request) {
	if h.credStore == nil || h.registry == nil || h.wfStore == nil || h.engine == nil {
		http.Error(w, "Agent Lab chưa được cấu hình", http.StatusServiceUnavailable)
		return
	}
	var req aiAgentRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Request không hợp lệ", http.StatusBadRequest)
		return
	}
	req.Goal = strings.TrimSpace(req.Goal)
	if req.Goal == "" || strings.TrimSpace(req.CredentialID) == "" {
		http.Error(w, "goal và credential_id là bắt buộc", http.StatusBadRequest)
		return
	}
	if len(req.Workflow.Nodes) == 0 {
		http.Error(w, "workflow phải có ít nhất một node", http.StatusBadRequest)
		return
	}
	if issues := h.validator.validateWorkflowDraft(req.Workflow); len(issues) > 0 {
		renderJSON(w, http.StatusBadRequest, aiAgentResponse{
			Summary:                  "Workflow hiện tại chưa hợp lệ để Agent Lab bắt đầu.",
			ProposalValidationIssues: issues,
			TestStatus:               "not_run",
		})
		return
	}

	cred, err := h.credStore.GetByID(req.CredentialID)
	if err != nil {
		http.Error(w, "Credential không tồn tại", http.StatusBadRequest)
		return
	}
	apiKey, err := h.credStore.GetDecryptedData(req.CredentialID)
	if err != nil {
		http.Error(w, "Không đọc được credential", http.StatusInternalServerError)
		return
	}
	endpoint, model, provider, ok := strictAIReviewProvider(cred)
	if !ok {
		http.Error(w, "Agent Lab chỉ hỗ trợ credential OpenAI hoặc DeepSeek có provider rõ ràng", http.StatusBadRequest)
		return
	}

	iterations := normalizeAgentIterations(req.MaxIterations)
	current := cloneWorkflowDraft(req.Workflow)
	var evidence interface{} = req.Execution
	feedback := ""
	var lastProposal aiAgentModelProposal
	var lastIssues []string
	var lastTest interface{}
	var lastTestErr error

	for iteration := 1; iteration <= iterations; iteration++ {
		content, err := callStructuredChatCompletion(endpoint, apiKey, model, provider, h.buildAgentMessages(req.Goal, current, evidence, feedback), 60*time.Second)
		if err != nil {
			http.Error(w, fmt.Sprintf("Agent Lab không gọi được model: %v", err), http.StatusBadGateway)
			return
		}
		proposal, err := parseAgentProposal(content)
		if err != nil {
			feedback = err.Error()
			lastTestErr = err
			continue
		}
		lastProposal = proposal
		issues, _ := h.validator.validateReviewProposal(current, proposal.ProposedWorkflow, nil)
		lastIssues = issues
		if len(issues) > 0 {
			feedback = "Proposal chưa qua validation:\n- " + strings.Join(issues, "\n- ") + "\nProposal bị từ chối: " + boundedReviewJSON(proposal.ProposedWorkflow)
			continue
		}

		safe, blocked := agentAutoTestSafety(*proposal.ProposedWorkflow)
		if !safe {
			renderJSON(w, http.StatusOK, aiAgentResponse{
				Summary:             proposal.Summary,
				ExpectedImprovement: proposal.ExpectedImprovement,
				ProposedWorkflow:    proposal.ProposedWorkflow,
				ProposalValidated:   true,
				Iterations:          iteration,
				TestStatus:          "blocked",
				TestBlockedReasons:  blocked,
				Provider:            provider,
				Model:               model,
			})
			return
		}

		lastTest, lastTestErr = h.runSafeDraftTest(*proposal.ProposedWorkflow)
		if lastTestErr == nil {
			renderJSON(w, http.StatusOK, aiAgentResponse{
				Summary:             proposal.Summary,
				ExpectedImprovement: proposal.ExpectedImprovement,
				ProposedWorkflow:    proposal.ProposedWorkflow,
				ProposalValidated:   true,
				Iterations:          iteration,
				TestStatus:          "passed",
				TestExecution:       lastTest,
				Provider:            provider,
				Model:               model,
			})
			return
		}

		current = cloneWorkflowDraft(*proposal.ProposedWorkflow)
		evidence = lastTest
		feedback = "Safe test-run thất bại: " + engine.RedactSensitiveString(lastTestErr.Error())
	}

	response := aiAgentResponse{
		Summary:                  "Agent Lab chưa tạo được proposal vượt qua toàn bộ gate trong số vòng cho phép.",
		ProposalValidationIssues: lastIssues,
		Iterations:               iterations,
		TestStatus:               "failed",
		TestExecution:            lastTest,
		Provider:                 provider,
		Model:                    model,
	}
	if lastProposal.ProposedWorkflow != nil && len(lastIssues) == 0 {
		response.Summary = lastProposal.Summary
		response.ExpectedImprovement = lastProposal.ExpectedImprovement
		response.ProposedWorkflow = lastProposal.ProposedWorkflow
		response.ProposalValidated = true
	}
	if lastTestErr != nil && response.TestExecution == nil {
		response.TestExecution = map[string]interface{}{"error": engine.RedactSensitiveString(lastTestErr.Error())}
	}
	renderJSON(w, http.StatusOK, response)
}
