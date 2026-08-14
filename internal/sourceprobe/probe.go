package sourceprobe

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"goflow/internal/apperror"
	"goflow/internal/jsoncontract"
)

const (
	DefaultMaxResponseBytes int64 = 1 << 20
	DefaultTimeout                = 10 * time.Second
	maxRedirects                  = 5
)

type Result struct {
	Data    interface{}
	Summary jsoncontract.Summary
}

func Check(ctx context.Context, baseClient *http.Client, rawURL string, contract jsoncontract.Contract, maxBytes int64) (*Result, error) {
	if err := ValidateURL(rawURL); err != nil {
		return nil, apperror.New("source_invalid_url", "Use an absolute http or https JSON endpoint URL.")
	}
	if maxBytes <= 0 {
		maxBytes = DefaultMaxResponseBytes
	}
	client := boundedClient(baseClient)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, apperror.New("source_invalid_url", "Use an absolute http or https JSON endpoint URL.")
	}
	req.Header.Set("Accept", "application/json")
	resp, err := client.Do(req)
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) || errors.Is(ctx.Err(), context.DeadlineExceeded) || isTimeout(err) {
			return nil, apperror.New("source_timeout", "The source did not respond within 10 seconds.")
		}
		if errors.Is(err, context.Canceled) || errors.Is(ctx.Err(), context.Canceled) {
			return nil, apperror.New("source_unreachable", "The source test was cancelled before it completed.")
		}
		return nil, apperror.New("source_unreachable", "Goflow could not connect to the source endpoint.")
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(io.LimitReader(resp.Body, maxBytes+1))
	if err != nil {
		return nil, apperror.New("source_unreachable", "Goflow could not read the source response.")
	}
	if int64(len(data)) > maxBytes {
		return nil, apperror.New("source_response_too_large", "The source response is larger than the allowed limit.")
	}
	return ValidateResponse(resp.StatusCode, resp.Header.Get("Content-Type"), data, contract)
}

func ValidateResponse(statusCode int, contentType string, data []byte, contract jsoncontract.Contract) (*Result, error) {
	if statusCode < 200 || statusCode >= 300 {
		return nil, apperror.New("source_http_error", fmt.Sprintf("The source returned HTTP %d instead of a successful response.", statusCode))
	}
	if looksLikeHTML(contentType, data) {
		return nil, apperror.New("source_non_json", "The source returned a web page instead of JSON.")
	}
	decoded, err := jsoncontract.Decode(data)
	if err != nil {
		return nil, apperror.New("source_invalid_json", "The source response is not valid JSON.")
	}
	summary, err := jsoncontract.Validate(decoded, contract)
	if err != nil {
		return nil, apperror.New("source_contract_invalid", "The source JSON does not match the required DailyOps fields and types.")
	}
	return &Result{Data: decoded, Summary: summary}, nil
}

func ValidateURL(raw string) error {
	parsed, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || !parsed.IsAbs() || parsed.Host == "" {
		return fmt.Errorf("URL must be absolute")
	}
	switch strings.ToLower(parsed.Scheme) {
	case "http", "https":
		return nil
	default:
		return fmt.Errorf("URL scheme must be http or https")
	}
}

func SafeRedirect(req *http.Request, via []*http.Request) error {
	if len(via) >= maxRedirects {
		return fmt.Errorf("stopped after %d redirects", maxRedirects)
	}
	if len(via) == 0 {
		return nil
	}
	previous := via[len(via)-1]
	if !sameOrigin(previous.URL, req.URL) {
		for _, header := range []string{"Authorization", "Cookie", "Proxy-Authorization"} {
			req.Header.Del(header)
		}
	}
	return nil
}

func boundedClient(base *http.Client) *http.Client {
	if base == nil {
		return &http.Client{Timeout: DefaultTimeout, CheckRedirect: SafeRedirect}
	}
	client := *base
	if client.Timeout <= 0 || client.Timeout > DefaultTimeout {
		client.Timeout = DefaultTimeout
	}
	client.CheckRedirect = SafeRedirect
	return &client
}

func looksLikeHTML(contentType string, data []byte) bool {
	mediaType := strings.ToLower(strings.TrimSpace(strings.Split(contentType, ";")[0]))
	if mediaType == "text/html" || mediaType == "application/xhtml+xml" {
		return true
	}
	prefix := strings.ToLower(strings.TrimSpace(string(data)))
	return strings.HasPrefix(prefix, "<!doctype html") || strings.HasPrefix(prefix, "<html")
}

func sameOrigin(first, second *url.URL) bool {
	return strings.EqualFold(first.Scheme, second.Scheme) && strings.EqualFold(first.Host, second.Host)
}

func isTimeout(err error) bool {
	type timeout interface{ Timeout() bool }
	var candidate timeout
	return errors.As(err, &candidate) && candidate.Timeout()
}
