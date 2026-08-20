package nodes

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

const (
	defaultNodeResponseLimit int64 = 2 << 20
	defaultNodeErrorLimit    int64 = 8 << 10
)

// resolveNodeCredential resolves a direct secret or an encrypted credential.
// If credential_id is explicitly configured it fails closed when that credential
// cannot be loaded instead of silently falling back to a plaintext parameter.
func resolveNodeCredential(ctx *ExecutionContext, node *Node, directParam, label string, expectedProviders ...string) (string, error) {
	direct, _ := node.Params[directParam].(string)
	credentialID, _ := node.Params["credential_id"].(string)
	credentialID = strings.TrimSpace(credentialID)
	if credentialID == "" {
		if strings.TrimSpace(direct) == "" {
			return "", fmt.Errorf("%s is required", label)
		}
		return strings.TrimSpace(direct), nil
	}
	if ctx == nil {
		return "", fmt.Errorf("%s credential context is unavailable", label)
	}
	secret, ok := ctx.Credentials[credentialID]
	if !ok || strings.TrimSpace(secret) == "" {
		return "", fmt.Errorf("%s credential %q is not available", label, credentialID)
	}
	if len(expectedProviders) > 0 {
		if metadata, ok := ctx.CredentialMetadata[credentialID]; ok {
			actual := strings.ToLower(strings.TrimSpace(metadata.Provider))
			if actual != "" {
				matched := false
				for _, expected := range expectedProviders {
					if actual == strings.ToLower(strings.TrimSpace(expected)) {
						matched = true
						break
					}
				}
				if !matched {
					return "", fmt.Errorf("%s credential %q belongs to provider %s", label, credentialID, actual)
				}
			}
		}
	}
	return strings.TrimSpace(secret), nil
}

func readNodeResponseBody(resp *http.Response, limit int64) ([]byte, error) {
	if resp == nil || resp.Body == nil {
		return nil, fmt.Errorf("response body is unavailable")
	}
	if limit <= 0 {
		limit = defaultNodeResponseLimit
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, limit+1))
	if err != nil {
		return nil, fmt.Errorf("response body could not be read")
	}
	if int64(len(body)) > limit {
		return nil, fmt.Errorf("response exceeds %d byte limit", limit)
	}
	return body, nil
}

func boundedNodeErrorText(body []byte) string {
	if int64(len(body)) > defaultNodeErrorLimit {
		body = body[:defaultNodeErrorLimit]
	}
	text := strings.TrimSpace(string(body))
	if text == "" {
		return "request failed"
	}
	return text
}

func validateAbsoluteHTTPURL(raw string) (*url.URL, error) {
	parsed, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || !parsed.IsAbs() || parsed.Host == "" {
		return nil, fmt.Errorf("URL must be an absolute http or https URL")
	}
	if parsed.User != nil {
		return nil, fmt.Errorf("URL must not contain embedded user information")
	}
	switch strings.ToLower(parsed.Scheme) {
	case "http", "https":
		return parsed, nil
	default:
		return nil, fmt.Errorf("URL must be an absolute http or https URL")
	}
}

func validateHTTPSHost(raw string, allowedHosts ...string) error {
	parsed, err := validateAbsoluteHTTPURL(raw)
	if err != nil {
		return err
	}
	if !strings.EqualFold(parsed.Scheme, "https") {
		return fmt.Errorf("URL must use https")
	}
	host := strings.ToLower(parsed.Hostname())
	for _, allowed := range allowedHosts {
		allowed = strings.ToLower(strings.TrimSpace(allowed))
		if host == allowed || (strings.HasPrefix(allowed, ".") && strings.HasSuffix(host, allowed)) {
			return nil
		}
	}
	return fmt.Errorf("URL host %q is not allowed", host)
}

func containsTemplateExpression(value string) bool {
	return strings.Contains(value, "{{") && strings.Contains(value, "}}")
}
