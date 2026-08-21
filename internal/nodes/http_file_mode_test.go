package nodes

import (
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"goflow/internal/fileref"
)

func TestHTTPRequestFileRefBody(t *testing.T) {
	store := fileref.NewStore(t.TempDir())
	ref, err := store.PutBytes("payload.bin", "application/octet-stream", []byte("binary-payload"))
	if err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		data, _ := io.ReadAll(r.Body)
		if string(data) != "binary-payload" {
			t.Fatalf("request body = %q", data)
		}
		if got := r.Header.Get("Content-Type"); got != "application/octet-stream" {
			t.Fatalf("content type = %q", got)
		}
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer server.Close()

	executor := NewHTTPRequestExecutorWithClientAndStore(server.Client(), store)
	out, err := executor.Execute(NewExecutionContext("wf", "exec"), &Node{Params: map[string]interface{}{
		"method":        "POST",
		"url":           server.URL,
		"body_mode":     "file",
		"file_ref":      ref,
		"response_mode": "json",
	}})
	if err != nil {
		t.Fatal(err)
	}
	if out.(map[string]interface{})["data"].(map[string]interface{})["ok"] != true {
		t.Fatalf("output = %#v", out)
	}
}

func TestHTTPRequestFileResponseReturnsManagedFileRef(t *testing.T) {
	store := fileref.NewStore(t.TempDir())
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/pdf")
		_, _ = w.Write([]byte("pdf-bytes"))
	}))
	defer server.Close()

	executor := NewHTTPRequestExecutorWithClientAndStore(server.Client(), store)
	out, err := executor.Execute(NewExecutionContext("wf", "exec"), &Node{Params: map[string]interface{}{
		"method":             "GET",
		"url":                server.URL,
		"response_mode":      "file",
		"response_file_name": "report.pdf",
	}})
	if err != nil {
		t.Fatal(err)
	}
	ref, ok := out.(map[string]interface{})["data"].(fileref.Ref)
	if !ok {
		t.Fatalf("response data is not FileRef: %#v", out)
	}
	if ref.Name != "report.pdf" || ref.MIME != "application/pdf" {
		t.Fatalf("ref = %#v", ref)
	}
	data, err := store.ReadAll(ref)
	if err != nil || string(data) != "pdf-bytes" {
		t.Fatalf("stored bytes = %q err=%v", data, err)
	}
}
