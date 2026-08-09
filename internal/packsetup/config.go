package packsetup

import (
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"goflow/internal/pack"
)

const (
	ConfigFileName        = "pack-config.json"
	ConfigSchemaVersion   = 1
	MaxConfigStorageBytes = 64 << 10
)

type ConfigFile struct {
	PackID              string                 `json:"pack_id"`
	ConfigSchemaVersion int                    `json:"config_schema_version"`
	Values              map[string]interface{} `json:"values"`
}

type LoadedConfig struct {
	Config ConfigFile
	Stale  map[string]interface{}
}

func LoadConfig(dataDir string, manifest pack.Manifest) (*LoadedConfig, error) {
	path := filepath.Join(dataDir, ConfigFileName)
	data, err := readFileLimit(path, MaxConfigStorageBytes)
	if err != nil {
		return nil, err
	}
	var cfg ConfigFile
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("pack setup: config file must be JSON: %w", err)
	}
	if cfg.PackID != manifest.ID {
		return nil, fmt.Errorf("pack setup: config pack_id mismatch: expected %q got %q", manifest.ID, cfg.PackID)
	}
	if cfg.ConfigSchemaVersion != ConfigSchemaVersion {
		return nil, fmt.Errorf("pack setup: unsupported config_schema_version %d", cfg.ConfigSchemaVersion)
	}
	if cfg.Values == nil {
		cfg.Values = map[string]interface{}{}
	}
	validated, stale, err := validateStoredValues(manifest, cfg.Values)
	if err != nil {
		return nil, err
	}
	cfg.Values = validated
	return &LoadedConfig{Config: cfg, Stale: stale}, nil
}

func SaveConfig(dataDir string, manifest pack.Manifest, values map[string]interface{}) (*ConfigFile, error) {
	if err := os.MkdirAll(dataDir, 0700); err != nil {
		return nil, fmt.Errorf("pack setup: create data directory: %w", err)
	}
	existing := map[string]interface{}{}
	if loaded, err := LoadConfig(dataDir, manifest); err == nil {
		for key, value := range loaded.Stale {
			existing[key] = value
		}
	} else if !os.IsNotExist(err) {
		return nil, err
	}
	for key, value := range values {
		existing[key] = value
	}
	validated, stale, err := validateStoredValues(manifest, existing)
	if err != nil {
		return nil, err
	}
	allValues := make(map[string]interface{}, len(validated)+len(stale))
	for key, value := range stale {
		allValues[key] = value
	}
	for key, value := range validated {
		allValues[key] = value
	}
	cfg := ConfigFile{
		PackID:              manifest.ID,
		ConfigSchemaVersion: ConfigSchemaVersion,
		Values:              allValues,
	}
	if err := writeConfigAtomic(filepath.Join(dataDir, ConfigFileName), cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}

func validateStoredValues(manifest pack.Manifest, values map[string]interface{}) (map[string]interface{}, map[string]interface{}, error) {
	fields := map[string]pack.ConfigField{}
	for _, field := range manifest.ConfigSchema {
		fields[field.Key] = field
	}
	validated := map[string]interface{}{}
	stale := map[string]interface{}{}
	for key, value := range values {
		field, ok := fields[key]
		if !ok {
			if !safeStaleValue(value) {
				return nil, nil, fmt.Errorf("pack setup: stale config field %q is not safe to retain", key)
			}
			stale[key] = value
			continue
		}
		normalized, err := validateValue(field, value)
		if err != nil {
			return nil, nil, fmt.Errorf("pack setup: config %q: %w", key, err)
		}
		validated[key] = normalized
	}
	for _, field := range manifest.ConfigSchema {
		if !field.Required {
			continue
		}
		if _, ok := validated[field.Key]; !ok {
			return nil, nil, fmt.Errorf("pack setup: config %q is required", field.Key)
		}
	}
	return validated, stale, nil
}

func validateValue(field pack.ConfigField, value interface{}) (interface{}, error) {
	if value == nil {
		if field.Required {
			return nil, fmt.Errorf("value is required")
		}
		return nil, nil
	}
	switch field.Type {
	case "string":
		text, ok := value.(string)
		if !ok {
			return nil, fmt.Errorf("must be a string")
		}
		if err := validateStringLength(field, text); err != nil {
			return nil, err
		}
		return text, nil
	case "url":
		text, ok := value.(string)
		if !ok {
			return nil, fmt.Errorf("must be a string")
		}
		if err := validateStringLength(field, text); err != nil {
			return nil, err
		}
		if err := validateHTTPURL(text); err != nil {
			return nil, err
		}
		return text, nil
	case "integer":
		n, err := numberAsInt(value)
		if err != nil {
			return nil, err
		}
		if field.Min != nil && n < *field.Min {
			return nil, fmt.Errorf("must be >= %d", *field.Min)
		}
		if field.Max != nil && n > *field.Max {
			return nil, fmt.Errorf("must be <= %d", *field.Max)
		}
		return n, nil
	case "boolean":
		boolean, ok := value.(bool)
		if !ok {
			return nil, fmt.Errorf("must be a boolean")
		}
		return boolean, nil
	case "select":
		key, err := scalarKey(value)
		if err != nil {
			return nil, err
		}
		for _, option := range field.Options {
			if optionKey, err := scalarKey(option); err == nil && optionKey == key {
				return value, nil
			}
		}
		return nil, fmt.Errorf("must match one configured option")
	default:
		return nil, fmt.Errorf("unsupported config type %q", field.Type)
	}
}

func validateStringLength(field pack.ConfigField, value string) error {
	if field.MinLength != nil && len(value) < *field.MinLength {
		return fmt.Errorf("must be at least %d characters", *field.MinLength)
	}
	if field.MaxLength != nil && len(value) > *field.MaxLength {
		return fmt.Errorf("must be at most %d characters", *field.MaxLength)
	}
	return nil
}

func validateHTTPURL(value string) error {
	parsed, err := url.Parse(value)
	if err != nil {
		return fmt.Errorf("must be an absolute http or https URL: %w", err)
	}
	if !parsed.IsAbs() || parsed.Host == "" {
		return fmt.Errorf("must be an absolute http or https URL")
	}
	switch strings.ToLower(parsed.Scheme) {
	case "http", "https":
		return nil
	default:
		return fmt.Errorf("must be an absolute http or https URL")
	}
}

func numberAsInt(value interface{}) (int, error) {
	switch typed := value.(type) {
	case int:
		return typed, nil
	case float64:
		if math.Trunc(typed) != typed || typed > math.MaxInt || typed < math.MinInt {
			return 0, fmt.Errorf("must be an integer")
		}
		return int(typed), nil
	case json.Number:
		n, err := typed.Int64()
		if err != nil || n > math.MaxInt || n < math.MinInt {
			return 0, fmt.Errorf("must be an integer")
		}
		return int(n), nil
	default:
		return 0, fmt.Errorf("must be an integer")
	}
}

func scalarKey(value interface{}) (string, error) {
	switch typed := value.(type) {
	case string:
		return "string:" + typed, nil
	case bool:
		return fmt.Sprintf("bool:%t", typed), nil
	case int:
		return fmt.Sprintf("number:%d", typed), nil
	case float64:
		if math.Trunc(typed) != typed {
			return "", fmt.Errorf("must be a scalar option")
		}
		return fmt.Sprintf("number:%0.f", typed), nil
	default:
		return "", fmt.Errorf("must be a scalar option")
	}
}

func safeStaleValue(value interface{}) bool {
	switch typed := value.(type) {
	case nil, bool, float64, string:
		return !looksSecretLike(typed)
	case []interface{}:
		for _, item := range typed {
			if !safeStaleValue(item) {
				return false
			}
		}
		return true
	case map[string]interface{}:
		for key, item := range typed {
			if secretWord(key) || !safeStaleValue(item) {
				return false
			}
		}
		return true
	default:
		return false
	}
}

func looksSecretLike(value interface{}) bool {
	text, ok := value.(string)
	if !ok {
		return false
	}
	lower := strings.ToLower(strings.TrimSpace(text))
	return strings.Contains(lower, "-----begin") ||
		strings.Contains(lower, "sk-") ||
		strings.Contains(lower, "secret:") ||
		strings.Contains(lower, "password:") ||
		strings.Contains(lower, "token:")
}

func secretWord(value string) bool {
	lower := strings.ToLower(value)
	return strings.Contains(lower, "token") ||
		strings.Contains(lower, "password") ||
		strings.Contains(lower, "secret") ||
		strings.Contains(lower, "api_key") ||
		strings.Contains(lower, "api key") ||
		strings.Contains(lower, "private_key")
}

func readFileLimit(path string, limit int64) ([]byte, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	data, err := io.ReadAll(io.LimitReader(file, limit+1))
	if err != nil {
		return nil, err
	}
	if int64(len(data)) > limit {
		return nil, fmt.Errorf("pack setup: config file exceeds %d byte limit", limit)
	}
	return data, nil
}

func writeConfigAtomic(path string, cfg ConfigFile) error {
	return writeJSONAtomic(path, cfg)
}

func writeJSONAtomic(path string, value interface{}) error {
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	tmp, err := os.CreateTemp(filepath.Dir(path), "."+filepath.Base(path)+".tmp-*")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	defer func() {
		_ = os.Remove(tmpPath)
	}()
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Chmod(0600); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmpPath, path)
}
