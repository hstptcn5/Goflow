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

func TestLoadRejectsManifestSymlinkOutsidePack(t *testing.T) {
	dir := writeValidPack(t)
	outside := filepath.Join(t.TempDir(), ManifestFile)
	writeFile(t, outside, validManifestJSON())
	manifestPath := filepath.Join(dir, ManifestFile)
	if err := os.Remove(manifestPath); err != nil {
		t.Fatalf("remove manifest: %v", err)
	}
	if err := os.Symlink(outside, manifestPath); err != nil {
		t.Skipf("symlink not supported: %v", err)
	}
	assertLoadError(t, dir, "pack.json must not be a symlink")
}

func TestLoadRejectsUnsupportedSchemaVersion(t *testing.T) {
	dir := writeValidPack(t, func(m *Manifest) {
		m.SchemaVersion = 2
	})
	assertLoadError(t, dir, "schema_version")
}

func TestLoadRejectsInvalidID(t *testing.T) {
	for _, id := range []string{"Example Bad", ".example", "example.", "-example", "example-", "example..pack", "example.-pack", "example--pack", "example-.pack"} {
		t.Run(id, func(t *testing.T) {
			dir := writeValidPack(t, func(m *Manifest) {
				m.ID = id
			})
			assertLoadError(t, dir, "id")
		})
	}
}

func TestLoadAcceptsValidIDExamples(t *testing.T) {
	for _, id := range []string{"example.daily-report", "kiotviet-report", "vendor2.pack1"} {
		t.Run(id, func(t *testing.T) {
			dir := writeValidPack(t, func(m *Manifest) {
				m.ID = id
			})
			if _, err := Load(dir); err != nil {
				t.Fatalf("expected valid ID %q, got %v", id, err)
			}
		})
	}
}

func TestLoadRejectsInvalidSemVer(t *testing.T) {
	for _, version := range []string{"1", "1.0.0-", "1.0.0-alpha..1", "1.0.0-01", "1.0.0-alpha.01"} {
		t.Run(version, func(t *testing.T) {
			dir := writeValidPack(t, func(m *Manifest) {
				m.Version = version
			})
			assertLoadError(t, dir, "SemVer")
		})
	}
}

func TestLoadAcceptsValidSemVerExamples(t *testing.T) {
	for _, version := range []string{"0.1.0", "1.0.0-alpha", "1.0.0-alpha.1", "1.0.0+build.5", "1.0.0-alpha.1+build.5", "1.0.0-alpha-beta", "1.0.0-alpha-beta.1", "1.0.0-x-y-z+build.1"} {
		t.Run(version, func(t *testing.T) {
			dir := writeValidPack(t, func(m *Manifest) {
				m.Version = version
			})
			if _, err := Load(dir); err != nil {
				t.Fatalf("expected valid SemVer %q, got %v", version, err)
			}
		})
	}
}

func TestLoadRejectsMissingRequiredCredentials(t *testing.T) {
	dir := writePackWithManifest(t, `{
		"schema_version":1,
		"id":"example.hello-webhook",
		"name":"Hello Webhook",
		"version":"0.1.0",
		"entry_workflow":"workflows/main.json",
		"supported_platforms":["windows-amd64"]
	}`)
	assertLoadError(t, dir, "required_credentials is required")
}

func TestLoadRejectsNullRequiredCredentials(t *testing.T) {
	dir := writePackWithManifest(t, `{
		"schema_version":1,
		"id":"example.hello-webhook",
		"name":"Hello Webhook",
		"version":"0.1.0",
		"entry_workflow":"workflows/main.json",
		"required_credentials":null,
		"supported_platforms":["windows-amd64"]
	}`)
	assertLoadError(t, dir, "required_credentials must not be null")
}

func TestLoadRejectsWrongTypeRequiredCredentials(t *testing.T) {
	dir := writePackWithManifest(t, `{
		"schema_version":1,
		"id":"example.hello-webhook",
		"name":"Hello Webhook",
		"version":"0.1.0",
		"entry_workflow":"workflows/main.json",
		"required_credentials":"none",
		"supported_platforms":["windows-amd64"]
	}`)
	assertLoadError(t, dir, "required_credentials must be a JSON array")
}

func TestLoadRejectsMissingSupportedPlatforms(t *testing.T) {
	dir := writePackWithManifest(t, `{
		"schema_version":1,
		"id":"example.hello-webhook",
		"name":"Hello Webhook",
		"version":"0.1.0",
		"entry_workflow":"workflows/main.json",
		"required_credentials":[]
	}`)
	assertLoadError(t, dir, "supported_platforms is required")
}

func TestLoadRejectsNullSupportedPlatforms(t *testing.T) {
	dir := writePackWithManifest(t, `{
		"schema_version":1,
		"id":"example.hello-webhook",
		"name":"Hello Webhook",
		"version":"0.1.0",
		"entry_workflow":"workflows/main.json",
		"required_credentials":[],
		"supported_platforms":null
	}`)
	assertLoadError(t, dir, "supported_platforms must not be null")
}

func TestLoadRejectsEmptySupportedPlatforms(t *testing.T) {
	dir := writeValidPack(t, func(m *Manifest) {
		m.SupportedPlatforms = []string{}
	})
	assertLoadError(t, dir, "supported_platforms")
}

func TestLoadRejectsEmptyPlatformEntry(t *testing.T) {
	dir := writeValidPack(t, func(m *Manifest) {
		m.SupportedPlatforms = []string{" "}
	})
	assertLoadError(t, dir, "supported_platforms[0]")
}

func TestLoadAcceptsEmptyRequiredCredentials(t *testing.T) {
	dir := writeValidPack(t, func(m *Manifest) {
		m.RequiredCredentials = []string{}
	})
	if _, err := Load(dir); err != nil {
		t.Fatalf("expected required_credentials empty array to be valid, got %v", err)
	}
}

func TestLoadRejectsMissingEntryWorkflow(t *testing.T) {
	dir := writeValidPack(t, func(m *Manifest) {
		m.EntryWorkflow = ""
	})
	assertLoadError(t, dir, "entry_workflow")
}

func TestLoadRejectsAbsoluteEntryWorkflow(t *testing.T) {
	dir := writeValidPack(t, func(m *Manifest) {
		m.EntryWorkflow = "/workflow.json"
	})
	assertLoadError(t, dir, "absolute paths")
}

func TestLoadRejectsEntryWorkflowPathTraversal(t *testing.T) {
	dir := writeValidPack(t, func(m *Manifest) {
		m.EntryWorkflow = "../workflow.json"
	})
	assertLoadError(t, dir, "path traversal")
}

func TestLoadRejectsNonPortableEntryWorkflowPathsOnEveryOS(t *testing.T) {
	paths := []string{
		"../outside.json",
		`..\outside.json`,
		"folder/../../outside.json",
		"C:/outside.json",
		`C:\outside.json`,
		"C:outside.json",
		`\\server\share\file`,
		"/workflows/main.json",
		".",
	}
	for _, path := range paths {
		t.Run(path, func(t *testing.T) {
			dir := writeValidPack(t, func(m *Manifest) {
				m.EntryWorkflow = path
			})
			assertLoadError(t, dir, "path")
		})
	}
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

func TestLoadAcceptsEntryWorkflowSymlinkInsidePack(t *testing.T) {
	dir := writeValidPack(t)
	targetPath := filepath.Join(dir, "workflows", "target.json")
	writeFile(t, targetPath, validWorkflowJSON())
	linkPath := filepath.Join(dir, DefaultWorkflowPath)
	if err := os.Remove(linkPath); err != nil {
		t.Fatalf("remove workflow: %v", err)
	}
	if err := os.Symlink(targetPath, linkPath); err != nil {
		t.Skipf("symlink not supported: %v", err)
	}
	if _, err := Load(dir); err != nil {
		t.Fatalf("expected internal workflow symlink to be valid, got %v", err)
	}
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

func TestLoadAcceptsValidPluginAndAssetFiles(t *testing.T) {
	dir := writeValidPack(t, func(m *Manifest) {
		m.Plugins = []string{"plugins/plugin.txt"}
		m.Assets = []string{"assets/sample.txt"}
	})
	writeFile(t, filepath.Join(dir, "plugins", "plugin.txt"), "plugin placeholder")
	writeFile(t, filepath.Join(dir, "assets", "sample.txt"), "asset placeholder")
	if _, err := Load(dir); err != nil {
		t.Fatalf("expected valid plugin and asset files, got %v", err)
	}
}

func TestLoadRejectsMissingPluginAndAssetFiles(t *testing.T) {
	tests := []struct {
		name string
		edit func(*Manifest)
		want string
	}{
		{name: "plugin", edit: func(m *Manifest) { m.Plugins = []string{"plugins/missing.txt"} }, want: "plugins"},
		{name: "asset", edit: func(m *Manifest) { m.Assets = []string{"assets/missing.txt"} }, want: "assets"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := writeValidPack(t, tt.edit)
			assertLoadError(t, dir, tt.want)
		})
	}
}

func TestLoadRejectsPluginAndAssetDirectories(t *testing.T) {
	tests := []struct {
		name string
		edit func(*Manifest)
		path string
		want string
	}{
		{name: "plugin", edit: func(m *Manifest) { m.Plugins = []string{"plugins/plugin-dir"} }, path: filepath.Join("plugins", "plugin-dir"), want: "plugins"},
		{name: "asset", edit: func(m *Manifest) { m.Assets = []string{"assets/asset-dir"} }, path: filepath.Join("assets", "asset-dir"), want: "assets"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := writeValidPack(t, tt.edit)
			if err := os.Mkdir(filepath.Join(dir, tt.path), 0700); err != nil {
				t.Fatalf("mkdir listed path: %v", err)
			}
			assertLoadError(t, dir, "regular file")
		})
	}
}

func TestLoadRejectsPluginSymlinkOutsidePack(t *testing.T) {
	dir := writeValidPack(t, func(m *Manifest) {
		m.Plugins = []string{"plugins/plugin.txt"}
	})
	outside := filepath.Join(t.TempDir(), "plugin.txt")
	writeFile(t, outside, "outside")
	if err := os.Symlink(outside, filepath.Join(dir, "plugins", "plugin.txt")); err != nil {
		t.Skipf("symlink not supported: %v", err)
	}
	assertLoadError(t, dir, "outside the pack")
}

func TestLoadRejectsAssetUnderSymlinkDirectoryOutsidePack(t *testing.T) {
	dir := writeValidPack(t, func(m *Manifest) {
		m.Assets = []string{"assets/external/sample.txt"}
	})
	outsideDir := t.TempDir()
	writeFile(t, filepath.Join(outsideDir, "sample.txt"), "outside")
	if err := os.Symlink(outsideDir, filepath.Join(dir, "assets", "external")); err != nil {
		t.Skipf("symlink not supported: %v", err)
	}
	assertLoadError(t, dir, "outside the pack")
}

func TestLoadAcceptsPluginSymlinkInsidePack(t *testing.T) {
	dir := writeValidPack(t, func(m *Manifest) {
		m.Plugins = []string{"plugins/plugin-link.txt"}
	})
	target := filepath.Join(dir, "plugins", "plugin-target.txt")
	writeFile(t, target, "inside")
	if err := os.Symlink(target, filepath.Join(dir, "plugins", "plugin-link.txt")); err != nil {
		t.Skipf("symlink not supported: %v", err)
	}
	if _, err := Load(dir); err != nil {
		t.Fatalf("expected internal plugin symlink to be valid, got %v", err)
	}
}

func TestLoadRejectsNonPortablePluginAndAssetPathsOnEveryOS(t *testing.T) {
	paths := []string{
		"../outside.json",
		`..\outside.json`,
		"folder/../../outside.json",
		"C:/outside.json",
		`C:\outside.json`,
		"C:outside.json",
		`\\server\share\file`,
		"/workflows/main.json",
		".",
	}
	for _, path := range paths {
		t.Run("plugin "+path, func(t *testing.T) {
			dir := writeValidPack(t, func(m *Manifest) {
				m.Plugins = []string{path}
			})
			assertLoadError(t, dir, "path")
		})
		t.Run("asset "+path, func(t *testing.T) {
			dir := writeValidPack(t, func(m *Manifest) {
				m.Assets = []string{path}
			})
			assertLoadError(t, dir, "path")
		})
	}
}

func TestLoadAcceptsValidSetupMetadata(t *testing.T) {
	dir := writeValidPack(t, func(m *Manifest) {
		m.ConfigSchema = []ConfigField{
			{Key: "store_name", Label: "Store name", Type: "string", Required: true, Default: "Demo Store", MinLength: intPtr(1), MaxLength: intPtr(80)},
			{Key: "source_url", Label: "Source URL", Type: "url", Required: true, Default: "https://example.test/report"},
			{Key: "threshold", Label: "Threshold", Type: "integer", Default: float64(10), Min: intPtr(0), Max: intPtr(1000)},
			{Key: "include_stock", Label: "Include stock", Type: "boolean", Default: true},
			{Key: "tone", Label: "Tone", Type: "select", Default: "short", Options: []interface{}{"short", "detailed"}},
		}
		m.CredentialRequirements = []CredentialRequirement{
			{Key: "telegram_bot", Label: "Telegram bot", Type: "TELEGRAM_BOT", Required: true, TestKind: "telegram_get_me"},
		}
		m.Bindings = []Binding{
			{Source: "config.store_name", Target: BindingTarget{NodeID: "telegram", Param: "message"}},
			{Source: "config.source_url", Target: BindingTarget{NodeID: "fetch", Param: "url"}},
			{Source: "credential.telegram_bot", Target: BindingTarget{NodeID: "telegram", Param: "credential_id"}},
		}
	})
	writeFile(t, filepath.Join(dir, DefaultWorkflowPath), setupWorkflowJSON())
	if _, err := Load(dir); err != nil {
		t.Fatalf("expected valid setup metadata, got %v", err)
	}
}

func TestLoadAcceptsLegacyPackWithoutSetupMetadata(t *testing.T) {
	dir := writeValidPack(t)
	if _, err := Load(dir); err != nil {
		t.Fatalf("expected legacy pack to remain valid, got %v", err)
	}
}

func TestLoadRejectsInvalidSetupMetadataArrays(t *testing.T) {
	dir := writePackWithManifest(t, `{
		"schema_version":1,
		"id":"example.setup",
		"name":"Setup",
		"version":"0.1.0",
		"entry_workflow":"workflows/main.json",
		"required_credentials":[],
		"supported_platforms":["windows-amd64"],
		"config_schema":{}
	}`)
	assertLoadError(t, dir, "config_schema must be a JSON array")
}

func TestLoadRejectsDuplicateSetupKeys(t *testing.T) {
	dir := writeValidPack(t, func(m *Manifest) {
		m.ConfigSchema = []ConfigField{
			{Key: "store_name", Label: "Store name", Type: "string", Required: true, DisplayOnly: true},
			{Key: "store_name", Label: "Other name", Type: "string"},
		}
	})
	assertLoadError(t, dir, "duplicates")

	dir = writeValidPack(t, func(m *Manifest) {
		m.CredentialRequirements = []CredentialRequirement{
			{Key: "telegram_bot", Label: "Telegram bot", Type: "TELEGRAM_BOT", Required: true, DisplayOnly: true},
			{Key: "telegram_bot", Label: "Other bot", Type: "TELEGRAM_BOT"},
		}
	})
	assertLoadError(t, dir, "duplicates")
}

func TestLoadRejectsInvalidConfigFieldTypesAndDefaults(t *testing.T) {
	tests := []struct {
		name string
		edit func(*Manifest)
		want string
	}{
		{
			name: "unknown type",
			edit: func(m *Manifest) {
				m.ConfigSchema = []ConfigField{{Key: "store_name", Label: "Store name", Type: "secret", Required: true, DisplayOnly: true}}
			},
			want: "type",
		},
		{
			name: "secret key",
			edit: func(m *Manifest) {
				m.ConfigSchema = []ConfigField{{Key: "api_token", Label: "API token", Type: "string"}}
			},
			want: "secret material",
		},
		{
			name: "secret default",
			edit: func(m *Manifest) {
				m.ConfigSchema = []ConfigField{{Key: "store_name", Label: "Store name", Type: "string", Default: "sk-real-secret"}}
			},
			want: "secret material",
		},
		{
			name: "integer default wrong type",
			edit: func(m *Manifest) {
				m.ConfigSchema = []ConfigField{{Key: "threshold", Label: "Threshold", Type: "integer", Default: "10"}}
			},
			want: "must be an integer",
		},
		{
			name: "select missing options",
			edit: func(m *Manifest) {
				m.ConfigSchema = []ConfigField{{Key: "tone", Label: "Tone", Type: "select"}}
			},
			want: "options is required",
		},
		{
			name: "select duplicate options",
			edit: func(m *Manifest) {
				m.ConfigSchema = []ConfigField{{Key: "tone", Label: "Tone", Type: "select", Options: []interface{}{"short", "short"}}}
			},
			want: "duplicates",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := writeValidPack(t, tt.edit)
			assertLoadError(t, dir, tt.want)
		})
	}
}

func TestLoadRejectsInvalidCredentialRequirement(t *testing.T) {
	tests := []struct {
		name string
		req  CredentialRequirement
		want string
	}{
		{name: "unknown type", req: CredentialRequirement{Key: "bot", Label: "Bot", Type: "UNKNOWN"}, want: "known credential type"},
		{name: "unknown test", req: CredentialRequirement{Key: "bot", Label: "Bot", Type: "TELEGRAM_BOT", TestKind: "https://example.test/check"}, want: "allowlisted"},
		{name: "bad key", req: CredentialRequirement{Key: "Bot", Label: "Bot", Type: "TELEGRAM_BOT"}, want: "lowercase"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := writeValidPack(t, func(m *Manifest) {
				m.CredentialRequirements = []CredentialRequirement{tt.req}
			})
			assertLoadError(t, dir, tt.want)
		})
	}
}

func TestLoadRejectsInvalidBindings(t *testing.T) {
	tests := []struct {
		name string
		edit func(*Manifest)
		want string
	}{
		{
			name: "unknown config source",
			edit: func(m *Manifest) {
				m.Bindings = []Binding{{Source: "config.missing", Target: BindingTarget{NodeID: "fetch", Param: "url"}}}
			},
			want: "unknown config key",
		},
		{
			name: "missing node",
			edit: func(m *Manifest) {
				m.ConfigSchema = []ConfigField{{Key: "source_url", Label: "Source URL", Type: "url", Required: true}}
				m.Bindings = []Binding{{Source: "config.source_url", Target: BindingTarget{NodeID: "missing", Param: "url"}}}
			},
			want: "does not exist",
		},
		{
			name: "missing param",
			edit: func(m *Manifest) {
				m.ConfigSchema = []ConfigField{{Key: "source_url", Label: "Source URL", Type: "url", Required: true}}
				m.Bindings = []Binding{{Source: "config.source_url", Target: BindingTarget{NodeID: "fetch", Param: "missing"}}}
			},
			want: "not defined",
		},
		{
			name: "credential to non credential",
			edit: func(m *Manifest) {
				m.CredentialRequirements = []CredentialRequirement{{Key: "telegram_bot", Label: "Telegram bot", Type: "TELEGRAM_BOT", Required: true}}
				m.Bindings = []Binding{{Source: "credential.telegram_bot", Target: BindingTarget{NodeID: "telegram", Param: "chat_id"}}}
			},
			want: "credential source",
		},
		{
			name: "config to credential",
			edit: func(m *Manifest) {
				m.ConfigSchema = []ConfigField{{Key: "bot_id", Label: "Bot ID", Type: "string", Required: true}}
				m.Bindings = []Binding{{Source: "config.bot_id", Target: BindingTarget{NodeID: "telegram", Param: "credential_id"}}}
			},
			want: "secret-like",
		},
		{
			name: "duplicate binding",
			edit: func(m *Manifest) {
				m.ConfigSchema = []ConfigField{{Key: "source_url", Label: "Source URL", Type: "url", Required: true}}
				binding := Binding{Source: "config.source_url", Target: BindingTarget{NodeID: "fetch", Param: "url"}}
				m.Bindings = []Binding{binding, binding}
			},
			want: "duplicates",
		},
		{
			name: "required config unbound",
			edit: func(m *Manifest) {
				m.ConfigSchema = []ConfigField{{Key: "source_url", Label: "Source URL", Type: "url", Required: true}}
			},
			want: "no binding",
		},
		{
			name: "required credential unbound",
			edit: func(m *Manifest) {
				m.CredentialRequirements = []CredentialRequirement{{Key: "telegram_bot", Label: "Telegram bot", Type: "TELEGRAM_BOT", Required: true}}
			},
			want: "no binding",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := writeValidPack(t, tt.edit)
			writeFile(t, filepath.Join(dir, DefaultWorkflowPath), setupWorkflowJSON())
			assertLoadError(t, dir, tt.want)
		})
	}
}

func TestLoadAllowsRequiredSetupDisplayOnly(t *testing.T) {
	dir := writeValidPack(t, func(m *Manifest) {
		m.ConfigSchema = []ConfigField{{Key: "store_name", Label: "Store name", Type: "string", Required: true, DisplayOnly: true}}
		m.CredentialRequirements = []CredentialRequirement{{Key: "telegram_bot", Label: "Telegram bot", Type: "TELEGRAM_BOT", Required: true, DisplayOnly: true}}
	})
	if _, err := Load(dir); err != nil {
		t.Fatalf("expected required display-only setup to be valid, got %v", err)
	}
}

func TestLoadRejectsOversizedSetupMetadata(t *testing.T) {
	dir := writeValidPack(t, func(m *Manifest) {
		m.ConfigSchema = []ConfigField{{
			Key:         "store_name",
			Label:       "Store name",
			Description: strings.Repeat("x", MaxSetupMetadataBytes),
			Type:        "string",
		}}
	})
	assertLoadError(t, dir, "setup metadata exceeds")
}

func TestLoadRejectsPackOnlyEmbeddedSecretParameters(t *testing.T) {
	dir := writeValidPack(t)
	writeFile(t, filepath.Join(dir, DefaultWorkflowPath), `{
		"name":"Secret Telegram",
		"nodes":[{"id":"telegram","type":"telegramBot","params":{"bot_token":"123456:ABC-secret","chat_id":"demo","message":"hello"}}],
		"edges":[]
	}`)
	assertLoadError(t, dir, "literal secret")
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

func writePackWithManifest(t *testing.T, manifest string) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "workflows"), 0700); err != nil {
		t.Fatalf("mkdir workflows: %v", err)
	}
	writeFile(t, filepath.Join(dir, DefaultWorkflowPath), validWorkflowJSON())
	writeFile(t, filepath.Join(dir, ManifestFile), manifest)
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

func setupWorkflowJSON() string {
	return `{
		"name":"Setup Workflow",
		"nodes":[
			{"id":"trigger","type":"webhookTrigger","params":{}},
			{"id":"fetch","type":"httpRequest","params":{"method":"GET","url":"https://example.test/report","headers":"{}"}},
			{"id":"telegram","type":"telegramBot","params":{"credential_id":"","chat_id":"demo","message":"hello"}}
		],
		"edges":[
			{"id":"e1","source":"trigger","target":"fetch"},
			{"id":"e2","source":"fetch","target":"telegram"}
		]
	}`
}

func intPtr(value int) *int {
	return &value
}

func validManifestJSON() string {
	return `{
		"schema_version":1,
		"id":"example.hello-webhook",
		"name":"Hello Webhook",
		"version":"0.1.0",
		"entry_workflow":"workflows/main.json",
		"required_credentials":[],
		"supported_platforms":["windows-amd64"]
	}`
}
