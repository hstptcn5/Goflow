package storage

import (
	"database/sql"
	"time"

	"github.com/google/uuid"
)

type AuditEvent struct {
	ID          string    `json:"id"`
	EventType   string    `json:"event_type"`
	Subject     string    `json:"subject,omitempty"`
	Scope       string    `json:"scope,omitempty"`
	WorkflowID  string    `json:"workflow_id,omitempty"`
	ExecutionID string    `json:"execution_id,omitempty"`
	Success     bool      `json:"success"`
	Message     string    `json:"message,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
}

type AuditStore struct {
	db *DB
}

func NewAuditStore(db *DB) *AuditStore {
	return &AuditStore{db: db}
}

func (s *AuditStore) Record(event AuditEvent) error {
	if s == nil || s.db == nil {
		return nil
	}
	if event.ID == "" {
		event.ID = uuid.New().String()
	}
	if event.CreatedAt.IsZero() {
		event.CreatedAt = time.Now()
	}
	_, err := s.db.WriteDB.Exec(`
		INSERT INTO audit_events (
			id, event_type, subject, scope, workflow_id, execution_id, success, message, created_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, event.ID, event.EventType, nullableString(event.Subject), nullableString(event.Scope), nullableString(event.WorkflowID), nullableString(event.ExecutionID), boolToInt(event.Success), nullableString(event.Message), event.CreatedAt)
	return err
}

func (s *AuditStore) List(limit int) ([]AuditEvent, error) {
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	rows, err := s.db.ReadDB.Query(`
		SELECT id, event_type, subject, scope, workflow_id, execution_id, success, message, created_at
		FROM audit_events
		ORDER BY created_at DESC
		LIMIT ?
	`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []AuditEvent
	for rows.Next() {
		var event AuditEvent
		var subject, scope, workflowID, executionID, message sql.NullString
		var success int
		if err := rows.Scan(&event.ID, &event.EventType, &subject, &scope, &workflowID, &executionID, &success, &message, &event.CreatedAt); err != nil {
			return nil, err
		}
		event.Subject = subject.String
		event.Scope = scope.String
		event.WorkflowID = workflowID.String
		event.ExecutionID = executionID.String
		event.Success = success == 1
		event.Message = message.String
		result = append(result, event)
	}
	return result, rows.Err()
}
