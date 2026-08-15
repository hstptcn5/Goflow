package storage

import (
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestNewDBFailsClosedOnFutureMigration(t *testing.T) {
	path := filepath.Join(t.TempDir(), "future.db")
	db, err := NewDB(path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.WriteDB.Exec(`INSERT INTO schema_migrations (version, name, applied_at) VALUES (?, ?, ?)`, 999, "future", time.Now()); err != nil {
		t.Fatal(err)
	}
	db.Close()
	if _, err := NewDB(path); err == nil || !strings.Contains(err.Error(), "unsupported") {
		t.Fatalf("expected unsupported future migration failure, got %v", err)
	}
}

func TestNewDBFailsClosedOnRenamedMigration(t *testing.T) {
	path := filepath.Join(t.TempDir(), "renamed.db")
	db, err := NewDB(path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.WriteDB.Exec(`UPDATE schema_migrations SET name = 'tampered' WHERE version = 1`); err != nil {
		t.Fatal(err)
	}
	db.Close()
	if _, err := NewDB(path); err == nil || !strings.Contains(err.Error(), "unexpected name") {
		t.Fatalf("expected migration-name failure, got %v", err)
	}
}
