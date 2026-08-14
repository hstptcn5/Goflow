package pack

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
)

const OfflineFixtureSchemaVersion = 1

// OfflineTestFixture contains deterministic, non-secret author inputs used only
// by `pack test`. Credential records remain synthetic and host-owned.
type OfflineTestFixture struct {
	SchemaVersion int                    `json:"schema_version"`
	Config        map[string]interface{} `json:"config"`
}

func LoadOfflineTestFixture(loaded *Pack) (*OfflineTestFixture, error) {
	if loaded == nil || loaded.OfflineFixturePath == "" {
		return nil, nil
	}
	return readOfflineTestFixture(loaded.OfflineFixturePath, loaded.Manifest)
}

func readOfflineTestFixture(path string, manifest Manifest) (*OfflineTestFixture, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("manifest: offline_test_fixture: open: %w", err)
	}
	defer file.Close()
	data, err := io.ReadAll(io.LimitReader(file, MaxOfflineFixtureBytes+1))
	if err != nil {
		return nil, fmt.Errorf("manifest: offline_test_fixture: read: %w", err)
	}
	if len(data) > MaxOfflineFixtureBytes {
		return nil, fmt.Errorf("manifest: offline_test_fixture exceeds %d byte limit", MaxOfflineFixtureBytes)
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	decoder.UseNumber()
	var fixture OfflineTestFixture
	if err := decoder.Decode(&fixture); err != nil {
		return nil, fmt.Errorf("manifest: offline_test_fixture must be strict JSON: %w", err)
	}
	if err := ensureJSONEOF(decoder); err != nil {
		return nil, fmt.Errorf("manifest: offline_test_fixture must contain one JSON object: %w", err)
	}
	if fixture.SchemaVersion != OfflineFixtureSchemaVersion {
		return nil, fmt.Errorf("manifest: offline_test_fixture.schema_version must be %d", OfflineFixtureSchemaVersion)
	}
	if fixture.Config == nil {
		fixture.Config = map[string]interface{}{}
	}
	fields := make(map[string]ConfigField, len(manifest.ConfigSchema))
	for _, field := range manifest.ConfigSchema {
		fields[field.Key] = field
	}
	for key, value := range fixture.Config {
		field, ok := fields[key]
		if !ok {
			return nil, fmt.Errorf("manifest: offline_test_fixture.config.%s references an unknown config key", key)
		}
		if containsSecretValue(value) {
			return nil, fmt.Errorf("manifest: offline_test_fixture.config.%s must not look like secret material", key)
		}
		if _, err := ValidateConfigValue(field, value); err != nil {
			return nil, fmt.Errorf("manifest: offline_test_fixture.config.%s %w", key, err)
		}
	}
	return &fixture, nil
}

func ensureJSONEOF(decoder *json.Decoder) error {
	var trailing interface{}
	if err := decoder.Decode(&trailing); err != io.EOF {
		if err == nil {
			return fmt.Errorf("unexpected trailing JSON value")
		}
		return err
	}
	return nil
}
