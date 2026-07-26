package storage

import (
	"database/sql"
	"errors"
	"time"

	"github.com/google/uuid"
)

type Workflow struct {
	ID                string    `json:"id"`
	Name              string    `json:"name"`
	Description       string    `json:"description"`
	IsActive          bool      `json:"is_active"`
	NodesJSON         string    `json:"nodes_json"`
	EdgesJSON         string    `json:"edges_json"`
	Slug              string    `json:"slug,omitempty"`
	InputSchemaJSON   string    `json:"input_schema_json"`
	OutputSchemaJSON  string    `json:"output_schema_json"`
	ExposeCLI         bool      `json:"expose_cli"`
	ExposeMCP         bool      `json:"expose_mcp"`
	MCPToolName       string    `json:"mcp_tool_name,omitempty"`
	MCPDescription    string    `json:"mcp_description,omitempty"`
	RiskLevel         string    `json:"risk_level"`
	RequiresApproval  bool      `json:"requires_approval"`
	MaxConcurrentRuns int       `json:"max_concurrent_runs"`
	ConcurrencyPolicy string    `json:"concurrency_policy"`
	CreatedAt         time.Time `json:"created_at"`
	UpdatedAt         time.Time `json:"updated_at"`
}

type WorkflowStore struct {
	db *DB
}

func NewWorkflowStore(db *DB) *WorkflowStore {
	return &WorkflowStore{db: db}
}

func (s *WorkflowStore) Create(wf *Workflow) error {
	if wf.ID == "" {
		wf.ID = uuid.New().String()
	}
	now := time.Now()
	wf.CreatedAt = now
	wf.UpdatedAt = now
	normalizeWorkflowInterfaceDefaults(wf)

	query := `
		INSERT INTO workflows (
			id, name, description, is_active, nodes_json, edges_json,
			slug, input_schema_json, output_schema_json, expose_cli, expose_mcp,
			mcp_tool_name, mcp_description, risk_level, requires_approval,
			max_concurrent_runs, concurrency_policy, created_at, updated_at
		)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`
	isActiveInt := 0
	if wf.IsActive {
		isActiveInt = 1
	}

	_, err := s.db.WriteDB.Exec(
		query,
		wf.ID,
		wf.Name,
		wf.Description,
		isActiveInt,
		wf.NodesJSON,
		wf.EdgesJSON,
		nullableString(wf.Slug),
		wf.InputSchemaJSON,
		wf.OutputSchemaJSON,
		boolToInt(wf.ExposeCLI),
		boolToInt(wf.ExposeMCP),
		nullableString(wf.MCPToolName),
		nullableString(wf.MCPDescription),
		wf.RiskLevel,
		boolToInt(wf.RequiresApproval),
		wf.MaxConcurrentRuns,
		wf.ConcurrencyPolicy,
		wf.CreatedAt,
		wf.UpdatedAt,
	)
	return err
}

func (s *WorkflowStore) Update(wf *Workflow) error {
	wf.UpdatedAt = time.Now()
	normalizeWorkflowInterfaceDefaults(wf)
	query := `
		UPDATE workflows
		SET
			name = ?, description = ?, is_active = ?, nodes_json = ?, edges_json = ?,
			slug = ?, input_schema_json = ?, output_schema_json = ?, expose_cli = ?, expose_mcp = ?,
			mcp_tool_name = ?, mcp_description = ?, risk_level = ?, requires_approval = ?,
			max_concurrent_runs = ?, concurrency_policy = ?, updated_at = ?
		WHERE id = ?
	`
	isActiveInt := 0
	if wf.IsActive {
		isActiveInt = 1
	}

	res, err := s.db.WriteDB.Exec(
		query,
		wf.Name,
		wf.Description,
		isActiveInt,
		wf.NodesJSON,
		wf.EdgesJSON,
		nullableString(wf.Slug),
		wf.InputSchemaJSON,
		wf.OutputSchemaJSON,
		boolToInt(wf.ExposeCLI),
		boolToInt(wf.ExposeMCP),
		nullableString(wf.MCPToolName),
		nullableString(wf.MCPDescription),
		wf.RiskLevel,
		boolToInt(wf.RequiresApproval),
		wf.MaxConcurrentRuns,
		wf.ConcurrencyPolicy,
		wf.UpdatedAt,
		wf.ID,
	)
	if err != nil {
		return err
	}
	affected, _ := res.RowsAffected()
	if affected == 0 {
		return errors.New("workflow not found")
	}
	return nil
}

func (s *WorkflowStore) ToggleActive(id string, isActive bool) error {
	query := `UPDATE workflows SET is_active = ?, updated_at = ? WHERE id = ?`
	isActiveInt := 0
	if isActive {
		isActiveInt = 1
	}
	_, err := s.db.WriteDB.Exec(query, isActiveInt, time.Now(), id)
	return err
}

func (s *WorkflowStore) GetByID(id string) (*Workflow, error) {
	query := `
		SELECT
			id, name, description, is_active, nodes_json, edges_json,
			slug, input_schema_json, output_schema_json, expose_cli, expose_mcp,
			mcp_tool_name, mcp_description, risk_level, requires_approval,
			max_concurrent_runs, concurrency_policy, created_at, updated_at
		FROM workflows WHERE id = ?
	`
	row := s.db.ReadDB.QueryRow(query, id)

	var wf Workflow
	err := scanWorkflow(row, &wf)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, errors.New("workflow not found")
		}
		return nil, err
	}
	return &wf, nil
}

func (s *WorkflowStore) ListAll() ([]Workflow, error) {
	query := `
		SELECT
			id, name, description, is_active, nodes_json, edges_json,
			slug, input_schema_json, output_schema_json, expose_cli, expose_mcp,
			mcp_tool_name, mcp_description, risk_level, requires_approval,
			max_concurrent_runs, concurrency_policy, created_at, updated_at
		FROM workflows ORDER BY updated_at DESC
	`
	rows, err := s.db.ReadDB.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []Workflow
	for rows.Next() {
		var wf Workflow
		if err := scanWorkflow(rows, &wf); err != nil {
			return nil, err
		}
		result = append(result, wf)
	}
	return result, nil
}

func (s *WorkflowStore) Delete(id string) error {
	query := `DELETE FROM workflows WHERE id = ?`
	_, err := s.db.WriteDB.Exec(query, id)
	return err
}

type workflowScanner interface {
	Scan(dest ...interface{}) error
}

func scanWorkflow(scanner workflowScanner, wf *Workflow) error {
	var isActiveInt, exposeCLIInt, exposeMCPInt, requiresApprovalInt int
	var slug, mcpToolName, mcpDescription sql.NullString
	err := scanner.Scan(
		&wf.ID,
		&wf.Name,
		&wf.Description,
		&isActiveInt,
		&wf.NodesJSON,
		&wf.EdgesJSON,
		&slug,
		&wf.InputSchemaJSON,
		&wf.OutputSchemaJSON,
		&exposeCLIInt,
		&exposeMCPInt,
		&mcpToolName,
		&mcpDescription,
		&wf.RiskLevel,
		&requiresApprovalInt,
		&wf.MaxConcurrentRuns,
		&wf.ConcurrencyPolicy,
		&wf.CreatedAt,
		&wf.UpdatedAt,
	)
	if err != nil {
		return err
	}
	wf.IsActive = isActiveInt == 1
	wf.Slug = slug.String
	wf.ExposeCLI = exposeCLIInt == 1
	wf.ExposeMCP = exposeMCPInt == 1
	wf.MCPToolName = mcpToolName.String
	wf.MCPDescription = mcpDescription.String
	wf.RequiresApproval = requiresApprovalInt == 1
	normalizeWorkflowInterfaceDefaults(wf)
	return nil
}

func normalizeWorkflowInterfaceDefaults(wf *Workflow) {
	if wf.InputSchemaJSON == "" {
		wf.InputSchemaJSON = "{}"
	}
	if wf.OutputSchemaJSON == "" {
		wf.OutputSchemaJSON = "{}"
	}
	if wf.RiskLevel == "" {
		wf.RiskLevel = "medium"
	}
	if wf.ConcurrencyPolicy == "" {
		wf.ConcurrencyPolicy = "global"
	}
}

func boolToInt(value bool) int {
	if value {
		return 1
	}
	return 0
}
