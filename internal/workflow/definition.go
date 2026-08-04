package workflow

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"

	"goflow/internal/application"
	"goflow/internal/client"
	"goflow/internal/nodes"
)

func ReadFile(path string) (client.Workflow, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return client.Workflow{}, err
	}
	return ParseDefinition(data)
}

func ReadFileLimit(path string, maxBytes int64) (client.Workflow, error) {
	file, err := os.Open(path)
	if err != nil {
		return client.Workflow{}, err
	}
	defer file.Close()

	data, err := io.ReadAll(io.LimitReader(file, maxBytes+1))
	if err != nil {
		return client.Workflow{}, err
	}
	if int64(len(data)) > maxBytes {
		return client.Workflow{}, fmt.Errorf("workflow file exceeds %d byte limit", maxBytes)
	}
	return ParseDefinition(data)
}

func ParseDefinition(data []byte) (client.Workflow, error) {
	var raw struct {
		ID                string          `json:"id"`
		Name              string          `json:"name"`
		Description       string          `json:"description"`
		IsActive          bool            `json:"is_active"`
		NodesJSON         string          `json:"nodes_json"`
		EdgesJSON         string          `json:"edges_json"`
		Nodes             json.RawMessage `json:"nodes"`
		Edges             json.RawMessage `json:"edges"`
		Slug              string          `json:"slug"`
		InputSchemaJSON   json.RawMessage `json:"input_schema_json"`
		OutputSchemaJSON  json.RawMessage `json:"output_schema_json"`
		ExposeCLI         bool            `json:"expose_cli"`
		ExposeMCP         bool            `json:"expose_mcp"`
		MCPToolName       string          `json:"mcp_tool_name"`
		MCPDescription    string          `json:"mcp_description"`
		RiskLevel         string          `json:"risk_level"`
		RequiresApproval  bool            `json:"requires_approval"`
		MaxConcurrentRuns int             `json:"max_concurrent_runs"`
		ConcurrencyPolicy string          `json:"concurrency_policy"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return client.Workflow{}, fmt.Errorf("workflow file must be JSON: %w", err)
	}
	if strings.TrimSpace(raw.Name) == "" {
		return client.Workflow{}, fmt.Errorf("workflow name is required")
	}
	nodesJSON := raw.NodesJSON
	if nodesJSON == "" && len(raw.Nodes) > 0 {
		nodesJSON = string(raw.Nodes)
	}
	edgesJSON := raw.EdgesJSON
	if edgesJSON == "" && len(raw.Edges) > 0 {
		edgesJSON = string(raw.Edges)
	}
	if nodesJSON == "" {
		nodesJSON = "[]"
	}
	if edgesJSON == "" {
		edgesJSON = "[]"
	}
	if !isJSONArray(nodesJSON) {
		return client.Workflow{}, fmt.Errorf("nodes_json or nodes must be a JSON array")
	}
	if !isJSONArray(edgesJSON) {
		return client.Workflow{}, fmt.Errorf("edges_json or edges must be a JSON array")
	}
	return client.Workflow{
		ID:                raw.ID,
		Name:              raw.Name,
		Description:       raw.Description,
		IsActive:          raw.IsActive,
		NodesJSON:         nodesJSON,
		EdgesJSON:         edgesJSON,
		Slug:              raw.Slug,
		InputSchemaJSON:   rawJSONFieldToString(raw.InputSchemaJSON, "{}"),
		OutputSchemaJSON:  rawJSONFieldToString(raw.OutputSchemaJSON, "{}"),
		ExposeCLI:         raw.ExposeCLI,
		ExposeMCP:         raw.ExposeMCP,
		MCPToolName:       raw.MCPToolName,
		MCPDescription:    raw.MCPDescription,
		RiskLevel:         raw.RiskLevel,
		RequiresApproval:  raw.RequiresApproval,
		MaxConcurrentRuns: raw.MaxConcurrentRuns,
		ConcurrencyPolicy: raw.ConcurrencyPolicy,
	}, nil
}

func ValidateDefinition(workflow client.Workflow) error {
	if strings.TrimSpace(workflow.Name) == "" {
		return fmt.Errorf("workflow name is required")
	}
	var nodeList []nodes.Node
	if err := json.Unmarshal([]byte(workflow.NodesJSON), &nodeList); err != nil {
		return fmt.Errorf("nodes_json or nodes must be a JSON array")
	}
	var edgeList []nodes.Edge
	if err := json.Unmarshal([]byte(workflow.EdgesJSON), &edgeList); err != nil {
		return fmt.Errorf("edges_json or edges must be a JSON array")
	}
	if err := application.ValidateWorkflowSchema(workflow.InputSchemaJSON, "input_schema_json"); err != nil {
		return err
	}
	if err := application.ValidateWorkflowSchema(workflow.OutputSchemaJSON, "output_schema_json"); err != nil {
		return err
	}
	registry := nodes.NewBuiltinRegistry()
	ids := map[string]bool{}
	for _, node := range nodeList {
		if strings.TrimSpace(node.ID) == "" {
			return fmt.Errorf("node ID is required")
		}
		if ids[node.ID] {
			return fmt.Errorf("duplicate node ID %q", node.ID)
		}
		ids[node.ID] = true
		executor, ok := registry.Get(node.Type)
		if !ok {
			return fmt.Errorf("unknown node type %q on node %q", node.Type, node.ID)
		}
		for _, param := range executor.GetDefinition().Params {
			if !param.Required {
				continue
			}
			value, exists := node.Params[param.Name]
			if !exists || value == nil || strings.TrimSpace(fmt.Sprint(value)) == "" {
				return fmt.Errorf("node %q missing required parameter %q", node.ID, param.Name)
			}
		}
	}
	indegree := map[string]int{}
	outgoing := map[string][]string{}
	for id := range ids {
		indegree[id] = 0
	}
	for _, edge := range edgeList {
		if !ids[edge.Source] {
			return fmt.Errorf("edge %q references missing source node %q", edge.ID, edge.Source)
		}
		if !ids[edge.Target] {
			return fmt.Errorf("edge %q references missing target node %q", edge.ID, edge.Target)
		}
		outgoing[edge.Source] = append(outgoing[edge.Source], edge.Target)
		indegree[edge.Target]++
	}
	queue := make([]string, 0, len(indegree))
	for id, degree := range indegree {
		if degree == 0 {
			queue = append(queue, id)
		}
	}
	visited := 0
	for len(queue) > 0 {
		id := queue[0]
		queue = queue[1:]
		visited++
		for _, target := range outgoing[id] {
			indegree[target]--
			if indegree[target] == 0 {
				queue = append(queue, target)
			}
		}
	}
	if visited != len(nodeList) {
		return fmt.Errorf("workflow graph contains a cycle")
	}
	return nil
}

func isJSONArray(raw string) bool {
	var decoded []interface{}
	return json.Unmarshal([]byte(raw), &decoded) == nil
}

func rawJSONFieldToString(raw json.RawMessage, fallback string) string {
	if len(raw) == 0 {
		return fallback
	}
	var text string
	if err := json.Unmarshal(raw, &text); err == nil {
		if strings.TrimSpace(text) == "" {
			return fallback
		}
		return text
	}
	return string(raw)
}
