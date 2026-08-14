package adapter

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"goflow/internal/apperror"
	"goflow/internal/jsoncontract"
	"goflow/internal/sourceprobe"
)

const (
	MaxResponseBytes = 1 << 20
	MaxPages         = 20
	MaxItems         = 5000
	MaxCursorLength  = 512
	MaxRetries       = 2
	MaxRetryAfter    = 5 * time.Second
)

type WaitFunc func(context.Context, time.Duration) error

type HTTPSource struct {
	client *http.Client
	wait   WaitFunc
}

type Request struct {
	URL              string
	AuthMode         string
	Credential       string
	APIKeyHeader     string
	Pagination       string
	CursorQueryParam string
	ItemsField       string
	NextCursorField  string
	MaxPages         int
	MaxItems         int
	Contract         jsoncontract.Contract
}

func NewHTTPSource(client *http.Client, wait WaitFunc) *HTTPSource {
	if client == nil {
		client = &http.Client{Timeout: 30 * time.Second}
	}
	bounded := *client
	if bounded.Timeout <= 0 || bounded.Timeout > 30*time.Second {
		bounded.Timeout = 30 * time.Second
	}
	bounded.CheckRedirect = sourceprobe.SafeRedirect
	if wait == nil {
		wait = func(ctx context.Context, delay time.Duration) error {
			timer := time.NewTimer(delay)
			defer timer.Stop()
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-timer.C:
				return nil
			}
		}
	}
	return &HTTPSource{client: &bounded, wait: wait}
}

func ValidateRequest(req Request) error {
	if err := sourceprobe.ValidateURL(req.URL); err != nil {
		return fmt.Errorf("adapter url must be an absolute http or https URL")
	}
	parsedURL, _ := url.Parse(req.URL)
	if parsedURL.User != nil {
		return fmt.Errorf("adapter url must not contain user information")
	}
	switch req.AuthMode {
	case "none":
		if req.Credential != "" {
			return fmt.Errorf("adapter auth mode none must not include a credential")
		}
	case "bearer":
		if req.Credential == "" {
			return fmt.Errorf("adapter bearer auth requires a credential")
		}
	case "api_key":
		if req.Credential == "" {
			return fmt.Errorf("adapter API-key auth requires a credential")
		}
		if err := validateHeaderName(req.APIKeyHeader); err != nil {
			return err
		}
	default:
		return fmt.Errorf("adapter auth_mode must be none, bearer, or api_key")
	}
	if len(req.Contract.Required) == 0 {
		return fmt.Errorf("adapter requires response_contract")
	}
	if req.Pagination == "none" {
		return nil
	}
	if req.Pagination != "cursor" {
		return fmt.Errorf("adapter pagination must be none or cursor")
	}
	for label, value := range map[string]string{"cursor_query_param": req.CursorQueryParam, "items_field": req.ItemsField, "next_cursor_field": req.NextCursorField} {
		if !validFieldName(value) {
			return fmt.Errorf("adapter %s is invalid", label)
		}
	}
	if req.MaxPages < 1 || req.MaxPages > MaxPages {
		return fmt.Errorf("adapter max_pages must be between 1 and %d", MaxPages)
	}
	if req.MaxItems < 1 || req.MaxItems > MaxItems {
		return fmt.Errorf("adapter max_items must be between 1 and %d", MaxItems)
	}
	return nil
}

func (source *HTTPSource) Fetch(ctx context.Context, request Request) (interface{}, error) {
	if err := ValidateRequest(request); err != nil {
		return nil, apperror.New("source_invalid_url", "The adapter configuration is invalid.")
	}
	if request.Pagination == "none" {
		data, err := source.fetchPage(ctx, request, "")
		if err != nil {
			return nil, err
		}
		normalized, err := normalizeObject(data, request.Contract)
		if err != nil {
			return nil, apperror.New("source_contract_invalid", "The source data does not match the adapter contract.")
		}
		return normalized, nil
	}
	items := make([]interface{}, 0)
	seen := map[string]bool{}
	cursor := ""
	for page := 1; page <= request.MaxPages; page++ {
		if cursor != "" {
			if seen[cursor] {
				return nil, apperror.New("source_contract_invalid", "The source repeated a pagination cursor.")
			}
			seen[cursor] = true
		}
		data, err := source.fetchPage(ctx, request, cursor)
		if err != nil {
			return nil, err
		}
		object, ok := data.(map[string]interface{})
		if !ok {
			return nil, apperror.New("source_contract_invalid", "The source page must be a JSON object.")
		}
		pageItems, ok := object[request.ItemsField].([]interface{})
		if !ok {
			return nil, apperror.New("source_contract_invalid", "The source page items field must be an array.")
		}
		if len(items)+len(pageItems) > request.MaxItems {
			return nil, apperror.New("source_response_too_large", "The adapter item limit was exceeded.")
		}
		for _, item := range pageItems {
			normalized, err := normalizeObject(item, request.Contract)
			if err != nil {
				return nil, apperror.New("source_contract_invalid", "A source item does not match the adapter contract.")
			}
			items = append(items, normalized)
		}
		next := object[request.NextCursorField]
		if next == nil || next == "" {
			return map[string]interface{}{"items": items, "page_count": page}, nil
		}
		cursor, ok = next.(string)
		if !ok || len(cursor) > MaxCursorLength {
			return nil, apperror.New("source_contract_invalid", "The source next cursor is invalid.")
		}
	}
	return nil, apperror.New("source_response_too_large", "The adapter page limit was exceeded.")
}

func normalizeObject(value interface{}, contract jsoncontract.Contract) (map[string]interface{}, error) {
	if _, err := jsoncontract.Validate(value, contract); err != nil {
		return nil, err
	}
	object := value.(map[string]interface{})
	normalized := make(map[string]interface{}, len(contract.Required))
	for field := range contract.Required {
		normalized[field] = object[field]
	}
	return normalized, nil
}

func (source *HTTPSource) fetchPage(ctx context.Context, request Request, cursor string) (interface{}, error) {
	for attempt := 0; attempt <= MaxRetries; attempt++ {
		pageURL, err := url.Parse(request.URL)
		if err != nil {
			return nil, apperror.New("source_invalid_url", "The adapter URL is invalid.")
		}
		if cursor != "" {
			query := pageURL.Query()
			query.Set(request.CursorQueryParam, cursor)
			pageURL.RawQuery = query.Encode()
		}
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, pageURL.String(), nil)
		if err != nil {
			return nil, apperror.New("source_invalid_url", "The adapter URL is invalid.")
		}
		req.Header.Set("Accept", "application/json")
		switch request.AuthMode {
		case "bearer":
			req.Header.Set("Authorization", "Bearer "+request.Credential)
		case "api_key":
			req.Header.Set(request.APIKeyHeader, request.Credential)
		}
		client := *source.client
		client.CheckRedirect = func(redirect *http.Request, via []*http.Request) error {
			if err := sourceprobe.SafeRedirect(redirect, via); err != nil {
				return err
			}
			if request.AuthMode == "api_key" && len(via) > 0 && !sameOrigin(via[len(via)-1].URL, redirect.URL) {
				redirect.Header.Del(request.APIKeyHeader)
			}
			return nil
		}
		response, err := client.Do(req)
		if err != nil {
			if errors.Is(err, context.DeadlineExceeded) || errors.Is(ctx.Err(), context.DeadlineExceeded) {
				return nil, apperror.New("source_timeout", "The adapter source timed out.")
			}
			if errors.Is(err, context.Canceled) || errors.Is(ctx.Err(), context.Canceled) {
				return nil, apperror.New(apperror.CategoryCancelled, "The adapter request was cancelled.")
			}
			return nil, apperror.New("source_unreachable", "Goflow could not reach the adapter source.")
		}
		body, readErr := io.ReadAll(io.LimitReader(response.Body, MaxResponseBytes+1))
		response.Body.Close()
		if readErr != nil {
			return nil, apperror.New("source_unreachable", "Goflow could not read the adapter response.")
		}
		if len(body) > MaxResponseBytes {
			return nil, apperror.New("source_response_too_large", "The adapter response is too large.")
		}
		if response.StatusCode == http.StatusTooManyRequests {
			delay, ok := parseRetryAfter(response.Header.Get("Retry-After"))
			if !ok || attempt == MaxRetries {
				return nil, apperror.New(apperror.CategoryRateLimited, "The adapter source is rate limited.")
			}
			if err := source.wait(ctx, delay); err != nil {
				return nil, apperror.New(apperror.CategoryCancelled, "The adapter retry was cancelled.")
			}
			continue
		}
		if response.StatusCode < 200 || response.StatusCode >= 300 {
			return nil, apperror.New("source_http_error", "The adapter source returned an error response.")
		}
		decoded, err := jsoncontract.Decode(body)
		if err != nil {
			return nil, apperror.New("source_invalid_json", "The adapter source returned invalid JSON.")
		}
		return decoded, nil
	}
	return nil, apperror.New(apperror.CategoryRateLimited, "The adapter source is rate limited.")
}

func sameOrigin(first, second *url.URL) bool {
	return strings.EqualFold(first.Scheme, second.Scheme) && strings.EqualFold(first.Host, second.Host)
}

func parseRetryAfter(value string) (time.Duration, bool) {
	seconds, err := strconv.Atoi(strings.TrimSpace(value))
	if err != nil || seconds < 0 {
		return 0, false
	}
	delay := time.Duration(seconds) * time.Second
	return delay, delay <= MaxRetryAfter
}

func validFieldName(value string) bool {
	if value == "" || len(value) > 64 {
		return false
	}
	for _, char := range value {
		if (char >= 'a' && char <= 'z') || (char >= 'A' && char <= 'Z') || (char >= '0' && char <= '9') || char == '_' {
			continue
		}
		return false
	}
	return true
}

func validateHeaderName(value string) error {
	if !validFieldName(strings.ReplaceAll(value, "-", "_")) {
		return fmt.Errorf("adapter api_key_header is invalid")
	}
	lower := strings.ToLower(value)
	if lower == "authorization" || lower == "cookie" || lower == "proxy-authorization" {
		return fmt.Errorf("adapter api_key_header is reserved")
	}
	return nil
}

func ParseContract(raw interface{}) (jsoncontract.Contract, error) {
	if raw == nil || raw == "" {
		return jsoncontract.Contract{}, fmt.Errorf("response_contract is required")
	}
	return jsoncontract.Parse(raw)
}

func ParsePositiveInt(raw interface{}, defaultValue int) (int, error) {
	if raw == nil {
		return defaultValue, nil
	}
	switch value := raw.(type) {
	case float64:
		if value == float64(int(value)) {
			return int(value), nil
		}
	case json.Number:
		parsed, err := strconv.Atoi(value.String())
		if err == nil {
			return parsed, nil
		}
	case int:
		return value, nil
	}
	return 0, fmt.Errorf("must be an integer")
}
