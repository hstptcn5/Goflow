package mcpserver

import (
	"context"
	"fmt"
	"os"
	"strings"

	"goflow/internal/client"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type Options struct {
	BaseURL string
	APIKey  string
}

type Server struct {
	client *client.Client
}

func New(opts Options) *Server {
	return &Server{
		client: client.New(opts.BaseURL, opts.APIKey),
	}
}

func RunStdio(ctx context.Context, opts Options) error {
	server := New(opts)
	mcpServer := mcp.NewServer(&mcp.Implementation{
		Name:    "goflow",
		Version: readVersion(),
	}, nil)
	server.registerTools(mcpServer)
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
}

type listWorkflowsInput struct {
	ActiveOnly bool `json:"active_only,omitempty" jsonschema:"deprecated; MCP always lists only active exposed workflows"`
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
		if !workflow.ExposeMCP || !workflow.IsActive {
			continue
		}
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

type executionRefInput struct {
	ExecutionID string `json:"execution_id" jsonschema:"Goflow execution ID"`
}

type executionOutput struct {
	Execution client.Execution `json:"execution"`
}

func (s *Server) getExecution(ctx context.Context, req *mcp.CallToolRequest, input executionRefInput) (*mcp.CallToolResult, executionOutput, error) {
	exec, err := s.client.GetExecution(input.ExecutionID)
	if err != nil {
		return nil, executionOutput{}, err
	}
	return nil, executionOutput{Execution: *exec}, nil
}

type listExecutionsOutput struct {
	Workflow   client.Workflow    `json:"workflow"`
	Executions []client.Execution `json:"executions"`
	Count      int                `json:"count"`
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
	return nil, listExecutionsOutput{Workflow: *workflow, Executions: executions, Count: len(executions)}, nil
}

func (s *Server) resolveAllowedWorkflow(ref string) (*client.Workflow, error) {
	workflow, err := s.client.ResolveWorkflow(ref)
	if err != nil {
		return nil, err
	}
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
		BaseURL: envDefault("GOFLOW_URL", "http://127.0.0.1:8080"),
		APIKey:  os.Getenv("GOFLOW_API_KEY"),
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
	return nil
}
