package packrun

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"goflow/internal/pack"
	"goflow/internal/storage"
)

func TestPrepareIdempotentlyUpsertsManagedWorkflow(t *testing.T) {
	packDir := writeRunPack(t, "example.packrun", "0.1.0", "Hello Pack", false, nil)
	loaded := loadPack(t, packDir)
	dataDir := t.TempDir()

	first, err := prepare(context.Background(), loaded, dataDir)
	if err != nil {
		t.Fatalf("first prepare failed: %v", err)
	}
	assertWorkflowCount(t, dataDir, 1, first.WorkflowID, "Hello Pack")

	second, err := prepare(context.Background(), loaded, dataDir)
	if err != nil {
		t.Fatalf("second prepare failed: %v", err)
	}
	if second.WorkflowID != first.WorkflowID {
		t.Fatalf("workflow ID changed: %s -> %s", first.WorkflowID, second.WorkflowID)
	}
	assertWorkflowCount(t, dataDir, 1, first.WorkflowID, "Hello Pack")

	writeRunPackFiles(t, packDir, "example.packrun", "0.2.0", "Hello Pack Updated", false, nil)
	upgraded := loadPack(t, packDir)
	third, err := prepare(context.Background(), upgraded, dataDir)
	if err != nil {
		t.Fatalf("upgrade prepare failed: %v", err)
	}
	if third.WorkflowID != first.WorkflowID {
		t.Fatalf("upgrade changed workflow ID: %s -> %s", first.WorkflowID, third.WorkflowID)
	}
	assertWorkflowCount(t, dataDir, 1, first.WorkflowID, "Hello Pack Updated")
}

func TestRunRejectsPackagedPluginsInMVP(t *testing.T) {
	packDir := writeRunPack(t, "example.plugins", "0.1.0", "Plugin Pack", false, []string{"plugins/tool.exe"})
	var stdout, stderr bytes.Buffer
	err := Run(context.Background(), Options{
		PackDir: packDir,
		DataDir: t.TempDir(),
		NoOpen:  true,
		Stdout:  &stdout,
		Stderr:  &stderr,
	})
	if err == nil || !strings.Contains(err.Error(), "packaged plugin execution is not supported") {
		t.Fatalf("expected plugin policy error, got %v", err)
	}
}

func TestDataLockExcludesSecondHolderAndReleases(t *testing.T) {
	dataDir := t.TempDir()
	first, acquired, err := acquireDataLock(dataDir)
	if err != nil || !acquired {
		t.Fatalf("first lock acquired=%t err=%v", acquired, err)
	}
	_, acquired, err = acquireDataLock(dataDir)
	if err != nil {
		t.Fatalf("second lock returned unexpected error: %v", err)
	}
	if acquired {
		t.Fatalf("second lock unexpectedly acquired")
	}
	first.Release()
	third, acquired, err := acquireDataLock(dataDir)
	if err != nil || !acquired {
		t.Fatalf("third lock after release acquired=%t err=%v", acquired, err)
	}
	third.Release()
}

func TestRunStateURLMustBeLoopback(t *testing.T) {
	for _, raw := range []string{"http://127.0.0.1:1234/workflows", "http://localhost:1234/credentials"} {
		if !isLoopbackURL(raw) {
			t.Fatalf("expected loopback URL: %s", raw)
		}
	}
	for _, raw := range []string{"https://127.0.0.1:1234", "http://192.168.1.2:1234", "http://example.com"} {
		if isLoopbackURL(raw) {
			t.Fatalf("expected non-loopback URL rejection: %s", raw)
		}
	}
}

func TestOpenURLUsesInjectedOpener(t *testing.T) {
	called := ""
	err := openURL(Options{Opener: func(rawURL string) error {
		called = rawURL
		return nil
	}}, "http://127.0.0.1:1234/workflows")
	if err != nil {
		t.Fatalf("openURL failed: %v", err)
	}
	if called != "http://127.0.0.1:1234/workflows" {
		t.Fatalf("opener called with %q", called)
	}
}

func writeRunPack(t *testing.T, id, version, name string, active bool, plugins []string) string {
	t.Helper()
	dir := t.TempDir()
	writeRunPackFiles(t, dir, id, version, name, active, plugins)
	return dir
}

func writeRunPackFiles(t *testing.T, dir, id, version, name string, active bool, plugins []string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Join(dir, "workflows"), 0700); err != nil {
		t.Fatalf("mkdir workflows: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(dir, "plugins"), 0700); err != nil {
		t.Fatalf("mkdir plugins: %v", err)
	}
	for _, plugin := range plugins {
		path := filepath.Join(dir, filepath.FromSlash(plugin))
		if err := os.WriteFile(path, []byte("plugin"), 0700); err != nil {
			t.Fatalf("write plugin: %v", err)
		}
	}
	manifest := map[string]interface{}{
		"schema_version":       1,
		"id":                   id,
		"name":                 name,
		"version":              version,
		"entry_workflow":       "workflows/main.json",
		"required_credentials": []string{},
		"supported_platforms":  []string{pack.CurrentPlatform()},
	}
	if len(plugins) > 0 {
		manifest["plugins"] = plugins
	}
	writeJSON(t, filepath.Join(dir, "pack.json"), manifest)
	workflowDef := map[string]interface{}{
		"name":        name,
		"description": "managed test workflow",
		"is_active":   active,
		"nodes": []map[string]interface{}{
			{"id": "trigger", "type": "webhookTrigger", "params": map[string]interface{}{}},
		},
		"edges": []interface{}{},
	}
	writeJSON(t, filepath.Join(dir, "workflows", "main.json"), workflowDef)
}

func writeJSON(t *testing.T, path string, value interface{}) {
	t.Helper()
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		t.Fatalf("marshal json: %v", err)
	}
	if err := os.WriteFile(path, append(data, '\n'), 0600); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

func loadPack(t *testing.T, dir string) *pack.Pack {
	t.Helper()
	loaded, err := pack.Load(dir)
	if err != nil {
		t.Fatalf("load pack: %v", err)
	}
	return loaded
}

func assertWorkflowCount(t *testing.T, dataDir string, wantCount int, wantID, wantName string) {
	t.Helper()
	db, err := storage.NewDB(filepath.Join(dataDir, "goflow.db"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer db.Close()
	wfs, err := storage.NewWorkflowStore(db).ListAll()
	if err != nil {
		t.Fatalf("list workflows: %v", err)
	}
	if len(wfs) != wantCount {
		t.Fatalf("workflow count got %d want %d: %#v", len(wfs), wantCount, wfs)
	}
	if wfs[0].ID != wantID || wfs[0].Name != wantName {
		t.Fatalf("workflow got id=%s name=%s, want id=%s name=%s", wfs[0].ID, wfs[0].Name, wantID, wantName)
	}
}
