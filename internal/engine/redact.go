package engine

import (
	"fmt"
	"net/url"
	"regexp"
	"strings"
)

const redactedValue = "[REDACTED]"

var secretValuePatterns = []*regexp.Regexp{
	regexp.MustCompile(`https://discord\.com/api/webhooks/[^\s"'<>]+`),
	regexp.MustCompile(`https://hooks\.slack\.com/services/[^\s"'<>]+`),
	regexp.MustCompile(`(?i)bearer\s+[a-z0-9._\-]+`),
	regexp.MustCompile(`(?i)sk-[a-z0-9_\-]{12,}`),
}

func redactSensitive(value interface{}) interface{} {
	switch v := value.(type) {
	case nil:
		return nil
	case string:
		return redactSensitiveString(v)
	case map[string]interface{}:
		out := make(map[string]interface{}, len(v))
		for key, item := range v {
			if isSensitiveKey(key) {
				out[key] = redactedValue
				continue
			}
			out[key] = redactSensitive(item)
		}
		return out
	case map[string]string:
		out := make(map[string]string, len(v))
		for key, item := range v {
			if isSensitiveKey(key) {
				out[key] = redactedValue
				continue
			}
			out[key] = redactSensitiveString(item)
		}
		return out
	case []interface{}:
		out := make([]interface{}, len(v))
		for i, item := range v {
			out[i] = redactSensitive(item)
		}
		return out
	case []string:
		out := make([]string, len(v))
		for i, item := range v {
			out[i] = redactSensitiveString(item)
		}
		return out
	default:
		return value
	}
}

func redactSensitiveString(value string) string {
	redacted := value
	for _, pattern := range secretValuePatterns {
		redacted = pattern.ReplaceAllString(redacted, redactedValue)
	}
	if parsed, err := url.Parse(redacted); err == nil && parsed.User != nil {
		parsed.User = url.UserPassword(redactedValue, redactedValue)
		redacted = parsed.String()
	}
	return redacted
}

func isSensitiveKey(key string) bool {
	normalized := strings.ToLower(strings.ReplaceAll(key, "-", "_"))
	sensitiveParts := []string{
		"api_key",
		"apikey",
		"authorization",
		"bot_token",
		"client_secret",
		"connection_string",
		"credential",
		"data_encrypted",
		"idempotency_key",
		"notion_token",
		"password",
		"private_key",
		"refresh_token",
		"secret",
		"token",
		"webhook_url",
	}
	for _, part := range sensitiveParts {
		if strings.Contains(normalized, part) {
			return true
		}
	}
	return false
}

func redactError(err error) string {
	if err == nil {
		return ""
	}
	return redactSensitiveString(fmt.Sprint(err))
}
