package cli

import (
	"archive/zip"
	"bytes"
	"encoding/json"
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

func TestPackInitScaffoldValidateTestBuildVerify(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "fresh-pack")
	var stdout, stderr bytes.Buffer
	runner := Runner{Stdout: &stdout, Stderr: &stderr, Stdin: strings.NewReader("")}
	code := runner.Run([]string{"pack", "init", dir, "--id", "example.fresh-pack", "--name", "Fresh Pack", "--target", pack.CurrentPlatform()})
	if code != ExitOK {
		t.Fatalf("pack init failed code=%d stderr=%q", code, stderr.String())
	}
	for _, rel := range []string{"pack.json", filepath.Join("workflows", "main.json"), "README.md"} {
		if _, err := os.Stat(filepath.Join(dir, rel)); err != nil {
			t.Fatalf("expected scaffold file %s: %v", rel, err)
		}
	}
	if data, err := os.ReadFile(filepath.Join(dir, "README.md")); err != nil || strings.Contains(strings.ToLower(string(data)), "secret") && strings.Contains(string(data), "123:") {
		t.Fatalf("README should not contain example secrets")
	}

	stdout.Reset()
	stderr.Reset()
	code = runner.Run([]string{"pack", "validate", dir})
	if code != ExitOK {
		t.Fatalf("pack validate failed code=%d stderr=%q", code, stderr.String())
	}

	stdout.Reset()
	stderr.Reset()
	code = runner.Run([]string{"pack", "test", dir, "--output", "json"})
	if code != ExitOK {
		t.Fatalf("pack test failed code=%d stderr=%q stdout=%q", code, stderr.String(), stdout.String())
	}
	var testResult map[string]interface{}
	if err := json.Unmarshal(stdout.Bytes(), &testResult); err != nil {
		t.Fatalf("decode pack test json: %v", err)
	}
	if testResult["status"] != "PASS" || testResult["managed_workflow"] != "PASS" {
		t.Fatalf("unexpected pack test result: %#v", testResult)
	}

	runtimePath := filepath.Join(t.TempDir(), "goflow-runtime")
	if err := os.WriteFile(runtimePath, []byte("runtime"), 0600); err != nil {
		t.Fatalf("write runtime fixture: %v", err)
	}
	outputDir := t.TempDir()
	stdout.Reset()
	stderr.Reset()
	buildRunner := Runner{Stdout: &stdout, Stderr: &stderr, Stdin: strings.NewReader(""), PackRuntimePath: runtimePath}
	code = buildRunner.Run([]string{"pack", "build", dir, "--output", outputDir})
	if code != ExitOK {
		t.Fatalf("pack build failed code=%d stderr=%q", code, stderr.String())
	}
	archive := filepath.Join(outputDir, "example.fresh-pack-0.1.0-"+pack.CurrentPlatform()+".zip")

	stdout.Reset()
	stderr.Reset()
	code = runner.Run([]string{"pack", "verify", archive, "--output", "json"})
	if code != ExitOK {
		t.Fatalf("pack verify failed code=%d stderr=%q", code, stderr.String())
	}
	var verifyResult map[string]interface{}
	if err := json.Unmarshal(stdout.Bytes(), &verifyResult); err != nil {
		t.Fatalf("decode verify json: %v", err)
	}
	if verifyResult["status"] != "PASS" || verifyResult["id"] != "example.fresh-pack" {
		t.Fatalf("unexpected verify result: %#v", verifyResult)
	}

	extracted := t.TempDir()
	reader, err := zip.OpenReader(archive)
	if err != nil {
		t.Fatalf("open bundle fixture: %v", err)
	}
	for _, file := range reader.File {
		target := filepath.Join(extracted, filepath.FromSlash(file.Name))
		if file.FileInfo().IsDir() {
			if err := os.MkdirAll(target, 0700); err != nil {
				t.Fatalf("create extracted fixture directory: %v", err)
			}
			continue
		}
		if err := os.MkdirAll(filepath.Dir(target), 0700); err != nil {
			t.Fatalf("create extracted fixture parent: %v", err)
		}
		input, err := file.Open()
		if err != nil {
			t.Fatalf("open bundle member: %v", err)
		}
		output, err := os.OpenFile(target, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0600)
		if err != nil {
			input.Close()
			t.Fatalf("create extracted member: %v", err)
		}
		_, copyErr := io.Copy(output, input)
		closeInputErr := input.Close()
		closeOutputErr := output.Close()
		if copyErr != nil || closeInputErr != nil || closeOutputErr != nil {
			t.Fatalf("extract bundle member: copy=%v input=%v output=%v", copyErr, closeInputErr, closeOutputErr)
		}
	}
	reader.Close()
	workflowPath := filepath.Join(extracted, "pack", "workflows", "main.json")
	file, err := os.OpenFile(workflowPath, os.O_APPEND|os.O_WRONLY, 0)
	if err != nil {
		t.Fatalf("open workflow for tamper: %v", err)
	}
	if _, err := file.WriteString("\n"); err != nil {
		file.Close()
		t.Fatalf("tamper workflow: %v", err)
	}
	file.Close()
	stdout.Reset()
	stderr.Reset()
	code = runner.Run([]string{"pack", "verify", extracted, "--output", "json"})
	if code == ExitOK {
		t.Fatalf("tampered pack verify returned success exit code: stdout=%q", stdout.String())
	}
	verifyResult = nil
	if err := json.Unmarshal(stdout.Bytes(), &verifyResult); err != nil {
		t.Fatalf("failed verify JSON was not parseable: %v", err)
	}
	if verifyResult["status"] != "FAILED" {
		t.Fatalf("tampered verify status = %#v", verifyResult)
	}
}

func TestPackInitRefusesNonEmptyDirectory(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "existing.txt"), []byte("keep"), 0600); err != nil {
		t.Fatalf("write existing file: %v", err)
	}
	var stdout, stderr bytes.Buffer
	code := Runner{Stdout: &stdout, Stderr: &stderr, Stdin: strings.NewReader("")}.Run([]string{"pack", "init", dir, "--id", "example.refuse", "--name", "Refuse"})
	if code != ExitInvalidInput {
		t.Fatalf("expected invalid input, code=%d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
	if _, err := os.Stat(filepath.Join(dir, "existing.txt")); err != nil {
		t.Fatalf("existing file should remain untouched: %v", err)
	}
}

func TestPackInspectJSONDoesNotPrintSecretBearingWorkflowValues(t *testing.T) {
	dir := writePackFixture(t, `{
		"schema_version":1,
		"id":"example.inspect",
		"name":"Inspect",
		"version":"0.1.0",
		"entry_workflow":"workflows/main.json",
		"required_credentials":[],
		"supported_platforms":["`+pack.CurrentPlatform()+`"]
	}`, `{
		"name":"Inspect",
		"nodes":[{"id":"send","type":"telegramBot","params":{"bot_token":"should-not-print","chat_id":"1","message":"hello"}}],
		"edges":[]
	}`)
	var stdout, stderr bytes.Buffer
	code := Runner{Stdout: &stdout, Stderr: &stderr, Stdin: strings.NewReader("")}.Run([]string{"pack", "inspect", dir, "--output", "json"})
	if code == ExitOK {
		t.Fatalf("expected embedded secret validation failure")
	}
	if strings.Contains(stdout.String(), "should-not-print") || strings.Contains(stderr.String(), "should-not-print") {
		t.Fatalf("inspect leaked workflow value stdout=%q stderr=%q", stdout.String(), stderr.String())
	}
}

func TestPackSubcommandsRejectInvalidOutput(t *testing.T) {
	dir := writePackFixture(t, `{
		"schema_version":1,
		"id":"example.output",
		"name":"Output",
		"version":"0.1.0",
		"entry_workflow":"workflows/main.json",
		"required_credentials":[],
		"supported_platforms":["`+pack.CurrentPlatform()+`"]
	}`, `{
		"name":"Output",
		"nodes":[{"id":"trigger","type":"webhookTrigger","params":{}}],
		"edges":[]
	}`)
	var stdout, stderr bytes.Buffer
	code := Runner{Stdout: &stdout, Stderr: &stderr, Stdin: strings.NewReader("")}.Run([]string{"pack", "inspect", dir, "--output", "xml"})
	if code != ExitInvalidInput || !strings.Contains(stderr.String(), "--output must be table or json") {
		t.Fatalf("expected invalid output error, code=%d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
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
