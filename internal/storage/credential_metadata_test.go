package storage

import (
	"path/filepath"
	"testing"

	"goflow/internal/crypto"
)

func TestCredentialMetadataMigrationAndCreateCompatibility(t *testing.T) {
	db, err := NewDB(filepath.Join(t.TempDir(), "goflow.db"))
	if err != nil {
		t.Fatalf("NewDB failed: %v", err)
	}
	defer db.Close()

	store := NewCredentialStore(db, crypto.NewCryptoManager("credential-metadata-test-key"))

	legacy, err := store.Create("old openai", "OpenAI", "sk-old")
	if err != nil {
		t.Fatalf("legacy Create failed: %v", err)
	}
	if legacy.Kind != CredentialKindAPIKey || legacy.Provider != "openai" {
		t.Fatalf("legacy metadata mismatch: kind=%q provider=%q", legacy.Kind, legacy.Provider)
	}

	zalo, err := store.CreateWithMetadata("zalo oa", CredentialKindBearerToken, "zalo", "oa-access-token")
	if err != nil {
		t.Fatalf("CreateWithMetadata failed: %v", err)
	}
	if zalo.Kind != CredentialKindBearerToken || zalo.Provider != "zalo" {
		t.Fatalf("zalo metadata mismatch: kind=%q provider=%q", zalo.Kind, zalo.Provider)
	}
	if zalo.Type != "BEARER_TOKEN" {
		t.Fatalf("expected legacy compatibility type BEARER_TOKEN, got %q", zalo.Type)
	}
	secret, err := store.GetDecryptedData(zalo.ID)
	if err != nil || secret != "oa-access-token" {
		t.Fatalf("zalo secret round-trip failed: secret=%q err=%v", secret, err)
	}

	listed, err := store.ListAll()
	if err != nil {
		t.Fatalf("ListAll failed: %v", err)
	}
	if len(listed) != 2 {
		t.Fatalf("expected 2 credentials, got %d", len(listed))
	}

	var migrationCount int
	if err := db.ReadDB.QueryRow(`SELECT COUNT(1) FROM schema_migrations WHERE version = 6 AND name = 'credential_metadata'`).Scan(&migrationCount); err != nil {
		t.Fatalf("failed to inspect credential metadata migration: %v", err)
	}
	if migrationCount != 1 {
		t.Fatalf("expected credential metadata migration to be applied")
	}
}

func TestCredentialMetadataRejectsProviderEnumsAndInvalidKinds(t *testing.T) {
	db, err := NewDB(filepath.Join(t.TempDir(), "goflow.db"))
	if err != nil {
		t.Fatalf("NewDB failed: %v", err)
	}
	defer db.Close()

	store := NewCredentialStore(db, crypto.NewCryptoManager("credential-metadata-validation-key"))
	custom, err := store.CreateWithMetadata("future provider", CredentialKindAPIKey, "future-crm", "secret")
	if err != nil {
		t.Fatalf("arbitrary provider id should be accepted: %v", err)
	}
	if custom.Provider != "future-crm" || custom.Kind != CredentialKindAPIKey {
		t.Fatalf("unexpected custom metadata: %+v", custom)
	}

	if _, err := store.CreateWithMetadata("bad kind", "ZALO_OA", "zalo", "secret"); err == nil {
		t.Fatalf("service-specific credential kinds should not be accepted")
	}
	if _, err := store.CreateWithMetadata("bad provider", CredentialKindAPIKey, "Bad Provider!", "secret"); err == nil {
		t.Fatalf("invalid provider id should be rejected")
	}
}
