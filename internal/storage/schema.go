package storage

import (
	"fmt"
)

// InitSchema tạo các bảng cơ sở dữ liệu và chỉ mục cần thiết cho Goflow
func (db *DB) InitSchema() error {
	schemaSQL := `
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
		status TEXT NOT NULL, -- 'RUNNING', 'SUCCESS', 'FAILED'
		duration_ms INTEGER DEFAULT 0,
		logs_json TEXT NOT NULL DEFAULT '[]',
		started_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		finished_at DATETIME,
		FOREIGN KEY(workflow_id) REFERENCES workflows(id) ON DELETE CASCADE
	);

	CREATE TABLE IF NOT EXISTS credentials (
		id TEXT PRIMARY KEY,
		name TEXT NOT NULL,
		type TEXT NOT NULL, -- 'API_KEY', 'TELEGRAM_BOT', 'BEARER_TOKEN', 'BASIC_AUTH'
		data_encrypted TEXT NOT NULL,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);

	-- Optimized Indexes
	CREATE INDEX IF NOT EXISTS idx_executions_workflow_status 
		ON executions(workflow_id, status, started_at DESC);

	CREATE INDEX IF NOT EXISTS idx_workflows_active 
		ON workflows(is_active) WHERE is_active = 1;

	CREATE INDEX IF NOT EXISTS idx_credentials_type 
		ON credentials(type, name);
	`

	_, err := db.WriteDB.Exec(schemaSQL)
	if err != nil {
		return fmt.Errorf("failed to create schema: %w", err)
	}
	return nil
}
