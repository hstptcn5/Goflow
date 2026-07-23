package storage

import (
	"path/filepath"
	"testing"

	"goflow/internal/crypto"
)

func TestCredentialStoreMigratesLegacyDefaultKey(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "goflow.db")
	db, err := NewDB(dbPath)
	if err != nil {
		t.Fatalf("NewDB failed: %v", err)
	}
	defer db.Close()

	legacyCrypto := crypto.NewCryptoManager(crypto.LegacyDefaultMasterKey)
	legacyStore := NewCredentialStore(db, legacyCrypto)
	cred, err := legacyStore.Create("old openai", "OpenAI", "sk-test-secret")
	if err != nil {
		t.Fatalf("Create with legacy key failed: %v", err)
	}
	before := encryptedDataForTest(t, db, cred.ID)

	newCrypto := crypto.NewCryptoManager("new-random-master-key")
	newStore := NewCredentialStore(db, newCrypto)
	decrypted, err := newStore.GetDecryptedData(cred.ID)
	if err != nil {
		t.Fatalf("GetDecryptedData with fallback failed: %v", err)
	}
	if decrypted != "sk-test-secret" {
		t.Fatalf("expected migrated secret, got %q", decrypted)
	}

	after := encryptedDataForTest(t, db, cred.ID)
	if after == before {
		t.Fatalf("expected credential ciphertext to be re-encrypted with new key")
	}
	if _, err := legacyCrypto.Decrypt(after); err == nil {
		t.Fatalf("expected migrated ciphertext to stop decrypting with legacy key")
	}
}

func encryptedDataForTest(t *testing.T, db *DB, id string) string {
	t.Helper()
	var encrypted string
	if err := db.ReadDB.QueryRow(`SELECT data_encrypted FROM credentials WHERE id = ?`, id).Scan(&encrypted); err != nil {
		t.Fatalf("failed to fetch encrypted data: %v", err)
	}
	return encrypted
}
