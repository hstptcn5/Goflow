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
