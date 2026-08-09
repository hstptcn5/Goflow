package packsetup

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"goflow/internal/pack"
)

func TestSaveAndLoadCredentialBindingsRoundTrip(t *testing.T) {
	dir := t.TempDir()
	manifest := testCredentialManifest()
	resolver := credentialResolver(map[string]string{
		"cred-telegram": "TELEGRAM_BOT",
		"cred-api":      "API_KEY",
	})
	saved, err := SaveCredentialBindings(dir, manifest, map[string]string{
		"telegram": "cred-telegram",
		"source":   "cred-api",
	}, resolver)
	if err != nil {
		t.Fatalf("SaveCredentialBindings failed: %v", err)
	}
	if saved.PackID != manifest.ID || saved.CredentialSchemaVersion != CredentialSchemaVersion {
		t.Fatalf("unexpected saved credentials: %#v", saved)
	}
	loaded, err := LoadCredentialBindings(dir, manifest, resolver)
	if err != nil {
		t.Fatalf("LoadCredentialBindings failed: %v", err)
	}
	if loaded.Credentials.Slots["telegram"].CredentialID != "cred-telegram" {
		t.Fatalf("unexpected loaded slots: %#v", loaded.Credentials.Slots)
	}
	if len(loaded.Stale) != 0 {
		t.Fatalf("unexpected stale slots: %#v", loaded.Stale)
	}
}

func TestCredentialBindingValidation(t *testing.T) {
	tests := []struct {
		name     string
		slots    map[string]string
		resolver CredentialResolver
		want     string
	}{
		{
			name:     "missing required slot",
			slots:    map[string]string{"source": "cred-api"},
			resolver: credentialResolver(map[string]string{"cred-api": "API_KEY"}),
			want:     "telegram",
		},
		{
			name:     "undeclared slot",
			slots:    map[string]string{"telegram": "cred-telegram", "extra": "cred-api"},
			resolver: credentialResolver(map[string]string{"cred-telegram": "TELEGRAM_BOT", "cred-api": "API_KEY"}),
			want:     "not declared",
		},
		{
			name:     "missing credential id",
			slots:    map[string]string{"telegram": "missing"},
			resolver: credentialResolver(map[string]string{}),
			want:     "credential not found",
		},
		{
			name:     "wrong resolved type",
			slots:    map[string]string{"telegram": "cred-api"},
			resolver: credentialResolver(map[string]string{"cred-api": "API_KEY"}),
			want:     "expected \"TELEGRAM_BOT\"",
		},
		{
			name:     "unsafe credential id",
			slots:    map[string]string{"telegram": "sk-secret"},
			resolver: credentialResolver(map[string]string{"sk-secret": "TELEGRAM_BOT"}),
			want:     "not safe",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := SaveCredentialBindings(t.TempDir(), testCredentialManifest(), tt.slots, tt.resolver)
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("expected %q error, got %v", tt.want, err)
			}
		})
	}
}

func TestLoadCredentialBindingsRejectsPackMismatchAndOversizedFile(t *testing.T) {
	dir := t.TempDir()
	manifest := testCredentialManifest()
	if err := os.WriteFile(filepath.Join(dir, CredentialFileName), []byte(`{"pack_id":"other","credential_schema_version":1,"slots":{}}`), 0600); err != nil {
		t.Fatalf("write credentials: %v", err)
	}
	if _, err := LoadCredentialBindings(dir, manifest, nil); err == nil || !strings.Contains(err.Error(), "pack_id mismatch") {
		t.Fatalf("expected pack mismatch, got %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, CredentialFileName), []byte(strings.Repeat("x", MaxCredentialStorageBytes+1)), 0600); err != nil {
		t.Fatalf("write oversized credentials: %v", err)
	}
	if _, err := LoadCredentialBindings(dir, manifest, nil); err == nil || !strings.Contains(err.Error(), "exceeds") {
		t.Fatalf("expected oversized error, got %v", err)
	}
}

func TestLoadCredentialBindingsRetainsSafeStaleSlots(t *testing.T) {
	dir := t.TempDir()
	manifest := testCredentialManifest()
	raw := CredentialFile{
		PackID:                  manifest.ID,
		CredentialSchemaVersion: CredentialSchemaVersion,
		Slots: map[string]CredentialSlot{
			"telegram": {CredentialID: "cred-telegram", CredentialType: "TELEGRAM_BOT"},
			"old_slot": {CredentialID: "cred-old", CredentialType: "API_KEY"},
		},
	}
	writeRawCredentials(t, dir, raw)
	loaded, err := LoadCredentialBindings(dir, manifest, credentialResolver(map[string]string{"cred-telegram": "TELEGRAM_BOT"}))
	if err != nil {
		t.Fatalf("LoadCredentialBindings failed: %v", err)
	}
	if _, ok := loaded.Credentials.Slots["old_slot"]; ok {
		t.Fatalf("stale slot was applied as current slot: %#v", loaded.Credentials.Slots)
	}
	if loaded.Stale["old_slot"].CredentialID != "cred-old" {
		t.Fatalf("expected stale slot retained, got %#v", loaded.Stale)
	}
	if _, err := SaveCredentialBindings(dir, manifest, map[string]string{
		"telegram": "cred-telegram",
		"source":   "cred-api",
	}, credentialResolver(map[string]string{"cred-telegram": "TELEGRAM_BOT", "cred-api": "API_KEY"})); err != nil {
		t.Fatalf("SaveCredentialBindings failed: %v", err)
	}
	data, err := os.ReadFile(filepath.Join(dir, CredentialFileName))
	if err != nil {
		t.Fatalf("read credentials: %v", err)
	}
	if !strings.Contains(string(data), "cred-old") {
		t.Fatalf("expected safe stale slot to remain persisted: %s", data)
	}
}

func TestLoadCredentialBindingsRejectsUnsafeStaleSlots(t *testing.T) {
	dir := t.TempDir()
	manifest := testCredentialManifest()
	raw := CredentialFile{
		PackID:                  manifest.ID,
		CredentialSchemaVersion: CredentialSchemaVersion,
		Slots: map[string]CredentialSlot{
			"telegram":  {CredentialID: "cred-telegram", CredentialType: "TELEGRAM_BOT"},
			"old_token": {CredentialID: "cred-old", CredentialType: "API_KEY"},
		},
	}
	writeRawCredentials(t, dir, raw)
	_, err := LoadCredentialBindings(dir, manifest, credentialResolver(map[string]string{"cred-telegram": "TELEGRAM_BOT"}))
	if err == nil || !strings.Contains(err.Error(), "credential slot key") {
		t.Fatalf("expected unsafe stale rejection, got %v", err)
	}
}

func TestSaveCredentialBindingsUsesRestrictedPermissionsWhereSupported(t *testing.T) {
	dir := t.TempDir()
	if _, err := SaveCredentialBindings(dir, testCredentialManifest(), map[string]string{
		"telegram": "cred-telegram",
		"source":   "cred-api",
	}, credentialResolver(map[string]string{"cred-telegram": "TELEGRAM_BOT", "cred-api": "API_KEY"})); err != nil {
		t.Fatalf("SaveCredentialBindings failed: %v", err)
	}
	stat, err := os.Stat(filepath.Join(dir, CredentialFileName))
	if err != nil {
		t.Fatalf("stat credentials file: %v", err)
	}
	if runtime.GOOS != "windows" && stat.Mode().Perm() != 0600 {
		t.Fatalf("expected 0600 credential mode, got %v", stat.Mode().Perm())
	}
}

func testCredentialManifest() pack.Manifest {
	return pack.Manifest{
		ID: "example.credentials",
		CredentialRequirements: []pack.CredentialRequirement{
			{Key: "telegram", Label: "Telegram bot", Type: "TELEGRAM_BOT", Required: true},
			{Key: "source", Label: "Source API key", Type: "API_KEY", Required: false},
		},
	}
}

func credentialResolver(values map[string]string) CredentialResolver {
	return CredentialLookupFunc(func(id string) (CredentialIdentity, error) {
		credentialType, ok := values[id]
		if !ok {
			return CredentialIdentity{}, errors.New("credential not found")
		}
		return CredentialIdentity{ID: id, Type: credentialType}, nil
	})
}

func writeRawCredentials(t *testing.T, dir string, cfg CredentialFile) {
	t.Helper()
	data, err := json.Marshal(cfg)
	if err != nil {
		t.Fatalf("marshal credentials: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, CredentialFileName), data, 0600); err != nil {
		t.Fatalf("write credentials: %v", err)
	}
}
