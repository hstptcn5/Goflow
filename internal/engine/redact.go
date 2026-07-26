package engine

import (
	"fmt"
	"net/url"
	"reflect"
	"regexp"
	"strings"
)

const redactedValue = "[REDACTED]"
const redactionCycleValue = "[REDACTED:CYCLE]"
const redactionDepthValue = "[REDACTED:MAX_DEPTH]"
const maxRedactionDepth = 32

var secretValuePatterns = []*regexp.Regexp{
	regexp.MustCompile(`https://discord\.com/api/webhooks/[^\s"'<>]+`),
	regexp.MustCompile(`https://hooks\.slack\.com/services/[^\s"'<>]+`),
	regexp.MustCompile(`(?i)bearer\s+[a-z0-9._\-]+`),
	regexp.MustCompile(`(?i)sk-[a-z0-9_\-]{12,}`),
}

func redactSensitive(value interface{}) interface{} {
	return redactSensitiveValue(value, 0, map[uintptr]bool{})
}

func redactSensitiveValue(value interface{}, depth int, seen map[uintptr]bool) interface{} {
	if depth > maxRedactionDepth {
		return redactionDepthValue
	}
	switch v := value.(type) {
	case nil:
		return nil
	case string:
		return redactSensitiveString(v)
	case map[string]interface{}:
		if isSeenMap(value, seen) {
			return redactionCycleValue
		}
		out := make(map[string]interface{}, len(v))
		for key, item := range v {
			if isSensitiveKey(key) {
				out[key] = redactedValue
				continue
			}
			out[key] = redactSensitiveValue(item, depth+1, seen)
		}
		return out
	case map[string]string:
		if isSeenMap(value, seen) {
			return redactionCycleValue
		}
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
		if isSeenSlice(value, seen) {
			return redactionCycleValue
		}
		out := make([]interface{}, len(v))
		for i, item := range v {
			out[i] = redactSensitiveValue(item, depth+1, seen)
		}
		return out
	case []string:
		if isSeenSlice(value, seen) {
			return redactionCycleValue
		}
		out := make([]string, len(v))
		for i, item := range v {
			out[i] = redactSensitiveString(item)
		}
		return out
	default:
		return value
	}
}

func isSeenMap(value interface{}, seen map[uintptr]bool) bool {
	rv := reflect.ValueOf(value)
	if rv.Kind() != reflect.Map || rv.IsNil() {
		return false
	}
	ptr := rv.Pointer()
	if seen[ptr] {
		return true
	}
	seen[ptr] = true
	return false
}

func isSeenSlice(value interface{}, seen map[uintptr]bool) bool {
	rv := reflect.ValueOf(value)
	if rv.Kind() != reflect.Slice || rv.IsNil() || rv.Len() == 0 {
		return false
	}
	ptr := rv.Pointer()
	if seen[ptr] {
		return true
	}
	seen[ptr] = true
	return false
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
