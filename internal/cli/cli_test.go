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

func TestCLIClientSetsTriggerSource(t *testing.T) {
	url := "http://127.0.0.1:8080"
	apiKey := "test-key"
	c := cliClient(clientOptions{url: &url, apiKey: &apiKey})
	if c.TriggerSource != "cli" {
		t.Fatalf("expected cli trigger source, got %q", c.TriggerSource)
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

func TestReadWorkflowFilePreservesInterfaceMetadata(t *testing.T) {
	path := filepath.Join(t.TempDir(), "workflow.json")
	data := `{
		"name":"Imported",
		"description":"From UI",
		"nodes":[],
		"edges":[],
		"slug":"imported",
		"input_schema_json":{"type":"object"},
		"output_schema_json":{"type":"object"},
		"expose_cli":false,
		"expose_mcp":true,
		"mcp_tool_name":"imported_tool",
		"mcp_description":"Imported tool",
		"risk_level":"high",
		"requires_approval":true,
		"max_concurrent_runs":1,
		"concurrency_policy":"reject"
	}`
	if err := os.WriteFile(path, []byte(data), 0600); err != nil {
		t.Fatalf("write workflow file: %v", err)
	}
	workflow, err := readWorkflowFile(path)
	if err != nil {
		t.Fatalf("readWorkflowFile failed: %v", err)
	}
	if workflow.ExposeCLI || !workflow.ExposeMCP || !workflow.RequiresApproval || workflow.MaxConcurrentRuns != 1 || workflow.ConcurrencyPolicy != "reject" {
		t.Fatalf("interface metadata was not preserved: %#v", workflow)
	}
	if workflow.MCPToolName != "imported_tool" || workflow.RiskLevel != "high" {
		t.Fatalf("unexpected metadata: %#v", workflow)
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
