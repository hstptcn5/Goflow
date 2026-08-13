package cli

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"goflow/internal/client"
	"goflow/internal/mcpserver"
	"goflow/internal/pack"
	"goflow/internal/packrun"
	"goflow/internal/packsetup"
	"goflow/internal/workflow"
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
	Stdout          io.Writer
	Stderr          io.Writer
	Stdin           io.Reader
	PackRuntimePath string
	UIFS            fs.FS
	AppVersion      string
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
	case "pack":
		return r.pack(args[1:])
	case "execution":
		return r.execution(args[1:])
	case "token":
		return r.token(args[1:])
	case "mcp":
		return r.mcp(args[1:])
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
  goflow workflow export <workflow-id|slug|name> [--output file]
  goflow workflow import <file> [--activate]
  goflow workflow validate <file>
  goflow pack init <directory> --id <id> --name <name> [--target <goos-goarch>] [--force]
  goflow pack validate <pack-directory>
  goflow pack inspect <pack-directory|bundle.zip|extracted-directory> [--output table|json]
  goflow pack test <pack-directory> [--output table|json]
  goflow pack verify <bundle.zip|extracted-directory> [--output table|json]
  goflow pack build <pack-directory> --output <output-directory> [--target goos-goarch] [--force]
  goflow pack run <pack-directory> [--data-dir directory] [--port port] [--no-open]
  goflow execution get <execution-id> [--output table|json]
  goflow execution watch <execution-id> [--timeout 60s] [--interval 1s] [--output table|json]
  goflow execution cancel <execution-id> [--output table|json]
  goflow token list [--output table|json]
  goflow token create <name> --scope scope [--scope scope] [--workflow workflow-id] [--output table|json]
  goflow token delete <token-id>
  goflow mcp stdio

Environment:
  GOFLOW_URL      Goflow server URL, default http://127.0.0.1:8080
  GOFLOW_API_KEY  Optional API key or scoped token for secured instances

PowerShell tip:
  Prefer --set or --input payload.json. Inline --json quoting can be fragile on Windows PowerShell.`)
}

func (r Runner) token(args []string) int {
	if len(args) == 0 {
		fmt.Fprintln(r.Stderr, "token subcommand is required")
		return ExitInvalidInput
	}
	switch args[0] {
	case "list":
		return r.tokenList(args[1:])
	case "create":
		return r.tokenCreate(args[1:])
	case "delete":
		return r.tokenDelete(args[1:])
	default:
		fmt.Fprintf(r.Stderr, "unknown token subcommand: %s\n", args[0])
		return ExitInvalidInput
	}
}

func (r Runner) tokenList(args []string) int {
	fs := flag.NewFlagSet("token list", flag.ContinueOnError)
	fs.SetOutput(r.Stderr)
	output := fs.String("output", "table", "Output format: table or json")
	clientOpts := addClientFlags(fs)
	if err := fs.Parse(args); err != nil {
		return ExitInvalidInput
	}
	c := cliClient(clientOpts)
	tokens, err := c.ListTokens()
	if err != nil {
		return r.apiError(err)
	}
	if *output == "json" {
		return writeJSON(r.Stdout, tokens)
	}
	fmt.Fprintf(r.Stdout, "%-38s  %-20s  %-28s  %s\n", "ID", "NAME", "SCOPES", "WORKFLOWS")
	for _, token := range tokens {
		fmt.Fprintf(r.Stdout, "%-38s  %-20s  %-28s  %s\n", token.ID, token.Name, strings.Join(token.Scopes, ","), strings.Join(token.AllowedWorkflows, ","))
	}
	return ExitOK
}

func (r Runner) tokenCreate(args []string) int {
	fs := flag.NewFlagSet("token create", flag.ContinueOnError)
	fs.SetOutput(r.Stderr)
	scopes := multiFlag{}
	workflows := multiFlag{}
	fs.Var(&scopes, "scope", "Token scope; can be repeated")
	fs.Var(&workflows, "workflow", "Allowed workflow ID; can be repeated. Omit for all workflows")
	output := fs.String("output", "table", "Output format: table or json")
	clientOpts := addClientFlags(fs)
	name, ok, code := parseOneRef(fs, args, "token name", r.Stderr)
	if !ok {
		return code
	}
	if len(scopes) == 0 {
		fmt.Fprintln(r.Stderr, "at least one --scope is required")
		return ExitInvalidInput
	}
	c := cliClient(clientOpts)
	token, err := c.CreateToken(client.CreateTokenRequest{
		Name:             name,
		Scopes:           scopes,
		AllowedWorkflows: workflows,
	})
	if err != nil {
		return r.apiError(err)
	}
	if *output == "json" {
		return writeJSON(r.Stdout, token)
	}
	fmt.Fprintf(r.Stdout, "Token created\nID: %s\nName: %s\nScopes: %s\nAllowed workflows: %s\nToken: %s\n", token.ID, token.Name, strings.Join(token.Scopes, ","), strings.Join(token.AllowedWorkflows, ","), token.Token)
	return ExitOK
}

func (r Runner) tokenDelete(args []string) int {
	fs := flag.NewFlagSet("token delete", flag.ContinueOnError)
	fs.SetOutput(r.Stderr)
	clientOpts := addClientFlags(fs)
	id, ok, code := parseOneRef(fs, args, "token ID", r.Stderr)
	if !ok {
		return code
	}
	c := cliClient(clientOpts)
	if err := c.DeleteToken(id); err != nil {
		return r.apiError(err)
	}
	fmt.Fprintf(r.Stdout, "Deleted token %s\n", id)
	return ExitOK
}

func (r Runner) mcp(args []string) int {
	if len(args) == 0 {
		fmt.Fprintln(r.Stderr, "mcp subcommand is required")
		return ExitInvalidInput
	}
	switch args[0] {
	case "stdio":
		fs := flag.NewFlagSet("mcp stdio", flag.ContinueOnError)
		fs.SetOutput(r.Stderr)
		clientOpts := addClientFlags(fs)
		if err := fs.Parse(args[1:]); err != nil {
			return ExitInvalidInput
		}
		opts := mcpserver.OptionsFromEnv()
		opts.BaseURL = *clientOpts.url
		opts.APIKey = *clientOpts.apiKey
		if err := mcpserver.ValidateOptions(opts); err != nil {
			fmt.Fprintln(r.Stderr, err)
			return ExitInvalidInput
		}
		if err := mcpserver.RunStdio(context.Background(), opts); err != nil {
			fmt.Fprintln(r.Stderr, err)
			return ExitExecutionFailed
		}
		return ExitOK
	default:
		fmt.Fprintf(r.Stderr, "unknown mcp subcommand: %s\n", args[0])
		return ExitInvalidInput
	}
}

func addClientFlags(fs *flag.FlagSet) clientOptions {
	return clientOptions{
		url:    fs.String("url", envDefault("GOFLOW_URL", "http://127.0.0.1:8080"), "Goflow server URL"),
		apiKey: fs.String("api-key", os.Getenv("GOFLOW_API_KEY"), "Goflow API key"),
	}
}

func cliClient(opts clientOptions) *client.Client {
	return client.New(*opts.url, *opts.apiKey).WithTriggerSource("cli")
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
	case "export":
		return r.workflowExport(args[1:])
	case "import":
		return r.workflowImport(args[1:])
	case "validate":
		return r.workflowValidate(args[1:])
	default:
		fmt.Fprintf(r.Stderr, "unknown workflow subcommand: %s\n", args[0])
		return ExitInvalidInput
	}
}

func (r Runner) pack(args []string) int {
	if len(args) == 0 {
		fmt.Fprintln(r.Stderr, "pack subcommand is required")
		return ExitInvalidInput
	}
	switch args[0] {
	case "init":
		return r.packInit(args[1:])
	case "validate":
		return r.packValidate(args[1:])
	case "inspect":
		return r.packInspect(args[1:])
	case "test":
		return r.packTest(args[1:])
	case "verify":
		return r.packVerify(args[1:])
	case "build":
		return r.packBuild(args[1:])
	case "run":
		return r.packRun(args[1:])
	default:
		fmt.Fprintf(r.Stderr, "unknown pack subcommand: %s\n", args[0])
		return ExitInvalidInput
	}
}

func (r Runner) packInit(args []string) int {
	fs := flag.NewFlagSet("pack init", flag.ContinueOnError)
	fs.SetOutput(r.Stderr)
	id := fs.String("id", "", "Pack ID, for example example.daily-report")
	name := fs.String("name", "", "Human-readable pack name")
	target := fs.String("target", pack.CurrentPlatform(), "Target platform in goos-goarch format")
	force := fs.Bool("force", false, "Replace an existing target directory")
	dir, ok, code := parseOneRef(fs, args, "pack directory", r.Stderr)
	if !ok {
		return code
	}
	if strings.TrimSpace(*id) == "" || strings.TrimSpace(*name) == "" {
		fmt.Fprintln(r.Stderr, "--id and --name are required")
		return ExitInvalidInput
	}
	if err := scaffoldPack(dir, *id, *name, *target, *force); err != nil {
		fmt.Fprintln(r.Stderr, err)
		return ExitInvalidInput
	}
	fmt.Fprintf(r.Stdout, "Initialized pack\nDirectory: %s\nID: %s\n", dir, *id)
	return ExitOK
}

func (r Runner) packValidate(args []string) int {
	fs := flag.NewFlagSet("pack validate", flag.ContinueOnError)
	fs.SetOutput(r.Stderr)
	dir, ok, code := parseOneRef(fs, args, "pack directory", r.Stderr)
	if !ok {
		return code
	}
	loaded, err := pack.Load(dir)
	if err != nil {
		fmt.Fprintln(r.Stderr, err)
		return ExitInvalidInput
	}
	fmt.Fprintf(r.Stdout, "Pack is valid\nID: %s\nVersion: %s\nEntry workflow: %s\n", loaded.Manifest.ID, loaded.Manifest.Version, loaded.Manifest.EntryWorkflow)
	return ExitOK
}

func (r Runner) packInspect(args []string) int {
	fs := flag.NewFlagSet("pack inspect", flag.ContinueOnError)
	fs.SetOutput(r.Stderr)
	output := fs.String("output", "table", "Output format: table or json")
	ref, ok, code := parseOneRef(fs, args, "pack reference", r.Stderr)
	if !ok {
		return code
	}
	result, err := inspectPackReference(ref)
	if err != nil {
		fmt.Fprintln(r.Stderr, err)
		return ExitInvalidInput
	}
	return writePackCommandOutput(r.Stdout, r.Stderr, *output, result, func() {
		fmt.Fprintf(r.Stdout, "Pack: %s\nVersion: %s\nKind: %s\nIntegrity: %s\nTarget: %s\nEntry workflow: %s\nConfig fields: %d\nCredential requirements: %d\nBindings: %d\nPlugins: %d\nAssets: %d\n", result.ID, result.Version, result.Kind, result.Integrity, result.Target, result.EntryWorkflow, result.ConfigFields, result.CredentialRequirements, result.Bindings, result.Plugins, result.Assets)
	})
}

func (r Runner) packTest(args []string) int {
	fs := flag.NewFlagSet("pack test", flag.ContinueOnError)
	fs.SetOutput(r.Stderr)
	output := fs.String("output", "table", "Output format: table or json")
	dir, ok, code := parseOneRef(fs, args, "pack directory", r.Stderr)
	if !ok {
		return code
	}
	result, err := testPackOffline(dir)
	if err != nil {
		result.Status = "FAILED"
		result.Error = err.Error()
		if *output == "json" {
			if code := writePackCommandOutput(r.Stdout, r.Stderr, *output, result, func() {}); code != ExitOK {
				return code
			}
			return ExitInvalidInput
		}
		fmt.Fprintln(r.Stderr, err)
		return ExitInvalidInput
	}
	return writePackCommandOutput(r.Stdout, r.Stderr, *output, result, func() {
		fmt.Fprintf(r.Stdout, "Pack test: %s\nID: %s\nValidation: %s\nSetup: %s\nManaged workflow: %s\nConnection tests: %s\n", result.Status, result.ID, result.Validation, result.Setup, result.ManagedWorkflow, result.ConnectionTests)
	})
}

func (r Runner) packVerify(args []string) int {
	fs := flag.NewFlagSet("pack verify", flag.ContinueOnError)
	fs.SetOutput(r.Stderr)
	output := fs.String("output", "table", "Output format: table or json")
	ref, ok, code := parseOneRef(fs, args, "bundle reference", r.Stderr)
	if !ok {
		return code
	}
	result, err := verifyPackReference(ref)
	if err != nil {
		result.Status = "FAILED"
		result.Error = err.Error()
		if *output == "json" {
			if code := writePackCommandOutput(r.Stdout, r.Stderr, *output, result, func() {}); code != ExitOK {
				return code
			}
			return ExitInvalidInput
		}
		fmt.Fprintln(r.Stderr, err)
		return ExitInvalidInput
	}
	return writePackCommandOutput(r.Stdout, r.Stderr, *output, result, func() {
		fmt.Fprintf(r.Stdout, "Pack verification: %s\nKind: %s\nPack: %s\nVersion: %s\nTarget: %s\n", result.Status, result.Kind, result.ID, result.Version, result.Target)
	})
}

func (r Runner) packBuild(args []string) int {
	fs := flag.NewFlagSet("pack build", flag.ContinueOnError)
	fs.SetOutput(r.Stderr)
	outputDir := fs.String("output", "", "Output directory for the portable pack bundle")
	target := fs.String("target", pack.CurrentPlatform(), "Target platform in goos-goarch format")
	force := fs.Bool("force", false, "Replace the destination archive if it already exists")
	dir, ok, code := parseOneRef(fs, args, "pack directory", r.Stderr)
	if !ok {
		return code
	}
	if strings.TrimSpace(*outputDir) == "" {
		fmt.Fprintln(r.Stderr, "--output is required")
		return ExitInvalidInput
	}
	result, err := pack.Build(pack.BuildOptions{
		PackDir:     dir,
		OutputDir:   *outputDir,
		Target:      *target,
		Force:       *force,
		RuntimePath: r.PackRuntimePath,
	})
	if err != nil {
		fmt.Fprintln(r.Stderr, err)
		return ExitInvalidInput
	}
	fmt.Fprintf(r.Stdout, "Built portable pack bundle\nArchive: %s\nTarget: %s\n", result.ArchivePath, result.Target)
	return ExitOK
}

func (r Runner) packRun(args []string) int {
	fs := flag.NewFlagSet("pack run", flag.ContinueOnError)
	fs.SetOutput(r.Stderr)
	dataDir := fs.String("data-dir", "", "Pack Run data directory")
	port := fs.Int("port", 0, "Loopback port; 0 asks the OS to choose a free port")
	noOpen := fs.Bool("no-open", false, "Print the URL without opening a browser")
	dir, ok, code := parseOneRef(fs, args, "pack directory", r.Stderr)
	if !ok {
		return code
	}
	if *port < 0 || *port > 65535 {
		fmt.Fprintln(r.Stderr, "--port must be between 0 and 65535")
		return ExitInvalidInput
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if err := packrun.Run(ctx, packrun.Options{
		PackDir:    dir,
		DataDir:    *dataDir,
		Port:       *port,
		NoOpen:     *noOpen,
		UIFS:       r.UIFS,
		AppVersion: r.AppVersion,
		Stdout:     r.Stdout,
		Stderr:     r.Stderr,
	}); err != nil {
		fmt.Fprintln(r.Stderr, err)
		return ExitExecutionFailed
	}
	return ExitOK
}

type packInspectResult struct {
	Kind                   string `json:"kind"`
	ID                     string `json:"id"`
	Name                   string `json:"name"`
	Version                string `json:"version"`
	Target                 string `json:"target,omitempty"`
	EntryWorkflow          string `json:"entry_workflow"`
	Integrity              string `json:"integrity"`
	ConfigFields           int    `json:"config_fields"`
	CredentialRequirements int    `json:"credential_requirements"`
	Bindings               int    `json:"bindings"`
	Plugins                int    `json:"plugins"`
	Assets                 int    `json:"assets"`
}

type packTestResult struct {
	Status          string   `json:"status"`
	ID              string   `json:"id,omitempty"`
	Validation      string   `json:"validation,omitempty"`
	Setup           string   `json:"setup,omitempty"`
	ManagedWorkflow string   `json:"managed_workflow,omitempty"`
	ConnectionTests string   `json:"connection_tests,omitempty"`
	Skipped         []string `json:"skipped,omitempty"`
	Error           string   `json:"error,omitempty"`
}

type packVerifyResult struct {
	Status  string `json:"status"`
	Kind    string `json:"kind,omitempty"`
	ID      string `json:"id,omitempty"`
	Version string `json:"version,omitempty"`
	Target  string `json:"target,omitempty"`
	Error   string `json:"error,omitempty"`
}

func writePackCommandOutput(stdout, stderr io.Writer, output string, value interface{}, writeTable func()) int {
	switch strings.ToLower(strings.TrimSpace(output)) {
	case "", "table":
		writeTable()
		return ExitOK
	case "json":
		data, err := json.MarshalIndent(value, "", "  ")
		if err != nil {
			fmt.Fprintln(stderr, err)
			return ExitInvalidInput
		}
		data = append(data, '\n')
		_, _ = stdout.Write(data)
		return ExitOK
	default:
		fmt.Fprintln(stderr, "--output must be table or json")
		return ExitInvalidInput
	}
}

func scaffoldPack(dir, id, name, target string, force bool) error {
	if strings.TrimSpace(target) == "" {
		target = pack.CurrentPlatform()
	}
	if info, err := os.Stat(dir); err == nil {
		if !info.IsDir() {
			return fmt.Errorf("target path exists and is not a directory")
		}
		entries, err := os.ReadDir(dir)
		if err != nil {
			return fmt.Errorf("read target directory: %w", err)
		}
		if len(entries) > 0 && !force {
			return fmt.Errorf("target directory is not empty; pass --force to replace it")
		}
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("check target directory: %w", err)
	}
	parent := filepath.Dir(dir)
	if err := os.MkdirAll(parent, 0700); err != nil {
		return fmt.Errorf("create parent directory: %w", err)
	}
	temp, err := os.MkdirTemp(parent, "."+filepath.Base(dir)+".tmp-*")
	if err != nil {
		return fmt.Errorf("create temporary scaffold: %w", err)
	}
	defer func() { _ = os.RemoveAll(temp) }()
	if err := os.MkdirAll(filepath.Join(temp, "workflows"), 0700); err != nil {
		return err
	}
	manifest := pack.Manifest{
		SchemaVersion:       pack.SupportedSchema,
		ID:                  id,
		Name:                name,
		Version:             "0.1.0",
		Description:         "A portable Goflow pack.",
		EntryWorkflow:       pack.DefaultWorkflowPath,
		RequiredCredentials: []string{},
		SupportedPlatforms:  []string{target},
	}
	workflowData := map[string]interface{}{
		"name":        name,
		"description": "Scaffolded safe workflow.",
		"is_active":   false,
		"nodes": []map[string]interface{}{
			{"id": "trigger", "type": "webhookTrigger", "name": "Manual input", "params": map[string]interface{}{}},
		},
		"edges": []interface{}{},
	}
	if err := writeJSONFile(filepath.Join(temp, pack.ManifestFile), manifest); err != nil {
		return err
	}
	if err := writeJSONFile(filepath.Join(temp, pack.DefaultWorkflowPath), workflowData); err != nil {
		return err
	}
	readme := fmt.Sprintf("# %s\n\nPack ID: `%s`\n\nThis scaffold contains no credentials or secret example values.\n", name, id)
	if err := os.WriteFile(filepath.Join(temp, "README.md"), []byte(readme), 0600); err != nil {
		return fmt.Errorf("write README.md: %w", err)
	}
	if _, err := pack.Load(temp); err != nil {
		return fmt.Errorf("validate scaffold: %w", err)
	}
	if force {
		if err := os.RemoveAll(dir); err != nil {
			return fmt.Errorf("replace target directory: %w", err)
		}
	}
	if err := os.Rename(temp, dir); err != nil {
		return fmt.Errorf("publish scaffold: %w", err)
	}
	return nil
}

func inspectPackReference(ref string) (packInspectResult, error) {
	if strings.HasSuffix(strings.ToLower(ref), ".zip") {
		info, err := pack.ReadBundleArchiveInfo(ref)
		if err != nil {
			return packInspectResult{}, err
		}
		integrity := "valid"
		if err := pack.VerifyBundleArchiveFile(ref); err != nil {
			integrity = "invalid: " + err.Error()
		}
		return packInspectResult{Kind: "bundle_zip", ID: info.PackID, Version: info.PackVersion, Target: info.Target, EntryWorkflow: info.EntryWorkflow, Integrity: integrity}, nil
	}
	if _, err := os.Stat(filepath.Join(ref, "PACK_INFO.json")); err == nil {
		info, err := pack.VerifyExtractedBundle(ref)
		if err != nil {
			return packInspectResult{}, err
		}
		return packInspectResult{Kind: "extracted_bundle", ID: info.PackID, Version: info.PackVersion, Target: info.Target, EntryWorkflow: info.EntryWorkflow, Integrity: "valid"}, nil
	}
	loaded, err := pack.Load(ref)
	if err != nil {
		return packInspectResult{}, err
	}
	m := loaded.Manifest
	return packInspectResult{
		Kind:                   "pack_directory",
		ID:                     m.ID,
		Name:                   m.Name,
		Version:                m.Version,
		Target:                 strings.Join(m.SupportedPlatforms, ","),
		EntryWorkflow:          m.EntryWorkflow,
		Integrity:              "source_valid",
		ConfigFields:           len(m.ConfigSchema),
		CredentialRequirements: len(m.CredentialRequirements),
		Bindings:               len(m.Bindings),
		Plugins:                len(m.Plugins),
		Assets:                 len(m.Assets),
	}, nil
}

func verifyPackReference(ref string) (packVerifyResult, error) {
	if strings.HasSuffix(strings.ToLower(ref), ".zip") {
		if err := pack.VerifyBundleArchiveFile(ref); err != nil {
			return packVerifyResult{Kind: "bundle_zip"}, err
		}
		info, err := pack.ReadBundleArchiveInfo(ref)
		if err != nil {
			return packVerifyResult{Kind: "bundle_zip"}, err
		}
		return packVerifyResult{Status: "PASS", Kind: "bundle_zip", ID: info.PackID, Version: info.PackVersion, Target: info.Target}, nil
	}
	info, err := pack.VerifyExtractedBundle(ref)
	if err != nil {
		return packVerifyResult{Kind: "extracted_bundle"}, err
	}
	return packVerifyResult{Status: "PASS", Kind: "extracted_bundle", ID: info.PackID, Version: info.PackVersion, Target: info.Target}, nil
}

func testPackOffline(dir string) (packTestResult, error) {
	loaded, err := pack.Load(dir)
	if err != nil {
		return packTestResult{Validation: "FAILED"}, err
	}
	result := packTestResult{Status: "PASS", ID: loaded.Manifest.ID, Validation: "PASS"}
	dataDir := filepath.Join(os.TempDir(), "goflow-pack-test-"+strings.ReplaceAll(loaded.Manifest.ID, ".", "-"))
	dataDir, err = os.MkdirTemp("", filepath.Base(dataDir)+"-*")
	if err != nil {
		return result, fmt.Errorf("create test data directory: %w", err)
	}
	defer func() { _ = os.RemoveAll(dataDir) }()
	configValues := syntheticConfigValues(loaded.Manifest.ConfigSchema)
	if _, err := packsetup.SaveConfig(dataDir, loaded.Manifest, configValues); err != nil {
		return result, err
	}
	credentialSlots := map[string]string{}
	for _, req := range loaded.Manifest.CredentialRequirements {
		credentialSlots[req.Key] = "fake-" + req.Key + "-credential"
		if req.TestKind != "" {
			result.Skipped = append(result.Skipped, req.Key+":"+req.TestKind)
		}
	}
	resolver := packsetup.CredentialLookupFunc(func(id string) (packsetup.CredentialIdentity, error) {
		for _, req := range loaded.Manifest.CredentialRequirements {
			if id == "fake-"+req.Key+"-credential" {
				return packsetup.CredentialIdentity{ID: id, Type: req.Type}, nil
			}
		}
		return packsetup.CredentialIdentity{}, fmt.Errorf("fake credential not found")
	})
	loadedCreds, err := packsetup.SaveCredentialBindings(dataDir, loaded.Manifest, credentialSlots, resolver)
	if err != nil {
		return result, err
	}
	result.Setup = "PASS"
	if len(result.Skipped) > 0 {
		result.ConnectionTests = "SKIPPED"
	} else {
		result.ConnectionTests = "NONE"
	}
	wf, err := workflow.ReadFileLimit(loaded.EntryWorkflowPath, pack.MaxWorkflowBytes)
	if err != nil {
		return result, err
	}
	cfg, err := packsetup.LoadConfig(dataDir, loaded.Manifest)
	if err != nil {
		return result, err
	}
	if _, err := packsetup.ApplyBindings(client.Workflow{
		ID:                "offline",
		Name:              wf.Name,
		NodesJSON:         wf.NodesJSON,
		EdgesJSON:         wf.EdgesJSON,
		InputSchemaJSON:   wf.InputSchemaJSON,
		OutputSchemaJSON:  wf.OutputSchemaJSON,
		ExposeCLI:         wf.ExposeCLI,
		ExposeMCP:         wf.ExposeMCP,
		RiskLevel:         wf.RiskLevel,
		MaxConcurrentRuns: wf.MaxConcurrentRuns,
		ConcurrencyPolicy: wf.ConcurrencyPolicy,
	}, loaded.Manifest, cfg.Config.Values, loadedCreds.Slots); err != nil {
		return result, err
	}
	if _, err := packrun.Prepare(context.Background(), loaded, dataDir); err != nil {
		return result, err
	}
	if _, err := packrun.Prepare(context.Background(), loaded, dataDir); err != nil {
		return result, err
	}
	result.ManagedWorkflow = "PASS"
	return result, nil
}

func syntheticConfigValues(fields []pack.ConfigField) map[string]interface{} {
	values := map[string]interface{}{}
	for _, field := range fields {
		if field.Default != nil {
			values[field.Key] = field.Default
			continue
		}
		switch field.Type {
		case "url":
			values[field.Key] = "https://example.test/data.json"
		case "integer":
			if field.Min != nil {
				values[field.Key] = *field.Min
			} else {
				values[field.Key] = 0
			}
		case "boolean":
			values[field.Key] = false
		case "select":
			if len(field.Options) > 0 {
				values[field.Key] = field.Options[0]
			} else {
				values[field.Key] = ""
			}
		default:
			values[field.Key] = "demo"
		}
	}
	return values
}

func writeJSONFile(path string, value interface{}) error {
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	if err := os.WriteFile(path, data, 0600); err != nil {
		return fmt.Errorf("write %s: %w", filepath.Base(path), err)
	}
	return nil
}

func (r Runner) workflowExport(args []string) int {
	fs := flag.NewFlagSet("workflow export", flag.ContinueOnError)
	fs.SetOutput(r.Stderr)
	outputPath := fs.String("output", "", "Write workflow JSON to file instead of stdout")
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
	data, err := json.MarshalIndent(workflow, "", "  ")
	if err != nil {
		fmt.Fprintln(r.Stderr, err)
		return ExitInvalidInput
	}
	data = append(data, '\n')
	if *outputPath != "" {
		if err := os.WriteFile(*outputPath, data, 0600); err != nil {
			fmt.Fprintln(r.Stderr, err)
			return ExitInvalidInput
		}
		fmt.Fprintf(r.Stdout, "Exported workflow %s to %s\n", workflow.ID, *outputPath)
		return ExitOK
	}
	_, _ = r.Stdout.Write(data)
	return ExitOK
}

func (r Runner) workflowImport(args []string) int {
	fs := flag.NewFlagSet("workflow import", flag.ContinueOnError)
	fs.SetOutput(r.Stderr)
	activate := fs.Bool("activate", false, "Import workflow as active")
	clientOpts := addClientFlags(fs)
	path, ok, code := parseOneRef(fs, args, "workflow file", r.Stderr)
	if !ok {
		return code
	}
	workflowDef, err := workflow.ReadFile(path)
	if err != nil {
		fmt.Fprintln(r.Stderr, err)
		return ExitInvalidInput
	}
	if err := workflow.ValidateDefinition(workflowDef); err != nil {
		fmt.Fprintln(r.Stderr, err)
		return ExitInvalidInput
	}
	workflowDef.ID = ""
	workflowDef.IsActive = *activate
	c := cliClient(clientOpts)
	created, err := c.CreateWorkflow(workflowDef)
	if err != nil {
		return r.apiError(err)
	}
	fmt.Fprintf(r.Stdout, "Imported workflow\nID: %s\nName: %s\nActive: %t\n", created.ID, created.Name, created.IsActive)
	return ExitOK
}

func (r Runner) workflowValidate(args []string) int {
	fs := flag.NewFlagSet("workflow validate", flag.ContinueOnError)
	fs.SetOutput(r.Stderr)
	path, ok, code := parseOneRef(fs, args, "workflow file", r.Stderr)
	if !ok {
		return code
	}
	workflowDef, err := workflow.ReadFile(path)
	if err != nil {
		fmt.Fprintln(r.Stderr, err)
		return ExitInvalidInput
	}
	if err := workflow.ValidateDefinition(workflowDef); err != nil {
		fmt.Fprintln(r.Stderr, err)
		return ExitInvalidInput
	}
	fmt.Fprintln(r.Stdout, "Workflow file is valid")
	return ExitOK
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
	case "cancel":
		return r.executionCancel(args[1:])
	default:
		fmt.Fprintf(r.Stderr, "unknown execution subcommand: %s\n", args[0])
		return ExitInvalidInput
	}
}

func (r Runner) executionCancel(args []string) int {
	fs := flag.NewFlagSet("execution cancel", flag.ContinueOnError)
	fs.SetOutput(r.Stderr)
	output := fs.String("output", "table", "Output format: table or json")
	clientOpts := addClientFlags(fs)
	ref, ok, code := parseOneRef(fs, args, "execution ID", r.Stderr)
	if !ok {
		return code
	}
	c := cliClient(clientOpts)
	result, err := c.CancelExecution(ref)
	if err != nil {
		return r.apiError(err)
	}
	if *output == "json" {
		return writeJSON(r.Stdout, result)
	}
	fmt.Fprintf(r.Stdout, "Cancellation requested\nExecution ID: %s\nStatus: %s\nMessage: %s\n", result.ID, result.Status, result.Message)
	return ExitOK
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
