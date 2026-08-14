package sourceprobe

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"goflow/internal/apperror"
	"goflow/internal/jsoncontract"
)

func TestCheckSourceResponses(t *testing.T) {
	contract := sourceContract(t)
	tests := []struct {
		name        string
		status      int
		contentType string
		body        string
		category    string
	}{
		{name: "valid", status: 200, contentType: "application/json", body: validSourceBody()},
		{name: "html", status: 200, contentType: "text/html", body: `<html>sign in</html>`, category: "source_non_json"},
		{name: "malformed", status: 200, contentType: "application/json", body: `{"report_date":`, category: "source_invalid_json"},
		{name: "missing field", status: 200, contentType: "application/json", body: `{"report_date":"2026-08-09"}`, category: "source_contract_invalid"},
		{name: "wrong field type", status: 200, contentType: "application/json", body: strings.Replace(validSourceBody(), `"order_count":314`, `"order_count":"314"`, 1), category: "source_contract_invalid"},
		{name: "non 2xx", status: 503, contentType: "text/plain", body: "secret upstream body", category: "source_http_error"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("Content-Type", tt.contentType)
				w.WriteHeader(tt.status)
				_, _ = w.Write([]byte(tt.body))
			}))
			defer server.Close()
			result, err := Check(context.Background(), server.Client(), server.URL+"/dailyops.json?access_token=secret-query", contract, DefaultMaxResponseBytes)
			if tt.category == "" {
				if err != nil || result == nil || result.Summary.RequiredFields != 7 {
					t.Fatalf("expected valid result, got result=%+v err=%v", result, err)
				}
				return
			}
			category, message, ok := apperror.Details(err)
			if !ok || category != tt.category {
				t.Fatalf("expected category %q, got %q err=%v", tt.category, category, err)
			}
			if strings.Contains(message, "secret-query") || strings.Contains(message, "secret upstream body") {
				t.Fatalf("public error leaked request or response data: %q", message)
			}
		})
	}
}

func TestCheckRejectsOversizedResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(strings.Repeat("x", 65)))
	}))
	defer server.Close()
	_, err := Check(context.Background(), server.Client(), server.URL, sourceContract(t), 64)
	assertCategory(t, err, "source_response_too_large")
}

func TestCheckHonorsTimeoutAndCancellation(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		time.Sleep(150 * time.Millisecond)
		_, _ = w.Write([]byte(validSourceBody()))
	}))
	defer server.Close()
	client := server.Client()
	client.Timeout = 20 * time.Millisecond
	_, err := Check(context.Background(), client, server.URL, sourceContract(t), DefaultMaxResponseBytes)
	assertCategory(t, err, "source_timeout")

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err = Check(ctx, server.Client(), server.URL, sourceContract(t), DefaultMaxResponseBytes)
	assertCategory(t, err, "source_unreachable")
}

func TestSafeRedirectRemovesSensitiveHeadersAcrossOrigins(t *testing.T) {
	previous := httptest.NewRequest(http.MethodGet, "https://first.example/data", nil)
	previous.Header.Set("Authorization", "Bearer secret")
	previous.Header.Set("Cookie", "session=secret")
	next := httptest.NewRequest(http.MethodGet, "https://second.example/data", nil)
	next.Header = previous.Header.Clone()
	if err := SafeRedirect(next, []*http.Request{previous}); err != nil {
		t.Fatalf("SafeRedirect: %v", err)
	}
	if next.Header.Get("Authorization") != "" || next.Header.Get("Cookie") != "" {
		t.Fatalf("sensitive headers survived cross-origin redirect: %#v", next.Header)
	}
}

func assertCategory(t *testing.T, err error, want string) {
	t.Helper()
	category, _, ok := apperror.Details(err)
	if !ok || category != want {
		t.Fatalf("expected %q, got %q err=%v", want, category, err)
	}
}

func sourceContract(t *testing.T) jsoncontract.Contract {
	t.Helper()
	contract, err := jsoncontract.Parse(map[string]interface{}{"required": map[string]interface{}{
		"report_date":              map[string]interface{}{"type": "string", "non_empty": true},
		"timezone":                 map[string]interface{}{"type": "string", "non_empty": true},
		"revenue":                  map[string]interface{}{"type": "number"},
		"order_count":              map[string]interface{}{"type": "integer", "minimum": 0},
		"cancelled_refunded_count": map[string]interface{}{"type": "integer", "minimum": 0},
		"low_stock_summary":        map[string]interface{}{"type": "string"},
		"comparison_summary":       map[string]interface{}{"type": "string"},
	}})
	if err != nil {
		t.Fatalf("parse contract: %v", err)
	}
	return contract
}

func validSourceBody() string {
	return `{"report_date":"2026-08-09","timezone":"Asia/Bangkok","revenue":48250.75,"order_count":314,"cancelled_refunded_count":7,"low_stock_summary":"3 SKUs below threshold","comparison_summary":"Revenue up 12.4% vs prior day","future":true}`
}
