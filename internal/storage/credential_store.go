package storage

import (
	"database/sql"
	"errors"
	"time"

	"goflow/internal/crypto"

	"github.com/google/uuid"
)

type Credential struct {
	ID            string    `json:"id"`
	Name          string    `json:"name"`
	Type          string    `json:"type"` // 'API_KEY', 'TELEGRAM_BOT', 'BEARER_TOKEN', 'BASIC_AUTH'
	DataEncrypted string    `json:"data_encrypted,omitempty"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

type CredentialStore struct {
	db     *DB
	crypto *crypto.CryptoManager
}

func NewCredentialStore(db *DB, cm *crypto.CryptoManager) *CredentialStore {
	return &CredentialStore{
		db:     db,
		crypto: cm,
	}
}

func (s *CredentialStore) Create(name, credType, rawData string) (*Credential, error) {
	encryptedData, err := s.crypto.Encrypt([]byte(rawData))
	if err != nil {
		return nil, err
	}

	cred := &Credential{
		ID:            uuid.New().String(),
		Name:          name,
		Type:          credType,
		DataEncrypted: encryptedData,
		CreatedAt:     time.Now(),
		UpdatedAt:     time.Now(),
	}

	query := `
		INSERT INTO credentials (id, name, type, data_encrypted, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?)
	`
	_, err = s.db.WriteDB.Exec(query, cred.ID, cred.Name, cred.Type, cred.DataEncrypted, cred.CreatedAt, cred.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return cred, nil
}

func (s *CredentialStore) GetDecryptedData(id string) (string, error) {
	query := `SELECT data_encrypted FROM credentials WHERE id = ?`
	row := s.db.ReadDB.QueryRow(query, id)

	var encryptedData string
	err := row.Scan(&encryptedData)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "", errors.New("credential not found")
		}
		return "", err
	}

	decryptedBytes, err := s.crypto.Decrypt(encryptedData)
	if err != nil {
		return "", err
	}
	return string(decryptedBytes), nil
}

func (s *CredentialStore) ListAll() ([]Credential, error) {
	query := `SELECT id, name, type, created_at, updated_at FROM credentials ORDER BY created_at DESC`
	rows, err := s.db.ReadDB.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []Credential
	for rows.Next() {
		var c Credential
		if err := rows.Scan(&c.ID, &c.Name, &c.Type, &c.CreatedAt, &c.UpdatedAt); err != nil {
			return nil, err
		}
		result = append(result, c)
	}
	return result, nil
}

func (s *CredentialStore) Delete(id string) error {
	query := `DELETE FROM credentials WHERE id = ?`
	_, err := s.db.WriteDB.Exec(query, id)
	return err
}

func (s *CredentialStore) UpdateData(id, rawData string) error {
	encryptedData, err := s.crypto.Encrypt([]byte(rawData))
	if err != nil {
		return err
	}
	query := `UPDATE credentials SET data_encrypted = ?, updated_at = ? WHERE id = ?`
	_, err = s.db.WriteDB.Exec(query, encryptedData, time.Now(), id)
	return err
}

func (s *CredentialStore) GetByID(id string) (*Credential, error) {
	query := `SELECT id, name, type, data_encrypted, created_at, updated_at FROM credentials WHERE id = ?`
	row := s.db.ReadDB.QueryRow(query, id)
	var c Credential
	err := row.Scan(&c.ID, &c.Name, &c.Type, &c.DataEncrypted, &c.CreatedAt, &c.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return &c, nil
}
