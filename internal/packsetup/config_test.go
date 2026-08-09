package packsetup

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"goflow/internal/pack"
)

func TestSaveAndLoadConfigRoundTrip(t *testing.T) {
	dir := t.TempDir()
	manifest := testManifest()
	saved, err := SaveConfig(dir, manifest, map[string]interface{}{
		"store_name":  "Demo",
		"source_url":  "https://example.test/report",
		"threshold":   10,
		"include_tax": true,
		"tone":        "short",
	})
	if err != nil {
		t.Fatalf("SaveConfig failed: %v", err)
	}
	if saved.PackID != manifest.ID || saved.ConfigSchemaVersion != ConfigSchemaVersion {
		t.Fatalf("unexpected saved config: %#v", saved)
	}
	loaded, err := LoadConfig(dir, manifest)
	if err != nil {
		t.Fatalf("LoadConfig failed: %v", err)
	}
	if loaded.Config.Values["store_name"] != "Demo" || loaded.Config.Values["threshold"] != 10 {
		t.Fatalf("unexpected loaded values: %#v", loaded.Config.Values)
	}
	if len(loaded.Stale) != 0 {
		t.Fatalf("unexpected stale values: %#v", loaded.Stale)
	}
}

func TestSaveConfigRejectsInvalidValues(t *testing.T) {
	tests := []struct {
		name   string
		values map[string]interface{}
		want   string
	}{
		{name: "missing required", values: map[string]interface{}{"store_name": "Demo", "threshold": 10, "include_tax": false, "tone": "short"}, want: "source_url"},
		{name: "bad url", values: validValues("file:///tmp/data.json"), want: "absolute http or https URL"},
		{name: "relative url", values: validValues("/report"), want: "absolute http or https URL"},
		{name: "integer low", values: withValue(validValues("https://example.test/report"), "threshold", -1), want: ">= 0"},
		{name: "wrong type", values: withValue(validValues("https://example.test/report"), "include_tax", "true"), want: "boolean"},
		{name: "bad select", values: withValue(validValues("https://example.test/report"), "tone", "verbose"), want: "option"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := SaveConfig(t.TempDir(), testManifest(), tt.values)
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("expected %q error, got %v", tt.want, err)
			}
		})
	}
}

func TestLoadConfigRejectsPackMismatchAndOversizedFile(t *testing.T) {
	dir := t.TempDir()
	manifest := testManifest()
	if err := os.WriteFile(filepath.Join(dir, ConfigFileName), []byte(`{"pack_id":"other","config_schema_version":1,"values":{}}`), 0600); err != nil {
		t.Fatalf("write config: %v", err)
	}
	if _, err := LoadConfig(dir, manifest); err == nil || !strings.Contains(err.Error(), "pack_id mismatch") {
		t.Fatalf("expected pack mismatch, got %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, ConfigFileName), []byte(strings.Repeat("x", MaxConfigStorageBytes+1)), 0600); err != nil {
		t.Fatalf("write oversized config: %v", err)
	}
	if _, err := LoadConfig(dir, manifest); err == nil || !strings.Contains(err.Error(), "exceeds") {
		t.Fatalf("expected oversized error, got %v", err)
	}
}

func TestLoadConfigRetainsSafeStaleFieldsWithoutApplyingThem(t *testing.T) {
	dir := t.TempDir()
	manifest := testManifest()
	raw := ConfigFile{
		PackID:              manifest.ID,
		ConfigSchemaVersion: ConfigSchemaVersion,
		Values: map[string]interface{}{
			"store_name":  "Demo",
			"source_url":  "https://example.test/report",
			"threshold":   10,
			"include_tax": false,
			"tone":        "short",
			"old_field":   "old-safe-value",
		},
	}
	writeRawConfig(t, dir, raw)
	loaded, err := LoadConfig(dir, manifest)
	if err != nil {
		t.Fatalf("LoadConfig failed: %v", err)
	}
	if _, ok := loaded.Config.Values["old_field"]; ok {
		t.Fatalf("stale value was applied as current config: %#v", loaded.Config.Values)
	}
	if loaded.Stale["old_field"] != "old-safe-value" {
		t.Fatalf("expected stale value retained, got %#v", loaded.Stale)
	}
	if _, err := SaveConfig(dir, manifest, validValues("https://example.test/next")); err != nil {
		t.Fatalf("SaveConfig failed: %v", err)
	}
	data, err := os.ReadFile(filepath.Join(dir, ConfigFileName))
	if err != nil {
		t.Fatalf("read saved config: %v", err)
	}
	if !strings.Contains(string(data), "old-safe-value") {
		t.Fatalf("expected safe stale value to remain persisted: %s", data)
	}
}

func TestLoadConfigRejectsUnsafeStaleFields(t *testing.T) {
	dir := t.TempDir()
	manifest := testManifest()
	raw := ConfigFile{
		PackID:              manifest.ID,
		ConfigSchemaVersion: ConfigSchemaVersion,
		Values: map[string]interface{}{
			"store_name":  "Demo",
			"source_url":  "https://example.test/report",
			"threshold":   10,
			"include_tax": false,
			"tone":        "short",
			"old_token":   "sk-secret",
		},
	}
	writeRawConfig(t, dir, raw)
	if _, err := LoadConfig(dir, manifest); err == nil || !strings.Contains(err.Error(), "not safe to retain") {
		t.Fatalf("expected unsafe stale rejection, got %v", err)
	}
}

func TestSaveConfigUsesRestrictedPermissionsWhereSupported(t *testing.T) {
	dir := t.TempDir()
	if _, err := SaveConfig(dir, testManifest(), validValues("https://example.test/report")); err != nil {
		t.Fatalf("SaveConfig failed: %v", err)
	}
	stat, err := os.Stat(filepath.Join(dir, ConfigFileName))
	if err != nil {
		t.Fatalf("stat config file: %v", err)
	}
	if runtime.GOOS != "windows" && stat.Mode().Perm() != 0600 {
		t.Fatalf("expected 0600 config mode, got %v", stat.Mode().Perm())
	}
}

func testManifest() pack.Manifest {
	return pack.Manifest{
		ID: "example.setup",
		ConfigSchema: []pack.ConfigField{
			{Key: "store_name", Label: "Store name", Type: "string", Required: true, MinLength: intPtr(1), MaxLength: intPtr(80)},
			{Key: "source_url", Label: "Source URL", Type: "url", Required: true},
			{Key: "threshold", Label: "Threshold", Type: "integer", Required: true, Min: intPtr(0), Max: intPtr(100)},
			{Key: "include_tax", Label: "Include tax", Type: "boolean", Required: true},
			{Key: "tone", Label: "Tone", Type: "select", Required: true, Options: []interface{}{"short", "detailed"}},
		},
	}
}

func validValues(sourceURL string) map[string]interface{} {
	return map[string]interface{}{
		"store_name":  "Demo",
		"source_url":  sourceURL,
		"threshold":   10,
		"include_tax": false,
		"tone":        "short",
	}
}

func withValue(values map[string]interface{}, key string, value interface{}) map[string]interface{} {
	next := map[string]interface{}{}
	for k, v := range values {
		next[k] = v
	}
	next[key] = value
	return next
}

func writeRawConfig(t *testing.T, dir string, cfg ConfigFile) {
	t.Helper()
	data, err := json.Marshal(cfg)
	if err != nil {
		t.Fatalf("marshal config: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, ConfigFileName), data, 0600); err != nil {
		t.Fatalf("write config: %v", err)
	}
}

func intPtr(value int) *int {
	return &value
}
