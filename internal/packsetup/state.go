package packsetup

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"goflow/internal/pack"
)

const (
	StateFileName        = "pack-setup-state.json"
	StateSchemaVersion   = 1
	MaxStateStorageBytes = 16 << 10
)

type StateFile struct {
	PackID             string `json:"pack_id"`
	PackVersion        string `json:"pack_version"`
	StateSchemaVersion int    `json:"state_schema_version"`
	Completed          bool   `json:"completed"`
	UpdatedAt          string `json:"updated_at"`
}

func LoadState(dataDir string, manifest pack.Manifest) (*StateFile, error) {
	data, err := readFileLimit(filepath.Join(dataDir, StateFileName), MaxStateStorageBytes)
	if err != nil {
		return nil, err
	}
	var state StateFile
	if err := json.Unmarshal(data, &state); err != nil {
		return nil, fmt.Errorf("pack setup: state file must be JSON: %w", err)
	}
	if state.PackID != manifest.ID {
		return nil, fmt.Errorf("pack setup: state pack_id mismatch: expected %q got %q", manifest.ID, state.PackID)
	}
	if state.StateSchemaVersion != StateSchemaVersion {
		return nil, fmt.Errorf("pack setup: unsupported state_schema_version %d", state.StateSchemaVersion)
	}
	if state.Completed && state.PackVersion != manifest.Version {
		return nil, fmt.Errorf("pack setup: completed state requires revalidation for pack version %q", manifest.Version)
	}
	return &state, nil
}

func SaveState(dataDir string, manifest pack.Manifest, completed bool, now time.Time) (*StateFile, error) {
	if err := os.MkdirAll(dataDir, 0700); err != nil {
		return nil, fmt.Errorf("pack setup: create data directory: %w", err)
	}
	if now.IsZero() {
		now = time.Now()
	}
	state := StateFile{
		PackID:             manifest.ID,
		PackVersion:        manifest.Version,
		StateSchemaVersion: StateSchemaVersion,
		Completed:          completed,
		UpdatedAt:          now.UTC().Format(time.RFC3339),
	}
	if err := writeJSONAtomic(filepath.Join(dataDir, StateFileName), state); err != nil {
		return nil, err
	}
	return &state, nil
}
