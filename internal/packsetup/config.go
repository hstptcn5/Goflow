package packsetup

import (
	"encoding/json"
	"fmt"
	"io"
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
		normalized, err := pack.ValidateConfigValue(field, value)
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

func safeStaleValue(value interface{}) bool {
	switch typed := value.(type) {
	case nil, bool, float64, string:
		return !looksSecretLike(typed)
	case int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64, json.Number:
		return true
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
