package pack

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestLoadValidPack(t *testing.T) {
	dir := writeValidPack(t)

	loaded, err := Load(dir)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	if loaded.Manifest.ID != "example.hello-webhook" || loaded.Manifest.Version != "0.1.0" {
		t.Fatalf("unexpected manifest: %#v", loaded.Manifest)
	}
}

func TestLoadRejectsMissingPackJSON(t *testing.T) {
	_, err := Load(t.TempDir())
	if err == nil || !strings.Contains(err.Error(), "manifest") {
		t.Fatalf("expected manifest error, got %v", err)
	}
}

func TestLoadRejectsInvalidManifestJSON(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, ManifestFile), `{`)
	_, err := Load(dir)
	if err == nil || !strings.Contains(err.Error(), "pack.json must be JSON") {
		t.Fatalf("expected invalid JSON error, got %v", err)
	}
}

func TestLoadRejectsUnsupportedSchemaVersion(t *testing.T) {
	dir := writeValidPack(t, func(m *Manifest) {
		m.SchemaVersion = 2
	})
	assertLoadError(t, dir, "schema_version")
}

func TestLoadRejectsInvalidID(t *testing.T) {
	dir := writeValidPack(t, func(m *Manifest) {
		m.ID = "Example Bad"
	})
	assertLoadError(t, dir, "id")
}

func TestLoadRejectsInvalidSemVer(t *testing.T) {
	dir := writeValidPack(t, func(m *Manifest) {
		m.Version = "1"
	})
	assertLoadError(t, dir, "SemVer")
}

func TestLoadRejectsMissingEntryWorkflow(t *testing.T) {
	dir := writeValidPack(t, func(m *Manifest) {
		m.EntryWorkflow = ""
	})
	assertLoadError(t, dir, "entry_workflow")
}

func TestLoadRejectsAbsoluteEntryWorkflow(t *testing.T) {
	absWorkflowPath := filepath.Join(t.TempDir(), "workflow.json")
	dir := writeValidPack(t, func(m *Manifest) {
		m.EntryWorkflow = absWorkflowPath
	})
	assertLoadError(t, dir, "absolute paths")
}

func TestLoadRejectsEntryWorkflowPathTraversal(t *testing.T) {
	dir := writeValidPack(t, func(m *Manifest) {
		m.EntryWorkflow = "../workflow.json"
	})
	assertLoadError(t, dir, "path traversal")
}

func TestLoadRejectsEntryWorkflowSymlinkOutsidePack(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("creating file symlinks may require elevated Windows privileges")
	}
	dir := writeValidPack(t)
	outside := filepath.Join(t.TempDir(), "outside.json")
	writeFile(t, outside, validWorkflowJSON())
	linkPath := filepath.Join(dir, DefaultWorkflowPath)
	if err := os.Remove(linkPath); err != nil {
		t.Fatalf("remove workflow: %v", err)
	}
	if err := os.Symlink(outside, linkPath); err != nil {
		t.Skipf("symlink not supported: %v", err)
	}
	assertLoadError(t, dir, "outside the pack")
}

func TestLoadRejectsNonexistentEntryWorkflow(t *testing.T) {
	dir := writeValidPack(t, func(m *Manifest) {
		m.EntryWorkflow = "workflows/missing.json"
	})
	assertLoadError(t, dir, "entry_workflow")
}

func TestLoadRejectsEntryWorkflowDirectory(t *testing.T) {
	dir := writeValidPack(t)
	path := filepath.Join(dir, DefaultWorkflowPath)
	if err := os.Remove(path); err != nil {
		t.Fatalf("remove workflow: %v", err)
	}
	if err := os.Mkdir(path, 0700); err != nil {
		t.Fatalf("mkdir workflow path: %v", err)
	}
	assertLoadError(t, dir, "regular file")
}

func TestLoadRejectsInvalidWorkflowJSON(t *testing.T) {
	dir := writeValidPack(t)
	writeFile(t, filepath.Join(dir, DefaultWorkflowPath), `{`)
	assertLoadError(t, dir, "workflow")
}

func TestLoadRejectsInvalidWorkflowStructure(t *testing.T) {
	dir := writeValidPack(t)
	writeFile(t, filepath.Join(dir, DefaultWorkflowPath), `{"name":"Invalid","nodes":[{"id":"n1","type":"missingType","params":{}}],"edges":[]}`)
	assertLoadError(t, dir, "unknown node type")
}

func TestLoadRejectsOversizedManifest(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, ManifestFile), strings.Repeat(" ", MaxManifestBytes+1))
	assertLoadError(t, dir, "exceeds")
}

func TestLoadRejectsOversizedWorkflow(t *testing.T) {
	dir := writeValidPack(t)
	writeFile(t, filepath.Join(dir, DefaultWorkflowPath), strings.Repeat(" ", MaxWorkflowBytes+1))
	assertLoadError(t, dir, "exceeds")
}

func TestLoadRejectsManifestCredentialSecret(t *testing.T) {
	dir := writeValidPack(t)
	raw := `{
		"schema_version":1,
		"id":"example.secret",
		"name":"Secret",
		"version":"0.1.0",
		"entry_workflow":"workflows/main.json",
		"required_credentials":[],
		"supported_platforms":["windows-amd64"],
		"secrets":{"api_key":"real-value"}
	}`
	writeFile(t, filepath.Join(dir, ManifestFile), raw)
	assertLoadError(t, dir, "credential secrets")
}

func writeValidPack(t *testing.T, edits ...func(*Manifest)) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "workflows"), 0700); err != nil {
		t.Fatalf("mkdir workflows: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(dir, "plugins"), 0700); err != nil {
		t.Fatalf("mkdir plugins: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(dir, "assets"), 0700); err != nil {
		t.Fatalf("mkdir assets: %v", err)
	}
	writeFile(t, filepath.Join(dir, DefaultWorkflowPath), validWorkflowJSON())
	manifest := Manifest{
		SchemaVersion:       SupportedSchema,
		ID:                  "example.hello-webhook",
		Name:                "Hello Webhook",
		Version:             "0.1.0",
		Description:         "Minimal test pack",
		EntryWorkflow:       DefaultWorkflowPath,
		RequiredCredentials: []string{},
		SupportedPlatforms:  []string{"windows-amd64"},
	}
	for _, edit := range edits {
		edit(&manifest)
	}
	data, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		t.Fatalf("marshal manifest: %v", err)
	}
	writeFile(t, filepath.Join(dir, ManifestFile), string(data))
	return dir
}

func assertLoadError(t *testing.T, dir, want string) {
	t.Helper()
	_, err := Load(dir)
	if err == nil {
		t.Fatalf("expected error containing %q", want)
	}
	if !strings.Contains(err.Error(), want) {
		t.Fatalf("expected error containing %q, got %v", want, err)
	}
}

func writeFile(t *testing.T, path, data string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(data), 0600); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

func validWorkflowJSON() string {
	return `{
		"name":"Hello Webhook",
		"description":"Minimal pack workflow",
		"nodes":[{"id":"trigger","type":"webhookTrigger","params":{}}],
		"edges":[]
	}`
}
