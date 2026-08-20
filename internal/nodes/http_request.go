package nodes

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"goflow/internal/apperror"
	"goflow/internal/jsoncontract"
	"goflow/internal/sourceprobe"
)

const (
	maxHTTPResponseBytes  int64 = 10 << 20
	maxHTTPRequestBytes   int64 = 1 << 20
	maxHTTPHeaders              = 32
	maxHTTPHeaderNameLen        = 128
	maxHTTPHeaderValueLen       = 4096
)

type HTTPRequestExecutor struct {
	client *http.Client
}

func NewHTTPRequestExecutor() *HTTPRequestExecutor {
	return &HTTPRequestExecutor{client: &http.Client{Timeout: 30 * time.Second, CheckRedirect: sourceprobe.SafeRedirect}}
}

func NewHTTPRequestExecutorWithClient(client *http.Client) *HTTPRequestExecutor {
	if client == nil {
		return NewHTTPRequestExecutor()
	}
	bounded := *client
	bounded.CheckRedirect = sourceprobe.SafeRedirect
	return &HTTPRequestExecutor{client: &bounded}
}

func declaredHTTPResponseContract(params map[string]interface{}) (interface{}, bool) {
	raw, ok := params["response_contract"]
	if !ok || raw == nil {
		return nil, false
	}
	if text, ok := raw.(string); ok && strings.TrimSpace(text) == "" {
		return nil, false
	}
	return raw, true
}

func (e *HTTPRequestExecutor) Execute(ctx *ExecutionContext, node *Node) (interface{}, error) {
	contractRaw, strictContract := declaredHTTPResponseContract(node.Params)
	urlStr, _ := node.Params["url"].(string)
	method, _ := node.Params["method"].(string)
	method = strings.ToUpper(strings.TrimSpace(method))
	if method == "" {
		method = "GET"
	}
	if !allowedHTTPMethod(method) {
		return nil, fmt.Errorf("HTTP method %q is not supported", method)
	}
	if err := validateHTTPRequestURL(urlStr); err != nil {
		return nil, err
	}

	bodyStr, _ := node.Params["body"].(string)
	if int64(len(bodyStr)) > maxHTTPRequestBytes {
		return nil, fmt.Errorf("HTTP request body exceeds %d byte limit", maxHTTPRequestBytes)
	}
	headersMapStr, _ := node.Params["headers"].(string)

	var reqBody io.Reader
	if bodyStr != "" && (method == "POST" || method == "PUT" || method == "PATCH") {
		reqBody = bytes.NewBufferString(bodyStr)
	}
	req, err := http.NewRequestWithContext(ctx.Context, method, urlStr, reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	if reqBody != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if headersMapStr != "" {
		var headers map[string]string
		if err := json.Unmarshal([]byte(headersMapStr), &headers); err != nil {
			return nil, fmt.Errorf("HTTP headers must be a JSON object of strings")
		}
		if len(headers) > maxHTTPHeaders {
			return nil, fmt.Errorf("HTTP headers exceed %d item limit", maxHTTPHeaders)
		}
		for k, v := range headers {
			if err := validateHTTPHeader(k, v); err != nil {
				return nil, err
			}
			req.Header.Set(k, v)
		}
	}
	if err := applyHTTPRequestCredential(ctx, node, req); err != nil {
		return nil, err
	}

	resp, err := e.client.Do(req)
	if err != nil {
		if strictContract {
			if errors.Is(err, context.DeadlineExceeded) || errors.Is(ctx.Context.Err(), context.DeadlineExceeded) {
				return nil, apperror.New("source_timeout", "The source request timed out.")
			}
			return nil, apperror.New("source_unreachable", "Goflow could not connect to the source endpoint.")
		}
		return nil, fmt.Errorf("http request failed: %w", err)
	}
	defer resp.Body.Close()

	respBytes, err := io.ReadAll(io.LimitReader(resp.Body, maxHTTPResponseBytes+1))
	if err != nil {
		if strictContract {
			return nil, apperror.New("source_unreachable", "Goflow could not read the source response.")
		}
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}
	if int64(len(respBytes)) > maxHTTPResponseBytes {
		if strictContract {
			return nil, apperror.New("source_response_too_large", "The source response is larger than the allowed limit.")
		}
		return nil, fmt.Errorf("HTTP response exceeds %d byte limit", maxHTTPResponseBytes)
	}

	var jsonResult interface{}
	if strictContract {
		contract, err := jsoncontract.Parse(contractRaw)
		if err != nil {
			return nil, fmt.Errorf("invalid response_contract: %w", err)
		}
		result, err := sourceprobe.ValidateResponse(resp.StatusCode, resp.Header.Get("Content-Type"), respBytes, contract)
		if err != nil {
			return nil, err
		}
		jsonResult = result.Data
	} else if err := json.Unmarshal(respBytes, &jsonResult); err != nil {
		jsonResult = string(respBytes)
	}

	return map[string]interface{}{"status_code": resp.StatusCode, "headers": resp.Header, "data": jsonResult}, nil
}

func applyHTTPRequestCredential(ctx *ExecutionContext, node *Node, req *http.Request) error {
	credID, _ := node.Params["credential_id"].(string)
	if strings.TrimSpace(credID) == "" {
		return nil
	}
	secret, ok := ctx.Credentials[credID]
	if !ok || strings.TrimSpace(secret) == "" {
		return fmt.Errorf("HTTP credential is not available")
	}
	header, _ := node.Params["credential_header"].(string)
	if strings.TrimSpace(header) == "" {
		header = "Authorization"
	}
	prefix, _ := node.Params["credential_prefix"].(string)
	if prefix == "" {
		prefix = "Bearer "
	}
	if err := validateHTTPHeader(header, prefix+secret); err != nil {
		return err
	}
	req.Header.Set(header, prefix+secret)
	return nil
}

func (e *HTTPRequestExecutor) Validate(node *Node) error {
	urlStr, _ := node.Params["url"].(string)
	if err := validateHTTPRequestURL(urlStr); err != nil {
		return err
	}
	method, _ := node.Params["method"].(string)
	method = strings.ToUpper(strings.TrimSpace(method))
	if method != "" && !allowedHTTPMethod(method) {
		return fmt.Errorf("HTTP method %q is not supported", method)
	}
	if raw, ok := declaredHTTPResponseContract(node.Params); ok {
		if _, err := jsoncontract.Parse(raw); err != nil {
			return err
		}
	}
	if header, ok := node.Params["credential_header"].(string); ok && strings.TrimSpace(header) != "" {
		if err := validateHTTPHeader(header, "placeholder"); err != nil {
			return err
		}
	}
	return nil
}

func (e *HTTPRequestExecutor) GetDefinition() NodeDefinition {
	return NodeDefinition{
		Type: TypeHTTPRequest, Name: "HTTP Request", Description: "Sends HTTP API requests such as GET, POST, PUT, and DELETE", Icon: "Globe", Category: "ACTION", Retryable: true,
		Params: []ParamDefinition{
			{Name: "method", Label: "HTTP Method", Type: "select", Default: "GET", Options: []string{"GET", "POST", "PUT", "DELETE", "PATCH"}, Required: true, Description: "HTTP method"},
			{Name: "url", Label: "Target URL", Type: "text", Default: "https://api.github.com", Required: true, Description: "Target request URL"},
			{Name: "headers", Label: "Headers (JSON)", Type: "json", Default: "{}", Required: false, Description: "Custom non-secret headers as a JSON object"},
			{Name: "body", Label: "Request Body", Type: "textarea", Default: "", Required: false, Description: "Request body for POST, PUT, or PATCH"},
			{Name: "credential_id", Label: "Bearer/API Credential", Type: "credential", Default: "", Required: false, Description: "Encrypted credential injected into a request header at runtime"},
			{Name: "credential_header", Label: "Credential Header", Type: "text", Default: "Authorization", Required: false, Description: "Header that receives the encrypted credential"},
			{Name: "credential_prefix", Label: "Credential Prefix", Type: "text", Default: "Bearer ", Required: false, Description: "Prefix placed before the encrypted credential"},
			{Name: "response_contract", Label: "JSON Response Contract", Type: "json", Default: "", Required: false, Description: "Optional required JSON fields and type constraints"},
		},
	}
}

func allowedHTTPMethod(method string) bool {
	switch method {
	case "GET", "POST", "PUT", "DELETE", "PATCH", "HEAD":
		return true
	default:
		return false
	}
}

func validateHTTPRequestURL(raw string) error {
	if strings.TrimSpace(raw) == "" {
		return fmt.Errorf("HTTP Node requires a 'url' parameter")
	}
	parsed, err := url.Parse(raw)
	if err != nil {
		return fmt.Errorf("HTTP url must be an absolute http or https URL: %w", err)
	}
	if !parsed.IsAbs() || parsed.Host == "" {
		return fmt.Errorf("HTTP url must be an absolute http or https URL")
	}
	switch strings.ToLower(parsed.Scheme) {
	case "http", "https":
		return nil
	default:
		return fmt.Errorf("HTTP url must be an absolute http or https URL")
	}
}

func validateHTTPHeader(name, value string) error {
	if strings.TrimSpace(name) == "" {
		return fmt.Errorf("HTTP header name is required")
	}
	if len(name) > maxHTTPHeaderNameLen {
		return fmt.Errorf("HTTP header name exceeds %d byte limit", maxHTTPHeaderNameLen)
	}
	if len(value) > maxHTTPHeaderValueLen {
		return fmt.Errorf("HTTP header %q value exceeds %d byte limit", name, maxHTTPHeaderValueLen)
	}
	for _, r := range name {
		if r <= 32 || r >= 127 || strings.ContainsRune("()<>@,;:\\\"/[]?={} \t", r) {
			return fmt.Errorf("HTTP header name %q is invalid", name)
		}
	}
	return nil
}
