package client

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

type Client struct {
	BaseURL string
	APIKey  string
	HTTP    *http.Client
}

type Workflow struct {
	ID                string `json:"id"`
	Name              string `json:"name"`
	Description       string `json:"description"`
	IsActive          bool   `json:"is_active"`
	Slug              string `json:"slug,omitempty"`
	InputSchemaJSON   string `json:"input_schema_json"`
	OutputSchemaJSON  string `json:"output_schema_json"`
	ExposeCLI         bool   `json:"expose_cli"`
	ExposeMCP         bool   `json:"expose_mcp"`
	RiskLevel         string `json:"risk_level"`
	ConcurrencyPolicy string `json:"concurrency_policy"`
}

type Execution struct {
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
	InputJSON        string `json:"input_json,omitempty"`
	ErrorMessage     string `json:"error_message,omitempty"`
}

type ExecutionAccepted struct {
	ExecutionID  string `json:"execution_id"`
	WorkflowID   string `json:"workflow_id"`
	Status       string `json:"status"`
	Deduplicated bool   `json:"deduplicated"`
}

func New(baseURL, apiKey string) *Client {
	baseURL = strings.TrimRight(strings.TrimSpace(baseURL), "/")
	if baseURL == "" {
		baseURL = "http://127.0.0.1:8080"
	}
	return &Client{
		BaseURL: baseURL,
		APIKey:  strings.TrimSpace(apiKey),
		HTTP:    &http.Client{Timeout: 30 * time.Second},
	}
}

func (c *Client) ListWorkflows() ([]Workflow, error) {
	var workflows []Workflow
	if err := c.doJSON(http.MethodGet, "/api/v1/workflows", nil, &workflows); err != nil {
		return nil, err
	}
	if workflows == nil {
		workflows = []Workflow{}
	}
	return workflows, nil
}

func (c *Client) GetWorkflow(id string) (*Workflow, error) {
	var workflow Workflow
	if err := c.doJSON(http.MethodGet, "/api/v1/workflows/"+id, nil, &workflow); err != nil {
		return nil, err
	}
	return &workflow, nil
}

func (c *Client) ResolveWorkflow(ref string) (*Workflow, error) {
	ref = strings.TrimSpace(ref)
	if ref == "" {
		return nil, fmt.Errorf("workflow reference is required")
	}
	if workflow, err := c.GetWorkflow(ref); err == nil {
		return workflow, nil
	}
	workflows, err := c.ListWorkflows()
	if err != nil {
		return nil, err
	}
	for _, workflow := range workflows {
		if workflow.ID == ref || workflow.Slug == ref || strings.EqualFold(workflow.Name, ref) {
			w := workflow
			return &w, nil
		}
	}
	return nil, fmt.Errorf("workflow not found: %s", ref)
}

func (c *Client) RunWorkflow(workflowID string, input interface{}, idempotencyKey string) (*ExecutionAccepted, error) {
	body := map[string]interface{}{
		"mode":  "async",
		"input": input,
	}
	if idempotencyKey != "" {
		body["idempotency_key"] = idempotencyKey
	}
	var accepted ExecutionAccepted
	if err := c.doJSON(http.MethodPost, "/api/v1/workflows/"+workflowID+"/executions", body, &accepted); err != nil {
		return nil, err
	}
	return &accepted, nil
}

func (c *Client) GetExecution(id string) (*Execution, error) {
	var execution Execution
	if err := c.doJSON(http.MethodGet, "/api/v1/executions/"+id, nil, &execution); err != nil {
		return nil, err
	}
	return &execution, nil
}

func (c *Client) doJSON(method, path string, in interface{}, out interface{}) error {
	var body io.Reader
	if in != nil {
		data, err := json.Marshal(in)
		if err != nil {
			return err
		}
		body = bytes.NewReader(data)
	}

	req, err := http.NewRequest(method, c.BaseURL+path, body)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/json")
	if in != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if c.APIKey != "" {
		req.Header.Set("Authorization", "Bearer "+c.APIKey)
	}

	resp, err := c.HTTP.Do(req)
	if err != nil {
		return fmt.Errorf("cannot connect to Goflow at %s: %w", c.BaseURL, err)
	}
	defer resp.Body.Close()

	data, _ := io.ReadAll(resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		msg := strings.TrimSpace(string(data))
		if msg == "" {
			msg = resp.Status
		}
		return fmt.Errorf("Goflow API error (%d): %s", resp.StatusCode, msg)
	}
	if out == nil {
		return nil
	}
	if len(data) == 0 {
		return nil
	}
	if err := json.Unmarshal(data, out); err != nil {
		return fmt.Errorf("failed to parse Goflow API response: %w", err)
	}
	return nil
}
