package mcpserver

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"

	"goflow/internal/client"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type Options struct {
	BaseURL       string
	APIKey        string
	MaxInflight   int
	TriggerSource string
	RunInflight   chan struct{}
}

type Server struct {
	client      *client.Client
	runInflight chan struct{}
}

func New(opts Options) *Server {
	if opts.MaxInflight <= 0 {
		opts.MaxInflight = 2
	}
	runInflight := opts.RunInflight
	if runInflight == nil {
		runInflight = make(chan struct{}, opts.MaxInflight)
	}
	return &Server{
		client:      client.New(opts.BaseURL, opts.APIKey).WithTriggerSource(opts.TriggerSource),
		runInflight: runInflight,
	}
}

func RunStdio(ctx context.Context, opts Options) error {
	server := New(opts)
	mcpServer := mcp.NewServer(&mcp.Implementation{
		Name:    "goflow",
		Version: readVersion(),
	}, nil)
	server.registerTools(mcpServer)
	server.registerDynamicWorkflowTools(mcpServer)
	return mcpServer.Run(ctx, &mcp.StdioTransport{})
}

func (s *Server) registerTools(server *mcp.Server) {
	mcp.AddTool(server, &mcp.Tool{
		Name:        "goflow_list_workflows",
		Title:       "List Goflow workflows",
		Description: "List workflows available on the configured Goflow server.",
	}, s.listWorkflows)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "goflow_get_workflow",
		Title:       "Get Goflow workflow",
		Description: "Get workflow metadata by ID, slug, or exact name.",
	}, s.getWorkflow)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "goflow_run_workflow",
		Title:       "Run Goflow workflow",
		Description: "Start a workflow asynchronously through Goflow and return the execution ID.",
	}, s.runWorkflow)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "goflow_get_execution",
		Title:       "Get Goflow execution",
		Description: "Get execution status and redacted logs by execution ID.",
	}, s.getExecution)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "goflow_list_executions",
		Title:       "List Goflow executions",
		Description: "List recent executions for a workflow ID, slug, or exact name.",
	}, s.listExecutions)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "goflow_cancel_execution",
		Title:       "Cancel Goflow execution",
		Description: "Request cancellation for a running Goflow execution.",
	}, s.cancelExecution)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "goflow_reload_tools",
		Title:       "Reload Goflow MCP tools",
		Description: "Ask the MCP client to reconnect so dynamic workflow tools are reloaded from the current Goflow server state.",
	}, s.reloadTools)
}

func (s *Server) registerDynamicWorkflowTools(server *mcp.Server) {
	workflows, err := s.client.ListWorkflows()
	if err != nil {
		return
	}
	registered := map[string]bool{
		"goflow_list_workflows":   true,
		"goflow_get_workflow":     true,
		"goflow_run_workflow":     true,
		"goflow_get_execution":    true,
		"goflow_list_executions":  true,
		"goflow_cancel_execution": true,
		"goflow_reload_tools":     true,
	}
	for _, workflow := range workflows {
		workflow := s.workflowWithInterfaceMetadata(workflow)
		if !workflow.ExposeMCP || !workflow.IsActive || workflow.RequiresApproval {
			continue
		}
		toolName := dynamicToolName(workflow)
		if toolName == "" || registered[toolName] {
			continue
		}
		registered[toolName] = true
		server.AddTool(&mcp.Tool{
			Name:         toolName,
			Title:        workflow.Name,
			Description:  dynamicToolDescription(workflow),
			InputSchema:  dynamicWorkflowInputSchema(workflow.InputSchemaJSON),
			OutputSchema: dynamicWorkflowOutputSchema(),
		}, s.dynamicWorkflowHandler(workflow))
	}
}

type listWorkflowsInput struct {
	ActiveOnly bool `json:"active_only,omitempty" jsonschema:"deprecated; MCP always lists only active exposed workflows"`
}

type reloadToolsInput struct{}

type reloadToolsOutput struct {
	Status  string `json:"status"`
	Message string `json:"message"`
}

func (s *Server) reloadTools(ctx context.Context, req *mcp.CallToolRequest, input reloadToolsInput) (*mcp.CallToolResult, reloadToolsOutput, error) {
	return nil, reloadToolsOutput{
		Status:  "reconnect_required",
		Message: "Dynamic workflow tools are registered when the MCP session is created. Reconnect the MCP client after changing workflow MCP exposure, name, or schema.",
	}, nil
}

type listWorkflowsOutput struct {
	Workflows []client.Workflow `json:"workflows"`
	Count     int               `json:"count"`
}

func (s *Server) listWorkflows(ctx context.Context, req *mcp.CallToolRequest, input listWorkflowsInput) (*mcp.CallToolResult, listWorkflowsOutput, error) {
	workflows, err := s.client.ListWorkflows()
	if err != nil {
		return nil, listWorkflowsOutput{}, err
	}
	filtered := make([]client.Workflow, 0, len(workflows))
	for _, workflow := range workflows {
		workflow = s.workflowWithInterfaceMetadata(workflow)
		if !workflow.ExposeMCP || !workflow.IsActive {
			continue
		}
		workflow.NodesJSON = ""
		workflow.EdgesJSON = ""
		filtered = append(filtered, workflow)
	}
	return nil, listWorkflowsOutput{Workflows: filtered, Count: len(filtered)}, nil
}

type workflowRefInput struct {
	Workflow string `json:"workflow" jsonschema:"workflow ID, slug, or exact workflow name"`
}

type workflowOutput struct {
	Workflow client.Workflow `json:"workflow"`
}

func (s *Server) getWorkflow(ctx context.Context, req *mcp.CallToolRequest, input workflowRefInput) (*mcp.CallToolResult, workflowOutput, error) {
	workflow, err := s.resolveAllowedWorkflow(input.Workflow)
	if err != nil {
		return nil, workflowOutput{}, err
	}
	return nil, workflowOutput{Workflow: *workflow}, nil
}

type runWorkflowInput struct {
	Workflow       string                 `json:"workflow" jsonschema:"workflow ID, slug, or exact workflow name"`
	Input          map[string]interface{} `json:"input,omitempty" jsonschema:"workflow input payload object"`
	IdempotencyKey string                 `json:"idempotency_key,omitempty" jsonschema:"idempotency key to avoid duplicate side effects on retry"`
}

type runWorkflowOutput struct {
	ExecutionID  string `json:"execution_id"`
	WorkflowID   string `json:"workflow_id"`
	Status       string `json:"status"`
	Deduplicated bool   `json:"deduplicated"`
	StatusTool   string `json:"status_tool"`
}

func (s *Server) runWorkflow(ctx context.Context, req *mcp.CallToolRequest, input runWorkflowInput) (*mcp.CallToolResult, runWorkflowOutput, error) {
	release, err := s.acquireRunSlot(ctx)
	if err != nil {
		return nil, runWorkflowOutput{}, err
	}
	defer release()

	workflow, err := s.resolveAllowedWorkflow(input.Workflow)
	if err != nil {
		return nil, runWorkflowOutput{}, err
	}
	payload := input.Input
	if payload == nil {
		payload = map[string]interface{}{}
	}
	accepted, err := s.client.RunWorkflow(workflow.ID, payload, input.IdempotencyKey)
	if err != nil {
		return nil, runWorkflowOutput{}, err
	}
	return nil, runWorkflowOutput{
		ExecutionID:  accepted.ExecutionID,
		WorkflowID:   accepted.WorkflowID,
		Status:       accepted.Status,
		Deduplicated: accepted.Deduplicated,
		StatusTool:   "goflow_get_execution",
	}, nil
}

func (s *Server) dynamicWorkflowHandler(workflow client.Workflow) mcp.ToolHandler {
	return func(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		release, err := s.acquireRunSlot(ctx)
		if err != nil {
			return nil, err
		}
		defer release()

		var input map[string]interface{}
		if len(req.Params.Arguments) > 0 {
			if err := json.Unmarshal(req.Params.Arguments, &input); err != nil {
				return toolErrorResult("workflow input must be a JSON object: " + err.Error()), nil
			}
		}
		if input == nil {
			input = map[string]interface{}{}
		}
		idempotencyKey := ""
		if control, ok := input["_goflow"].(map[string]interface{}); ok {
			if value, ok := control["idempotency_key"].(string); ok {
				idempotencyKey = value
			}
			delete(input, "_goflow")
		}

		accepted, err := s.client.RunWorkflow(workflow.ID, input, idempotencyKey)
		if err != nil {
			return toolErrorResult(err.Error()), nil
		}
		output := runWorkflowOutput{
			ExecutionID:  accepted.ExecutionID,
			WorkflowID:   accepted.WorkflowID,
			Status:       accepted.Status,
			Deduplicated: accepted.Deduplicated,
			StatusTool:   "goflow_get_execution",
		}
		return toolSuccessResult(output), nil
	}
}

type executionRefInput struct {
	ExecutionID string `json:"execution_id" jsonschema:"Goflow execution ID"`
}

type executionOutput struct {
	Execution mcpExecution `json:"execution"`
}

func (s *Server) getExecution(ctx context.Context, req *mcp.CallToolRequest, input executionRefInput) (*mcp.CallToolResult, executionOutput, error) {
	exec, err := s.client.GetExecution(input.ExecutionID)
	if err != nil {
		return nil, executionOutput{}, err
	}
	return nil, executionOutput{Execution: executionForMCP(*exec)}, nil
}

type cancelExecutionOutput struct {
	Result client.ExecutionCancelResult `json:"result"`
}

func (s *Server) cancelExecution(ctx context.Context, req *mcp.CallToolRequest, input executionRefInput) (*mcp.CallToolResult, cancelExecutionOutput, error) {
	result, err := s.client.CancelExecution(input.ExecutionID)
	if err != nil {
		return nil, cancelExecutionOutput{}, err
	}
	return nil, cancelExecutionOutput{Result: *result}, nil
}

type listExecutionsOutput struct {
	Workflow   client.Workflow `json:"workflow"`
	Executions []mcpExecution  `json:"executions"`
	Count      int             `json:"count"`
}

func (s *Server) listExecutions(ctx context.Context, req *mcp.CallToolRequest, input workflowRefInput) (*mcp.CallToolResult, listExecutionsOutput, error) {
	workflow, err := s.resolveAllowedWorkflow(input.Workflow)
	if err != nil {
		return nil, listExecutionsOutput{}, err
	}
	executions, err := s.client.ListExecutions(workflow.ID)
	if err != nil {
		return nil, listExecutionsOutput{}, err
	}
	safeExecutions := make([]mcpExecution, 0, len(executions))
	for _, exec := range executions {
		safeExecutions = append(safeExecutions, executionForMCP(exec))
	}
	return nil, listExecutionsOutput{Workflow: *workflow, Executions: safeExecutions, Count: len(safeExecutions)}, nil
}

type mcpExecution struct {
	ID               string `json:"id"`
	WorkflowID       string `json:"workflow_id"`
	Status           string `json:"status"`
	DurationMs       int64  `json:"duration_ms"`
	LogsJSON         string `json:"logs_json"`
	StartedAt        string `json:"started_at"`
	FinishedAt       string `json:"finished_at,omitempty"`
	TriggerSource    string `json:"trigger_source,omitempty"`
	TriggerPrincipal string `json:"trigger_principal,omitempty"`
	RequestID        string `json:"request_id,omitempty"`
	IdempotencyKey   string `json:"idempotency_key,omitempty"`
	ErrorMessage     string `json:"error_message,omitempty"`
}

func executionForMCP(exec client.Execution) mcpExecution {
	return mcpExecution{
		ID:               exec.ID,
		WorkflowID:       exec.WorkflowID,
		Status:           exec.Status,
		DurationMs:       exec.DurationMs,
		LogsJSON:         exec.LogsJSON,
		StartedAt:        exec.StartedAt,
		FinishedAt:       exec.FinishedAt,
		TriggerSource:    exec.TriggerSource,
		TriggerPrincipal: exec.TriggerPrincipal,
		RequestID:        exec.RequestID,
		IdempotencyKey:   exec.IdempotencyKey,
		ErrorMessage:     exec.ErrorMessage,
	}
}

func (s *Server) resolveAllowedWorkflow(ref string) (*client.Workflow, error) {
	workflow, err := s.client.ResolveWorkflow(ref)
	if err != nil {
		return nil, err
	}
	*workflow = s.workflowWithInterfaceMetadata(*workflow)
	if !workflow.ExposeMCP {
		return nil, fmt.Errorf("workflow is not exposed to MCP")
	}
	if !workflow.IsActive {
		return nil, fmt.Errorf("workflow is inactive")
	}
	if workflow.RequiresApproval {
		return nil, fmt.Errorf("workflow requires approval and cannot be run through MCP alpha")
	}
	return workflow, nil
}

func (s *Server) workflowWithInterfaceMetadata(workflow client.Workflow) client.Workflow {
	if workflow.ID == "" {
		return workflow
	}
	iface, err := s.client.GetWorkflowInterface(workflow.ID)
	if err != nil {
		return workflow
	}
	workflow.Slug = firstNonEmpty(iface.Slug, workflow.Slug)
	workflow.InputSchemaJSON = firstNonEmpty(iface.InputSchemaJSON, workflow.InputSchemaJSON)
	workflow.OutputSchemaJSON = firstNonEmpty(iface.OutputSchemaJSON, workflow.OutputSchemaJSON)
	workflow.ExposeCLI = iface.ExposeCLI
	workflow.ExposeMCP = iface.ExposeMCP
	workflow.MCPToolName = firstNonEmpty(iface.MCPToolName, workflow.MCPToolName)
	workflow.MCPDescription = firstNonEmpty(iface.MCPDescription, workflow.MCPDescription)
	workflow.RiskLevel = firstNonEmpty(iface.RiskLevel, workflow.RiskLevel)
	workflow.RequiresApproval = iface.RequiresApproval
	workflow.MaxConcurrentRuns = iface.MaxConcurrentRuns
	workflow.ConcurrencyPolicy = firstNonEmpty(iface.ConcurrencyPolicy, workflow.ConcurrencyPolicy)
	return workflow
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

var toolNameUnsafeChars = regexp.MustCompile(`[^a-zA-Z0-9_-]+`)

func dynamicToolName(workflow client.Workflow) string {
	base := strings.TrimSpace(workflow.MCPToolName)
	if base == "" {
		base = strings.TrimSpace(workflow.Slug)
	}
	if base == "" {
		base = strings.TrimSpace(workflow.Name)
	}
	base = toolNameUnsafeChars.ReplaceAllString(base, "_")
	base = strings.Trim(base, "_-")
	if base == "" {
		return ""
	}
	return "goflow." + base
}

func dynamicToolDescription(workflow client.Workflow) string {
	if strings.TrimSpace(workflow.MCPDescription) != "" {
		return workflow.MCPDescription
	}
	if strings.TrimSpace(workflow.Description) != "" {
		return workflow.Description
	}
	return "Run the " + workflow.Name + " workflow asynchronously in Goflow."
}

func schemaOrEmptyObject(raw string) json.RawMessage {
	var decoded map[string]interface{}
	if err := json.Unmarshal([]byte(raw), &decoded); err == nil {
		if schemaType, ok := decoded["type"].(string); !ok {
			decoded["type"] = "object"
			data, _ := json.Marshal(decoded)
			return json.RawMessage(data)
		} else if schemaType == "object" {
			data, _ := json.Marshal(decoded)
			return json.RawMessage(data)
		}
	}
	return json.RawMessage(`{"type":"object"}`)
}

func dynamicWorkflowOutputSchema() json.RawMessage {
	return json.RawMessage(`{
		"type":"object",
		"properties":{
			"execution_id":{"type":"string"},
			"workflow_id":{"type":"string"},
			"status":{"type":"string"},
			"deduplicated":{"type":"boolean"},
			"status_tool":{"type":"string"}
		},
		"required":["execution_id","workflow_id","status","deduplicated","status_tool"],
		"additionalProperties":false
	}`)
}

func toolSuccessResult(output runWorkflowOutput) *mcp.CallToolResult {
	return &mcp.CallToolResult{
		Content:           []mcp.Content{&mcp.TextContent{Text: mustJSON(output)}},
		StructuredContent: output,
	}
}

func toolErrorResult(message string) *mcp.CallToolResult {
	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{Text: message},
		},
		IsError: true,
	}
}

func mustJSON(value interface{}) string {
	data, err := json.Marshal(value)
	if err != nil {
		return "{}"
	}
	return string(data)
}

func (s *Server) acquireRunSlot(ctx context.Context) (func(), error) {
	select {
	case s.runInflight <- struct{}{}:
		return func() { <-s.runInflight }, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
		return nil, fmt.Errorf("MCP per-client inflight limit reached")
	}
}

func readVersion() string {
	data, err := os.ReadFile("VERSION")
	if err != nil {
		return "dev"
	}
	version := strings.TrimSpace(string(data))
	if version == "" {
		return "dev"
	}
	return version
}

func OptionsFromEnv() Options {
	return Options{
		BaseURL:       envDefault("GOFLOW_URL", "http://127.0.0.1:8080"),
		APIKey:        os.Getenv("GOFLOW_API_KEY"),
		MaxInflight:   envInt("GOFLOW_MCP_MAX_INFLIGHT_PER_CLIENT", 2),
		TriggerSource: "mcp_stdio",
	}
}

func envDefault(name, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(name)); value != "" {
		return value
	}
	return fallback
}

func ValidateOptions(opts Options) error {
	if strings.TrimSpace(opts.BaseURL) == "" {
		return fmt.Errorf("GOFLOW_URL is required")
	}
	if opts.MaxInflight < 0 {
		return fmt.Errorf("GOFLOW_MCP_MAX_INFLIGHT_PER_CLIENT cannot be negative")
	}
	return nil
}

func envInt(name string, fallback int) int {
	raw := strings.TrimSpace(os.Getenv(name))
	if raw == "" {
		return fallback
	}
	value, err := strconv.Atoi(raw)
	if err != nil || value < 0 {
		return fallback
	}
	return value
}
