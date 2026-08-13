package storage

import (
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

const (
	DefaultExecutionRetentionDays = 30
	MinExecutionRetentionDays     = 1
	MaxExecutionRetentionDays     = 365
	DefaultExecutionsPerWorkflow  = 1000
	MinExecutionsPerWorkflow      = 1
	MaxExecutionsPerWorkflow      = 10000
)

type Execution struct {
	ID               string     `json:"id"`
	WorkflowID       string     `json:"workflow_id"`
	Status           string     `json:"status"` // 'RUNNING', 'SUCCESS', 'FAILED', 'INTERRUPTED'
	DurationMs       int64      `json:"duration_ms"`
	LogsJSON         string     `json:"logs_json"`
	StartedAt        time.Time  `json:"started_at"`
	FinishedAt       *time.Time `json:"finished_at,omitempty"`
	TriggerSource    string     `json:"trigger_source,omitempty"`
	TriggerPrincipal string     `json:"trigger_principal,omitempty"`
	RequestID        string     `json:"request_id,omitempty"`
	IdempotencyKey   string     `json:"idempotency_key,omitempty"`
	InputJSON        string     `json:"input_json,omitempty"`
	ErrorMessage     string     `json:"error_message,omitempty"`
	CancelledAt      *time.Time `json:"cancelled_at,omitempty"`
}

func IsExecutionIdempotencyConflict(err error) bool {
	if err == nil {
		return false
	}
	message := strings.ToLower(err.Error())
	return strings.Contains(message, "idx_execution_idempotency") ||
		(strings.Contains(message, "constraint") && strings.Contains(message, "idempotency_key"))
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
		INSERT INTO executions (
			id, workflow_id, status, duration_ms, logs_json, started_at,
			trigger_source, trigger_principal, request_id, idempotency_key, input_json
		)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`
	_, err := s.db.WriteDB.Exec(
		query,
		exec.ID,
		exec.WorkflowID,
		exec.Status,
		exec.DurationMs,
		exec.LogsJSON,
		exec.StartedAt,
		nullableString(exec.TriggerSource),
		nullableString(exec.TriggerPrincipal),
		nullableString(exec.RequestID),
		nullableString(exec.IdempotencyKey),
		nullableString(exec.InputJSON),
	)
	return err
}

func (s *ExecutionStore) UpdateStatus(id string, status string, durationMs int64, logsJSON string) error {
	return s.UpdateStatusWithError(id, status, durationMs, logsJSON, "")
}

func (s *ExecutionStore) UpdateStatusWithError(id string, status string, durationMs int64, logsJSON string, errorMessage string) error {
	now := time.Now()
	cancelledAt := interface{}(nil)
	if status == "CANCELLED" {
		cancelledAt = now
	}
	query := `
		UPDATE executions
		SET status = ?, duration_ms = ?, logs_json = ?, finished_at = ?, error_message = ?, cancelled_at = COALESCE(cancelled_at, ?)
		WHERE id = ?
	`
	_, err := s.db.WriteDB.Exec(query, status, durationMs, logsJSON, now, nullableString(errorMessage), cancelledAt, id)
	return err
}

func (s *ExecutionStore) GetByID(id string) (*Execution, error) {
	query := `
		SELECT
			id, workflow_id, status, duration_ms, logs_json, started_at, finished_at,
			trigger_source, trigger_principal, request_id, idempotency_key, input_json,
			error_message, cancelled_at
		FROM executions WHERE id = ?
	`
	row := s.db.ReadDB.QueryRow(query, id)

	var exec Execution
	var finishedAt sql.NullTime
	var cancelledAt sql.NullTime
	var triggerSource, triggerPrincipal, requestID, idempotencyKey, inputJSON, errorMessage sql.NullString
	err := row.Scan(
		&exec.ID,
		&exec.WorkflowID,
		&exec.Status,
		&exec.DurationMs,
		&exec.LogsJSON,
		&exec.StartedAt,
		&finishedAt,
		&triggerSource,
		&triggerPrincipal,
		&requestID,
		&idempotencyKey,
		&inputJSON,
		&errorMessage,
		&cancelledAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, errors.New("execution log not found")
		}
		return nil, err
	}
	if finishedAt.Valid {
		exec.FinishedAt = &finishedAt.Time
	}
	if cancelledAt.Valid {
		exec.CancelledAt = &cancelledAt.Time
	}
	exec.TriggerSource = triggerSource.String
	exec.TriggerPrincipal = triggerPrincipal.String
	exec.RequestID = requestID.String
	exec.IdempotencyKey = idempotencyKey.String
	exec.InputJSON = inputJSON.String
	exec.ErrorMessage = errorMessage.String
	return &exec, nil
}

func (s *ExecutionStore) GetByIdempotencyKey(workflowID, key string) (*Execution, error) {
	if key == "" {
		return nil, errors.New("idempotency key is required")
	}
	query := `
		SELECT id
		FROM executions
		WHERE workflow_id = ? AND idempotency_key = ?
		ORDER BY started_at DESC
		LIMIT 1
	`
	var id string
	err := s.db.ReadDB.QueryRow(query, workflowID, key).Scan(&id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, errors.New("execution log not found")
		}
		return nil, err
	}
	return s.GetByID(id)
}

func (s *ExecutionStore) ListByWorkflow(workflowID string, limit int) ([]Execution, error) {
	if limit <= 0 {
		limit = 50
	}
	query := `
		SELECT
			id, workflow_id, status, duration_ms, logs_json, started_at, finished_at,
			trigger_source, trigger_principal, request_id, idempotency_key, input_json,
			error_message, cancelled_at
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
		var cancelledAt sql.NullTime
		var triggerSource, triggerPrincipal, requestID, idempotencyKey, inputJSON, errorMessage sql.NullString
		if err := rows.Scan(
			&exec.ID,
			&exec.WorkflowID,
			&exec.Status,
			&exec.DurationMs,
			&exec.LogsJSON,
			&exec.StartedAt,
			&finishedAt,
			&triggerSource,
			&triggerPrincipal,
			&requestID,
			&idempotencyKey,
			&inputJSON,
			&errorMessage,
			&cancelledAt,
		); err != nil {
			return nil, err
		}
		if finishedAt.Valid {
			exec.FinishedAt = &finishedAt.Time
		}
		if cancelledAt.Valid {
			exec.CancelledAt = &cancelledAt.Time
		}
		exec.TriggerSource = triggerSource.String
		exec.TriggerPrincipal = triggerPrincipal.String
		exec.RequestID = requestID.String
		exec.IdempotencyKey = idempotencyKey.String
		exec.InputJSON = inputJSON.String
		exec.ErrorMessage = errorMessage.String
		result = append(result, exec)
	}
	return result, nil
}

func (s *ExecutionStore) MarkRunningInterrupted() (int64, error) {
	now := time.Now()
	query := `
		UPDATE executions
		SET status = 'INTERRUPTED', finished_at = ?
		WHERE status = 'RUNNING'
	`
	res, err := s.db.WriteDB.Exec(query, now)
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}

func (s *ExecutionStore) Cleanup(retentionDays int, maxPerWorkflow int) (int64, error) {
	if retentionDays < MinExecutionRetentionDays || retentionDays > MaxExecutionRetentionDays {
		return 0, fmt.Errorf("execution retention days must be between %d and %d", MinExecutionRetentionDays, MaxExecutionRetentionDays)
	}
	if maxPerWorkflow < MinExecutionsPerWorkflow || maxPerWorkflow > MaxExecutionsPerWorkflow {
		return 0, fmt.Errorf("executions per workflow must be between %d and %d", MinExecutionsPerWorkflow, MaxExecutionsPerWorkflow)
	}
	tx, err := s.db.WriteDB.Begin()
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()
	var total int64
	cutoff := time.Now().AddDate(0, 0, -retentionDays)
	res, err := tx.Exec(`DELETE FROM executions WHERE status != 'RUNNING' AND started_at < ?`, cutoff)
	if err != nil {
		return 0, err
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return 0, err
	}
	total += affected
	res, err = tx.Exec(`
			DELETE FROM executions
			WHERE status != 'RUNNING' AND id IN (
				SELECT id FROM (
					SELECT id,
						ROW_NUMBER() OVER (PARTITION BY workflow_id ORDER BY started_at DESC) AS rn
					FROM executions
				)
				WHERE rn > ?
			)
		`, maxPerWorkflow)
	if err != nil {
		return 0, err
	}
	affected, err = res.RowsAffected()
	if err != nil {
		return 0, err
	}
	total += affected
	if err := tx.Commit(); err != nil {
		return 0, err
	}
	return total, nil
}

func nullableString(value string) interface{} {
	if value == "" {
		return nil
	}
	return value
}
