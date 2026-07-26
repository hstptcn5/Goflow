package storage

import (
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
)

type AccessToken struct {
	ID               string     `json:"id"`
	Name             string     `json:"name"`
	Scopes           []string   `json:"scopes"`
	AllowedWorkflows []string   `json:"allowed_workflows"`
	CreatedAt        time.Time  `json:"created_at"`
	LastUsedAt       *time.Time `json:"last_used_at,omitempty"`
}

type AccessTokenStore struct {
	db *DB
}

func NewAccessTokenStore(db *DB) *AccessTokenStore {
	return &AccessTokenStore{db: db}
}

func (s *AccessTokenStore) Create(name string, scopes []string, allowedWorkflows []string) (*AccessToken, string, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, "", errors.New("token name is required")
	}
	rawToken, err := generateAccessToken()
	if err != nil {
		return nil, "", err
	}
	token := &AccessToken{
		ID:               uuid.New().String(),
		Name:             name,
		Scopes:           normalizeStringList(scopes),
		AllowedWorkflows: normalizeStringList(allowedWorkflows),
		CreatedAt:        time.Now(),
	}
	scopesJSON, err := json.Marshal(token.Scopes)
	if err != nil {
		return nil, "", err
	}
	allowedJSON, err := json.Marshal(token.AllowedWorkflows)
	if err != nil {
		return nil, "", err
	}
	_, err = s.db.WriteDB.Exec(`
		INSERT INTO access_tokens (
			id, name, token_hash, scopes_json, allowed_workflows_json, created_at
		) VALUES (?, ?, ?, ?, ?, ?)
	`, token.ID, token.Name, tokenHash(rawToken), string(scopesJSON), string(allowedJSON), token.CreatedAt)
	if err != nil {
		return nil, "", err
	}
	return token, rawToken, nil
}

func (s *AccessTokenStore) Authenticate(rawToken string) (*AccessToken, error) {
	rawToken = strings.TrimSpace(rawToken)
	if rawToken == "" {
		return nil, errors.New("token is required")
	}
	row := s.db.WriteDB.QueryRow(`
		SELECT id, name, scopes_json, allowed_workflows_json, created_at, last_used_at
		FROM access_tokens
		WHERE token_hash = ?
	`, tokenHash(rawToken))
	token, err := scanAccessToken(row)
	if err != nil {
		return nil, err
	}
	now := time.Now()
	if _, err := s.db.WriteDB.Exec(`UPDATE access_tokens SET last_used_at = ? WHERE id = ?`, now, token.ID); err != nil {
		return nil, err
	}
	token.LastUsedAt = &now
	return token, nil
}

func (s *AccessTokenStore) List() ([]AccessToken, error) {
	rows, err := s.db.ReadDB.Query(`
		SELECT id, name, scopes_json, allowed_workflows_json, created_at, last_used_at
		FROM access_tokens
		ORDER BY created_at DESC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []AccessToken
	for rows.Next() {
		token, err := scanAccessToken(rows)
		if err != nil {
			return nil, err
		}
		result = append(result, *token)
	}
	return result, rows.Err()
}

func (s *AccessTokenStore) Delete(id string) (bool, error) {
	res, err := s.db.WriteDB.Exec(`DELETE FROM access_tokens WHERE id = ?`, strings.TrimSpace(id))
	if err != nil {
		return false, err
	}
	affected, _ := res.RowsAffected()
	return affected > 0, nil
}

func (t AccessToken) HasScope(scope string) bool {
	scope = strings.TrimSpace(scope)
	if scope == "" {
		return true
	}
	for _, item := range t.Scopes {
		if item == "*" || item == scope {
			return true
		}
		if prefix, ok := strings.CutSuffix(item, ":*"); ok && strings.HasPrefix(scope, prefix+":") {
			return true
		}
	}
	return false
}

func (t AccessToken) AllowsWorkflow(workflowID string) bool {
	workflowID = strings.TrimSpace(workflowID)
	if workflowID == "" || len(t.AllowedWorkflows) == 0 {
		return true
	}
	for _, item := range t.AllowedWorkflows {
		if item == "*" || item == workflowID {
			return true
		}
	}
	return false
}

type accessTokenScanner interface {
	Scan(dest ...interface{}) error
}

func scanAccessToken(scanner accessTokenScanner) (*AccessToken, error) {
	var token AccessToken
	var scopesJSON string
	var allowedJSON string
	var lastUsedAt sql.NullTime
	err := scanner.Scan(&token.ID, &token.Name, &scopesJSON, &allowedJSON, &token.CreatedAt, &lastUsedAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, errors.New("access token not found")
		}
		return nil, err
	}
	_ = json.Unmarshal([]byte(scopesJSON), &token.Scopes)
	_ = json.Unmarshal([]byte(allowedJSON), &token.AllowedWorkflows)
	token.Scopes = normalizeStringList(token.Scopes)
	token.AllowedWorkflows = normalizeStringList(token.AllowedWorkflows)
	if lastUsedAt.Valid {
		token.LastUsedAt = &lastUsedAt.Time
	}
	return &token, nil
}

func generateAccessToken() (string, error) {
	randomBytes := make([]byte, 32)
	if _, err := rand.Read(randomBytes); err != nil {
		return "", err
	}
	return "gf_" + base64.RawURLEncoding.EncodeToString(randomBytes), nil
}

func tokenHash(rawToken string) string {
	sum := sha256.Sum256([]byte(rawToken))
	return hex.EncodeToString(sum[:])
}

func normalizeStringList(values []string) []string {
	seen := map[string]bool{}
	result := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" || seen[value] {
			continue
		}
		seen[value] = true
		result = append(result, value)
	}
	return result
}
