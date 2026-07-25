package cli

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"goflow/internal/client"
)

const (
	ExitOK              = 0
	ExitExecutionFailed = 1
	ExitInvalidInput    = 2
	ExitAuthFailed      = 3
	ExitUnavailable     = 4
	ExitNotFound        = 5
	ExitTimeout         = 7
	ExitCancelled       = 8
)

type Runner struct {
	Stdout io.Writer
	Stderr io.Writer
	Stdin  io.Reader
}

type clientOptions struct {
	url    *string
	apiKey *string
}

func Run(args []string, stdout, stderr io.Writer) int {
	return Runner{Stdout: stdout, Stderr: stderr, Stdin: os.Stdin}.Run(args)
}

func (r Runner) Run(args []string) int {
	if len(args) == 0 {
		r.usage()
		return ExitInvalidInput
	}

	switch args[0] {
	case "help", "-h", "--help":
		r.usage()
		return ExitOK
	case "status":
		return r.status(args[1:])
	case "workflow":
		return r.workflow(args[1:])
	case "execution":
		return r.execution(args[1:])
	default:
		fmt.Fprintf(r.Stderr, "unknown command: %s\n\n", args[0])
		r.usage()
		return ExitInvalidInput
	}
}

func (r Runner) usage() {
	fmt.Fprintln(r.Stdout, `Goflow CLI

Usage:
  goflow serve
  goflow status [--output table|json]
  goflow workflow list [--active] [--output table|json]
  goflow workflow describe <workflow-id|slug|name> [--output table|json]
  goflow workflow run <workflow-id|slug|name> [--json JSON | --input file | --stdin | --set key=value] [--idempotency-key key] [--wait] [--timeout 60s] [--output table|json]
  goflow execution get <execution-id> [--output table|json]
  goflow execution watch <execution-id> [--timeout 60s] [--interval 1s] [--output table|json]

Environment:
  GOFLOW_URL      Goflow server URL, default http://127.0.0.1:8080
  GOFLOW_API_KEY  Optional API key for secured instances`)
}

func addClientFlags(fs *flag.FlagSet) clientOptions {
	return clientOptions{
		url:    fs.String("url", envDefault("GOFLOW_URL", "http://127.0.0.1:8080"), "Goflow server URL"),
		apiKey: fs.String("api-key", os.Getenv("GOFLOW_API_KEY"), "Goflow API key"),
	}
}

func cliClient(opts clientOptions) *client.Client {
	return client.New(*opts.url, *opts.apiKey)
}

func (r Runner) status(args []string) int {
	fs := flag.NewFlagSet("status", flag.ContinueOnError)
	fs.SetOutput(r.Stderr)
	output := fs.String("output", "table", "Output format: table or json")
	clientOpts := addClientFlags(fs)
	if err := fs.Parse(args); err != nil {
		return ExitInvalidInput
	}

	c := cliClient(clientOpts)
	workflows, err := c.ListWorkflows()
	if err != nil {
		return r.apiError(err)
	}
	active := 0
	for _, workflow := range workflows {
		if workflow.IsActive {
			active++
		}
	}
	payload := map[string]interface{}{
		"status":           "online",
		"url":              c.BaseURL,
		"workflows":        len(workflows),
		"active_workflows": active,
	}
	if *output == "json" {
		return writeJSON(r.Stdout, payload)
	}
	fmt.Fprintf(r.Stdout, "Goflow server: online\nURL: %s\nWorkflows: %d\nActive workflows: %d\n", c.BaseURL, len(workflows), active)
	return ExitOK
}

func (r Runner) workflow(args []string) int {
	if len(args) == 0 {
		fmt.Fprintln(r.Stderr, "workflow subcommand is required")
		return ExitInvalidInput
	}
	switch args[0] {
	case "list":
		return r.workflowList(args[1:])
	case "describe", "get":
		return r.workflowDescribe(args[1:])
	case "run":
		return r.workflowRun(args[1:])
	default:
		fmt.Fprintf(r.Stderr, "unknown workflow subcommand: %s\n", args[0])
		return ExitInvalidInput
	}
}

func (r Runner) workflowList(args []string) int {
	fs := flag.NewFlagSet("workflow list", flag.ContinueOnError)
	fs.SetOutput(r.Stderr)
	activeOnly := fs.Bool("active", false, "Show active workflows only")
	output := fs.String("output", "table", "Output format: table or json")
	clientOpts := addClientFlags(fs)
	if err := fs.Parse(args); err != nil {
		return ExitInvalidInput
	}
	c := cliClient(clientOpts)
	workflows, err := c.ListWorkflows()
	if err != nil {
		return r.apiError(err)
	}
	filtered := workflows[:0]
	for _, workflow := range workflows {
		if *activeOnly && !workflow.IsActive {
			continue
		}
		filtered = append(filtered, workflow)
	}
	if *output == "json" {
		return writeJSON(r.Stdout, filtered)
	}
	fmt.Fprintf(r.Stdout, "%-38s  %-7s  %-22s  %s\n", "ID", "ACTIVE", "SLUG", "NAME")
	for _, workflow := range filtered {
		fmt.Fprintf(r.Stdout, "%-38s  %-7t  %-22s  %s\n", workflow.ID, workflow.IsActive, workflow.Slug, workflow.Name)
	}
	return ExitOK
}

func (r Runner) workflowDescribe(args []string) int {
	fs := flag.NewFlagSet("workflow describe", flag.ContinueOnError)
	fs.SetOutput(r.Stderr)
	output := fs.String("output", "table", "Output format: table or json")
	clientOpts := addClientFlags(fs)
	ref, ok, code := parseOneRef(fs, args, "workflow reference", r.Stderr)
	if !ok {
		return code
	}
	c := cliClient(clientOpts)
	workflow, err := c.ResolveWorkflow(ref)
	if err != nil {
		return r.apiError(err)
	}
	if *output == "json" {
		return writeJSON(r.Stdout, workflow)
	}
	fmt.Fprintf(r.Stdout, "ID: %s\nName: %s\nActive: %t\nSlug: %s\nDescription: %s\nCLI exposed: %t\nMCP exposed: %t\nRisk: %s\nConcurrency: %s\n", workflow.ID, workflow.Name, workflow.IsActive, workflow.Slug, workflow.Description, workflow.ExposeCLI, workflow.ExposeMCP, workflow.RiskLevel, workflow.ConcurrencyPolicy)
	return ExitOK
}

func (r Runner) workflowRun(args []string) int {
	fs := flag.NewFlagSet("workflow run", flag.ContinueOnError)
	fs.SetOutput(r.Stderr)
	jsonInput := fs.String("json", "", "JSON input object")
	inputPath := fs.String("input", "", "Path to JSON input file")
	useStdin := fs.Bool("stdin", false, "Read JSON input from stdin")
	setValues := multiFlag{}
	fs.Var(&setValues, "set", "Set input key=value; can be repeated")
	idempotencyKey := fs.String("idempotency-key", "", "Idempotency key")
	wait := fs.Bool("wait", false, "Wait for execution to finish")
	timeout := fs.Duration("timeout", 60*time.Second, "Wait timeout")
	interval := fs.Duration("interval", time.Second, "Watch polling interval")
	output := fs.String("output", "table", "Output format: table or json")
	clientOpts := addClientFlags(fs)
	ref, ok, code := parseOneRef(fs, args, "workflow reference", r.Stderr)
	if !ok {
		return code
	}

	input, err := readInput(*jsonInput, *inputPath, *useStdin, setValues, r.Stdin)
	if err != nil {
		fmt.Fprintln(r.Stderr, err)
		return ExitInvalidInput
	}

	c := cliClient(clientOpts)
	workflow, err := c.ResolveWorkflow(ref)
	if err != nil {
		return r.apiError(err)
	}
	accepted, err := c.RunWorkflow(workflow.ID, input, *idempotencyKey)
	if err != nil {
		return r.apiError(err)
	}
	if *wait {
		exec, code := r.watchExecution(c, accepted.ExecutionID, *timeout, *interval)
		if code != ExitOK {
			return code
		}
		if *output == "json" {
			return writeJSON(r.Stdout, exec)
		}
		printExecution(r.Stdout, exec)
		return executionExitCode(exec.Status)
	}
	if *output == "json" {
		return writeJSON(r.Stdout, accepted)
	}
	fmt.Fprintf(r.Stdout, "Execution started\nExecution ID: %s\nWorkflow ID: %s\nStatus: %s\nDeduplicated: %t\n", accepted.ExecutionID, accepted.WorkflowID, accepted.Status, accepted.Deduplicated)
	return ExitOK
}

func (r Runner) execution(args []string) int {
	if len(args) == 0 {
		fmt.Fprintln(r.Stderr, "execution subcommand is required")
		return ExitInvalidInput
	}
	switch args[0] {
	case "get":
		return r.executionGet(args[1:])
	case "watch":
		return r.executionWatch(args[1:])
	default:
		fmt.Fprintf(r.Stderr, "unknown execution subcommand: %s\n", args[0])
		return ExitInvalidInput
	}
}

func (r Runner) executionGet(args []string) int {
	fs := flag.NewFlagSet("execution get", flag.ContinueOnError)
	fs.SetOutput(r.Stderr)
	output := fs.String("output", "table", "Output format: table or json")
	clientOpts := addClientFlags(fs)
	ref, ok, code := parseOneRef(fs, args, "execution ID", r.Stderr)
	if !ok {
		return code
	}
	c := cliClient(clientOpts)
	exec, err := c.GetExecution(ref)
	if err != nil {
		return r.apiError(err)
	}
	if *output == "json" {
		return writeJSON(r.Stdout, exec)
	}
	printExecution(r.Stdout, exec)
	return executionExitCode(exec.Status)
}

func (r Runner) executionWatch(args []string) int {
	fs := flag.NewFlagSet("execution watch", flag.ContinueOnError)
	fs.SetOutput(r.Stderr)
	timeout := fs.Duration("timeout", 60*time.Second, "Watch timeout")
	interval := fs.Duration("interval", time.Second, "Polling interval")
	output := fs.String("output", "table", "Output format: table or json")
	clientOpts := addClientFlags(fs)
	ref, ok, code := parseOneRef(fs, args, "execution ID", r.Stderr)
	if !ok {
		return code
	}
	c := cliClient(clientOpts)
	exec, code := r.watchExecution(c, ref, *timeout, *interval)
	if code != ExitOK {
		return code
	}
	if *output == "json" {
		return writeJSON(r.Stdout, exec)
	}
	printExecution(r.Stdout, exec)
	return executionExitCode(exec.Status)
}

func (r Runner) watchExecution(c *client.Client, executionID string, timeout, interval time.Duration) (*client.Execution, int) {
	deadline := time.Now().Add(timeout)
	for {
		exec, err := c.GetExecution(executionID)
		if err != nil {
			return nil, r.apiError(err)
		}
		if isTerminal(exec.Status) {
			return exec, ExitOK
		}
		if time.Now().After(deadline) {
			fmt.Fprintf(r.Stderr, "timed out waiting for execution %s\n", executionID)
			return nil, ExitTimeout
		}
		time.Sleep(interval)
	}
}

func parseOneRef(fs *flag.FlagSet, args []string, label string, stderr io.Writer) (string, bool, int) {
	ref := ""
	parseArgs := args
	if len(args) > 0 && !strings.HasPrefix(args[0], "-") {
		ref = args[0]
		parseArgs = args[1:]
	}
	if err := fs.Parse(parseArgs); err != nil {
		return "", false, ExitInvalidInput
	}
	if ref == "" && fs.NArg() > 0 {
		ref = fs.Arg(0)
	}
	if ref == "" {
		fmt.Fprintf(stderr, "%s is required\n", label)
		return "", false, ExitInvalidInput
	}
	if fs.NArg() > 1 || (len(args) > 0 && !strings.HasPrefix(args[0], "-") && fs.NArg() > 0) {
		fmt.Fprintf(stderr, "too many arguments for %s\n", label)
		return "", false, ExitInvalidInput
	}
	return ref, true, ExitOK
}

func readInput(jsonInput, inputPath string, useStdin bool, setValues []string, stdin io.Reader) (map[string]interface{}, error) {
	sources := 0
	for _, used := range []bool{jsonInput != "", inputPath != "", useStdin, len(setValues) > 0} {
		if used {
			sources++
		}
	}
	if sources > 1 {
		return nil, fmt.Errorf("use only one input source: --json, --input, --stdin, or --set")
	}
	if jsonInput != "" {
		return parseJSONObject([]byte(jsonInput))
	}
	if inputPath != "" {
		data, err := os.ReadFile(inputPath)
		if err != nil {
			return nil, err
		}
		return parseJSONObject(data)
	}
	if useStdin {
		data, err := io.ReadAll(stdin)
		if err != nil {
			return nil, err
		}
		return parseJSONObject(data)
	}
	if len(setValues) > 0 {
		out := map[string]interface{}{}
		for _, item := range setValues {
			key, value, ok := strings.Cut(item, "=")
			if !ok || strings.TrimSpace(key) == "" {
				return nil, fmt.Errorf("invalid --set value %q, expected key=value", item)
			}
			out[strings.TrimSpace(key)] = value
		}
		return out, nil
	}
	return map[string]interface{}{}, nil
}

func parseJSONObject(data []byte) (map[string]interface{}, error) {
	var out map[string]interface{}
	if err := json.Unmarshal(data, &out); err != nil {
		return nil, fmt.Errorf("input must be a JSON object: %w", err)
	}
	if out == nil {
		out = map[string]interface{}{}
	}
	return out, nil
}

func printExecution(w io.Writer, exec *client.Execution) {
	fmt.Fprintf(w, "ID: %s\nWorkflow ID: %s\nStatus: %s\nDuration: %dms\nStarted: %s\n", exec.ID, exec.WorkflowID, exec.Status, exec.DurationMs, exec.StartedAt)
	if exec.FinishedAt != "" {
		fmt.Fprintf(w, "Finished: %s\n", exec.FinishedAt)
	}
	if exec.TriggerSource != "" {
		fmt.Fprintf(w, "Trigger source: %s\n", exec.TriggerSource)
	}
	if exec.IdempotencyKey != "" {
		fmt.Fprintf(w, "Idempotency key: %s\n", exec.IdempotencyKey)
	}
	if exec.ErrorMessage != "" {
		fmt.Fprintf(w, "Error: %s\n", exec.ErrorMessage)
	}
}

func writeJSON(w io.Writer, value interface{}) int {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	if err := enc.Encode(value); err != nil {
		return ExitInvalidInput
	}
	return ExitOK
}

func (r Runner) apiError(err error) int {
	msg := err.Error()
	fmt.Fprintln(r.Stderr, msg)
	lower := strings.ToLower(msg)
	switch {
	case strings.Contains(lower, "401") || strings.Contains(lower, "unauthorized"):
		return ExitAuthFailed
	case strings.Contains(lower, "not found") || strings.Contains(lower, "404"):
		return ExitNotFound
	case strings.Contains(lower, "cannot connect"):
		return ExitUnavailable
	default:
		return ExitExecutionFailed
	}
}

func executionExitCode(status string) int {
	switch strings.ToUpper(status) {
	case "SUCCESS":
		return ExitOK
	case "CANCELLED":
		return ExitCancelled
	case "RUNNING", "QUEUED":
		return ExitOK
	default:
		return ExitExecutionFailed
	}
}

func isTerminal(status string) bool {
	switch strings.ToUpper(status) {
	case "SUCCESS", "FAILED", "CANCELLED", "INTERRUPTED", "REJECTED":
		return true
	default:
		return false
	}
}

func envDefault(name, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(name)); value != "" {
		return value
	}
	return fallback
}

type multiFlag []string

func (m *multiFlag) String() string {
	return strings.Join(*m, ",")
}

func (m *multiFlag) Set(value string) error {
	*m = append(*m, value)
	return nil
}
