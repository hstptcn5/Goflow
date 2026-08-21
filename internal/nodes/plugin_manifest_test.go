package nodes

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadPluginNodeManifest(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "echo.goflow-node.json")
	content := `{
  "schema_version": 1,
  "type": "custom.echo",
  "name": "Echo",
  "version": "1.2.0",
  "executable": "echo-plugin",
  "capabilities": ["network:none"],
  "params": [{"name":"message","label":"Message","type":"text","required":true}],
  "outputs": [{"name":"result","type":"string"}]
}`
	if err := os.WriteFile(path, []byte(content), 0600); err != nil {
		t.Fatal(err)
	}
	manifest, err := LoadPluginNodeManifest(path)
	if err != nil {
		t.Fatal(err)
	}
	if manifest.Type != "custom.echo" || manifest.Version != "1.2.0" || len(manifest.Params) != 1 {
		t.Fatalf("manifest = %#v", manifest)
	}
}

func TestPluginManifestRejectsExecutablePathEscape(t *testing.T) {
	manifest := PluginNodeManifest{
		SchemaVersion: 1,
		Type:          "custom.bad",
		Name:          "Bad",
		Version:       "1.0.0",
		Executable:    "../bad",
	}
	if err := validatePluginNodeManifest(manifest); err == nil {
		t.Fatal("path escape executable accepted")
	}
}

func TestDiscoverPluginNodeExecutorsSkipsInvalidManifest(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "bad.goflow-node.json"), []byte(`{"schema_version":1}`), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "ok.goflow-node.json"), []byte(`{"schema_version":1,"type":"custom.ok","name":"OK","version":"1","executable":"ok"}`), 0600); err != nil {
		t.Fatal(err)
	}
	executors, errs := DiscoverPluginNodeExecutors(dir)
	if len(executors) != 1 || len(errs) != 1 {
		t.Fatalf("executors=%d errors=%d", len(executors), len(errs))
	}
}
