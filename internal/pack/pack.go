package pack

import (
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"goflow/internal/jsoncontract"
	"goflow/internal/nodes"
	"goflow/internal/workflow"
)

const (
	ManifestFile        = "pack.json"
	MaxManifestBytes    = 1 << 20
	MaxWorkflowBytes    = 10 << 20
	SupportedSchema     = 1
	DefaultWorkflowPath = "workflows/main.json"

	MaxSetupMetadataBytes      = 64 << 10
	MaxConfigFields            = 32
	MaxCredentialRequirements  = 32
	MaxBindings                = 128
	MaxSetupKeyLength          = 64
	MaxSetupLabelLength        = 120
	MaxSetupDescriptionLength  = 1000
	MaxSetupOptionLength       = 120
	MaxSetupScalarStringLength = 1000
	MaxSetupIntegerAbsValue    = 1_000_000_000
	MaxRequiredCapabilities    = 32
	MaxCapabilityLength        = 96
	MaxOfflineFixtureBytes     = 64 << 10
	MaxRunFields               = 64
)

const (
	CapabilityPackV1          = "goflow.pack.v1"
	CapabilitySetupBindingsV1 = "goflow.setup.bindings.v1"
	CapabilityConnectionV1    = "goflow.setup.connection-tests.v1"
	CapabilityDailyScheduleV1 = "goflow.schedule.daily.v1"
	CapabilityHostMigrationV1 = "goflow.migration.host-managed.v1"
	CapabilityHTTPAdapterV1   = "goflow.adapter.normalized-http.v1"
	CapabilityAppUIV1         = "goflow.app.ui.v1"
)

type Manifest struct {
	SchemaVersion          int                     `json:"schema_version"`
	ID                     string                  `json:"id"`
	Name                   string                  `json:"name"`
	Version                string                  `json:"version"`
	Description            string                  `json:"description"`
	EntryWorkflow          string                  `json:"entry_workflow"`
	RequiredCredentials    []string                `json:"required_credentials"`
	SupportedPlatforms     []string                `json:"supported_platforms"`
	Plugins                []string                `json:"plugins,omitempty"`
	Assets                 []string                `json:"assets,omitempty"`
	ConfigSchema           []ConfigField           `json:"config_schema,omitempty"`
	CredentialRequirements []CredentialRequirement `json:"credential_requirements,omitempty"`
	Bindings               []Binding               `json:"bindings,omitempty"`
	RequiredCapabilities   []string                `json:"required_capabilities,omitempty"`
	ExecutionTier          string                  `json:"execution_tier,omitempty"`
	OfflineTestFixture     string                  `json:"offline_test_fixture,omitempty"`
	RunUI                  *RunUI                  `json:"run_ui,omitempty"`
	Branding               *Branding               `json:"branding,omitempty"`
}

// RunUI describes the focused input and output surface shown by a generated app.
// It is intentionally data-only so a Pack never ships executable UI code.
type RunUI struct {
	InputMode    string     `json:"input_mode,omitempty"`
	InputFields  []RunField `json:"input_fields,omitempty"`
	OutputNodeID string     `json:"output_node_id,omitempty"`
	OutputMode   string     `json:"output_mode,omitempty"`
	SubmitLabel  string     `json:"submit_label,omitempty"`
}

type RunField struct {
	Key         string        `json:"key"`
	Label       string        `json:"label"`
	Description string        `json:"description,omitempty"`
	Type        string        `json:"type"`
	Required    bool          `json:"required,omitempty"`
	Default     interface{}   `json:"default,omitempty"`
	Options     []interface{} `json:"options,omitempty"`
	Placeholder string        `json:"placeholder,omitempty"`
}

type Branding struct {
	Icon        string `json:"icon,omitempty"`
	AccentColor string `json:"accent_color,omitempty"`
}

type Pack struct {
	Root               string
	Manifest           Manifest
	ManifestPath       string
	EntryWorkflowPath  string
	OfflineFixturePath string
}

type ConfigField struct {
	Key         string        `json:"key"`
	Label       string        `json:"label"`
	Description string        `json:"description,omitempty"`
	Type        string        `json:"type"`
	Required    bool          `json:"required"`
	TestKind    string        `json:"test_kind,omitempty"`
	Default     interface{}   `json:"default,omitempty"`
	Options     []interface{} `json:"options,omitempty"`
	Min         *int          `json:"min,omitempty"`
	Max         *int          `json:"max,omitempty"`
	MinLength   *int          `json:"min_length,omitempty"`
	MaxLength   *int          `json:"max_length,omitempty"`
	DisplayOnly bool          `json:"display_only,omitempty"`
}

type CredentialRequirement struct {
	Key         string `json:"key"`
	Label       string `json:"label"`
	Description string `json:"description,omitempty"`
	Type        string `json:"type"`
	Kind        string `json:"kind,omitempty"`
	Provider    string `json:"provider,omitempty"`
	Required    bool   `json:"required"`
	TestKind    string `json:"test_kind,omitempty"`
	DisplayOnly bool   `json:"display_only,omitempty"`
}

type Binding struct {
	Source string        `json:"source"`
	Target BindingTarget `json:"target"`
}

type BindingTarget struct {
	NodeID string `json:"node_id"`
	Param  string `json:"param"`
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
	var offlineFixturePath string
	if manifest.OfflineTestFixture != "" {
		offlineFixturePath, err = resolveOfflineFixture(root, manifest.OfflineTestFixture)
		if err != nil {
			return nil, fmt.Errorf("manifest: offline_test_fixture: %w", err)
		}
		if _, err := readOfflineTestFixture(offlineFixturePath, manifest); err != nil {
			return nil, err
		}
	}

	workflowDef, err := workflow.ReadFileLimit(entryPath, MaxWorkflowBytes)
	if err != nil {
		return nil, fmt.Errorf("workflow: %w", err)
	}
	if err := workflow.ValidateDefinition(workflowDef); err != nil {
		return nil, fmt.Errorf("workflow: %w", err)
	}
	if err := validateSetupMetadata(manifest, workflowDef.NodesJSON); err != nil {
		return nil, err
	}
	if err := validateRunUI(manifest, workflowDef.NodesJSON); err != nil {
		return nil, err
	}
	if err := rejectPackEmbeddedSecrets(workflowDef.NodesJSON); err != nil {
		return nil, err
	}
	if err := validatePackExecutionTier(manifest, workflowDef.NodesJSON); err != nil {
		return nil, err
	}

	return &Pack{
		Root:               root,
		Manifest:           manifest,
		ManifestPath:       manifestPath,
		EntryWorkflowPath:  entryPath,
		OfflineFixturePath: offlineFixturePath,
	}, nil
}

func resolveOfflineFixture(root, slashPath string) (string, error) {
	osPath, err := portablePathToOS(slashPath)
	if err != nil {
		return "", err
	}
	candidate := root
	for _, segment := range strings.Split(osPath, string(filepath.Separator)) {
		candidate = filepath.Join(candidate, segment)
		info, err := os.Lstat(candidate)
		if err != nil {
			if os.IsNotExist(err) {
				return "", fmt.Errorf("path does not exist")
			}
			return "", err
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return "", fmt.Errorf("fixture path must not contain symlinks")
		}
	}
	return resolveExistingRegularInside(root, slashPath)
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
	for _, field := range []string{"config_schema", "credential_requirements", "bindings"} {
		if value, ok := raw[field]; ok {
			if isJSONNull(value) {
				return fmt.Errorf("manifest: %s must not be null", field)
			}
			if !isJSONArrayRaw(value) {
				return fmt.Errorf("manifest: %s must be a JSON array", field)
			}
		}
	}
	if value, ok := raw["required_capabilities"]; ok {
		if isJSONNull(value) {
			return fmt.Errorf("manifest: required_capabilities must not be null")
		}
		if err := requireStringArray(value, "required_capabilities", false); err != nil {
			return err
		}
	}
	if value, ok := raw["offline_test_fixture"]; ok {
		var path string
		if isJSONNull(value) || json.Unmarshal(value, &path) != nil || strings.TrimSpace(path) == "" {
			return fmt.Errorf("manifest: offline_test_fixture must be a non-empty string")
		}
	}
	for _, field := range []string{"schedule", "schedules"} {
		if _, ok := raw[field]; ok {
			return fmt.Errorf("manifest: %s is not supported; appliance schedules are host-managed", field)
		}
	}
	for _, field := range []string{"migration", "migrations"} {
		if _, ok := raw[field]; ok {
			return fmt.Errorf("manifest: %s is not supported; setup migrations are host-managed", field)
		}
	}
	if setupMetadataSize(raw) > MaxSetupMetadataBytes {
		return fmt.Errorf("manifest: setup metadata exceeds %d byte limit", MaxSetupMetadataBytes)
	}
	return nil
}

func isJSONNull(raw json.RawMessage) bool {
	return strings.TrimSpace(string(raw)) == "null"
}

func isJSONArrayRaw(raw json.RawMessage) bool {
	var values []interface{}
	return json.Unmarshal(raw, &values) == nil
}

func setupMetadataSize(raw map[string]json.RawMessage) int {
	size := 0
	for _, field := range []string{"config_schema", "credential_requirements", "bindings"} {
		if value, ok := raw[field]; ok {
			size += len(value)
		}
	}
	return size
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

func validateSetupMetadata(manifest Manifest, nodesJSON string) error {
	if len(manifest.ConfigSchema) > MaxConfigFields {
		return fmt.Errorf("manifest: config_schema exceeds %d item limit", MaxConfigFields)
	}
	if len(manifest.CredentialRequirements) > MaxCredentialRequirements {
		return fmt.Errorf("manifest: credential_requirements exceeds %d item limit", MaxCredentialRequirements)
	}
	if len(manifest.Bindings) > MaxBindings {
		return fmt.Errorf("manifest: bindings exceeds %d item limit", MaxBindings)
	}

	configKeys := map[string]ConfigField{}
	for i, field := range manifest.ConfigSchema {
		if err := validateConfigField(i, field); err != nil {
			return err
		}
		if _, exists := configKeys[field.Key]; exists {
			return fmt.Errorf("manifest: config_schema[%d].key duplicates %q", i, field.Key)
		}
		configKeys[field.Key] = field
	}

	credentialKeys := map[string]CredentialRequirement{}
	for i, req := range manifest.CredentialRequirements {
		if err := validateCredentialRequirement(i, req); err != nil {
			return err
		}
		if _, exists := credentialKeys[req.Key]; exists {
			return fmt.Errorf("manifest: credential_requirements[%d].key duplicates %q", i, req.Key)
		}
		credentialKeys[req.Key] = req
	}

	nodeIndex, err := workflowNodeIndex(nodesJSON)
	if err != nil {
		return fmt.Errorf("manifest: bindings: %w", err)
	}
	if manifest.RequiredCapabilities != nil {
		declared := make(map[string]bool, len(manifest.RequiredCapabilities))
		for _, capability := range manifest.RequiredCapabilities {
			declared[capability] = true
		}
		for _, node := range nodeIndex {
			if node.Type == nodes.TypeNormalizedHTTPSource && !declared[CapabilityHTTPAdapterV1] {
				return fmt.Errorf("manifest: required_capabilities must include %q when normalizedHttpSource is used", CapabilityHTTPAdapterV1)
			}
		}
	}
	boundSources := map[string]bool{}
	testableSources := map[string]bool{}
	bindingKeys := map[string]bool{}
	targetKeys := map[string]bool{}
	for i, binding := range manifest.Bindings {
		sourceKind, sourceKey, err := parseBindingSource(binding.Source)
		if err != nil {
			return fmt.Errorf("manifest: bindings[%d].source: %w", i, err)
		}
		switch sourceKind {
		case "config":
			if _, ok := configKeys[sourceKey]; !ok {
				return fmt.Errorf("manifest: bindings[%d].source references unknown config key %q", i, sourceKey)
			}
		case "credential":
			if _, ok := credentialKeys[sourceKey]; !ok {
				return fmt.Errorf("manifest: bindings[%d].source references unknown credential key %q", i, sourceKey)
			}
		}
		param, err := bindingTargetParam(binding.Target, nodeIndex)
		if err != nil {
			return fmt.Errorf("manifest: bindings[%d].target: %w", i, err)
		}
		if sourceKind == "credential" && param.Type != "credential" {
			return fmt.Errorf("manifest: bindings[%d] credential source may bind only to credential parameters", i)
		}
		if sourceKind == "config" && (param.Type == "credential" || isSecretParam(param.Name, param.Label)) {
			return fmt.Errorf("manifest: bindings[%d] config source may not bind to secret-like parameter %q", i, param.Name)
		}
		pair := binding.Source + "->" + binding.Target.NodeID + "." + binding.Target.Param
		if bindingKeys[pair] {
			return fmt.Errorf("manifest: bindings[%d] duplicates binding %q", i, pair)
		}
		bindingKeys[pair] = true
		targetKey := binding.Target.NodeID + "." + binding.Target.Param
		if targetKeys[targetKey] {
			return fmt.Errorf("manifest: bindings[%d] duplicates target %q", i, targetKey)
		}
		targetKeys[targetKey] = true
		boundSources[binding.Source] = true
		if sourceKind == "config" && configKeys[sourceKey].TestKind == "http_json_contract" {
			targetNode := nodeIndex[binding.Target.NodeID]
			if targetNode.Type == nodes.TypeHTTPRequest && binding.Target.Param == "url" {
				if _, err := jsoncontract.Parse(targetNode.Params["response_contract"]); err != nil {
					return fmt.Errorf("manifest: bindings[%d] source test requires a valid HTTP response_contract: %w", i, err)
				}
				testableSources[binding.Source] = true
			}
		}
	}
	for _, field := range configKeys {
		if field.Required && !field.DisplayOnly && !boundSources["config."+field.Key] {
			return fmt.Errorf("manifest: config_schema key %q is required but has no binding or display_only marker", field.Key)
		}
		if field.TestKind != "" && !testableSources["config."+field.Key] {
			return fmt.Errorf("manifest: config_schema key %q declares %q but is not bound to an HTTP url with response_contract", field.Key, field.TestKind)
		}
	}
	for _, req := range credentialKeys {
		if req.Required && !req.DisplayOnly && !boundSources["credential."+req.Key] {
			return fmt.Errorf("manifest: credential_requirements key %q is required but has no binding or display_only marker", req.Key)
		}
	}
	return nil
}

func validateConfigField(index int, field ConfigField) error {
	prefix := fmt.Sprintf("manifest: config_schema[%d]", index)
	if err := validateSetupKey(field.Key, prefix+".key"); err != nil {
		return err
	}
	if isSecretText(field.Key) {
		return fmt.Errorf("%s.key must not imply secret material", prefix)
	}
	if err := validateBoundedText(field.Label, prefix+".label", true, MaxSetupLabelLength); err != nil {
		return err
	}
	if isSecretText(field.Label) {
		return fmt.Errorf("%s.label must not imply secret material", prefix)
	}
	if err := validateBoundedText(field.Description, prefix+".description", false, MaxSetupDescriptionLength); err != nil {
		return err
	}
	if isSecretText(field.Description) {
		return fmt.Errorf("%s.description must not imply secret material", prefix)
	}
	switch field.Type {
	case "string", "url", "integer", "boolean", "select":
	default:
		return fmt.Errorf("%s.type must be one of string, url, integer, boolean, or select", prefix)
	}
	if field.TestKind != "" {
		if field.TestKind != "http_json_contract" {
			return fmt.Errorf("%s.test_kind must be allowlisted", prefix)
		}
		if field.Type != "url" {
			return fmt.Errorf("%s.test_kind %q is compatible only with url fields", prefix, field.TestKind)
		}
	}
	if field.Type != "select" && len(field.Options) > 0 {
		return fmt.Errorf("%s.options is allowed only for select fields", prefix)
	}
	if field.Type == "select" {
		if len(field.Options) == 0 {
			return fmt.Errorf("%s.options is required for select fields", prefix)
		}
		seen := map[string]bool{}
		for optionIndex, option := range field.Options {
			key, err := validateScalarOption(option, fmt.Sprintf("%s.options[%d]", prefix, optionIndex))
			if err != nil {
				return err
			}
			if seen[key] {
				return fmt.Errorf("%s.options[%d] duplicates another option", prefix, optionIndex)
			}
			seen[key] = true
		}
	}
	if field.Default != nil {
		if err := validateConfigDefault(field, prefix+".default"); err != nil {
			return err
		}
	}
	if field.Min != nil && field.Max != nil && *field.Min > *field.Max {
		return fmt.Errorf("%s.min must be less than or equal to max", prefix)
	}
	if field.MinLength != nil && field.MaxLength != nil && *field.MinLength > *field.MaxLength {
		return fmt.Errorf("%s.min_length must be less than or equal to max_length", prefix)
	}
	return nil
}

func validateCredentialRequirement(index int, req CredentialRequirement) error {
	prefix := fmt.Sprintf("manifest: credential_requirements[%d]", index)
	if err := validateSetupKey(req.Key, prefix+".key"); err != nil {
		return err
	}
	if err := validateBoundedText(req.Label, prefix+".label", true, MaxSetupLabelLength); err != nil {
		return err
	}
	if err := validateBoundedText(req.Description, prefix+".description", false, MaxSetupDescriptionLength); err != nil {
		return err
	}
	if !isKnownCredentialType(req.Type) {
		return fmt.Errorf("%s.type must be a known credential type", prefix)
	}
	if req.Kind != "" {
		switch req.Kind {
		case "API_KEY", "BEARER_TOKEN", "BASIC_AUTH", "OAUTH2", "USERNAME_PASSWORD", "SERVICE_ACCOUNT", "CUSTOM":
		default:
			return fmt.Errorf("%s.kind must be a known credential kind", prefix)
		}
		if strings.TrimSpace(req.Provider) == "" {
			return fmt.Errorf("%s.provider is required when kind is declared", prefix)
		}
	}
	if req.Provider != "" {
		if req.Kind == "" {
			return fmt.Errorf("%s.kind is required when provider is declared", prefix)
		}
		if len(req.Provider) > 80 {
			return fmt.Errorf("%s.provider exceeds 80 character limit", prefix)
		}
		for _, r := range req.Provider {
			if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-' || r == '_' || r == '.' {
				continue
			}
			return fmt.Errorf("%s.provider must use lowercase letters, numbers, dot, dash, or underscore", prefix)
		}
	}
	if req.TestKind != "" && !isKnownConnectionTest(req.TestKind) {
		return fmt.Errorf("%s.test_kind must be allowlisted", prefix)
	}
	if req.TestKind != "" && !credentialTestCompatible(req.Type, req.TestKind) {
		return fmt.Errorf("%s.test_kind %q is not compatible with credential type %q", prefix, req.TestKind, req.Type)
	}
	return nil
}

func validateSetupKey(key, field string) error {
	if strings.TrimSpace(key) == "" {
		return fmt.Errorf("%s is required", field)
	}
	if len(key) > MaxSetupKeyLength {
		return fmt.Errorf("%s exceeds %d character limit", field, MaxSetupKeyLength)
	}
	for _, r := range key {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '_' {
			continue
		}
		return fmt.Errorf("%s must use lowercase letters, numbers, and underscores", field)
	}
	return nil
}

func validateBoundedText(value, field string, required bool, max int) error {
	if required && strings.TrimSpace(value) == "" {
		return fmt.Errorf("%s is required", field)
	}
	if len(value) > max {
		return fmt.Errorf("%s exceeds %d character limit", field, max)
	}
	return nil
}

func validateScalarOption(value interface{}, field string) (string, error) {
	switch typed := value.(type) {
	case string:
		if strings.TrimSpace(typed) == "" {
			return "", fmt.Errorf("%s must not be empty", field)
		}
		if len(typed) > MaxSetupOptionLength {
			return "", fmt.Errorf("%s exceeds %d character limit", field, MaxSetupOptionLength)
		}
		if looksLikeSecretValue(typed) {
			return "", fmt.Errorf("%s must not look like secret material", field)
		}
		return "string:" + typed, nil
	case bool:
		return fmt.Sprintf("bool:%t", typed), nil
	case float64:
		if !isWholeNumber(typed) {
			return "", fmt.Errorf("%s numeric options must be integers", field)
		}
		if math.Abs(typed) > MaxSetupIntegerAbsValue {
			return "", fmt.Errorf("%s exceeds integer limit", field)
		}
		return fmt.Sprintf("number:%0.f", typed), nil
	default:
		return "", fmt.Errorf("%s must be a bounded scalar value", field)
	}
}

func validateConfigDefault(field ConfigField, label string) error {
	if containsSecretValue(field.Default) {
		return fmt.Errorf("%s must not look like secret material", label)
	}
	switch field.Type {
	case "string", "url":
		value, ok := field.Default.(string)
		if !ok {
			return fmt.Errorf("%s must be a string", label)
		}
		if len(value) > MaxSetupScalarStringLength {
			return fmt.Errorf("%s exceeds %d character limit", label, MaxSetupScalarStringLength)
		}
		if field.Type == "url" {
			if err := validateSafeDefaultURL(value); err != nil {
				return fmt.Errorf("%s must be an absolute http or https URL: %w", label, err)
			}
		}
	case "integer":
		value, ok := field.Default.(float64)
		if !ok || !isWholeNumber(value) {
			return fmt.Errorf("%s must be an integer", label)
		}
		if math.Abs(value) > MaxSetupIntegerAbsValue {
			return fmt.Errorf("%s exceeds integer limit", label)
		}
	case "boolean":
		if _, ok := field.Default.(bool); !ok {
			return fmt.Errorf("%s must be a boolean", label)
		}
	case "select":
		defaultKey, err := validateScalarOption(field.Default, label)
		if err != nil {
			return err
		}
		for _, option := range field.Options {
			optionKey, _ := validateScalarOption(option, label)
			if optionKey == defaultKey {
				return nil
			}
		}
		return fmt.Errorf("%s must match one of the select options", label)
	}
	return nil
}

func isWholeNumber(value float64) bool {
	return !math.IsNaN(value) && !math.IsInf(value, 0) && math.Trunc(value) == value
}

func containsSecretValue(value interface{}) bool {
	switch typed := value.(type) {
	case string:
		return looksLikeSecretValue(typed)
	case []interface{}:
		for _, item := range typed {
			if containsSecretValue(item) {
				return true
			}
		}
	case map[string]interface{}:
		for key, item := range typed {
			if isSecretText(key) || containsSecretValue(item) {
				return true
			}
		}
	}
	return false
}

func isKnownCredentialType(value string) bool {
	switch value {
	case "API_KEY", "TELEGRAM_BOT", "BEARER_TOKEN", "BASIC_AUTH", "OPENAI_API_KEY", "DEEPSEEK_API_KEY", "GOOGLE_SERVICE_ACCOUNT", "DATABASE_URL", "SSH_KEY", "SMTP_ACCOUNT":
		return true
	default:
		return false
	}
}

func isKnownConnectionTest(value string) bool {
	switch value {
	case "telegram_get_me", "http_head", "smtp_noop", "database_ping":
		return true
	default:
		return false
	}
}

func credentialTestCompatible(credentialType, testKind string) bool {
	switch testKind {
	case "":
		return true
	case "telegram_get_me":
		return credentialType == "TELEGRAM_BOT"
	case "http_head":
		return credentialType == "API_KEY" || credentialType == "BEARER_TOKEN" || credentialType == "BASIC_AUTH"
	case "smtp_noop":
		return credentialType == "SMTP_ACCOUNT"
	case "database_ping":
		return credentialType == "DATABASE_URL"
	default:
		return false
	}
}

func validateSafeDefaultURL(value string) error {
	parsed, err := url.Parse(value)
	if err != nil {
		return err
	}
	if !parsed.IsAbs() || parsed.Host == "" {
		return fmt.Errorf("URL must include scheme and host")
	}
	switch strings.ToLower(parsed.Scheme) {
	case "http", "https":
		return nil
	default:
		return fmt.Errorf("unsupported scheme %q", parsed.Scheme)
	}
}

func workflowNodeIndex(nodesJSON string) (map[string]nodes.Node, error) {
	var list []nodes.Node
	if err := json.Unmarshal([]byte(nodesJSON), &list); err != nil {
		return nil, err
	}
	index := make(map[string]nodes.Node, len(list))
	for _, node := range list {
		index[node.ID] = node
	}
	return index, nil
}

func parseBindingSource(source string) (string, string, error) {
	if strings.HasPrefix(source, "config.") {
		key := strings.TrimPrefix(source, "config.")
		return "config", key, validateSetupKey(key, "source key")
	}
	if strings.HasPrefix(source, "credential.") {
		key := strings.TrimPrefix(source, "credential.")
		return "credential", key, validateSetupKey(key, "source key")
	}
	return "", "", fmt.Errorf("must start with config. or credential.")
}

func bindingTargetParam(target BindingTarget, nodeIndex map[string]nodes.Node) (nodes.ParamDefinition, error) {
	if strings.TrimSpace(target.NodeID) == "" {
		return nodes.ParamDefinition{}, fmt.Errorf("node_id is required")
	}
	if strings.TrimSpace(target.Param) == "" {
		return nodes.ParamDefinition{}, fmt.Errorf("param is required")
	}
	node, ok := nodeIndex[target.NodeID]
	if !ok {
		return nodes.ParamDefinition{}, fmt.Errorf("node_id %q does not exist", target.NodeID)
	}
	executor, ok := nodes.NewBuiltinRegistry().Get(node.Type)
	if !ok {
		return nodes.ParamDefinition{}, fmt.Errorf("node %q has unknown type %q", target.NodeID, node.Type)
	}
	for _, param := range executor.GetDefinition().Params {
		if param.Name == target.Param {
			return param, nil
		}
	}
	return nodes.ParamDefinition{}, fmt.Errorf("param %q is not defined by node %q", target.Param, target.NodeID)
}

func rejectPackEmbeddedSecrets(nodesJSON string) error {
	var nodeList []nodes.Node
	if err := json.Unmarshal([]byte(nodesJSON), &nodeList); err != nil {
		return err
	}
	registry := nodes.NewBuiltinRegistry()
	for _, node := range nodeList {
		executor, ok := registry.Get(node.Type)
		if !ok {
			continue
		}
		for _, param := range executor.GetDefinition().Params {
			if param.Type == "credential" {
				continue
			}
			if !isSecretParam(param.Name, param.Label) {
				continue
			}
			value, exists := node.Params[param.Name]
			if !exists || !hasSecretValue(value) {
				continue
			}
			return fmt.Errorf("workflow: node %q parameter %q must not contain literal secret values in a pack", node.ID, param.Name)
		}
	}
	return nil
}

func isSecretParam(name, label string) bool {
	return isSecretText(name) || isSecretText(label)
}

func isSecretText(value string) bool {
	lower := strings.ToLower(value)
	return strings.Contains(lower, "token") ||
		strings.Contains(lower, "password") ||
		strings.Contains(lower, "secret") ||
		strings.Contains(lower, "api key") ||
		strings.Contains(lower, "api_key") ||
		strings.Contains(lower, "private key") ||
		strings.Contains(lower, "authorization") ||
		strings.Contains(lower, "credential")
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
	if err := validatePlatforms(manifest.SupportedPlatforms); err != nil {
		return err
	}
	if err := validateCapabilities(manifest); err != nil {
		return err
	}
	return nil
}

var supportedCapabilities = map[string]struct{}{
	CapabilityPackV1:          {},
	CapabilitySetupBindingsV1: {},
	CapabilityConnectionV1:    {},
	CapabilityDailyScheduleV1: {},
	CapabilityHostMigrationV1: {},
	CapabilityHTTPAdapterV1:   {},
	CapabilityAppUIV1:         {},
}

var supportedPlatforms = map[string]struct{}{
	"windows-amd64": {},
	"linux-amd64":   {},
	"linux-arm64":   {},
	"darwin-amd64":  {},
	"darwin-arm64":  {},
}

func validatePlatforms(platforms []string) error {
	seen := map[string]bool{}
	for i, platform := range platforms {
		if _, ok := supportedPlatforms[platform]; !ok {
			return fmt.Errorf("manifest: supported_platforms[%d] %q is not supported", i, platform)
		}
		if seen[platform] {
			return fmt.Errorf("manifest: supported_platforms[%d] duplicates %q", i, platform)
		}
		seen[platform] = true
	}
	return nil
}

func validateCapabilities(manifest Manifest) error {
	if len(manifest.RequiredCapabilities) > MaxRequiredCapabilities {
		return fmt.Errorf("manifest: required_capabilities exceeds %d item limit", MaxRequiredCapabilities)
	}
	seen := map[string]bool{}
	for i, capability := range manifest.RequiredCapabilities {
		if len(capability) > MaxCapabilityLength || !isValidCapability(capability) {
			return fmt.Errorf("manifest: required_capabilities[%d] must be a bounded lowercase capability identifier", i)
		}
		if _, ok := supportedCapabilities[capability]; !ok {
			return fmt.Errorf("manifest: required_capabilities[%d] %q is not supported by this runtime", i, capability)
		}
		if seen[capability] {
			return fmt.Errorf("manifest: required_capabilities[%d] duplicates %q", i, capability)
		}
		seen[capability] = true
	}
	// A nil declaration is a legacy Pack Format v1 manifest. Preserve it as-is.
	if manifest.RequiredCapabilities == nil {
		if manifest.RunUI != nil || manifest.Branding != nil {
			return fmt.Errorf("manifest: required_capabilities must include %q when app UI metadata is declared", CapabilityAppUIV1)
		}
		return nil
	}
	if (manifest.RunUI != nil || manifest.Branding != nil) && !seen[CapabilityAppUIV1] {
		return fmt.Errorf("manifest: required_capabilities must include %q when app UI metadata is declared", CapabilityAppUIV1)
	}
	if len(manifest.Bindings) > 0 && !seen[CapabilitySetupBindingsV1] {
		return fmt.Errorf("manifest: required_capabilities must include %q when bindings are declared", CapabilitySetupBindingsV1)
	}
	for _, field := range manifest.ConfigSchema {
		if field.TestKind != "" && !seen[CapabilityConnectionV1] {
			return fmt.Errorf("manifest: required_capabilities must include %q when connection tests are declared", CapabilityConnectionV1)
		}
	}
	for _, requirement := range manifest.CredentialRequirements {
		if requirement.TestKind != "" && !seen[CapabilityConnectionV1] {
			return fmt.Errorf("manifest: required_capabilities must include %q when connection tests are declared", CapabilityConnectionV1)
		}
	}
	return nil
}

func validateRunUI(manifest Manifest, nodesJSON string) error {
	if manifest.RunUI == nil && manifest.Branding == nil {
		return nil
	}
	if manifest.Branding != nil {
		if len(manifest.Branding.Icon) > 8 {
			return fmt.Errorf("manifest: branding.icon exceeds 8 character limit")
		}
		color := strings.TrimSpace(manifest.Branding.AccentColor)
		if color != "" && (len(color) != 7 || color[0] != '#' || !isHex(color[1:])) {
			return fmt.Errorf("manifest: branding.accent_color must be a #RRGGBB color")
		}
	}
	if manifest.RunUI == nil {
		return nil
	}
	runUI := manifest.RunUI
	if runUI.InputMode != "" && runUI.InputMode != "direct" && runUI.InputMode != "webhook_body" {
		return fmt.Errorf("manifest: run_ui.input_mode must be direct or webhook_body")
	}
	if runUI.OutputMode != "" && runUI.OutputMode != "auto" && runUI.OutputMode != "json" && runUI.OutputMode != "cards" && runUI.OutputMode != "table" {
		return fmt.Errorf("manifest: run_ui.output_mode must be auto, json, cards, or table")
	}
	if len(runUI.SubmitLabel) > MaxSetupLabelLength {
		return fmt.Errorf("manifest: run_ui.submit_label exceeds %d character limit", MaxSetupLabelLength)
	}
	if len(runUI.InputFields) > MaxRunFields {
		return fmt.Errorf("manifest: run_ui.input_fields exceeds %d item limit", MaxRunFields)
	}
	seen := map[string]bool{}
	allowed := map[string]bool{"string": true, "textarea": true, "number": true, "integer": true, "boolean": true, "select": true, "json": true, "number_list": true}
	for i, field := range runUI.InputFields {
		if err := validateSetupKey(field.Key, fmt.Sprintf("manifest: run_ui.input_fields[%d].key", i)); err != nil {
			return err
		}
		if seen[field.Key] {
			return fmt.Errorf("manifest: run_ui.input_fields[%d] duplicates key %q", i, field.Key)
		}
		seen[field.Key] = true
		if strings.TrimSpace(field.Label) == "" || len(field.Label) > MaxSetupLabelLength {
			return fmt.Errorf("manifest: run_ui.input_fields[%d].label is required and bounded", i)
		}
		if !allowed[field.Type] {
			return fmt.Errorf("manifest: run_ui.input_fields[%d].type %q is not supported", i, field.Type)
		}
		if field.Type == "select" && len(field.Options) == 0 {
			return fmt.Errorf("manifest: run_ui.input_fields[%d].options is required for select", i)
		}
	}
	if strings.TrimSpace(runUI.OutputNodeID) != "" {
		var nodeList []nodes.Node
		if err := json.Unmarshal([]byte(nodesJSON), &nodeList); err != nil {
			return fmt.Errorf("manifest: validate run_ui.output_node_id: %w", err)
		}
		found := false
		for _, node := range nodeList {
			if node.ID == runUI.OutputNodeID {
				found = true
				break
			}
		}
		if !found {
			return fmt.Errorf("manifest: run_ui.output_node_id references missing node %q", runUI.OutputNodeID)
		}
	}
	return nil
}

func isHex(value string) bool {
	for _, r := range value {
		if !((r >= '0' && r <= '9') || (r >= 'a' && r <= 'f') || (r >= 'A' && r <= 'F')) {
			return false
		}
	}
	return true
}

func isValidCapability(value string) bool {
	if value == "" || strings.HasPrefix(value, ".") || strings.HasSuffix(value, ".") || strings.Contains(value, "..") {
		return false
	}
	for _, r := range value {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '.' || r == '-' {
			continue
		}
		return false
	}
	return true
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

// IsValidSemVer exposes the Pack format's single SemVer validator to other
// internal lifecycle components.
func IsValidSemVer(version string) bool {
	return isValidSemVer(version)
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
	return resolved, nil
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
