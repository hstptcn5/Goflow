package packsetup

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"goflow/internal/pack"
)

func TestSaveAndLoadStateRoundTrip(t *testing.T) {
	dir := t.TempDir()
	manifest := pack.Manifest{ID: "example.state"}
	now := time.Date(2026, 8, 9, 3, 0, 0, 0, time.UTC)
	saved, err := SaveState(dir, manifest, true, now)
	if err != nil {
		t.Fatalf("SaveState failed: %v", err)
	}
	if !saved.Completed || saved.UpdatedAt != "2026-08-09T03:00:00Z" {
		t.Fatalf("unexpected saved state: %#v", saved)
	}
	loaded, err := LoadState(dir, manifest)
	if err != nil {
		t.Fatalf("LoadState failed: %v", err)
	}
	if !loaded.Completed || loaded.PackID != manifest.ID {
		t.Fatalf("unexpected loaded state: %#v", loaded)
	}
	stat, err := os.Stat(filepath.Join(dir, StateFileName))
	if err != nil {
		t.Fatalf("stat state file: %v", err)
	}
	if runtime.GOOS != "windows" && stat.Mode().Perm() != 0600 {
		t.Fatalf("expected 0600 state mode, got %v", stat.Mode().Perm())
	}
}

func TestLoadStateRejectsPackMismatchAndOversizedFile(t *testing.T) {
	dir := t.TempDir()
	manifest := pack.Manifest{ID: "example.state"}
	if err := os.WriteFile(filepath.Join(dir, StateFileName), []byte(`{"pack_id":"other","state_schema_version":1,"completed":true}`), 0600); err != nil {
		t.Fatalf("write state: %v", err)
	}
	if _, err := LoadState(dir, manifest); err == nil || !strings.Contains(err.Error(), "pack_id mismatch") {
		t.Fatalf("expected pack mismatch, got %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, StateFileName), []byte(strings.Repeat("x", MaxStateStorageBytes+1)), 0600); err != nil {
		t.Fatalf("write oversized state: %v", err)
	}
	if _, err := LoadState(dir, manifest); err == nil || !strings.Contains(err.Error(), "exceeds") {
		t.Fatalf("expected oversized error, got %v", err)
	}
}

func TestCompletedStateRequiresCurrentPackVersion(t *testing.T) {
	dir := t.TempDir()
	v1 := pack.Manifest{ID: "example.state", Version: "0.1.0"}
	if _, err := SaveState(dir, v1, true, time.Now()); err != nil {
		t.Fatalf("save v1 state: %v", err)
	}
	if _, err := LoadState(dir, pack.Manifest{ID: v1.ID, Version: "0.2.0"}); err == nil || !strings.Contains(err.Error(), "requires revalidation") {
		t.Fatalf("expected pack upgrade revalidation, got %v", err)
	}
	if _, err := LoadState(dir, v1); err != nil {
		t.Fatalf("same-version restart should remain complete: %v", err)
	}
}
