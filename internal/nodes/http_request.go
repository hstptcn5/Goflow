package nodes

import (
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

type httpPageResult struct {
	StatusCode int
	Headers    http.Header
	Data       interface{}
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

func normalizedHTTPPaginationMode(raw interface{}) (string, error) {
	mode := strings.ToLower(strings.TrimSpace(conditionValueString(raw)))
	switch mode {
	case "", "none":
		return "none", nil
	case "cursor":
		return "cursor", nil
	case "page", "page_number", "page number":
		return "page_number", nil
	default:
		return "", fmt.Errorf("unsupported HTTP pagination mode %q", mode)
	}
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
	baseURL, err := appendHTTPQuery(urlStr, node.Params["query_params"])
	if err != nil {
		return nil, err
	}
	pagination, err := normalizedHTTPPaginationMode(node.Params["pagination_mode"])
	if err != nil {
		return nil, err
	}
	maxPages, err := parseHTTPPositiveInt(node.Params["max_pages"], 5, maxHTTPPages)
	if err != nil {
		return nil, fmt.Errorf("HTTP max_pages %w", err)
	}

	if pagination == "none" {
		page, err := e.executeHTTPPage(ctx, node, method, baseURL)
		if err != nil {
			return nil, err
		}
		return map[string]interface{}{"status_code": page.StatusCode, "headers": page.Headers, "data": page.Data}, nil
	}

	itemsField := strings.TrimSpace(conditionValueString(node.Params["items_field"]))
	if itemsField == "" {
		itemsField = "items"
	}
	allItems := make([]interface{}, 0)
	pageSummaries := make([]map[string]interface{}, 0, maxPages)
	currentCursor := strings.TrimSpace(conditionValueString(node.Params["cursor_start"]))
	startPage, err := parseHTTPPositiveInt(node.Params["start_page"], 1, 1_000_000)
	if err != nil {
		return nil, fmt.Errorf("HTTP start_page %w", err)
	}
	var last httpPageResult

	for pageIndex := 0; pageIndex < maxPages; pageIndex++ {
		pageURL, err := url.Parse(baseURL)
		if err != nil {
			return nil, err
		}
		query := pageURL.Query()
		if pagination == "cursor" {
			cursorParam := strings.TrimSpace(conditionValueString(node.Params["cursor_query_param"]))
			if cursorParam == "" {
				cursorParam = "cursor"
			}
			if currentCursor != "" {
				query.Set(cursorParam, currentCursor)
			}
		} else {
			pageParam := strings.TrimSpace(conditionValueString(node.Params["page_query_param"]))
			if pageParam == "" {
				pageParam = "page"
			}
			query.Set(pageParam, fmt.Sprint(startPage+pageIndex))
		}
		pageURL.RawQuery = query.Encode()

		last, err = e.executeHTTPPage(ctx, node, method, pageURL.String())
		if err != nil {
			return nil, err
		}
		pageItems := httpPageItems(last.Data, itemsField)
		allItems = append(allItems, pageItems...)
		pageSummaries = append(pageSummaries, map[string]interface{}{
			"page":        pageIndex + 1,
			"status_code": last.StatusCode,
			"item_count":  len(pageItems),
		})

		if pagination == "cursor" {
			nextField := strings.TrimSpace(conditionValueString(node.Params["next_cursor_field"]))
			if nextField == "" {
				nextField = "next_cursor"
			}
			next := strings.TrimSpace(conditionValueString(getJSONField(last.Data, nextField)))
			if next == "" || next == currentCursor {
				break
			}
			currentCursor = next
		} else if len(pageItems) == 0 {
			break
		}
	}

	return map[string]interface{}{
		"status_code": last.StatusCode,
		"headers":     last.Headers,
		"data":        allItems,
		"pages":       pageSummaries,
		"page_count":  len(pageSummaries),
		"item_count":  len(allItems),
	}, nil
}

func httpPageItems(data interface{}, itemsField string) []interface{} {
	if direct, ok := data.([]interface{}); ok {
		return direct
	}
	value := getJSONField(data, itemsField)
	items, _ := value.([]interface{})
	return items
}

func (e *HTTPRequestExecutor) executeHTTPPage(ctx *ExecutionContext, node *Node, method, requestURL string) (httpPageResult, error) {
	contractRaw, strictContract := declaredHTTPResponseContract(node.Params)
	body, contentType, err := buildHTTPRequestBody(node, method)
	if err != nil {
		return httpPageResult{}, err
	}
	req, err := http.NewRequestWithContext(ctx.Context, method, requestURL, body)
	if err != nil {
		return httpPageResult{}, fmt.Errorf("failed to create request: %w", err)
	}
	if body != nil && contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}

	headers, err := parseHTTPObject(node.Params["headers"], "headers")
	if err != nil {
		return httpPageResult{}, err
	}
	if len(headers) > maxHTTPHeaders {
		return httpPageResult{}, fmt.Errorf("HTTP headers exceed %d item limit", maxHTTPHeaders)
	}
	for key, raw := range headers {
		value, ok := raw.(string)
		if !ok {
			return httpPageResult{}, fmt.Errorf("HTTP header %q value must be a string", key)
		}
		if err := validateHTTPHeader(key, value); err != nil {
			return httpPageResult{}, err
		}
		req.Header.Set(key, value)
	}
	if err := applyHTTPAuthentication(ctx, node, req); err != nil {
		return httpPageResult{}, err
	}

	resp, err := e.client.Do(req)
	if err != nil {
		if strictContract {
			if errors.Is(err, context.DeadlineExceeded) || errors.Is(ctx.Context.Err(), context.DeadlineExceeded) {
				return httpPageResult{}, apperror.New("source_timeout", "The source request timed out.")
			}
			return httpPageResult{}, apperror.New("source_unreachable", "Goflow could not connect to the source endpoint.")
		}
		return httpPageResult{}, fmt.Errorf("http request failed: %w", err)
	}
	defer resp.Body.Close()

	respBytes, err := io.ReadAll(io.LimitReader(resp.Body, maxHTTPResponseBytes+1))
	if err != nil {
		if strictContract {
			return httpPageResult{}, apperror.New("source_unreachable", "Goflow could not read the source response.")
		}
		return httpPageResult{}, fmt.Errorf("failed to read response body: %w", err)
	}
	if int64(len(respBytes)) > maxHTTPResponseBytes {
		if strictContract {
			return httpPageResult{}, apperror.New("source_response_too_large", "The source response is larger than the allowed limit.")
		}
		return httpPageResult{}, fmt.Errorf("HTTP response exceeds %d byte limit", maxHTTPResponseBytes)
	}

	var data interface{}
	if strictContract {
		contract, err := jsoncontract.Parse(contractRaw)
		if err != nil {
			return httpPageResult{}, fmt.Errorf("invalid response_contract: %w", err)
		}
		result, err := sourceprobe.ValidateResponse(resp.StatusCode, resp.Header.Get("Content-Type"), respBytes, contract)
		if err != nil {
			return httpPageResult{}, err
		}
		data = result.Data
	} else {
		mode := strings.ToLower(strings.TrimSpace(conditionValueString(node.Params["response_mode"])))
		if mode == "" {
			mode = "auto"
		}
		switch mode {
		case "auto":
			if err := json.Unmarshal(respBytes, &data); err != nil {
				data = string(respBytes)
			}
		case "json":
			if err := json.Unmarshal(respBytes, &data); err != nil {
				return httpPageResult{}, fmt.Errorf("HTTP response is not valid JSON: %w", err)
			}
		case "text":
			data = string(respBytes)
		default:
			return httpPageResult{}, fmt.Errorf("unsupported HTTP response mode %q", mode)
		}
	}

	return httpPageResult{StatusCode: resp.StatusCode, Headers: resp.Header, Data: data}, nil
}

// applyHTTPRequestCredential preserves the pre-v2 custom credential-header behavior.
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
	if _, err := appendHTTPQuery(urlStr, node.Params["query_params"]); err != nil {
		return err
	}
	method, _ := node.Params["method"].(string)
	method = strings.ToUpper(strings.TrimSpace(method))
	if method != "" && !allowedHTTPMethod(method) {
		return fmt.Errorf("HTTP method %q is not supported", method)
	}
	if _, err := normalizeHTTPAuthMode(node.Params["auth_mode"]); err != nil {
		return err
	}
	if _, err := normalizedHTTPPaginationMode(node.Params["pagination_mode"]); err != nil {
		return err
	}
	if _, err := parseHTTPPositiveInt(node.Params["max_pages"], 5, maxHTTPPages); err != nil {
		return fmt.Errorf("HTTP max_pages %w", err)
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
	if header := strings.TrimSpace(conditionValueString(node.Params["auth_header"])); header != "" {
		if err := validateHTTPHeader(header, "placeholder"); err != nil {
			return err
		}
	}
	return nil
}

func (e *HTTPRequestExecutor) GetDefinition() NodeDefinition {
	return NodeDefinition{
		Type: TypeHTTPRequest, Name: "HTTP Request", Description: "Sends bounded HTTP requests with structured query, generic auth, body/response modes and pagination", Icon: "Globe", Category: "ACTION", Retryable: true,
		Params: []ParamDefinition{
			{Name: "method", Label: "HTTP Method", Type: "select", Default: "GET", Options: []string{"GET", "HEAD", "POST", "PUT", "DELETE", "PATCH"}, Required: true, Description: "HTTP method"},
			{Name: "url", Label: "Target URL", Type: "text", Default: "https://api.github.com", Required: true, Description: "Target request URL"},
			{Name: "query_params", Label: "Query Parameters", Type: "json", Default: "{}", Required: false, Description: "Structured query parameters as a JSON object; array values become repeated query keys"},
			{Name: "headers", Label: "Headers (JSON)", Type: "json", Default: "{}", Required: false, Description: "Custom non-secret headers as a JSON object"},
			{Name: "auth_mode", Label: "Authentication", Type: "select", Default: "legacy", Options: []string{"legacy", "none", "bearer", "api_key", "basic", "oauth2", "custom_header"}, Required: false, Description: "Generic authentication mode. Legacy preserves existing credential_header/prefix workflows."},
			{Name: "credential_id", Label: "Credential", Type: "credential", Default: "", Required: false, Description: "Encrypted credential used by the selected authentication mode"},
			{Name: "auth_header", Label: "Auth Header", Type: "text", Default: "X-API-Key", Required: false, Description: "Header name for API Key or Custom Header authentication"},
			{Name: "auth_prefix", Label: "Auth Prefix", Type: "text", Default: "", Required: false, Description: "Optional prefix for Custom Header authentication"},
			{Name: "credential_header", Label: "Legacy Credential Header", Type: "text", Default: "Authorization", Required: false, Description: "Legacy header that receives the encrypted credential"},
			{Name: "credential_prefix", Label: "Legacy Credential Prefix", Type: "text", Default: "Bearer ", Required: false, Description: "Legacy prefix placed before the encrypted credential"},
			{Name: "body_mode", Label: "Request Body Mode", Type: "select", Default: "json", Options: []string{"json", "raw", "x-www-form-urlencoded", "multipart/form-data"}, Required: false, Description: "Body encoding. File mode is added when FileRef is available."},
			{Name: "body", Label: "Request Body", Type: "textarea", Default: "", Required: false, Description: "JSON or raw request body"},
			{Name: "content_type", Label: "Raw Content Type", Type: "text", Default: "text/plain; charset=utf-8", Required: false, Description: "Content-Type used for Raw body mode"},
			{Name: "form_fields", Label: "Form Fields", Type: "json", Default: "{}", Required: false, Description: "Fields for urlencoded or multipart form bodies"},
			{Name: "response_mode", Label: "Response Mode", Type: "select", Default: "auto", Options: []string{"auto", "json", "text"}, Required: false, Description: "Auto attempts JSON then falls back to text. File mode is added with FileRef."},
			{Name: "pagination_mode", Label: "Pagination", Type: "select", Default: "none", Options: []string{"none", "cursor", "page_number"}, Required: false, Description: "Bounded cursor or page-number pagination"},
			{Name: "items_field", Label: "Items Field", Type: "text", Default: "items", Required: false, Description: "Dot path to the response array; direct array responses are also accepted"},
			{Name: "max_pages", Label: "Maximum Pages", Type: "integer", Default: 5, Required: false, Description: "Maximum pagination requests, up to 100"},
			{Name: "cursor_query_param", Label: "Cursor Query Parameter", Type: "text", Default: "cursor", Required: false},
			{Name: "cursor_start", Label: "Initial Cursor", Type: "text", Default: "", Required: false},
			{Name: "next_cursor_field", Label: "Next Cursor Field", Type: "text", Default: "next_cursor", Required: false, Description: "Dot path to the next cursor in a cursor response"},
			{Name: "page_query_param", Label: "Page Query Parameter", Type: "text", Default: "page", Required: false},
			{Name: "start_page", Label: "Start Page", Type: "integer", Default: 1, Required: false},
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
