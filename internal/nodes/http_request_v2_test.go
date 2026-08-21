package nodes

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"reflect"
	"testing"
)

func TestHTTPRequestStructuredQueryAndBearerAuth(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("status") != "open" {
			t.Fatalf("status query = %q", r.URL.Query().Get("status"))
		}
		if got := r.URL.Query()["tag"]; !reflect.DeepEqual(got, []string{"a", "b"}) {
			t.Fatalf("tag query = %#v", got)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer private-token" {
			t.Fatalf("Authorization = %q", got)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer server.Close()

	ctx := NewExecutionContextWithContext(context.Background(), "wf", "exec")
	ctx.Credentials["cred"] = "private-token"
	node := &Node{Params: map[string]interface{}{
		"method":        "GET",
		"url":           server.URL,
		"query_params":  map[string]interface{}{"status": "open", "tag": []interface{}{"a", "b"}},
		"headers":       "{}",
		"auth_mode":     "bearer",
		"credential_id": "cred",
	}}
	if _, err := NewHTTPRequestExecutor().Execute(ctx, node); err != nil {
		t.Fatal(err)
	}
}

func TestHTTPRequestBasicAndCustomHeaderAuth(t *testing.T) {
	cases := []struct {
		name   string
		params map[string]interface{}
		secret string
		check  func(*testing.T, *http.Request)
	}{
		{
			name:   "basic",
			params: map[string]interface{}{"auth_mode": "basic"},
			secret: `{"username":"alice","password":"secret"}`,
			check: func(t *testing.T, r *http.Request) {
				user, pass, ok := r.BasicAuth()
				if !ok || user != "alice" || pass != "secret" {
					t.Fatalf("BasicAuth = %q/%q/%v", user, pass, ok)
				}
			},
		},
		{
			name:   "custom header",
			params: map[string]interface{}{"auth_mode": "custom_header", "auth_header": "X-Token", "auth_prefix": "Token "},
			secret: "abc",
			check: func(t *testing.T, r *http.Request) {
				if got := r.Header.Get("X-Token"); got != "Token abc" {
					t.Fatalf("X-Token = %q", got)
				}
			},
		},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				tt.check(t, r)
				_, _ = w.Write([]byte(`{"ok":true}`))
			}))
			defer server.Close()
			ctx := NewExecutionContext("wf", "exec")
			ctx.Credentials["cred"] = tt.secret
			params := map[string]interface{}{"method": "GET", "url": server.URL, "credential_id": "cred"}
			for key, value := range tt.params {
				params[key] = value
			}
			if _, err := NewHTTPRequestExecutor().Execute(ctx, &Node{Params: params}); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestHTTPRequestBodyAndResponseModes(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		switch r.URL.Path {
		case "/form":
			values, err := url.ParseQuery(string(body))
			if err != nil || values.Get("name") != "goflow" {
				t.Fatalf("form body = %q, err=%v", body, err)
			}
			_, _ = w.Write([]byte("ok"))
		case "/json":
			var payload map[string]interface{}
			if err := json.Unmarshal(body, &payload); err != nil || payload["n"] != float64(2) {
				t.Fatalf("json body = %q, err=%v", body, err)
			}
			_, _ = w.Write([]byte(`{"received":true}`))
		}
	}))
	defer server.Close()

	executor := NewHTTPRequestExecutor()
	out, err := executor.Execute(NewExecutionContext("wf", "exec"), &Node{Params: map[string]interface{}{
		"method": "POST", "url": server.URL + "/form", "body_mode": "x-www-form-urlencoded", "form_fields": `{"name":"goflow"}`, "response_mode": "text",
	}})
	if err != nil {
		t.Fatal(err)
	}
	if got := out.(map[string]interface{})["data"]; got != "ok" {
		t.Fatalf("text response = %#v", got)
	}

	out, err = executor.Execute(NewExecutionContext("wf", "exec"), &Node{Params: map[string]interface{}{
		"method": "POST", "url": server.URL + "/json", "body_mode": "json", "body": `{"n":2}`, "response_mode": "json",
	}})
	if err != nil {
		t.Fatal(err)
	}
	if got := out.(map[string]interface{})["data"].(map[string]interface{})["received"]; got != true {
		t.Fatalf("JSON response = %#v", out)
	}
}

func TestHTTPRequestCursorPagination(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cursor := r.URL.Query().Get("cursor")
		w.Header().Set("Content-Type", "application/json")
		switch cursor {
		case "":
			_, _ = fmt.Fprint(w, `{"items":[1,2],"next_cursor":"next"}`)
		case "next":
			_, _ = fmt.Fprint(w, `{"items":[3],"next_cursor":""}`)
		default:
			t.Fatalf("unexpected cursor %q", cursor)
		}
	}))
	defer server.Close()

	out, err := NewHTTPRequestExecutor().Execute(NewExecutionContext("wf", "exec"), &Node{Params: map[string]interface{}{
		"method": "GET", "url": server.URL, "pagination_mode": "cursor", "max_pages": 5,
	}})
	if err != nil {
		t.Fatal(err)
	}
	result := out.(map[string]interface{})
	items := result["data"].([]interface{})
	if !reflect.DeepEqual(items, []interface{}{float64(1), float64(2), float64(3)}) {
		t.Fatalf("items = %#v", items)
	}
	if result["page_count"] != 2 || result["item_count"] != 3 {
		t.Fatalf("pagination metadata = %#v", result)
	}
}

func TestHTTPRequestPageNumberPaginationStopsOnEmptyPage(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		page := r.URL.Query().Get("page")
		if page == "3" {
			_, _ = w.Write([]byte(`[]`))
			return
		}
		_, _ = fmt.Fprintf(w, `[%s]`, page)
	}))
	defer server.Close()

	out, err := NewHTTPRequestExecutor().Execute(NewExecutionContext("wf", "exec"), &Node{Params: map[string]interface{}{
		"method": "GET", "url": server.URL, "pagination_mode": "page_number", "max_pages": 10,
	}})
	if err != nil {
		t.Fatal(err)
	}
	result := out.(map[string]interface{})
	if result["page_count"] != 3 || result["item_count"] != 2 {
		t.Fatalf("pagination metadata = %#v", result)
	}
}
