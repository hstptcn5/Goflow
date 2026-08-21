package nodes

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"strconv"
	"strings"
)

const (
	maxHTTPQueryParams = 64
	maxHTTPFormFields  = 64
	maxHTTPPages       = 100
)

func parseHTTPObject(raw interface{}, field string) (map[string]interface{}, error) {
	if raw == nil {
		return nil, nil
	}
	if typed, ok := raw.(map[string]interface{}); ok {
		return typed, nil
	}
	text, ok := raw.(string)
	if !ok {
		return nil, fmt.Errorf("HTTP %s must be a JSON object", field)
	}
	text = strings.TrimSpace(text)
	if text == "" {
		return nil, nil
	}
	var result map[string]interface{}
	if err := json.Unmarshal([]byte(text), &result); err != nil {
		return nil, fmt.Errorf("HTTP %s must be a JSON object: %w", field, err)
	}
	return result, nil
}

func appendHTTPQuery(rawURL string, raw interface{}) (string, error) {
	params, err := parseHTTPObject(raw, "query_params")
	if err != nil {
		return "", err
	}
	if len(params) > maxHTTPQueryParams {
		return "", fmt.Errorf("HTTP query_params exceed %d item limit", maxHTTPQueryParams)
	}
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return "", err
	}
	query := parsed.Query()
	for key, value := range params {
		key = strings.TrimSpace(key)
		if key == "" {
			return "", fmt.Errorf("HTTP query parameter name is required")
		}
		switch typed := value.(type) {
		case []interface{}:
			for _, item := range typed {
				query.Add(key, conditionValueString(item))
			}
		case nil:
			query.Add(key, "")
		default:
			query.Set(key, conditionValueString(typed))
		}
	}
	parsed.RawQuery = query.Encode()
	return parsed.String(), nil
}

func normalizeHTTPAuthMode(raw interface{}) (string, error) {
	mode := strings.ToLower(strings.TrimSpace(conditionValueString(raw)))
	switch mode {
	case "", "legacy":
		return "legacy", nil
	case "none":
		return "none", nil
	case "bearer":
		return "bearer", nil
	case "api_key", "api key":
		return "api_key", nil
	case "basic", "basic_auth", "basic auth":
		return "basic", nil
	case "oauth2", "oauth":
		return "oauth2", nil
	case "custom_header", "custom header":
		return "custom_header", nil
	default:
		return "", fmt.Errorf("unsupported HTTP auth mode %q", mode)
	}
}

func requestCredential(ctx *ExecutionContext, node *Node) (string, error) {
	credID, _ := node.Params["credential_id"].(string)
	credID = strings.TrimSpace(credID)
	if credID == "" {
		return "", fmt.Errorf("HTTP credential_id is required for the selected authentication mode")
	}
	secret, ok := ctx.Credentials[credID]
	if !ok || strings.TrimSpace(secret) == "" {
		return "", fmt.Errorf("HTTP credential is not available")
	}
	return secret, nil
}

func applyHTTPAuthentication(ctx *ExecutionContext, node *Node, req *http.Request) error {
	mode, err := normalizeHTTPAuthMode(node.Params["auth_mode"])
	if err != nil {
		return err
	}
	credID, _ := node.Params["credential_id"].(string)
	if mode == "none" {
		return nil
	}
	if mode == "legacy" {
		if strings.TrimSpace(credID) == "" {
			return nil
		}
		return applyHTTPRequestCredential(ctx, node, req)
	}

	if mode == "oauth2" {
		credID = strings.TrimSpace(credID)
		if credID == "" {
			return fmt.Errorf("HTTP credential_id is required for OAuth2")
		}
		var token string
		if ctx.RefreshCredential != nil {
			token, err = ctx.RefreshCredential(credID)
		} else {
			token, err = requestCredential(ctx, node)
		}
		if err != nil {
			return fmt.Errorf("HTTP OAuth2 credential failed: %w", err)
		}
		req.Header.Set("Authorization", "Bearer "+token)
		return nil
	}

	secret, err := requestCredential(ctx, node)
	if err != nil {
		return err
	}
	switch mode {
	case "bearer":
		req.Header.Set("Authorization", "Bearer "+secret)
	case "api_key":
		header := strings.TrimSpace(conditionValueString(node.Params["auth_header"]))
		if header == "" {
			header = "X-API-Key"
		}
		if err := validateHTTPHeader(header, secret); err != nil {
			return err
		}
		req.Header.Set(header, secret)
	case "custom_header":
		header := strings.TrimSpace(conditionValueString(node.Params["auth_header"]))
		if header == "" {
			return fmt.Errorf("HTTP auth_header is required for Custom Header authentication")
		}
		prefix := conditionValueString(node.Params["auth_prefix"])
		if err := validateHTTPHeader(header, prefix+secret); err != nil {
			return err
		}
		req.Header.Set(header, prefix+secret)
	case "basic":
		username, password, err := parseBasicCredential(secret)
		if err != nil {
			return err
		}
		req.Header.Set("Authorization", "Basic "+base64.StdEncoding.EncodeToString([]byte(username+":"+password)))
	}
	return nil
}

func parseBasicCredential(secret string) (string, string, error) {
	var payload struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if json.Unmarshal([]byte(secret), &payload) == nil && payload.Username != "" {
		return payload.Username, payload.Password, nil
	}
	username, password, ok := strings.Cut(secret, ":")
	if !ok || strings.TrimSpace(username) == "" {
		return "", "", fmt.Errorf("HTTP Basic credential must be JSON {username,password} or username:password")
	}
	return username, password, nil
}

func buildHTTPRequestBody(node *Node, method string) (io.Reader, string, error) {
	if method != "POST" && method != "PUT" && method != "PATCH" && method != "DELETE" {
		return nil, "", nil
	}
	mode := strings.ToLower(strings.TrimSpace(conditionValueString(node.Params["body_mode"])))
	if mode == "" {
		mode = "json"
	}
	body := conditionValueString(node.Params["body"])
	if int64(len(body)) > maxHTTPRequestBytes {
		return nil, "", fmt.Errorf("HTTP request body exceeds %d byte limit", maxHTTPRequestBytes)
	}
	switch mode {
	case "json":
		if body == "" {
			return nil, "", nil
		}
		return bytes.NewBufferString(body), "application/json", nil
	case "raw":
		if body == "" {
			return nil, "", nil
		}
		contentType := strings.TrimSpace(conditionValueString(node.Params["content_type"]))
		if contentType == "" {
			contentType = "text/plain; charset=utf-8"
		}
		return bytes.NewBufferString(body), contentType, nil
	case "x-www-form-urlencoded", "form":
		fields, err := parseHTTPObject(node.Params["form_fields"], "form_fields")
		if err != nil {
			return nil, "", err
		}
		if len(fields) > maxHTTPFormFields {
			return nil, "", fmt.Errorf("HTTP form_fields exceed %d item limit", maxHTTPFormFields)
		}
		values := url.Values{}
		for key, value := range fields {
			values.Set(key, conditionValueString(value))
		}
		encoded := values.Encode()
		if int64(len(encoded)) > maxHTTPRequestBytes {
			return nil, "", fmt.Errorf("HTTP form body exceeds %d byte limit", maxHTTPRequestBytes)
		}
		return strings.NewReader(encoded), "application/x-www-form-urlencoded", nil
	case "multipart/form-data", "multipart":
		fields, err := parseHTTPObject(node.Params["form_fields"], "form_fields")
		if err != nil {
			return nil, "", err
		}
		if len(fields) > maxHTTPFormFields {
			return nil, "", fmt.Errorf("HTTP form_fields exceed %d item limit", maxHTTPFormFields)
		}
		var buffer bytes.Buffer
		writer := multipart.NewWriter(&buffer)
		for key, value := range fields {
			if err := writer.WriteField(key, conditionValueString(value)); err != nil {
				return nil, "", err
			}
		}
		if err := writer.Close(); err != nil {
			return nil, "", err
		}
		if int64(buffer.Len()) > maxHTTPRequestBytes {
			return nil, "", fmt.Errorf("HTTP multipart body exceeds %d byte limit", maxHTTPRequestBytes)
		}
		return bytes.NewReader(buffer.Bytes()), writer.FormDataContentType(), nil
	default:
		return nil, "", fmt.Errorf("unsupported HTTP body mode %q", mode)
	}
}

func parseHTTPPositiveInt(raw interface{}, fallback, max int) (int, error) {
	if raw == nil || strings.TrimSpace(conditionValueString(raw)) == "" {
		return fallback, nil
	}
	value, err := strconv.Atoi(strings.TrimSpace(conditionValueString(raw)))
	if err != nil || value < 1 || value > max {
		return 0, fmt.Errorf("must be between 1 and %d", max)
	}
	return value, nil
}

func getJSONField(data interface{}, path string) interface{} {
	path = strings.TrimSpace(path)
	if path == "" {
		return data
	}
	current := data
	for _, part := range strings.Split(path, ".") {
		object, ok := current.(map[string]interface{})
		if !ok {
			return nil
		}
		current = object[part]
	}
	return current
}
