package storage

import (
	"database/sql"
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

func TestValidateAndOrderMigrationsRejectsDuplicateVersions(t *testing.T) {
	noop := func(*sql.Tx) error { return nil }
	_, err := validateAndOrderMigrations([]migration{
		{version: 6, name: "credential_metadata", up: noop},
		{version: 6, name: "workflow_state", up: noop},
	})
	if err == nil || !strings.Contains(err.Error(), "duplicate database migration version 0006") {
		t.Fatalf("expected duplicate migration version failure, got %v", err)
	}
}

func TestValidateAndOrderMigrationsSortsByVersion(t *testing.T) {
	noop := func(*sql.Tx) error { return nil }
	ordered, err := validateAndOrderMigrations([]migration{
		{version: 7, name: "workflow_state", up: noop},
		{version: 6, name: "credential_metadata", up: noop},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(ordered) != 2 || ordered[0].version != 6 || ordered[1].version != 7 {
		t.Fatalf("unexpected migration order: %#v", ordered)
	}
}
