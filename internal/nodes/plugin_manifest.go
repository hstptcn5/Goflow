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

const pluginManifestSchemaVersion = 1

var customNodeTypePattern = regexp.MustCompile(`^custom\.[A-Za-z][A-Za-z0-9_.-]{0,95}$`)

type PluginOutputDefinition struct {
	Name        string `json:"name"`
	Type        string `json:"type,omitempty"`
	Description string `json:"description,omitempty"`
}

type PluginNodeManifest struct {
	SchemaVersion int                      `json:"schema_version"`
	Type          NodeType                 `json:"type"`
	Name          string                   `json:"name"`
	Version       string                   `json:"version"`
	Description   string                   `json:"description,omitempty"`
	Icon          string                   `json:"icon,omitempty"`
	Category      string                   `json:"category,omitempty"`
	Executable    string                   `json:"executable"`
	Retryable     bool                     `json:"retryable,omitempty"`
	Capabilities  []string                 `json:"capabilities,omitempty"`
	Params        []ParamDefinition        `json:"params,omitempty"`
	Outputs       []PluginOutputDefinition `json:"outputs,omitempty"`
}

type ManifestPluginExecutor struct {
	manifest PluginNodeManifest
	runner   *GoflowPluginExecutor
}

func LoadPluginNodeManifest(path string) (PluginNodeManifest, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return PluginNodeManifest{}, err
	}
	if len(data) > 1<<20 {
		return PluginNodeManifest{}, fmt.Errorf("plugin manifest exceeds 1 MiB")
	}
	var manifest PluginNodeManifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		return PluginNodeManifest{}, fmt.Errorf("invalid plugin manifest JSON: %w", err)
	}
	if err := validatePluginNodeManifest(manifest); err != nil {
		return PluginNodeManifest{}, err
	}
	return manifest, nil
}

func validatePluginNodeManifest(manifest PluginNodeManifest) error {
	if manifest.SchemaVersion != pluginManifestSchemaVersion {
		return fmt.Errorf("plugin manifest schema_version must be %d", pluginManifestSchemaVersion)
	}
	if !customNodeTypePattern.MatchString(string(manifest.Type)) {
		return fmt.Errorf("plugin node type must match custom.<name>")
	}
	if strings.TrimSpace(manifest.Name) == "" || len(manifest.Name) > 128 {
		return fmt.Errorf("plugin manifest name is required and must be at most 128 characters")
	}
	if strings.TrimSpace(manifest.Version) == "" || len(manifest.Version) > 64 {
		return fmt.Errorf("plugin manifest version is required and must be at most 64 characters")
	}
	if strings.TrimSpace(manifest.Executable) == "" || manifest.Executable != filepath.Base(manifest.Executable) || filepath.IsAbs(manifest.Executable) || strings.ContainsAny(manifest.Executable, `/\\`) {
		return fmt.Errorf("plugin manifest executable must be a file name in the plugins directory")
	}
	seen := map[string]bool{}
	for _, param := range manifest.Params {
		name := strings.TrimSpace(param.Name)
		if name == "" || name == "plugin_name" {
			return fmt.Errorf("plugin manifest contains an invalid parameter name %q", name)
		}
		if seen[name] {
			return fmt.Errorf("plugin manifest parameter %q is duplicated", name)
		}
		seen[name] = true
	}
	return nil
}

func DiscoverPluginNodeExecutors(pluginsDir string) ([]NodeExecutor, []error) {
	entries, err := filepath.Glob(filepath.Join(pluginsDir, "*.goflow-node.json"))
	if err != nil {
		return nil, []error{err}
	}
	sort.Strings(entries)
	executors := make([]NodeExecutor, 0, len(entries))
	errs := make([]error, 0)
	for _, path := range entries {
		manifest, err := LoadPluginNodeManifest(path)
		if err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", filepath.Base(path), err))
			continue
		}
		executors = append(executors, &ManifestPluginExecutor{manifest: manifest, runner: NewGoflowPluginExecutor()})
	}
	return executors, errs
}

func (e *ManifestPluginExecutor) Execute(ctx *ExecutionContext, node *Node) (interface{}, error) {
	params := make(map[string]interface{}, len(node.Params)+1)
	for key, value := range node.Params {
		params[key] = value
	}
	params["plugin_name"] = e.manifest.Executable
	clone := *node
	clone.Params = params
	return e.runner.Execute(ctx, &clone)
}

func (e *ManifestPluginExecutor) Validate(node *Node) error {
	for _, param := range e.manifest.Params {
		if !param.Required {
			continue
		}
		value, ok := node.Params[param.Name]
		if !ok || isManifestBlank(value) {
			return fmt.Errorf("%s is required", param.Label)
		}
	}
	return e.runner.Validate(&Node{Params: map[string]interface{}{"plugin_name": e.manifest.Executable}})
}

func isManifestBlank(value interface{}) bool {
	if value == nil {
		return true
	}
	if text, ok := value.(string); ok {
		return strings.TrimSpace(text) == ""
	}
	return false
}

func (e *ManifestPluginExecutor) GetDefinition() NodeDefinition {
	category := strings.TrimSpace(e.manifest.Category)
	if category == "" {
		category = "CUSTOM"
	}
	icon := strings.TrimSpace(e.manifest.Icon)
	if icon == "" {
		icon = "Puzzle"
	}
	description := strings.TrimSpace(e.manifest.Description)
	if description == "" {
		description = fmt.Sprintf("Custom plugin node %s %s", e.manifest.Name, e.manifest.Version)
	}
	return NodeDefinition{
		Type: e.manifest.Type, Name: e.manifest.Name, Description: description, Icon: icon, Category: category, Retryable: e.manifest.Retryable,
		Version: e.manifest.Version, Capabilities: append([]string(nil), e.manifest.Capabilities...), Outputs: append([]PluginOutputDefinition(nil), e.manifest.Outputs...),
		Params: append([]ParamDefinition(nil), e.manifest.Params...),
	}
}
