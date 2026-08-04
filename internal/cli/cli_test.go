package cli

import (
	"bytes"
	"flag"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"goflow/internal/pack"
	"goflow/internal/workflow"
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
	workflowDef, err := workflow.ReadFile(path)
	if err != nil {
		t.Fatalf("readWorkflowFile failed: %v", err)
	}
	if workflowDef.Name != "Imported" || workflowDef.NodesJSON != "[]" || workflowDef.EdgesJSON != "[]" {
		t.Fatalf("unexpected workflow: %#v", workflowDef)
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
	workflowDef, err := workflow.ReadFile(path)
	if err != nil {
		t.Fatalf("readWorkflowFile failed: %v", err)
	}
	if workflowDef.ExposeCLI || !workflowDef.ExposeMCP || !workflowDef.RequiresApproval || workflowDef.MaxConcurrentRuns != 1 || workflowDef.ConcurrencyPolicy != "reject" {
		t.Fatalf("interface metadata was not preserved: %#v", workflowDef)
	}
	if workflowDef.MCPToolName != "imported_tool" || workflowDef.RiskLevel != "high" {
		t.Fatalf("unexpected metadata: %#v", workflowDef)
	}
}

func TestReadWorkflowFileRejectsMissingName(t *testing.T) {
	path := filepath.Join(t.TempDir(), "workflow.json")
	if err := os.WriteFile(path, []byte(`{"nodes":[],"edges":[]}`), 0600); err != nil {
		t.Fatalf("write workflow file: %v", err)
	}
	if _, err := workflow.ReadFile(path); err == nil {
		t.Fatalf("expected missing name error")
	}
}

func TestWorkflowValidateRejectsDuplicateNodeID(t *testing.T) {
	path := writeWorkflowFixture(t, `{
		"name":"Invalid",
		"nodes":[
			{"id":"n1","type":"webhookTrigger","params":{}},
			{"id":"n1","type":"webhookTrigger","params":{}}
		],
		"edges":[]
	}`)
	code, stderr := runValidateFixture(path)
	if code != ExitInvalidInput || !strings.Contains(stderr, "duplicate node ID") {
		t.Fatalf("expected duplicate node ID validation error, code=%d stderr=%q", code, stderr)
	}
}

func TestWorkflowValidateRejectsBadEdgeReference(t *testing.T) {
	path := writeWorkflowFixture(t, `{
		"name":"Invalid",
		"nodes":[{"id":"n1","type":"webhookTrigger","params":{}}],
		"edges":[{"id":"e1","source":"n1","target":"missing"}]
	}`)
	code, stderr := runValidateFixture(path)
	if code != ExitInvalidInput || !strings.Contains(stderr, "missing target node") {
		t.Fatalf("expected bad edge validation error, code=%d stderr=%q", code, stderr)
	}
}

func TestWorkflowValidateRejectsCycle(t *testing.T) {
	path := writeWorkflowFixture(t, `{
		"name":"Invalid",
		"nodes":[
			{"id":"a","type":"webhookTrigger","params":{}},
			{"id":"b","type":"webhookTrigger","params":{}}
		],
		"edges":[
			{"id":"e1","source":"a","target":"b"},
			{"id":"e2","source":"b","target":"a"}
		]
	}`)
	code, stderr := runValidateFixture(path)
	if code != ExitInvalidInput || !strings.Contains(stderr, "cycle") {
		t.Fatalf("expected cycle validation error, code=%d stderr=%q", code, stderr)
	}
}

func TestWorkflowValidateRejectsUnknownNodeType(t *testing.T) {
	path := writeWorkflowFixture(t, `{
		"name":"Invalid",
		"nodes":[{"id":"n1","type":"missingType","params":{}}],
		"edges":[]
	}`)
	code, stderr := runValidateFixture(path)
	if code != ExitInvalidInput || !strings.Contains(stderr, "unknown node type") {
		t.Fatalf("expected unknown node type validation error, code=%d stderr=%q", code, stderr)
	}
}

func TestWorkflowValidateRejectsMissingRequiredParam(t *testing.T) {
	path := writeWorkflowFixture(t, `{
		"name":"Invalid",
		"nodes":[{"id":"sleep","type":"delaySleep","params":{}}],
		"edges":[]
	}`)
	code, stderr := runValidateFixture(path)
	if code != ExitInvalidInput || !strings.Contains(stderr, "missing required parameter") {
		t.Fatalf("expected missing required parameter validation error, code=%d stderr=%q", code, stderr)
	}
}

func TestWorkflowValidateRejectsUnsupportedSchemaKeyword(t *testing.T) {
	path := writeWorkflowFixture(t, `{
		"name":"Invalid",
		"nodes":[],
		"edges":[],
		"input_schema_json":{"type":"object","format":"date-time"}
	}`)
	code, stderr := runValidateFixture(path)
	if code != ExitInvalidInput || !strings.Contains(stderr, "unsupported schema keyword") {
		t.Fatalf("expected schema keyword validation error, code=%d stderr=%q", code, stderr)
	}
}

func TestPackValidateReturnsSuccessForValidPack(t *testing.T) {
	dir := writePackFixture(t, `{
		"schema_version":1,
		"id":"example.hello-webhook",
		"name":"Hello Webhook",
		"version":"0.1.0",
		"entry_workflow":"workflows/main.json",
		"required_credentials":[],
		"supported_platforms":["windows-amd64"]
	}`, `{
		"name":"Hello Webhook",
		"nodes":[{"id":"trigger","type":"webhookTrigger","params":{}}],
		"edges":[]
	}`)
	var stdout, stderr bytes.Buffer
	code := Runner{Stdout: &stdout, Stderr: &stderr, Stdin: strings.NewReader("")}.Run([]string{"pack", "validate", dir})
	if code != ExitOK {
		t.Fatalf("expected success, code=%d stderr=%q", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "example.hello-webhook") || !strings.Contains(stdout.String(), "0.1.0") {
		t.Fatalf("unexpected stdout: %q", stdout.String())
	}
}

func TestPackValidateReturnsInvalidInputForInvalidPack(t *testing.T) {
	dir := writePackFixture(t, `{
		"schema_version":1,
		"id":"Example Bad",
		"name":"Hello Webhook",
		"version":"0.1.0",
		"entry_workflow":"workflows/main.json",
		"required_credentials":[],
		"supported_platforms":["windows-amd64"]
	}`, `{
		"name":"Hello Webhook",
		"nodes":[],
		"edges":[]
	}`)
	var stdout, stderr bytes.Buffer
	code := Runner{Stdout: &stdout, Stderr: &stderr, Stdin: strings.NewReader("")}.Run([]string{"pack", "validate", dir})
	if code != ExitInvalidInput {
		t.Fatalf("expected invalid input, code=%d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
	if !strings.Contains(stderr.String(), "manifest") {
		t.Fatalf("expected manifest error, got %q", stderr.String())
	}
}

func TestPackBuildReturnsSuccessAndArchivePath(t *testing.T) {
	dir := writePackFixture(t, `{
		"schema_version":1,
		"id":"example.hello-webhook",
		"name":"Hello Webhook",
		"version":"0.1.0",
		"entry_workflow":"workflows/main.json",
		"required_credentials":[],
		"supported_platforms":["`+pack.CurrentPlatform()+`"]
	}`, `{
		"name":"Hello Webhook",
		"nodes":[{"id":"trigger","type":"webhookTrigger","params":{}}],
		"edges":[]
	}`)
	runtimePath := filepath.Join(t.TempDir(), "goflow-runtime")
	if err := os.WriteFile(runtimePath, []byte("runtime"), 0600); err != nil {
		t.Fatalf("write runtime fixture: %v", err)
	}
	outputDir := t.TempDir()
	var stdout, stderr bytes.Buffer
	code := Runner{
		Stdout:          &stdout,
		Stderr:          &stderr,
		Stdin:           strings.NewReader(""),
		PackRuntimePath: runtimePath,
	}.Run([]string{"pack", "build", dir, "--output", outputDir})
	if code != ExitOK {
		t.Fatalf("expected success, code=%d stderr=%q", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "Built portable pack bundle") || !strings.Contains(stdout.String(), outputDir) {
		t.Fatalf("expected archive path in stdout, got %q", stdout.String())
	}
}

func TestPackBuildReturnsInvalidInput(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := Runner{Stdout: &stdout, Stderr: &stderr, Stdin: strings.NewReader("")}.Run([]string{"pack", "build", t.TempDir()})
	if code == ExitOK {
		t.Fatalf("expected non-zero exit code")
	}
	if !strings.Contains(stderr.String(), "--output is required") {
		t.Fatalf("expected output error, got %q", stderr.String())
	}
}

func writeWorkflowFixture(t *testing.T, data string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "workflow.json")
	if err := os.WriteFile(path, []byte(data), 0600); err != nil {
		t.Fatalf("write workflow fixture: %v", err)
	}
	return path
}

func writePackFixture(t *testing.T, manifest, workflow string) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "workflows"), 0700); err != nil {
		t.Fatalf("mkdir workflows: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "pack.json"), []byte(manifest), 0600); err != nil {
		t.Fatalf("write manifest: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "workflows", "main.json"), []byte(workflow), 0600); err != nil {
		t.Fatalf("write workflow: %v", err)
	}
	return dir
}

func runValidateFixture(path string) (int, string) {
	var stdout, stderr bytes.Buffer
	code := Runner{Stdout: &stdout, Stderr: &stderr, Stdin: strings.NewReader("")}.Run([]string{"workflow", "validate", path})
	return code, stderr.String()
}
