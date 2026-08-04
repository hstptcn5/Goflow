package pack

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"goflow/internal/workflow"
)

const (
	ManifestFile        = "pack.json"
	MaxManifestBytes    = 1 << 20
	MaxWorkflowBytes    = 10 << 20
	SupportedSchema     = 1
	DefaultWorkflowPath = "workflows/main.json"
)

var (
	idPattern     = regexp.MustCompile(`^[a-z0-9][a-z0-9.-]*$`)
	semverPattern = regexp.MustCompile(`^(0|[1-9]\d*)\.(0|[1-9]\d*)\.(0|[1-9]\d*)(?:-[0-9A-Za-z-]+(?:\.[0-9A-Za-z-]+)*)?(?:\+[0-9A-Za-z-]+(?:\.[0-9A-Za-z-]+)*)?$`)
)

type Manifest struct {
	SchemaVersion       int      `json:"schema_version"`
	ID                  string   `json:"id"`
	Name                string   `json:"name"`
	Version             string   `json:"version"`
	Description         string   `json:"description"`
	EntryWorkflow       string   `json:"entry_workflow"`
	RequiredCredentials []string `json:"required_credentials"`
	SupportedPlatforms  []string `json:"supported_platforms"`
	Plugins             []string `json:"plugins,omitempty"`
	Assets              []string `json:"assets,omitempty"`
}

type Pack struct {
	Root              string
	Manifest          Manifest
	ManifestPath      string
	EntryWorkflowPath string
}

func Load(dir string) (*Pack, error) {
	root, err := filepath.Abs(dir)
	if err != nil {
		return nil, fmt.Errorf("path: resolve pack directory: %w", err)
	}
	info, err := os.Stat(root)
	if err != nil {
		return nil, fmt.Errorf("path: pack directory is not accessible: %w", err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("path: pack path must be a directory")
	}
	root, err = filepath.EvalSymlinks(root)
	if err != nil {
		return nil, fmt.Errorf("path: resolve pack directory symlinks: %w", err)
	}

	manifestPath := filepath.Join(root, ManifestFile)
	manifest, err := readManifest(manifestPath)
	if err != nil {
		return nil, err
	}
	if err := validateManifest(manifest); err != nil {
		return nil, err
	}

	entryPath, err := resolveInside(root, manifest.EntryWorkflow, true)
	if err != nil {
		return nil, fmt.Errorf("path: entry_workflow: %w", err)
	}
	entryInfo, err := os.Stat(entryPath)
	if err != nil {
		return nil, fmt.Errorf("path: entry_workflow does not exist: %w", err)
	}
	if !entryInfo.Mode().IsRegular() {
		return nil, fmt.Errorf("path: entry_workflow must be a regular file")
	}

	for _, pluginPath := range manifest.Plugins {
		if _, err := resolveInside(root, pluginPath, false); err != nil {
			return nil, fmt.Errorf("path: plugins entry %q: %w", pluginPath, err)
		}
	}
	for _, assetPath := range manifest.Assets {
		if _, err := resolveInside(root, assetPath, false); err != nil {
			return nil, fmt.Errorf("path: assets entry %q: %w", assetPath, err)
		}
	}

	workflowDef, err := workflow.ReadFileLimit(entryPath, MaxWorkflowBytes)
	if err != nil {
		return nil, fmt.Errorf("workflow: %w", err)
	}
	if err := workflow.ValidateDefinition(workflowDef); err != nil {
		return nil, fmt.Errorf("workflow: %w", err)
	}

	return &Pack{
		Root:              root,
		Manifest:          manifest,
		ManifestPath:      manifestPath,
		EntryWorkflowPath: entryPath,
	}, nil
}

func readManifest(path string) (Manifest, error) {
	file, err := os.Open(path)
	if err != nil {
		return Manifest{}, fmt.Errorf("manifest: open pack.json: %w", err)
	}
	defer file.Close()

	data, err := io.ReadAll(io.LimitReader(file, MaxManifestBytes+1))
	if err != nil {
		return Manifest{}, fmt.Errorf("manifest: read pack.json: %w", err)
	}
	if len(data) > MaxManifestBytes {
		return Manifest{}, fmt.Errorf("manifest: pack.json exceeds %d byte limit", MaxManifestBytes)
	}
	if err := rejectSecretFields(data); err != nil {
		return Manifest{}, err
	}

	var manifest Manifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		return Manifest{}, fmt.Errorf("manifest: pack.json must be JSON: %w", err)
	}
	return manifest, nil
}

func validateManifest(manifest Manifest) error {
	if manifest.SchemaVersion != SupportedSchema {
		return fmt.Errorf("manifest: schema_version must be %d", SupportedSchema)
	}
	if !idPattern.MatchString(manifest.ID) {
		return fmt.Errorf("manifest: id must contain only lowercase letters, numbers, dots, or hyphens")
	}
	if strings.Contains(manifest.ID, "..") {
		return fmt.Errorf("manifest: id must not contain consecutive dots")
	}
	if strings.TrimSpace(manifest.Name) == "" {
		return fmt.Errorf("manifest: name is required")
	}
	if !semverPattern.MatchString(manifest.Version) {
		return fmt.Errorf("manifest: version must be valid SemVer")
	}
	if strings.TrimSpace(manifest.EntryWorkflow) == "" {
		return fmt.Errorf("manifest: entry_workflow is required")
	}
	for _, credential := range manifest.RequiredCredentials {
		if strings.TrimSpace(credential) == "" {
			return fmt.Errorf("manifest: required_credentials entries must be non-empty logical names")
		}
		if looksLikeSecretValue(credential) {
			return fmt.Errorf("manifest: required_credentials must not contain credential values")
		}
	}
	return nil
}

func resolveInside(root, relPath string, requireExisting bool) (string, error) {
	if strings.TrimSpace(relPath) == "" {
		return "", fmt.Errorf("path is required")
	}
	if filepath.IsAbs(relPath) {
		return "", fmt.Errorf("absolute paths are not allowed")
	}
	clean := filepath.Clean(relPath)
	if clean == "." || clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("path traversal is not allowed")
	}
	candidate := filepath.Join(root, clean)
	absCandidate, err := filepath.Abs(candidate)
	if err != nil {
		return "", err
	}
	rootEval, err := filepath.EvalSymlinks(root)
	if err != nil {
		return "", err
	}
	checkedPath := absCandidate
	if _, statErr := os.Lstat(absCandidate); statErr == nil || requireExisting {
		checkedPath, err = filepath.EvalSymlinks(absCandidate)
		if err != nil {
			return "", err
		}
	} else if !os.IsNotExist(statErr) {
		return "", statErr
	}
	if !isWithin(rootEval, checkedPath) {
		return "", fmt.Errorf("path resolves outside the pack directory")
	}
	return absCandidate, nil
}

func isWithin(root, path string) bool {
	rel, err := filepath.Rel(root, path)
	return err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}

func rejectSecretFields(data []byte) error {
	var decoded interface{}
	if err := json.Unmarshal(data, &decoded); err != nil {
		return nil
	}
	return walkManifest(decoded, "")
}

func walkManifest(value interface{}, key string) error {
	switch typed := value.(type) {
	case map[string]interface{}:
		for childKey, childValue := range typed {
			if isSecretField(childKey) && hasSecretValue(childValue) {
				return fmt.Errorf("manifest: field %q must not contain credential secrets", childKey)
			}
			if err := walkManifest(childValue, childKey); err != nil {
				return err
			}
		}
	case []interface{}:
		for _, item := range typed {
			if err := walkManifest(item, key); err != nil {
				return err
			}
		}
	}
	return nil
}

func isSecretField(key string) bool {
	lower := strings.ToLower(key)
	if lower == "required_credentials" {
		return false
	}
	return lower == "credential" ||
		lower == "credentials" ||
		lower == "secret" ||
		lower == "secrets" ||
		lower == "password" ||
		lower == "token" ||
		lower == "api_key" ||
		strings.HasSuffix(lower, "_secret") ||
		strings.HasSuffix(lower, "_token") ||
		strings.HasSuffix(lower, "_password")
}

func hasSecretValue(value interface{}) bool {
	switch typed := value.(type) {
	case string:
		return strings.TrimSpace(typed) != ""
	case []interface{}:
		return len(typed) > 0
	case map[string]interface{}:
		return len(typed) > 0
	default:
		return value != nil
	}
}

func looksLikeSecretValue(value string) bool {
	lower := strings.ToLower(strings.TrimSpace(value))
	return strings.Contains(lower, "=") ||
		strings.Contains(lower, "-----begin") ||
		strings.Contains(lower, "sk-") ||
		strings.Contains(lower, "secret:") ||
		strings.Contains(lower, "password:")
}
