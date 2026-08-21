package storage

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
)

const (
	maxStateKeyBytes   = 256
	maxStateValueBytes = 1 << 20
)

func init() {
	migrations = append(migrations, migration{version: 7, name: "workflow_state", up: migrationWorkflowState})
}

func migrationWorkflowState(tx *sql.Tx) error {
	_, err := tx.Exec(`
		CREATE TABLE IF NOT EXISTS workflow_state (
			scope TEXT NOT NULL,
			owner_id TEXT NOT NULL,
			state_key TEXT NOT NULL,
			value_json TEXT NOT NULL,
			created_at DATETIME NOT NULL,
			updated_at DATETIME NOT NULL,
			PRIMARY KEY(scope, owner_id, state_key)
		);
		CREATE INDEX IF NOT EXISTS idx_workflow_state_updated
			ON workflow_state(updated_at DESC);
	`)
	return err
}

type StateStore struct {
	db *DB
}

func NewStateStore(db *DB) *StateStore { return &StateStore{db: db} }

func normalizeStateAddress(scope, workflowID, key string) (string, string, string, error) {
	scope = strings.ToLower(strings.TrimSpace(scope))
	key = strings.TrimSpace(key)
	if key == "" {
		return "", "", "", fmt.Errorf("state key is required")
	}
	if len(key) > maxStateKeyBytes {
		return "", "", "", fmt.Errorf("state key exceeds %d byte limit", maxStateKeyBytes)
	}
	switch scope {
	case "", "workflow":
		if strings.TrimSpace(workflowID) == "" {
			return "", "", "", fmt.Errorf("workflow state requires a workflow id")
		}
		return "workflow", workflowID, key, nil
	case "global":
		return "global", "*", key, nil
	default:
		return "", "", "", fmt.Errorf("state scope must be Workflow or Global")
	}
}

func encodeStateValue(value interface{}) (string, error) {
	encoded, err := json.Marshal(value)
	if err != nil {
		return "", fmt.Errorf("state value could not be encoded: %w", err)
	}
	if len(encoded) > maxStateValueBytes {
		return "", fmt.Errorf("state value exceeds %d byte limit", maxStateValueBytes)
	}
	return string(encoded), nil
}

func (s *StateStore) Get(scope, workflowID, key string) (interface{}, bool, error) {
	scope, owner, key, err := normalizeStateAddress(scope, workflowID, key)
	if err != nil {
		return nil, false, err
	}
	var raw string
	err = s.db.ReadDB.QueryRow(`SELECT value_json FROM workflow_state WHERE scope = ? AND owner_id = ? AND state_key = ?`, scope, owner, key).Scan(&raw)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	var value interface{}
	if err := json.Unmarshal([]byte(raw), &value); err != nil {
		return nil, false, fmt.Errorf("stored state value is invalid JSON: %w", err)
	}
	return value, true, nil
}

func (s *StateStore) Set(scope, workflowID, key string, value interface{}) error {
	scope, owner, key, err := normalizeStateAddress(scope, workflowID, key)
	if err != nil {
		return err
	}
	raw, err := encodeStateValue(value)
	if err != nil {
		return err
	}
	now := time.Now()
	_, err = s.db.WriteDB.Exec(`
		INSERT INTO workflow_state(scope, owner_id, state_key, value_json, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?)
		ON CONFLICT(scope, owner_id, state_key) DO UPDATE SET value_json = excluded.value_json, updated_at = excluded.updated_at
	`, scope, owner, key, raw, now, now)
	return err
}

func (s *StateStore) Delete(scope, workflowID, key string) (bool, error) {
	scope, owner, key, err := normalizeStateAddress(scope, workflowID, key)
	if err != nil {
		return false, err
	}
	result, err := s.db.WriteDB.Exec(`DELETE FROM workflow_state WHERE scope = ? AND owner_id = ? AND state_key = ?`, scope, owner, key)
	if err != nil {
		return false, err
	}
	count, _ := result.RowsAffected()
	return count > 0, nil
}

func (s *StateStore) Increment(scope, workflowID, key string, delta float64) (float64, error) {
	scope, owner, key, err := normalizeStateAddress(scope, workflowID, key)
	if err != nil {
		return 0, err
	}
	initial, err := encodeStateValue(delta)
	if err != nil {
		return 0, err
	}

	// Increment in one SQLite write statement. The previous read-then-write
	// transaction could acquire a read snapshot and then fail to upgrade it to
	// a writer when the main execution store committed concurrently on another
	// connection (SQLITE_BUSY_SNAPSHOT). A single UPSERT lets SQLite serialize
	// writers using the configured busy timeout and keeps the increment atomic.
	var next float64
	now := time.Now()
	err = s.db.WriteDB.QueryRow(`
		INSERT INTO workflow_state(scope, owner_id, state_key, value_json, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?)
		ON CONFLICT(scope, owner_id, state_key) DO UPDATE SET
			value_json = CAST(CAST(workflow_state.value_json AS REAL) + ? AS TEXT),
			updated_at = excluded.updated_at
		WHERE json_type(workflow_state.value_json) IN ('integer', 'real')
		RETURNING CAST(value_json AS REAL)
	`, scope, owner, key, initial, now, now, delta).Scan(&next)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, fmt.Errorf("state value for %q is not numeric", key)
	}
	if err != nil {
		return 0, err
	}
	return next, nil
}

func stateNumber(value interface{}) (float64, bool) {
	switch typed := value.(type) {
	case float64:
		return typed, true
	case json.Number:
		v, err := typed.Float64()
		return v, err == nil
	case int:
		return float64(typed), true
	case int64:
		return float64(typed), true
	default:
		return 0, false
	}
}
