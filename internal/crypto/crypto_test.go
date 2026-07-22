package crypto

import (
	"testing"
)

func TestAES256GCMEncryption(t *testing.T) {
	masterKey := "my-secret-master-passphrase-goflow"
	cryptoMgr := NewCryptoManager(masterKey)

	originalText := "telegram_bot_token_secret_123456"

	encrypted, err := cryptoMgr.Encrypt([]byte(originalText))
	if err != nil {
		t.Fatalf("Encryption failed: %v", err)
	}

	if encrypted == originalText {
		t.Fatalf("Encrypted string matches original text")
	}

	decryptedBytes, err := cryptoMgr.Decrypt(encrypted)
	if err != nil {
		t.Fatalf("Decryption failed: %v", err)
	}

	decryptedText := string(decryptedBytes)
	if decryptedText != originalText {
		t.Fatalf("Expected '%s', got '%s'", originalText, decryptedText)
	}
}
