package storage

import (
	"database/sql"
	"errors"
	"time"

	"github.com/google/uuid"
)

type Execution struct {
	ID          string     `json:"id"`
	WorkflowID  string     `json:"workflow_id"`
	Status      string     `json:"status"` // 'RUNNING', 'SUCCESS', 'FAILED'
	DurationMs  int64      `json:"duration_ms"`
	LogsJSON    string     `json:"logs_json"`
	StartedAt   time.Time  `json:"started_at"`
	FinishedAt  *time.Time `json:"finished_at,omitempty"`
}

type ExecutionStore struct {
	db *DB
}

func NewExecutionStore(db *DB) *ExecutionStore {
	return &ExecutionStore{db: db}
}

func (s *ExecutionStore) Create(exec *Execution) error {
	if exec.ID == "" {
		exec.ID = uuid.New().String()
	}
	exec.StartedAt = time.Now()
	query := `
		INSERT INTO executions (id, workflow_id, status, duration_ms, logs_json, started_at)
		VALUES (?, ?, ?, ?, ?, ?)
	`
	_, err := s.db.WriteDB.Exec(query, exec.ID, exec.WorkflowID, exec.Status, exec.DurationMs, exec.LogsJSON, exec.StartedAt)
	return err
}

func (s *ExecutionStore) UpdateStatus(id string, status string, durationMs int64, logsJSON string) error {
	now := time.Now()
	query := `
		UPDATE executions
		SET status = ?, duration_ms = ?, logs_json = ?, finished_at = ?
		WHERE id = ?
	`
	_, err := s.db.WriteDB.Exec(query, status, durationMs, logsJSON, now, id)
	return err
}

func (s *ExecutionStore) GetByID(id string) (*Execution, error) {
	query := `
		SELECT id, workflow_id, status, duration_ms, logs_json, started_at, finished_at
		FROM executions WHERE id = ?
	`
	row := s.db.ReadDB.QueryRow(query, id)

	var exec Execution
	var finishedAt sql.NullTime
	err := row.Scan(&exec.ID, &exec.WorkflowID, &exec.Status, &exec.DurationMs, &exec.LogsJSON, &exec.StartedAt, &finishedAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, errors.New("execution log not found")
		}
		return nil, err
	}
	if finishedAt.Valid {
		exec.FinishedAt = &finishedAt.Time
	}
	return &exec, nil
}

func (s *ExecutionStore) ListByWorkflow(workflowID string, limit int) ([]Execution, error) {
	if limit <= 0 {
		limit = 50
	}
	query := `
		SELECT id, workflow_id, status, duration_ms, logs_json, started_at, finished_at
		FROM executions
		WHERE workflow_id = ?
		ORDER BY started_at DESC
		LIMIT ?
	`
	rows, err := s.db.ReadDB.Query(query, workflowID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []Execution
	for rows.Next() {
		var exec Execution
		var finishedAt sql.NullTime
		if err := rows.Scan(&exec.ID, &exec.WorkflowID, &exec.Status, &exec.DurationMs, &exec.LogsJSON, &exec.StartedAt, &finishedAt); err != nil {
			return nil, err
		}
		if finishedAt.Valid {
			exec.FinishedAt = &finishedAt.Time
		}
		result = append(result, exec)
	}
	return result, nil
}
