package nodes

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

const reusableCodeSchemaVersion = 1

var reusableCodeTypePattern = regexp.MustCompile(`^user\.[A-Za-z][A-Za-z0-9_.-]{0,95}$`)

type ReusableCodeManifest struct {
	SchemaVersion int                      `json:"schema_version"`
	Type          NodeType                 `json:"type"`
	Name          string                   `json:"name"`
	Version       string                   `json:"version"`
	Description   string                   `json:"description,omitempty"`
	Runtime       string                   `json:"runtime"`
	Code          string                   `json:"code"`
	Inputs        []ParamDefinition        `json:"inputs,omitempty"`
	Outputs       []PluginOutputDefinition `json:"outputs,omitempty"`
	Timeout       int                      `json:"timeout_seconds,omitempty"`
}

type ReusableCodeExecutor struct {
	manifest ReusableCodeManifest
}

func DefaultReusableCodeDir() string {
	if configured := strings.TrimSpace(os.Getenv("GOFLOW_CUSTOM_NODE_DIR")); configured != "" {
		return configured
	}
	return "custom_nodes"
}

func validateReusableCodeManifest(manifest ReusableCodeManifest) error {
	if manifest.SchemaVersion != reusableCodeSchemaVersion {
		return fmt.Errorf("reusable code schema_version must be %d", reusableCodeSchemaVersion)
	}
	if !reusableCodeTypePattern.MatchString(string(manifest.Type)) {
		return fmt.Errorf("reusable code type must match user.<name>")
	}
	if strings.TrimSpace(manifest.Name) == "" || len(manifest.Name) > 128 {
		return fmt.Errorf("reusable code name is required and must be at most 128 characters")
	}
	if strings.TrimSpace(manifest.Version) == "" || len(manifest.Version) > 64 {
		return fmt.Errorf("reusable code version is required and must be at most 64 characters")
	}
	runtimeName := strings.ToLower(strings.TrimSpace(manifest.Runtime))
	if runtimeName != "js" && runtimeName != "python" {
		return fmt.Errorf("reusable code runtime must be js or python")
	}
	if strings.TrimSpace(manifest.Code) == "" || len(manifest.Code) > maxPythonCodeBytes {
		return fmt.Errorf("reusable code is required and must not exceed %d bytes", maxPythonCodeBytes)
	}
	seen := map[string]bool{}
	for _, param := range manifest.Inputs {
		name := strings.TrimSpace(param.Name)
		if name == "" || seen[name] {
			return fmt.Errorf("reusable code input names must be non-empty and unique")
		}
		seen[name] = true
	}
	return nil
}

func LoadReusableCodeManifest(path string) (ReusableCodeManifest, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return ReusableCodeManifest{}, err
	}
	if len(data) > 1<<20 {
		return ReusableCodeManifest{}, fmt.Errorf("reusable code manifest exceeds 1 MiB")
	}
	var manifest ReusableCodeManifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		return ReusableCodeManifest{}, err
	}
	if err := validateReusableCodeManifest(manifest); err != nil {
		return ReusableCodeManifest{}, err
	}
	return manifest, nil
}

func SaveReusableCodeManifest(dir string, manifest ReusableCodeManifest) (string, error) {
	if err := validateReusableCodeManifest(manifest); err != nil {
		return "", err
	}
	if strings.TrimSpace(dir) == "" {
		dir = DefaultReusableCodeDir()
	}
	if err := os.MkdirAll(dir, 0700); err != nil {
		return "", err
	}
	name := strings.ReplaceAll(string(manifest.Type), ".", "_") + ".goflow-code.json"
	path := filepath.Join(dir, name)
	encoded, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return "", err
	}
	if err := os.WriteFile(path, append(encoded, '\n'), 0600); err != nil {
		return "", err
	}
	return path, nil
}

func DiscoverReusableCodeExecutors(dir string) ([]NodeExecutor, []error) {
	entries, err := filepath.Glob(filepath.Join(dir, "*.goflow-code.json"))
	if err != nil {
		return nil, []error{err}
	}
	sort.Strings(entries)
	executors := make([]NodeExecutor, 0, len(entries))
	errs := make([]error, 0)
	for _, path := range entries {
		manifest, err := LoadReusableCodeManifest(path)
		if err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", filepath.Base(path), err))
			continue
		}
		executors = append(executors, &ReusableCodeExecutor{manifest: manifest})
	}
	return executors, errs
}

func (e *ReusableCodeExecutor) Validate(node *Node) error {
	for _, input := range e.manifest.Inputs {
		if !input.Required {
			continue
		}
		value, ok := node.Params[input.Name]
		if !ok || isManifestBlank(value) {
			return fmt.Errorf("%s is required", input.Label)
		}
	}
	return nil
}

func (e *ReusableCodeExecutor) Execute(ctx *ExecutionContext, node *Node) (interface{}, error) {
	var input interface{}
	if len(e.manifest.Inputs) == 1 && e.manifest.Inputs[0].Name == "input" {
		input = node.Params["input"]
	} else {
		mapped := make(map[string]interface{}, len(e.manifest.Inputs))
		for _, definition := range e.manifest.Inputs {
			mapped[definition.Name] = node.Params[definition.Name]
		}
		input = mapped
	}
	runtimeName := strings.ToLower(strings.TrimSpace(e.manifest.Runtime))
	if runtimeName == "python" {
		timeout := e.manifest.Timeout
		if timeout <= 0 {
			timeout = defaultPythonTimeoutSeconds
		}
		return NewPythonCodeExecutor().Execute(ctx, &Node{Params: map[string]interface{}{
			"environment": node.Params["python_environment"],
			"input":       input,
			"code":        e.manifest.Code,
			"timeout":     timeout,
		}})
	}
	return NewJSCodeRunnerExecutor().Execute(ctx, &Node{Params: map[string]interface{}{
		"input":   input,
		"code":    e.manifest.Code,
		"timeout": defaultJSTimeoutSeconds,
	}})
}

func (e *ReusableCodeExecutor) GetDefinition() NodeDefinition {
	params := append([]ParamDefinition(nil), e.manifest.Inputs...)
	if strings.EqualFold(e.manifest.Runtime, "python") {
		params = append(params, ParamDefinition{Name: "python_environment", Label: "Python Environment", Type: "text", Default: "default", Advanced: true, Description: "Optional Python runtime profile override"})
	}
	return NodeDefinition{
		Type: e.manifest.Type, Name: e.manifest.Name, Description: e.manifest.Description, Icon: "Boxes", Category: "CUSTOM", Retryable: false,
		Version: e.manifest.Version, Capabilities: []string{"trusted-code:" + strings.ToLower(e.manifest.Runtime)}, Outputs: append([]PluginOutputDefinition(nil), e.manifest.Outputs...), Params: params,
	}
}
