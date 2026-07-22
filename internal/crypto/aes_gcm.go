package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"io"

	"golang.org/x/crypto/argon2"
)

// CryptoManager quản lý mã hóa và giải mã AES-256-GCM
type CryptoManager struct {
	key []byte
}

// NewCryptoManager tạo một crypto manager từ master passphrase
func NewCryptoManager(masterPassphrase string) *CryptoManager {
	// Sử dụng Argon2id với salt cố định để derive 32-byte key từ master passphrase
	salt := []byte("goflow-argon2id-salt-constant")
	key := argon2.IDKey([]byte(masterPassphrase), salt, 1, 64*1024, 4, 32)
	return &CryptoManager{key: key}
}

// Encrypt mã hóa dữ liệu theo chuẩn AES-256-GCM
func (c *CryptoManager) Encrypt(plaintext []byte) (string, error) {
	block, err := aes.NewCipher(c.key)
	if err != nil {
		return "", err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}

	// GCM Seal chèn nonce ở đầu dữ liệu
	ciphertext := gcm.Seal(nonce, nonce, plaintext, nil)
	return base64.StdEncoding.EncodeToString(ciphertext), nil
}

// Decrypt giải mã dữ liệu Base64 AES-256-GCM
func (c *CryptoManager) Decrypt(encodedCiphertext string) ([]byte, error) {
	ciphertext, err := base64.StdEncoding.DecodeString(encodedCiphertext)
	if err != nil {
		return nil, err
	}

	block, err := aes.NewCipher(c.key)
	if err != nil {
		return nil, err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	nonceSize := gcm.NonceSize()
	if len(ciphertext) < nonceSize {
		return nil, errors.New("ciphertext too short")
	}

	nonce, ciphertextBytes := ciphertext[:nonceSize], ciphertext[nonceSize:]
	return gcm.Open(nil, nonce, ciphertextBytes, nil)
}
