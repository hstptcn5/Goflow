package storage

import (
	"database/sql"
	"fmt"
	"strings"
	"time"
)

type migration struct {
	version int
	name    string
	up      func(tx *sql.Tx) error
}

// InitSchema runs versioned migrations for new and existing databases.
func (db *DB) InitSchema() error {
	return db.RunMigrations()
}

func (db *DB) RunMigrations() error {
	if _, err := db.WriteDB.Exec(`
		CREATE TABLE IF NOT EXISTS schema_migrations (
			version INTEGER PRIMARY KEY,
			name TEXT NOT NULL,
			applied_at DATETIME NOT NULL
		);
	`); err != nil {
		return fmt.Errorf("failed to create schema_migrations: %w", err)
	}
	if err := db.validateMigrationHistory(); err != nil {
		return err
	}

	for _, m := range migrations {
		applied, err := db.isMigrationApplied(m.version)
		if err != nil {
			return err
		}
		if applied {
			continue
		}

		tx, err := db.WriteDB.Begin()
		if err != nil {
			return fmt.Errorf("failed to start migration %04d: %w", m.version, err)
		}

		if err := m.up(tx); err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("migration %04d_%s failed: %w", m.version, m.name, err)
		}
		if _, err := tx.Exec(`INSERT INTO schema_migrations (version, name, applied_at) VALUES (?, ?, ?)`, m.version, m.name, time.Now()); err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("failed to record migration %04d: %w", m.version, err)
		}
		if err := tx.Commit(); err != nil {
			return fmt.Errorf("failed to commit migration %04d: %w", m.version, err)
		}
	}
	return nil
}

func (db *DB) validateMigrationHistory() error {
	expected := make(map[int]string, len(migrations))
	for _, m := range migrations {
		expected[m.version] = m.name
	}
	rows, err := db.WriteDB.Query(`SELECT version, name FROM schema_migrations ORDER BY version`)
	if err != nil {
		return fmt.Errorf("failed to inspect migration history: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var version int
		var name string
		if err := rows.Scan(&version, &name); err != nil {
			return fmt.Errorf("failed to read migration history: %w", err)
		}
		want, ok := expected[version]
		if !ok {
			return fmt.Errorf("database schema version %04d is newer than or unsupported by this Goflow runtime", version)
		}
		if name != want {
			return fmt.Errorf("database migration %04d has unexpected name %q", version, name)
		}
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("failed to inspect migration history: %w", err)
	}
	return nil
}

func (db *DB) isMigrationApplied(version int) (bool, error) {
	var n int
	err := db.WriteDB.QueryRow(`SELECT COUNT(1) FROM schema_migrations WHERE version = ?`, version).Scan(&n)
	if err != nil {
		return false, fmt.Errorf("failed to check migration %04d: %w", version, err)
	}
	return n > 0, nil
}

var migrations = []migration{
	{version: 1, name: "initial", up: migrationInitial},
	{version: 2, name: "workflow_interfaces", up: migrationWorkflowInterfaces},
	{version: 3, name: "execution_invocation", up: migrationExecutionInvocation},
	{version: 4, name: "access_tokens_audit", up: migrationAccessTokensAudit},
	{version: 5, name: "workflow_schedules", up: migrationWorkflowSchedules},
}

func migrationInitial(tx *sql.Tx) error {
	_, err := tx.Exec(`
		CREATE TABLE IF NOT EXISTS workflows (
			id TEXT PRIMARY KEY,
			name TEXT NOT NULL,
			description TEXT,
			is_active INTEGER NOT NULL DEFAULT 0,
			nodes_json TEXT NOT NULL DEFAULT '[]',
			edges_json TEXT NOT NULL DEFAULT '[]',
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
		);

		CREATE TABLE IF NOT EXISTS executions (
			id TEXT PRIMARY KEY,
			workflow_id TEXT NOT NULL,
			status TEXT NOT NULL,
			duration_ms INTEGER DEFAULT 0,
			logs_json TEXT NOT NULL DEFAULT '[]',
			started_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			finished_at DATETIME,
			FOREIGN KEY(workflow_id) REFERENCES workflows(id) ON DELETE CASCADE
		);

		CREATE TABLE IF NOT EXISTS credentials (
			id TEXT PRIMARY KEY,
			name TEXT NOT NULL,
			type TEXT NOT NULL,
			data_encrypted TEXT NOT NULL,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
		);

		CREATE INDEX IF NOT EXISTS idx_executions_workflow_status
			ON executions(workflow_id, status, started_at DESC);
		CREATE INDEX IF NOT EXISTS idx_executions_started_at
			ON executions(started_at DESC);
		CREATE INDEX IF NOT EXISTS idx_workflows_active
			ON workflows(is_active) WHERE is_active = 1;
		CREATE INDEX IF NOT EXISTS idx_credentials_type
			ON credentials(type, name);
	`)
	return err
}

func migrationWorkflowInterfaces(tx *sql.Tx) error {
	columns := map[string]string{
		"slug":                "TEXT",
		"input_schema_json":   "TEXT NOT NULL DEFAULT '{}'",
		"output_schema_json":  "TEXT NOT NULL DEFAULT '{}'",
		"expose_cli":          "INTEGER NOT NULL DEFAULT 1",
		"expose_mcp":          "INTEGER NOT NULL DEFAULT 0",
		"mcp_tool_name":       "TEXT",
		"mcp_description":     "TEXT",
		"risk_level":          "TEXT NOT NULL DEFAULT 'medium'",
		"requires_approval":   "INTEGER NOT NULL DEFAULT 0",
		"max_concurrent_runs": "INTEGER NOT NULL DEFAULT 0",
		"concurrency_policy":  "TEXT NOT NULL DEFAULT 'global'",
	}
	if err := ensureColumns(tx, "workflows", columns); err != nil {
		return err
	}
	_, err := tx.Exec(`
		CREATE UNIQUE INDEX IF NOT EXISTS idx_workflows_slug
			ON workflows(slug)
			WHERE slug IS NOT NULL;
		CREATE INDEX IF NOT EXISTS idx_workflows_mcp
			ON workflows(expose_mcp, is_active);
	`)
	return err
}

func migrationExecutionInvocation(tx *sql.Tx) error {
	columns := map[string]string{
		"trigger_source":    "TEXT",
		"trigger_principal": "TEXT",
		"request_id":        "TEXT",
		"idempotency_key":   "TEXT",
		"input_json":        "TEXT",
		"error_message":     "TEXT",
		"cancelled_at":      "DATETIME",
	}
	if err := ensureColumns(tx, "executions", columns); err != nil {
		return err
	}
	_, err := tx.Exec(`
		CREATE INDEX IF NOT EXISTS idx_executions_source
			ON executions(trigger_source, started_at DESC);
		CREATE UNIQUE INDEX IF NOT EXISTS idx_execution_idempotency
			ON executions(workflow_id, idempotency_key)
			WHERE idempotency_key IS NOT NULL;
	`)
	return err
}

func migrationAccessTokensAudit(tx *sql.Tx) error {
	_, err := tx.Exec(`
		CREATE TABLE IF NOT EXISTS access_tokens (
			id TEXT PRIMARY KEY,
			name TEXT NOT NULL,
			token_hash TEXT NOT NULL UNIQUE,
			scopes_json TEXT NOT NULL DEFAULT '[]',
			allowed_workflows_json TEXT NOT NULL DEFAULT '[]',
			created_at DATETIME NOT NULL,
			last_used_at DATETIME
		);

		CREATE INDEX IF NOT EXISTS idx_access_tokens_name
			ON access_tokens(name);

		CREATE TABLE IF NOT EXISTS audit_events (
			id TEXT PRIMARY KEY,
			event_type TEXT NOT NULL,
			subject TEXT,
			scope TEXT,
			workflow_id TEXT,
			execution_id TEXT,
			success INTEGER NOT NULL DEFAULT 0,
			message TEXT,
			created_at DATETIME NOT NULL
		);

		CREATE INDEX IF NOT EXISTS idx_audit_events_created_at
			ON audit_events(created_at DESC);
		CREATE INDEX IF NOT EXISTS idx_audit_events_workflow
			ON audit_events(workflow_id, created_at DESC);
		CREATE INDEX IF NOT EXISTS idx_audit_events_execution
			ON audit_events(execution_id, created_at DESC);
	`)
	return err
}

func migrationWorkflowSchedules(tx *sql.Tx) error {
	_, err := tx.Exec(`
		CREATE TABLE IF NOT EXISTS workflow_schedules (
			workflow_id TEXT PRIMARY KEY,
			pack_id TEXT NOT NULL,
			schema_version INTEGER NOT NULL,
			revision INTEGER NOT NULL DEFAULT 1,
			enabled INTEGER NOT NULL DEFAULT 0,
			kind TEXT NOT NULL,
			local_time TEXT NOT NULL,
			timezone TEXT NOT NULL,
			missed_run_policy TEXT NOT NULL,
			last_scheduled_for DATETIME,
			next_run_at DATETIME,
			last_execution_id TEXT,
			state TEXT NOT NULL,
			error_category TEXT,
			created_at DATETIME NOT NULL,
			updated_at DATETIME NOT NULL,
			FOREIGN KEY(workflow_id) REFERENCES workflows(id) ON DELETE CASCADE
		);

		CREATE INDEX IF NOT EXISTS idx_workflow_schedules_due
			ON workflow_schedules(enabled, next_run_at);
		CREATE INDEX IF NOT EXISTS idx_workflow_schedules_pack
			ON workflow_schedules(pack_id, workflow_id);
	`)
	return err
}

func ensureColumns(tx *sql.Tx, table string, columns map[string]string) error {
	existing, err := tableColumns(tx, table)
	if err != nil {
		return err
	}
	for name, definition := range columns {
		if existing[strings.ToLower(name)] {
			continue
		}
		if _, err := tx.Exec(fmt.Sprintf("ALTER TABLE %s ADD COLUMN %s %s", table, name, definition)); err != nil {
			return fmt.Errorf("failed to add column %s.%s: %w", table, name, err)
		}
	}
	return nil
}

func tableColumns(tx *sql.Tx, table string) (map[string]bool, error) {
	rows, err := tx.Query(fmt.Sprintf("PRAGMA table_info(%s)", table))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make(map[string]bool)
	for rows.Next() {
		var cid int
		var name, colType string
		var notNull int
		var defaultValue sql.NullString
		var pk int
		if err := rows.Scan(&cid, &name, &colType, &notNull, &defaultValue, &pk); err != nil {
			return nil, err
		}
		result[strings.ToLower(name)] = true
	}
	return result, rows.Err()
}
