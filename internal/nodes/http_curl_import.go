package nodes

import (
	"encoding/json"
	"fmt"
	"strings"
	"unicode"
)

type CURLImportResult struct {
	Params           map[string]interface{} `json:"params"`
	CredentialSecret string                 `json:"credential_secret,omitempty"`
	CredentialHint   string                 `json:"credential_hint,omitempty"`
}

func splitCURLCommand(command string) ([]string, error) {
	var args []string
	var current strings.Builder
	var quote rune
	escaped := false
	flush := func() {
		if current.Len() > 0 {
			args = append(args, current.String())
			current.Reset()
		}
	}
	for _, r := range strings.TrimSpace(command) {
		if escaped {
			current.WriteRune(r)
			escaped = false
			continue
		}
		if r == '\\' && quote != '\'' {
			escaped = true
			continue
		}
		if quote != 0 {
			if r == quote {
				quote = 0
			} else {
				current.WriteRune(r)
			}
			continue
		}
		if r == '\'' || r == '"' {
			quote = r
			continue
		}
		if unicode.IsSpace(r) {
			flush()
			continue
		}
		current.WriteRune(r)
	}
	if escaped {
		current.WriteRune('\\')
	}
	if quote != 0 {
		return nil, fmt.Errorf("cURL command has an unterminated quote")
	}
	flush()
	return args, nil
}

func isSensitiveHTTPHeader(name string) bool {
	lower := strings.ToLower(strings.TrimSpace(name))
	return lower == "authorization" || lower == "proxy-authorization" || strings.Contains(lower, "api-key") || strings.Contains(lower, "apikey") || strings.Contains(lower, "token")
}

// ParseCURLCommand converts common cURL syntax to HTTP Request node params.
// Sensitive header values are deliberately returned separately so callers can
// create an encrypted Credential instead of persisting secrets in workflow JSON.
func ParseCURLCommand(command string) (CURLImportResult, error) {
	args, err := splitCURLCommand(command)
	if err != nil {
		return CURLImportResult{}, err
	}
	if len(args) == 0 || strings.ToLower(args[0]) != "curl" {
		return CURLImportResult{}, fmt.Errorf("command must start with curl")
	}

	params := map[string]interface{}{
		"method":          "GET",
		"headers":         "{}",
		"query_params":    "{}",
		"auth_mode":       "none",
		"body_mode":       "json",
		"response_mode":   "auto",
		"pagination_mode": "none",
	}
	headers := map[string]string{}
	methodExplicit := false
	var body string
	var result CURLImportResult
	result.Params = params

	nextValue := func(index *int, flag string) (string, error) {
		*index++
		if *index >= len(args) {
			return "", fmt.Errorf("%s requires a value", flag)
		}
		return args[*index], nil
	}

	for i := 1; i < len(args); i++ {
		arg := args[i]
		switch arg {
		case "-X", "--request":
			value, err := nextValue(&i, arg)
			if err != nil {
				return CURLImportResult{}, err
			}
			params["method"] = strings.ToUpper(value)
			methodExplicit = true
		case "-H", "--header":
			value, err := nextValue(&i, arg)
			if err != nil {
				return CURLImportResult{}, err
			}
			name, headerValue, ok := strings.Cut(value, ":")
			if !ok || strings.TrimSpace(name) == "" {
				return CURLImportResult{}, fmt.Errorf("invalid cURL header %q", value)
			}
			name = strings.TrimSpace(name)
			headerValue = strings.TrimSpace(headerValue)
			if isSensitiveHTTPHeader(name) {
				if result.CredentialSecret != "" {
					return CURLImportResult{}, fmt.Errorf("cURL import contains multiple sensitive credentials; import them separately")
				}
				result.CredentialHint = name
				switch strings.ToLower(name) {
				case "authorization":
					if strings.HasPrefix(strings.ToLower(headerValue), "bearer ") {
						params["auth_mode"] = "bearer"
						result.CredentialSecret = strings.TrimSpace(headerValue[len("Bearer "):])
					} else if strings.HasPrefix(strings.ToLower(headerValue), "basic ") {
						return CURLImportResult{}, fmt.Errorf("encoded Basic Authorization headers are not imported; use curl -u so the secret can be migrated safely")
					} else {
						params["auth_mode"] = "custom_header"
						params["auth_header"] = name
						result.CredentialSecret = headerValue
					}
				default:
					params["auth_mode"] = "api_key"
					params["auth_header"] = name
					result.CredentialSecret = headerValue
				}
			} else {
				headers[name] = headerValue
			}
		case "-d", "--data", "--data-raw", "--data-binary":
			value, err := nextValue(&i, arg)
			if err != nil {
				return CURLImportResult{}, err
			}
			body = value
			if !methodExplicit {
				params["method"] = "POST"
			}
		case "-u", "--user":
			value, err := nextValue(&i, arg)
			if err != nil {
				return CURLImportResult{}, err
			}
			if result.CredentialSecret != "" {
				return CURLImportResult{}, fmt.Errorf("cURL import contains multiple credentials")
			}
			params["auth_mode"] = "basic"
			result.CredentialSecret = value
			result.CredentialHint = "Basic Auth"
		case "-L", "--location", "--compressed", "-s", "--silent", "-S", "--show-error", "--fail", "--fail-with-body":
			// Runtime redirect and transport policies remain controlled by Goflow.
		case "--url":
			value, err := nextValue(&i, arg)
			if err != nil {
				return CURLImportResult{}, err
			}
			params["url"] = value
		default:
			if strings.HasPrefix(arg, "-") {
				return CURLImportResult{}, fmt.Errorf("unsupported cURL option %q", arg)
			}
			if _, exists := params["url"]; exists {
				return CURLImportResult{}, fmt.Errorf("cURL import contains more than one URL")
			}
			params["url"] = arg
		}
	}

	if strings.TrimSpace(conditionValueString(params["url"])) == "" {
		return CURLImportResult{}, fmt.Errorf("cURL command does not contain a URL")
	}
	if err := validateHTTPRequestURL(conditionValueString(params["url"])); err != nil {
		return CURLImportResult{}, err
	}
	if body != "" {
		params["body"] = body
		if !json.Valid([]byte(body)) {
			params["body_mode"] = "raw"
		}
	}
	encodedHeaders, _ := json.Marshal(headers)
	params["headers"] = string(encodedHeaders)
	return result, nil
}
