package cli

import (
	"flag"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestReadInputRejectsMultipleSources(t *testing.T) {
	_, err := readInput(`{"a":1}`, "", false, []string{"b=2"}, strings.NewReader(""))
	if err == nil {
		t.Fatalf("expected multiple input sources to be rejected")
	}
}

func TestReadInputFromSet(t *testing.T) {
	got, err := readInput("", "", false, []string{"date=2026-07-25", "env=prod"}, strings.NewReader(""))
	if err != nil {
		t.Fatalf("readInput failed: %v", err)
	}
	want := map[string]interface{}{"date": "2026-07-25", "env": "prod"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("unexpected input map: got %#v want %#v", got, want)
	}
}

func TestParseOneRefAllowsFlagsAfterReference(t *testing.T) {
	fs := flag.NewFlagSet("test", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	timeout := fs.Duration("timeout", time.Second, "")

	ref, ok, code := parseOneRef(fs, []string{"exec-1", "--timeout", "5s"}, "execution ID", io.Discard)
	if !ok || code != ExitOK {
		t.Fatalf("parseOneRef failed: ok=%t code=%d", ok, code)
	}
	if ref != "exec-1" {
		t.Fatalf("expected exec-1, got %s", ref)
	}
	if *timeout != 5*time.Second {
		t.Fatalf("expected timeout 5s, got %s", *timeout)
	}
}

func TestParseOneRefAllowsFlagsBeforeReference(t *testing.T) {
	fs := flag.NewFlagSet("test", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	output := fs.String("output", "table", "")

	ref, ok, code := parseOneRef(fs, []string{"--output", "json", "wf-1"}, "workflow reference", io.Discard)
	if !ok || code != ExitOK {
		t.Fatalf("parseOneRef failed: ok=%t code=%d", ok, code)
	}
	if ref != "wf-1" {
		t.Fatalf("expected wf-1, got %s", ref)
	}
	if *output != "json" {
		t.Fatalf("expected output json, got %s", *output)
	}
}

func TestReadWorkflowFileAcceptsUIExportShape(t *testing.T) {
	path := filepath.Join(t.TempDir(), "workflow.json")
	data := `{"name":"Imported","description":"From UI","nodes":[],"edges":[]}`
	if err := os.WriteFile(path, []byte(data), 0600); err != nil {
		t.Fatalf("write workflow file: %v", err)
	}
	workflow, err := readWorkflowFile(path)
	if err != nil {
		t.Fatalf("readWorkflowFile failed: %v", err)
	}
	if workflow.Name != "Imported" || workflow.NodesJSON != "[]" || workflow.EdgesJSON != "[]" {
		t.Fatalf("unexpected workflow: %#v", workflow)
	}
}

func TestReadWorkflowFileRejectsMissingName(t *testing.T) {
	path := filepath.Join(t.TempDir(), "workflow.json")
	if err := os.WriteFile(path, []byte(`{"nodes":[],"edges":[]}`), 0600); err != nil {
		t.Fatalf("write workflow file: %v", err)
	}
	if _, err := readWorkflowFile(path); err == nil {
		t.Fatalf("expected missing name error")
	}
}
