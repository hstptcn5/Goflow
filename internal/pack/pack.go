package pack

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
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

	manifestPath, err := resolveManifestPath(root)
	if err != nil {
		return nil, err
	}
	manifest, err := readManifest(manifestPath)
	if err != nil {
		return nil, err
	}
	if err := validateManifest(manifest); err != nil {
		return nil, err
	}

	entryPath, err := resolveExistingRegularInside(root, manifest.EntryWorkflow)
	if err != nil {
		return nil, fmt.Errorf("path: entry_workflow: %w", err)
	}

	for _, pluginPath := range manifest.Plugins {
		if _, err := resolveExistingRegularInside(root, pluginPath); err != nil {
			return nil, fmt.Errorf("path: plugins entry %q: %w", pluginPath, err)
		}
	}
	for _, assetPath := range manifest.Assets {
		if _, err := resolveExistingRegularInside(root, assetPath); err != nil {
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

func resolveManifestPath(root string) (string, error) {
	path := filepath.Join(root, ManifestFile)
	info, err := os.Lstat(path)
	if err != nil {
		return "", fmt.Errorf("manifest: pack.json is required: %w", err)
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return "", fmt.Errorf("manifest: pack.json must not be a symlink")
	}
	if !info.Mode().IsRegular() {
		return "", fmt.Errorf("manifest: pack.json must be a regular file")
	}
	return path, nil
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

	if err := validateRequiredFields(data); err != nil {
		return Manifest{}, err
	}

	var manifest Manifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		return Manifest{}, fmt.Errorf("manifest: pack.json must be JSON: %w", err)
	}
	return manifest, nil
}

func validateRequiredFields(data []byte) error {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil
	}
	required := []string{"schema_version", "id", "name", "version", "entry_workflow", "required_credentials", "supported_platforms"}
	for _, field := range required {
		value, ok := raw[field]
		if !ok {
			return fmt.Errorf("manifest: %s is required", field)
		}
		if isJSONNull(value) {
			return fmt.Errorf("manifest: %s must not be null", field)
		}
	}
	if err := requireStringArray(raw["required_credentials"], "required_credentials", false); err != nil {
		return err
	}
	if err := requireStringArray(raw["supported_platforms"], "supported_platforms", true); err != nil {
		return err
	}
	if value, ok := raw["plugins"]; ok {
		if isJSONNull(value) {
			return fmt.Errorf("manifest: plugins must not be null")
		}
		if err := requireStringArray(value, "plugins", false); err != nil {
			return err
		}
	}
	if value, ok := raw["assets"]; ok {
		if isJSONNull(value) {
			return fmt.Errorf("manifest: assets must not be null")
		}
		if err := requireStringArray(value, "assets", false); err != nil {
			return err
		}
	}
	return nil
}

func isJSONNull(raw json.RawMessage) bool {
	return strings.TrimSpace(string(raw)) == "null"
}

func requireStringArray(raw json.RawMessage, field string, requireNonEmpty bool) error {
	var values []interface{}
	if err := json.Unmarshal(raw, &values); err != nil {
		return fmt.Errorf("manifest: %s must be a JSON array", field)
	}
	if requireNonEmpty && len(values) == 0 {
		return fmt.Errorf("manifest: %s must include at least one platform", field)
	}
	for index, value := range values {
		text, ok := value.(string)
		if !ok {
			return fmt.Errorf("manifest: %s[%d] must be a string", field, index)
		}
		if strings.TrimSpace(text) == "" {
			return fmt.Errorf("manifest: %s[%d] must be a non-empty string", field, index)
		}
	}
	return nil
}

func validateManifest(manifest Manifest) error {
	if manifest.SchemaVersion != SupportedSchema {
		return fmt.Errorf("manifest: schema_version must be %d", SupportedSchema)
	}
	if !isValidID(manifest.ID) {
		return fmt.Errorf("manifest: id must use lowercase alphanumeric segments separated by dots or hyphens")
	}
	if strings.TrimSpace(manifest.Name) == "" {
		return fmt.Errorf("manifest: name is required")
	}
	if !isValidSemVer(manifest.Version) {
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

func isValidID(id string) bool {
	if id == "" {
		return false
	}
	if strings.HasPrefix(id, ".") || strings.HasPrefix(id, "-") || strings.HasSuffix(id, ".") || strings.HasSuffix(id, "-") {
		return false
	}
	if strings.Contains(id, "..") || strings.Contains(id, ".-") || strings.Contains(id, "-.") {
		return false
	}
	if strings.Contains(id, "--") {
		return false
	}
	for _, r := range id {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '.' || r == '-' {
			continue
		}
		return false
	}
	return true
}

func isValidSemVer(version string) bool {
	coreAndBuild := strings.Split(version, "+")
	if len(coreAndBuild) > 2 || coreAndBuild[0] == "" {
		return false
	}
	if len(coreAndBuild) == 2 && !validDotIdentifiers(coreAndBuild[1], false) {
		return false
	}
	coreText := coreAndBuild[0]
	preText := ""
	if core, pre, ok := strings.Cut(coreAndBuild[0], "-"); ok {
		coreText = core
		preText = pre
	}
	core := strings.Split(coreText, ".")
	if len(core) != 3 {
		return false
	}
	for _, part := range core {
		if !validNumericIdentifier(part, false) {
			return false
		}
	}
	if preText != "" && !validDotIdentifiers(preText, true) {
		return false
	}
	if strings.HasSuffix(coreAndBuild[0], "-") {
		return false
	}
	return true
}

func validDotIdentifiers(value string, rejectNumericLeadingZero bool) bool {
	if value == "" {
		return false
	}
	parts := strings.Split(value, ".")
	for _, part := range parts {
		if part == "" {
			return false
		}
		hasNonDigit := false
		for _, r := range part {
			if (r >= '0' && r <= '9') || (r >= 'A' && r <= 'Z') || (r >= 'a' && r <= 'z') || r == '-' {
				if r < '0' || r > '9' {
					hasNonDigit = true
				}
				continue
			}
			return false
		}
		if rejectNumericLeadingZero && !hasNonDigit && !validNumericIdentifier(part, false) {
			return false
		}
	}
	return true
}

func validNumericIdentifier(value string, allowLeadingZero bool) bool {
	if value == "" {
		return false
	}
	if len(value) > 1 && strings.HasPrefix(value, "0") && !allowLeadingZero {
		return false
	}
	for _, r := range value {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

func resolveExistingRegularInside(root, slashPath string) (string, error) {
	osRelPath, err := portablePathToOS(slashPath)
	if err != nil {
		return "", err
	}
	candidate := filepath.Join(root, osRelPath)
	resolved, err := filepath.EvalSymlinks(candidate)
	if err != nil {
		if os.IsNotExist(err) {
			return "", fmt.Errorf("path does not exist")
		}
		return "", err
	}
	rootEval, err := filepath.EvalSymlinks(root)
	if err != nil {
		return "", err
	}
	if !isWithin(rootEval, resolved) {
		return "", fmt.Errorf("path resolves outside the pack directory")
	}
	info, err := os.Stat(resolved)
	if err != nil {
		return "", err
	}
	if !info.Mode().IsRegular() {
		return "", fmt.Errorf("path must be a regular file")
	}
	return candidate, nil
}

func portablePathToOS(slashPath string) (string, error) {
	if strings.TrimSpace(slashPath) == "" {
		return "", fmt.Errorf("path is required")
	}
	if strings.Contains(slashPath, `\`) {
		return "", fmt.Errorf("backslash paths are not allowed")
	}
	if strings.HasPrefix(slashPath, "/") {
		return "", fmt.Errorf("absolute paths are not allowed")
	}
	if len(slashPath) >= 2 && slashPath[1] == ':' && ((slashPath[0] >= 'A' && slashPath[0] <= 'Z') || (slashPath[0] >= 'a' && slashPath[0] <= 'z')) {
		return "", fmt.Errorf("Windows drive paths are not allowed")
	}
	parts := strings.Split(slashPath, "/")
	for _, part := range parts {
		if part == "" {
			return "", fmt.Errorf("empty path segments are not allowed")
		}
		if part == "." {
			return "", fmt.Errorf("dot path segments are not allowed")
		}
		if part == ".." {
			return "", fmt.Errorf("path traversal is not allowed")
		}
		if strings.Contains(part, ":") {
			return "", fmt.Errorf("path segments must not contain colon")
		}
		if strings.HasSuffix(part, ".") || strings.HasSuffix(part, " ") {
			return "", fmt.Errorf("path segments must not end with dot or space")
		}
		if isWindowsReservedName(part) {
			return "", fmt.Errorf("path segment %q is reserved on Windows", part)
		}
	}
	return filepath.FromSlash(slashPath), nil
}

func isWindowsReservedName(segment string) bool {
	name := segment
	if before, _, ok := strings.Cut(name, "."); ok {
		name = before
	}
	name = strings.ToUpper(name)
	switch name {
	case "CON", "PRN", "AUX", "NUL":
		return true
	}
	if len(name) == 4 {
		prefix := name[:3]
		suffix := name[3]
		if (prefix == "COM" || prefix == "LPT") && suffix >= '1' && suffix <= '9' {
			return true
		}
	}
	return false
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
