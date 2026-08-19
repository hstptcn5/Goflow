package storage

import (
	"database/sql"
	"fmt"
	"strings"
)

const (
	CredentialKindAPIKey          = "API_KEY"
	CredentialKindBearerToken     = "BEARER_TOKEN"
	CredentialKindBasicAuth       = "BASIC_AUTH"
	CredentialKindOAuth2          = "OAUTH2"
	CredentialKindUsernamePassword = "USERNAME_PASSWORD"
	CredentialKindServiceAccount  = "SERVICE_ACCOUNT"
	CredentialKindCustom          = "CUSTOM"
)

func init() {
	migrations = append(migrations, migration{version: 6, name: "credential_metadata", up: migrationCredentialMetadata})
}

func migrationCredentialMetadata(tx *sql.Tx) error {
	if err := ensureColumns(tx, "credentials", map[string]string{
		"kind":     "TEXT NOT NULL DEFAULT ''",
		"provider": "TEXT NOT NULL DEFAULT 'custom'",
	}); err != nil {
		return err
	}

	if _, err := tx.Exec(`
		UPDATE credentials
		SET kind = CASE LOWER(type)
			WHEN 'openai' THEN 'API_KEY'
			WHEN 'deepseek' THEN 'API_KEY'
			WHEN 'telegram_bot' THEN 'API_KEY'
			WHEN 'api_key' THEN 'API_KEY'
			WHEN 'bearer_token' THEN 'BEARER_TOKEN'
			WHEN 'basic_auth' THEN 'BASIC_AUTH'
			WHEN 'oauth2' THEN 'OAUTH2'
			WHEN 'google_service_account' THEN 'SERVICE_ACCOUNT'
			ELSE 'CUSTOM'
		END
		WHERE kind IS NULL OR TRIM(kind) = '';

		UPDATE credentials
		SET provider = CASE LOWER(type)
			WHEN 'openai' THEN 'openai'
			WHEN 'deepseek' THEN 'deepseek'
			WHEN 'telegram_bot' THEN 'telegram'
			ELSE 'custom'
		END
		WHERE provider IS NULL OR TRIM(provider) = '' OR provider = 'custom';

		CREATE INDEX IF NOT EXISTS idx_credentials_kind_provider
			ON credentials(kind, provider, name);
	`); err != nil {
		return fmt.Errorf("failed to backfill credential metadata: %w", err)
	}
	return nil
}

func normalizeCredentialKind(value string) (string, error) {
	kind := strings.ToUpper(strings.TrimSpace(value))
	switch kind {
	case CredentialKindAPIKey,
		CredentialKindBearerToken,
		CredentialKindBasicAuth,
		CredentialKindOAuth2,
		CredentialKindUsernamePassword,
		CredentialKindServiceAccount,
		CredentialKindCustom:
		return kind, nil
	default:
		return "", fmt.Errorf("unsupported credential kind %q", value)
	}
}

func normalizeCredentialProvider(value string) (string, error) {
	provider := strings.ToLower(strings.TrimSpace(value))
	if provider == "" {
		provider = "custom"
	}
	if len(provider) > 80 {
		return "", fmt.Errorf("credential provider exceeds 80 characters")
	}
	for _, r := range provider {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-' || r == '_' || r == '.' {
			continue
		}
		return "", fmt.Errorf("credential provider must use lowercase letters, numbers, dot, dash, or underscore")
	}
	return provider, nil
}

func credentialMetadataForLegacyType(legacyType string) (kind, provider string) {
	switch strings.ToLower(strings.TrimSpace(legacyType)) {
	case "openai", "openai_api_key":
		return CredentialKindAPIKey, "openai"
	case "deepseek", "deepseek_api_key":
		return CredentialKindAPIKey, "deepseek"
	case "telegram_bot":
		return CredentialKindAPIKey, "telegram"
	case "api_key":
		return CredentialKindAPIKey, "custom"
	case "bearer_token":
		return CredentialKindBearerToken, "custom"
	case "basic_auth":
		return CredentialKindBasicAuth, "custom"
	case "oauth2":
		return CredentialKindOAuth2, "custom"
	case "google_service_account":
		return CredentialKindServiceAccount, "google"
	default:
		return CredentialKindCustom, "custom"
	}
}

func legacyCredentialTypeFor(kind, provider string) string {
	switch provider {
	case "openai":
		if kind == CredentialKindAPIKey {
			return "OpenAI"
		}
	case "deepseek":
		if kind == CredentialKindAPIKey {
			return "DeepSeek"
		}
	case "telegram":
		if kind == CredentialKindAPIKey {
			return "TELEGRAM_BOT"
		}
	}
	if kind == CredentialKindOAuth2 {
		return "oauth2"
	}
	return kind
}
