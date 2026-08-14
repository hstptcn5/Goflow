package packsetup

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"goflow/internal/pack"
)

func TestDailyOpsMigrationFromV010ToV030IsOrderedAndIdempotent(t *testing.T) {
	dir := t.TempDir()
	manifest := migrationManifest("0.3.0")
	writeMigrationFixture(t, dir, "0.1.0")
	now := time.Date(2026, 8, 13, 10, 0, 0, 0, time.UTC)

	result, err := ApplyMigrations(dir, manifest, "0.1.0", DefaultMigrationRegistry(), MigrationOptions{Now: now})
	if err != nil {
		t.Fatalf("ApplyMigrations failed: %v", err)
	}
	if !result.Changed || result.Category != MigrationConfig {
		t.Fatalf("unexpected migration result: %+v", result)
	}
	if got := result.State.AppliedSteps; len(got) != 2 || got[0] != "0.1.0->0.2.0" || got[1] != "0.2.0->0.3.0" {
		t.Fatalf("unexpected migration order: %#v", got)
	}
	loaded, err := LoadConfig(dir, manifest)
	if err != nil {
		t.Fatalf("LoadConfig after migration: %v", err)
	}
	if _, exists := loaded.Stale["report_title"]; exists {
		t.Fatal("obsolete report_title survived migration")
	}
	if _, exists := loaded.Stale["low_stock_threshold"]; exists {
		t.Fatal("obsolete low_stock_threshold survived migration")
	}
	if loaded.Config.Values["source_url"] != "https://source.example.test/daily.json" ||
		loaded.Config.Values["chat_id"] != "@dailyops" {
		t.Fatalf("current config was not preserved: %#v", loaded.Config.Values)
	}
	credentials, err := LoadCredentialBindings(dir, manifest, nil)
	if err != nil {
		t.Fatalf("LoadCredentialBindings after migration: %v", err)
	}
	if credentials.Credentials.Slots["telegram"].CredentialID != "credential-1" {
		t.Fatalf("credential reference was not preserved: %#v", credentials.Credentials.Slots)
	}
	state, err := LoadState(dir, manifest)
	if err != nil || state.Completed {
		t.Fatalf("migrated setup did not require revalidation: state=%#v err=%v", state, err)
	}
	backupInfo := filepath.Join(dir, filepath.FromSlash(result.State.BackupRelative), "BACKUP_INFO.json")
	if _, err := os.Stat(backupInfo); err != nil {
		t.Fatalf("migration backup missing: %v", err)
	}

	repeated, err := ApplyMigrations(dir, manifest, "0.1.0", DefaultMigrationRegistry(), MigrationOptions{Now: now.Add(time.Hour)})
	if err != nil || !repeated.AlreadyApplied || repeated.Changed {
		t.Fatalf("repeated migration = %+v, %v", repeated, err)
	}
	again, err := loadMigrationState(dir, manifest.ID)
	if err != nil || again.AppliedAt != now.Format(time.RFC3339) {
		t.Fatalf("repeated migration changed state: %+v, %v", again, err)
	}
}

func TestMigrationRollbackRestoresEverySetupFile(t *testing.T) {
	dir := t.TempDir()
	manifest := migrationManifest("0.3.0")
	writeMigrationFixture(t, dir, "0.1.0")
	before := migrationFileBytes(t, dir)
	injected := errors.New("injected persistence failure")

	_, err := ApplyMigrations(dir, manifest, "0.1.0", DefaultMigrationRegistry(), MigrationOptions{
		AfterWrite: func(path string) error {
			if path == CredentialFileName {
				return injected
			}
			return nil
		},
	})
	if !errors.Is(err, injected) {
		t.Fatalf("ApplyMigrations error = %v, want injected failure", err)
	}
	after := migrationFileBytes(t, dir)
	for name, value := range before {
		if !bytes.Equal(value, after[name]) {
			t.Fatalf("%s changed after rollback", name)
		}
	}
	if _, err := os.Stat(filepath.Join(dir, MigrationFileName)); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("migration state survived rollback: %v", err)
	}
}

func TestMigrationSupportsIntentionalRenameAndUnknownChangeRequiresReview(t *testing.T) {
	registry, err := NewMigrationRegistry(MigrationStep{
		PackID: "example.rename", FromVersion: "1.0.0", ToVersion: "1.1.0",
		Category: MigrationConfig,
		Transform: func(data *MigrationData) error {
			data.ConfigValues["new_name"] = data.ConfigValues["old_name"]
			delete(data.ConfigValues, "old_name")
			return nil
		},
	})
	if err != nil {
		t.Fatalf("NewMigrationRegistry: %v", err)
	}
	dir := t.TempDir()
	writeJSONAtomic(filepath.Join(dir, ConfigFileName), ConfigFile{
		PackID: "example.rename", ConfigSchemaVersion: ConfigSchemaVersion,
		Values: map[string]interface{}{"old_name": "retained"},
	})
	manifest := pack.Manifest{
		ID: "example.rename", Version: "1.2.0",
		ConfigSchema: []pack.ConfigField{{Key: "new_name", Type: "string", Required: true}},
	}
	result, err := ApplyMigrations(dir, manifest, "1.0.0", registry, MigrationOptions{})
	if err != nil {
		t.Fatalf("ApplyMigrations: %v", err)
	}
	if result.Category != MigrationUserReview {
		t.Fatalf("unknown trailing change category = %q", result.Category)
	}
	loaded, err := LoadConfig(dir, manifest)
	if err != nil || loaded.Config.Values["new_name"] != "retained" {
		t.Fatalf("renamed config = %#v, %v", loaded, err)
	}
}

func TestMigrationRejectsDowngradeAndFutureStateSchema(t *testing.T) {
	dir := t.TempDir()
	manifest := migrationManifest("0.3.0")
	if _, err := ApplyMigrations(dir, manifest, "0.4.0", DefaultMigrationRegistry(), MigrationOptions{}); err == nil {
		t.Fatal("automatic downgrade was accepted")
	}
	if err := os.WriteFile(filepath.Join(dir, MigrationFileName), []byte(`{
		"schema_version": 99,
		"pack_id": "official.dailyops-rest-telegram",
		"from_version": "0.2.0",
		"to_version": "0.3.0",
		"category": "revalidation"
	}`), 0600); err != nil {
		t.Fatalf("write future state: %v", err)
	}
	if _, err := ApplyMigrations(dir, manifest, "0.3.0", DefaultMigrationRegistry(), MigrationOptions{}); err == nil {
		t.Fatal("future migration state schema was accepted")
	}
}

func TestMigrationSemVerOrderingMatchesPrereleaseRules(t *testing.T) {
	ordered := []string{
		"1.0.0-alpha",
		"1.0.0-alpha.1",
		"1.0.0-alpha.beta",
		"1.0.0-beta",
		"1.0.0-beta.2",
		"1.0.0-beta.11",
		"1.0.0-rc.1",
		"1.0.0",
	}
	for index := 0; index < len(ordered)-1; index++ {
		if !validVersion(ordered[index]) || compareVersions(ordered[index], ordered[index+1]) >= 0 {
			t.Fatalf("SemVer order mismatch: %q must precede %q", ordered[index], ordered[index+1])
		}
	}
	for _, invalid := range []string{"1.0.0-01", "1.0.0-alpha..1", "1.0", "01.0.0"} {
		if validVersion(invalid) {
			t.Fatalf("invalid SemVer accepted: %q", invalid)
		}
	}
	if compareVersions("1.0.0+build.1", "1.0.0+build.2") != 0 {
		t.Fatal("build metadata changed SemVer precedence")
	}
}

func migrationManifest(version string) pack.Manifest {
	return pack.Manifest{
		ID: "official.dailyops-rest-telegram", Version: version,
		ConfigSchema: []pack.ConfigField{
			{Key: "source_url", Type: "url", Required: true},
			{Key: "chat_id", Type: "string", Required: true},
		},
		CredentialRequirements: []pack.CredentialRequirement{
			{Key: "telegram", Type: "TELEGRAM_BOT", Required: true},
		},
	}
}

func writeMigrationFixture(t *testing.T, dir, version string) {
	t.Helper()
	if err := writeJSONAtomic(filepath.Join(dir, ConfigFileName), ConfigFile{
		PackID: "official.dailyops-rest-telegram", ConfigSchemaVersion: ConfigSchemaVersion,
		Values: map[string]interface{}{
			"source_url": "https://source.example.test/daily.json",
			"chat_id":    "@dailyops", "report_title": "Daily report",
			"low_stock_threshold": 3,
		},
	}); err != nil {
		t.Fatalf("write config fixture: %v", err)
	}
	if err := writeJSONAtomic(filepath.Join(dir, CredentialFileName), CredentialFile{
		PackID: "official.dailyops-rest-telegram", CredentialSchemaVersion: CredentialSchemaVersion,
		Slots: map[string]CredentialSlot{
			"telegram": {CredentialID: "credential-1", CredentialType: "TELEGRAM_BOT"},
		},
	}); err != nil {
		t.Fatalf("write credential fixture: %v", err)
	}
	if err := writeJSONAtomic(filepath.Join(dir, StateFileName), StateFile{
		PackID: "official.dailyops-rest-telegram", PackVersion: version,
		StateSchemaVersion: StateSchemaVersion, Completed: true,
		UpdatedAt: "2026-08-09T00:00:00Z",
	}); err != nil {
		t.Fatalf("write state fixture: %v", err)
	}
}

func migrationFileBytes(t *testing.T, dir string) map[string][]byte {
	t.Helper()
	result := map[string][]byte{}
	for _, name := range []string{ConfigFileName, CredentialFileName, StateFileName} {
		value, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			t.Fatalf("read %s: %v", name, err)
		}
		result[name] = value
	}
	return result
}

func TestMigrationStateJSONContainsNoCredentialValue(t *testing.T) {
	dir := t.TempDir()
	writeMigrationFixture(t, dir, "0.2.0")
	result, err := ApplyMigrations(dir, migrationManifest("0.3.0"), "0.2.0", DefaultMigrationRegistry(), MigrationOptions{})
	if err != nil {
		t.Fatalf("ApplyMigrations: %v", err)
	}
	raw, err := json.Marshal(result.State)
	if err != nil {
		t.Fatalf("marshal state: %v", err)
	}
	if bytes.Contains(raw, []byte("credential-1")) {
		t.Fatal("migration state contains a credential reference")
	}
}

func TestMigrationRejectsUnsafeStaleConfigBeforeSnapshot(t *testing.T) {
	dir := t.TempDir()
	writeMigrationFixture(t, dir, "0.2.0")
	var file ConfigFile
	raw, err := os.ReadFile(filepath.Join(dir, ConfigFileName))
	if err != nil || json.Unmarshal(raw, &file) != nil {
		t.Fatalf("load config fixture: %v", err)
	}
	file.Values["removed_field"] = map[string]interface{}{"api_key": "secret-canary"}
	if err := writeJSONAtomic(filepath.Join(dir, ConfigFileName), file); err != nil {
		t.Fatalf("write unsafe fixture: %v", err)
	}
	if _, err := ApplyMigrations(dir, migrationManifest("0.3.0"), "0.2.0", DefaultMigrationRegistry(), MigrationOptions{}); err == nil {
		t.Fatal("unsafe stale config was migrated")
	}
	if entries, err := os.ReadDir(filepath.Join(dir, "setup-backups")); err == nil && len(entries) > 0 {
		t.Fatalf("unsafe setup was copied into a snapshot: %v", entries)
	} else if err != nil && !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("inspect backup directory: %v", err)
	}
}
