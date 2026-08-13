package adapter

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"goflow/internal/apperror"
	"goflow/internal/jsoncontract"
)

func TestHTTPSourceSingleObjectAndCursorPagination(t *testing.T) {
	t.Run("single object", func(t *testing.T) {
		var authorization string
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			authorization = r.Header.Get("Authorization")
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"report_date":"2026-08-09","revenue":48250.75}`))
		}))
		defer server.Close()
		contract, err := jsoncontract.Parse(map[string]interface{}{"required": map[string]interface{}{"report_date": map[string]interface{}{"type": "string", "non_empty": true}, "revenue": map[string]interface{}{"type": "number"}}})
		if err != nil {
			t.Fatal(err)
		}
		result, err := NewHTTPSource(server.Client(), nil).Fetch(context.Background(), Request{URL: server.URL, AuthMode: "bearer", Credential: "adapter-token-canary", Pagination: "none", Contract: contract})
		if err != nil {
			t.Fatal(err)
		}
		if authorization != "Bearer adapter-token-canary" {
			t.Fatalf("authorization = %q", authorization)
		}
		encoded, _ := json.Marshal(result)
		if strings.Contains(string(encoded), "adapter-token-canary") || strings.Contains(string(encoded), server.URL) {
			t.Fatalf("normalized output leaked secret or URL: %s", encoded)
		}
		if object := result.(map[string]interface{}); len(object) != 2 {
			t.Fatalf("normalized output retained undeclared fields: %#v", object)
		}
	})

	t.Run("cursor pages", func(t *testing.T) {
		var mu sync.Mutex
		var cursors []string
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			cursor := r.URL.Query().Get("after")
			mu.Lock()
			cursors = append(cursors, cursor)
			mu.Unlock()
			w.Header().Set("Content-Type", "application/json")
			if cursor == "" {
				_, _ = w.Write([]byte(`{"records":[{"id":1,"vendor":"discard"},{"id":2}],"next":"page-2"}`))
				return
			}
			_, _ = w.Write([]byte(`{"records":[{"id":3}],"next":null}`))
		}))
		defer server.Close()
		result, err := NewHTTPSource(server.Client(), nil).Fetch(context.Background(), cursorRequest(server.URL))
		if err != nil {
			t.Fatal(err)
		}
		object := result.(map[string]interface{})
		if object["page_count"] != 2 || len(object["items"].([]interface{})) != 3 {
			t.Fatalf("unexpected normalized output: %#v", object)
		}
		if first := object["items"].([]interface{})[0].(map[string]interface{}); len(first) != 1 || first["vendor"] != nil {
			t.Fatalf("cursor output retained undeclared fields: %#v", first)
		}
		if strings.Join(cursors, ",") != ",page-2" {
			t.Fatalf("cursors = %#v", cursors)
		}
	})
}

func TestHTTPSourcePaginationBoundsAndErrors(t *testing.T) {
	tests := []struct {
		name, response, want string
		mutate               func(*Request)
	}{
		{name: "repeated cursor", response: `{"records":[],"next":"same"}`, want: "source_contract_invalid"},
		{name: "items wrong type", response: `{"records":{},"next":null}`, want: "source_contract_invalid"},
		{name: "cursor wrong type", response: `{"records":[],"next":4}`, want: "source_contract_invalid"},
		{name: "oversized cursor", response: `{"records":[],"next":"` + strings.Repeat("x", MaxCursorLength+1) + `"}`, want: "source_contract_invalid"},
		{name: "item overflow", response: `{"records":[1,2],"next":null}`, want: "source_response_too_large", mutate: func(r *Request) { r.MaxItems = 1 }},
		{name: "page overflow", response: `{"records":[],"next":"more"}`, want: "source_response_too_large", mutate: func(r *Request) { r.MaxPages = 1 }},
		{name: "invalid json", response: `{`, want: "source_invalid_json"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte(tt.response)) }))
			defer server.Close()
			request := cursorRequest(server.URL)
			if tt.mutate != nil {
				tt.mutate(&request)
			}
			_, err := NewHTTPSource(server.Client(), nil).Fetch(context.Background(), request)
			assertCategory(t, err, tt.want)
		})
	}
}

func TestHTTPSourceRateLimitRetryUsesInjectedWait(t *testing.T) {
	requests := 0
	var waits []time.Duration
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		requests++
		if requests < 3 {
			w.Header().Set("Retry-After", "2")
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}
		_, _ = w.Write([]byte(`{"records":[],"next":null}`))
	}))
	defer server.Close()
	source := NewHTTPSource(server.Client(), func(_ context.Context, duration time.Duration) error { waits = append(waits, duration); return nil })
	if _, err := source.Fetch(context.Background(), cursorRequest(server.URL)); err != nil {
		t.Fatal(err)
	}
	if requests != 3 || len(waits) != 2 || waits[0] != 2*time.Second {
		t.Fatalf("requests=%d waits=%v", requests, waits)
	}

	for _, value := range []string{"not-a-delta", "6", "-1"} {
		t.Run(value, func(t *testing.T) {
			limited := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("Retry-After", value)
				w.WriteHeader(http.StatusTooManyRequests)
			}))
			defer limited.Close()
			_, err := source.Fetch(context.Background(), cursorRequest(limited.URL))
			assertCategory(t, err, apperror.CategoryRateLimited)
		})
	}
}

func TestHTTPSourceStripsCredentialAcrossOriginRedirect(t *testing.T) {
	for _, tt := range []struct {
		name, authMode, header string
	}{
		{name: "bearer", authMode: "bearer", header: "Authorization"},
		{name: "api key", authMode: "api_key", header: "X-Store-Key"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			var destinationCredential string
			destination := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				destinationCredential = r.Header.Get(tt.header)
				_, _ = w.Write([]byte(`{"records":[],"next":null}`))
			}))
			defer destination.Close()
			sourceServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("Location", destination.URL)
				w.WriteHeader(http.StatusFound)
			}))
			defer sourceServer.Close()
			request := cursorRequest(sourceServer.URL)
			request.AuthMode = tt.authMode
			request.Credential = "redirect-token-canary"
			request.APIKeyHeader = "X-Store-Key"
			if _, err := NewHTTPSource(sourceServer.Client(), nil).Fetch(context.Background(), request); err != nil {
				t.Fatal(err)
			}
			if destinationCredential != "" {
				t.Fatalf("credential crossed origin: %q", destinationCredential)
			}
		})
	}
}

func TestHTTPSourceCancellationAndHTTPErrorCategories(t *testing.T) {
	for _, status := range []int{http.StatusUnauthorized, http.StatusForbidden, http.StatusInternalServerError} {
		t.Run(http.StatusText(status), func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(status) }))
			defer server.Close()
			_, err := NewHTTPSource(server.Client(), nil).Fetch(context.Background(), cursorRequest(server.URL))
			assertCategory(t, err, "source_http_error")
		})
	}
	client := &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) { return nil, context.Canceled })}
	_, err := NewHTTPSource(client, nil).Fetch(context.Background(), cursorRequest("https://example.test"))
	assertCategory(t, err, apperror.CategoryCancelled)
}

func TestValidateRequestTable(t *testing.T) {
	valid := cursorRequest("https://example.test/items")
	tests := []struct {
		name   string
		mutate func(*Request)
	}{
		{name: "bad url", mutate: func(r *Request) { r.URL = "/relative" }},
		{name: "URL user information", mutate: func(r *Request) { r.URL = "https://user:secret@example.test/items" }},
		{name: "bad auth", mutate: func(r *Request) { r.AuthMode = "oauth_magic" }},
		{name: "bearer missing", mutate: func(r *Request) { r.AuthMode = "bearer"; r.Credential = "" }},
		{name: "reserved header", mutate: func(r *Request) { r.AuthMode = "api_key"; r.Credential = "x"; r.APIKeyHeader = "Authorization" }},
		{name: "bad pagination", mutate: func(r *Request) { r.Pagination = "page" }},
		{name: "bad field", mutate: func(r *Request) { r.ItemsField = "items.path" }},
		{name: "pages bound", mutate: func(r *Request) { r.MaxPages = MaxPages + 1 }},
		{name: "items bound", mutate: func(r *Request) { r.MaxItems = MaxItems + 1 }},
	}
	if err := ValidateRequest(valid); err != nil {
		t.Fatalf("valid request: %v", err)
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			request := valid
			tt.mutate(&request)
			if ValidateRequest(request) == nil {
				t.Fatal("invalid request accepted")
			}
		})
	}
}

func cursorRequest(rawURL string) Request {
	contract, err := jsoncontract.Parse(map[string]interface{}{"required": map[string]interface{}{"id": map[string]interface{}{"type": "integer"}}})
	if err != nil {
		panic(err)
	}
	return Request{URL: rawURL, AuthMode: "none", Pagination: "cursor", CursorQueryParam: "after", ItemsField: "records", NextCursorField: "next", MaxPages: 3, MaxItems: 10, Contract: contract}
}

func assertCategory(t *testing.T, err error, want string) {
	t.Helper()
	category, _, ok := apperror.Details(err)
	if !ok || category != want {
		t.Fatalf("category=%q want=%q err=%v", category, want, err)
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (function roundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return function(request)
}
