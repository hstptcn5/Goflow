package nodes

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
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
	return &HTTPRequestExecutor{
		client: &http.Client{
			Timeout:       30 * time.Second,
			CheckRedirect: safeHTTPRedirect,
		},
	}
}

func NewHTTPRequestExecutorWithClient(client *http.Client) *HTTPRequestExecutor {
	if client == nil {
		return NewHTTPRequestExecutor()
	}
	return &HTTPRequestExecutor{client: client}
}

func (e *HTTPRequestExecutor) Execute(ctx *ExecutionContext, node *Node) (interface{}, error) {
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

	// Default Content-Type n???u c?? body
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

	resp, err := e.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("http request failed: %w", err)
	}
	defer resp.Body.Close()

	respBytes, err := io.ReadAll(io.LimitReader(resp.Body, maxHTTPResponseBytes+1))
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}
	if int64(len(respBytes)) > maxHTTPResponseBytes {
		return nil, fmt.Errorf("HTTP response exceeds %d byte limit", maxHTTPResponseBytes)
	}

	var jsonResult interface{}
	if err := json.Unmarshal(respBytes, &jsonResult); err != nil {
		// Tr??? v??? d???ng string n???u kh??ng ph???i JSON
		jsonResult = string(respBytes)
	}

	return map[string]interface{}{
		"status_code": resp.StatusCode,
		"headers":     resp.Header,
		"data":        jsonResult,
	}, nil
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
	return nil
}

func (e *HTTPRequestExecutor) GetDefinition() NodeDefinition {
	return NodeDefinition{
		Type:        TypeHTTPRequest,
		Name:        "HTTP Request",
		Description: "Sends HTTP API requests such as GET, POST, PUT, and DELETE",
		Icon:        "Globe",
		Category:    "ACTION",
		Retryable:   true,
		Params: []ParamDefinition{
			{
				Name:        "method",
				Label:       "HTTP Method",
				Type:        "select",
				Default:     "GET",
				Options:     []string{"GET", "POST", "PUT", "DELETE", "PATCH"},
				Required:    true,
				Description: "HTTP method",
			},
			{
				Name:        "url",
				Label:       "Target URL",
				Type:        "text",
				Default:     "https://api.github.com",
				Required:    true,
				Description: "Target request URL",
			},
			{
				Name:        "headers",
				Label:       "Headers (JSON)",
				Type:        "json",
				Default:     "{}",
				Required:    false,
				Description: "Custom headers as a JSON object",
			},
			{
				Name:        "body",
				Label:       "Request Body",
				Type:        "textarea",
				Default:     "",
				Required:    false,
				Description: "Request body for POST, PUT, or PATCH",
			},
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

func safeHTTPRedirect(req *http.Request, via []*http.Request) error {
	if len(via) >= 10 {
		return fmt.Errorf("stopped after 10 redirects")
	}
	previous := via[len(via)-1]
	carriesAuth := previous.Header.Get("Authorization") != "" || previous.Header.Get("Cookie") != ""
	if carriesAuth && !sameHTTPOrigin(previous.URL, req.URL) {
		return http.ErrUseLastResponse
	}
	return nil
}

func sameHTTPOrigin(a, b *url.URL) bool {
	return strings.EqualFold(a.Scheme, b.Scheme) && strings.EqualFold(a.Host, b.Host)
}
