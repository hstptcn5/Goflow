package storage

import (
	"path/filepath"
	"testing"
	"time"
)

func TestExecutionStoreMarkRunningInterrupted(t *testing.T) {
	db := newTestDB(t)
	insertWorkflowForTest(t, db, "wf-1")
	store := NewExecutionStore(db)

	exec := &Execution{ID: "exec-running", WorkflowID: "wf-1", Status: "RUNNING", LogsJSON: "[]"}
	if err := store.Create(exec); err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	affected, err := store.MarkRunningInterrupted()
	if err != nil {
		t.Fatalf("MarkRunningInterrupted failed: %v", err)
	}
	if affected != 1 {
		t.Fatalf("expected 1 affected row, got %d", affected)
	}

	got, err := store.GetByID(exec.ID)
	if err != nil {
		t.Fatalf("GetByID failed: %v", err)
	}
	if got.Status != "INTERRUPTED" {
		t.Fatalf("expected INTERRUPTED, got %s", got.Status)
	}
	if got.FinishedAt == nil {
		t.Fatalf("expected finished_at to be set")
	}
}

func TestExecutionStoreCleanup(t *testing.T) {
	db := newTestDB(t)
	insertWorkflowForTest(t, db, "wf-1")
	store := NewExecutionStore(db)

	insertExecutionForTest(t, db, "old", "wf-1", time.Now().AddDate(0, 0, -10))
	insertExecutionForTest(t, db, "new-1", "wf-1", time.Now().Add(-3*time.Hour))
	insertExecutionForTest(t, db, "new-2", "wf-1", time.Now().Add(-2*time.Hour))
	insertExecutionForTest(t, db, "new-3", "wf-1", time.Now().Add(-1*time.Hour))

	deleted, err := store.Cleanup(7, 2)
	if err != nil {
		t.Fatalf("Cleanup failed: %v", err)
	}
	if deleted != 2 {
		t.Fatalf("expected 2 deleted rows, got %d", deleted)
	}

	remaining, err := store.ListByWorkflow("wf-1", 10)
	if err != nil {
		t.Fatalf("ListByWorkflow failed: %v", err)
	}
	if len(remaining) != 2 {
		t.Fatalf("expected 2 remaining executions, got %d", len(remaining))
	}
	if remaining[0].ID != "new-3" || remaining[1].ID != "new-2" {
		t.Fatalf("expected newest executions to remain, got %+v", remaining)
	}
}

func TestMigrationsAddWorkflowAndExecutionMetadata(t *testing.T) {
	db := newTestDB(t)

	workflowColumns := mustTableColumns(t, db, "workflows")
	for _, col := range []string{"slug", "input_schema_json", "output_schema_json", "expose_cli", "expose_mcp", "mcp_tool_name", "risk_level", "requires_approval"} {
		if !workflowColumns[col] {
			t.Fatalf("expected workflows.%s column", col)
		}
	}

	executionColumns := mustTableColumns(t, db, "executions")
	for _, col := range []string{"trigger_source", "trigger_principal", "request_id", "idempotency_key", "input_json", "error_message", "cancelled_at"} {
		if !executionColumns[col] {
			t.Fatalf("expected executions.%s column", col)
		}
	}

	tokenColumns := mustTableColumns(t, db, "access_tokens")
	for _, col := range []string{"id", "name", "token_hash", "scopes_json", "allowed_workflows_json", "created_at", "last_used_at"} {
		if !tokenColumns[col] {
			t.Fatalf("expected access_tokens.%s column", col)
		}
	}

	auditColumns := mustTableColumns(t, db, "audit_events")
	for _, col := range []string{"id", "event_type", "subject", "scope", "workflow_id", "execution_id", "success", "message", "created_at"} {
		if !auditColumns[col] {
			t.Fatalf("expected audit_events.%s column", col)
		}
	}
}

func TestRunMigrationsIsIdempotentAndPreservesData(t *testing.T) {
	db := newTestDB(t)
	insertWorkflowForTest(t, db, "wf-1")
	insertExecutionForTest(t, db, "exec-1", "wf-1", time.Now())

	before := migrationCount(t, db)
	if err := db.RunMigrations(); err != nil {
		t.Fatalf("RunMigrations failed: %v", err)
	}
	after := migrationCount(t, db)
	if after != before {
		t.Fatalf("expected migration count to remain %d, got %d", before, after)
	}
	var workflowCount int
	if err := db.WriteDB.QueryRow(`SELECT COUNT(1) FROM workflows WHERE id = 'wf-1'`).Scan(&workflowCount); err != nil {
		t.Fatalf("count workflow: %v", err)
	}
	if workflowCount != 1 {
		t.Fatalf("expected workflow to be preserved, got count %d", workflowCount)
	}
	var executionCount int
	if err := db.WriteDB.QueryRow(`SELECT COUNT(1) FROM executions WHERE id = 'exec-1'`).Scan(&executionCount); err != nil {
		t.Fatalf("count execution: %v", err)
	}
	if executionCount != 1 {
		t.Fatalf("expected execution to be preserved, got count %d", executionCount)
	}
}

func TestAccessTokenStoreCreateAuthenticateListDelete(t *testing.T) {
	db := newTestDB(t)
	store := NewAccessTokenStore(db)

	token, raw, err := store.Create("ci-runner", []string{"workflow:run", "workflow:run"}, []string{"wf-1"})
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}
	if raw == "" || token.ID == "" || len(token.Scopes) != 1 || token.Scopes[0] != "workflow:run" {
		t.Fatalf("unexpected token: token=%#v raw=%q", token, raw)
	}
	if token.HasScope("execution:read") {
		t.Fatalf("token should not have execution:read")
	}
	if !token.AllowsWorkflow("wf-1") || token.AllowsWorkflow("wf-2") {
		t.Fatalf("workflow allowlist failed")
	}

	authenticated, err := store.Authenticate(raw)
	if err != nil {
		t.Fatalf("Authenticate failed: %v", err)
	}
	if authenticated.ID != token.ID || authenticated.LastUsedAt == nil {
		t.Fatalf("unexpected authenticated token: %#v", authenticated)
	}
	if _, err := store.Authenticate("wrong"); err == nil {
		t.Fatalf("expected invalid token error")
	}

	list, err := store.List()
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}
	if len(list) != 1 || list[0].ID != token.ID {
		t.Fatalf("unexpected token list: %#v", list)
	}
	deleted, err := store.Delete(token.ID)
	if err != nil || !deleted {
		t.Fatalf("Delete failed: deleted=%t err=%v", deleted, err)
	}
}

func TestExecutionStoreIdempotencyLookup(t *testing.T) {
	db := newTestDB(t)
	insertWorkflowForTest(t, db, "wf-1")
	store := NewExecutionStore(db)

	exec := &Execution{
		ID:             "exec-idem",
		WorkflowID:     "wf-1",
		Status:         "RUNNING",
		LogsJSON:       "[]",
		IdempotencyKey: "daily-2026-07-25",
		TriggerSource:  "api",
		InputJSON:      `{"date":"2026-07-25"}`,
	}
	if err := store.Create(exec); err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	got, err := store.GetByIdempotencyKey("wf-1", "daily-2026-07-25")
	if err != nil {
		t.Fatalf("GetByIdempotencyKey failed: %v", err)
	}
	if got.ID != exec.ID {
		t.Fatalf("expected %s, got %s", exec.ID, got.ID)
	}
	if got.TriggerSource != "api" || got.InputJSON == "" {
		t.Fatalf("metadata was not persisted: %+v", got)
	}
}

func newTestDB(t *testing.T) *DB {
	t.Helper()
	db, err := NewDB(filepath.Join(t.TempDir(), "goflow.db"))
	if err != nil {
		t.Fatalf("NewDB failed: %v", err)
	}
	t.Cleanup(db.Close)
	return db
}

func insertExecutionForTest(t *testing.T, db *DB, id, workflowID string, startedAt time.Time) {
	t.Helper()
	_, err := db.WriteDB.Exec(`
		INSERT INTO executions (id, workflow_id, status, duration_ms, logs_json, started_at, finished_at)
		VALUES (?, ?, 'SUCCESS', 1, '[]', ?, ?)
	`, id, workflowID, startedAt, startedAt.Add(time.Second))
	if err != nil {
		t.Fatalf("insert execution failed: %v", err)
	}
}

func insertWorkflowForTest(t *testing.T, db *DB, id string) {
	t.Helper()
	_, err := db.WriteDB.Exec(`
		INSERT INTO workflows (id, name, description, is_active, nodes_json, edges_json, created_at, updated_at)
		VALUES (?, 'Test Workflow', '', 1, '[]', '[]', ?, ?)
	`, id, time.Now(), time.Now())
	if err != nil {
		t.Fatalf("insert workflow failed: %v", err)
	}
}

func mustTableColumns(t *testing.T, db *DB, table string) map[string]bool {
	t.Helper()
	rows, err := db.WriteDB.Query(`PRAGMA table_info(` + table + `)`)
	if err != nil {
		t.Fatalf("table_info failed: %v", err)
	}
	defer rows.Close()

	cols := make(map[string]bool)
	for rows.Next() {
		var cid int
		var name, colType string
		var notNull int
		var defaultValue interface{}
		var pk int
		if err := rows.Scan(&cid, &name, &colType, &notNull, &defaultValue, &pk); err != nil {
			t.Fatalf("scan table_info failed: %v", err)
		}
		cols[name] = true
	}
	return cols
}

func migrationCount(t *testing.T, db *DB) int {
	t.Helper()
	var count int
	if err := db.WriteDB.QueryRow(`SELECT COUNT(1) FROM schema_migrations`).Scan(&count); err != nil {
		t.Fatalf("count migrations: %v", err)
	}
	return count
}
