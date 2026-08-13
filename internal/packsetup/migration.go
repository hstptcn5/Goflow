package packsetup

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"goflow/internal/pack"
)

const (
	MigrationFileName        = "pack-migration-state.json"
	MigrationSchemaVersion   = 1
	MaxMigrationStorageBytes = 32 << 10

	MigrationRevalidation = "revalidation"
	MigrationConfig       = "config"
	MigrationUserReview   = "user_review"
)

type MigrationState struct {
	SchemaVersion  int      `json:"schema_version"`
	PackID         string   `json:"pack_id"`
	FromVersion    string   `json:"from_version"`
	ToVersion      string   `json:"to_version"`
	Category       string   `json:"category"`
	AppliedSteps   []string `json:"applied_steps"`
	BackupRelative string   `json:"backup_relative"`
	AppliedAt      string   `json:"applied_at"`
}

type MigrationData struct {
	ConfigValues    map[string]interface{}
	CredentialSlots map[string]CredentialSlot
}

type MigrationStep struct {
	PackID      string
	FromVersion string
	ToVersion   string
	Category    string
	Transform   func(*MigrationData) error
}

type MigrationRegistry struct {
	steps map[string]MigrationStep
}

type MigrationResult struct {
	Changed        bool
	AlreadyApplied bool
	Category       string
	State          *MigrationState
}

type MigrationOptions struct {
	Now        time.Time
	AfterWrite func(path string) error
}

func NewMigrationRegistry(steps ...MigrationStep) (*MigrationRegistry, error) {
	registry := &MigrationRegistry{steps: map[string]MigrationStep{}}
	for _, step := range steps {
		if strings.TrimSpace(step.PackID) == "" || !validVersion(step.FromVersion) || !validVersion(step.ToVersion) {
			return nil, fmt.Errorf("pack setup: migration step identity is invalid")
		}
		if compareVersions(step.FromVersion, step.ToVersion) >= 0 {
			return nil, fmt.Errorf("pack setup: migration step must move forward")
		}
		switch step.Category {
		case MigrationRevalidation, MigrationConfig, MigrationUserReview:
		default:
			return nil, fmt.Errorf("pack setup: migration category is invalid")
		}
		key := migrationStepKey(step.PackID, step.FromVersion)
		if _, exists := registry.steps[key]; exists {
			return nil, fmt.Errorf("pack setup: duplicate migration from %s %s", step.PackID, step.FromVersion)
		}
		registry.steps[key] = step
	}
	return registry, nil
}

func DefaultMigrationRegistry() *MigrationRegistry {
	registry, err := NewMigrationRegistry(
		MigrationStep{
			PackID:      "official.dailyops-rest-telegram",
			FromVersion: "0.1.0",
			ToVersion:   "0.2.0",
			Category:    MigrationConfig,
			Transform: func(data *MigrationData) error {
				delete(data.ConfigValues, "report_title")
				delete(data.ConfigValues, "low_stock_threshold")
				return nil
			},
		},
		MigrationStep{
			PackID:      "official.dailyops-rest-telegram",
			FromVersion: "0.2.0",
			ToVersion:   "0.3.0",
			Category:    MigrationRevalidation,
		},
	)
	if err != nil {
		panic(err)
	}
	return registry
}

func ApplyMigrations(dataDir string, manifest pack.Manifest, fromVersion string, registry *MigrationRegistry, options MigrationOptions) (*MigrationResult, error) {
	if registry == nil {
		return nil, fmt.Errorf("pack setup: migration registry is required")
	}
	if !validVersion(fromVersion) || !validVersion(manifest.Version) {
		return nil, fmt.Errorf("pack setup: migration versions are invalid")
	}
	if compareVersions(fromVersion, manifest.Version) > 0 {
		return nil, fmt.Errorf("pack setup: automatic downgrade from %q to %q is not allowed", fromVersion, manifest.Version)
	}
	if existing, err := loadMigrationState(dataDir, manifest.ID); err == nil {
		if existing.ToVersion == manifest.Version && existing.FromVersion == fromVersion {
			return &MigrationResult{AlreadyApplied: true, Category: existing.Category, State: existing}, nil
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return nil, err
	}
	if fromVersion == manifest.Version {
		return &MigrationResult{}, nil
	}
	if options.Now.IsZero() {
		options.Now = time.Now()
	}

	steps, category, err := registry.path(manifest.ID, fromVersion, manifest.Version)
	if err != nil {
		return nil, err
	}
	data, original, err := loadMigrationData(dataDir, manifest.ID)
	if err != nil {
		return nil, err
	}
	applied := make([]string, 0, len(steps))
	for _, step := range steps {
		if step.Transform != nil {
			if err := step.Transform(data); err != nil {
				return nil, fmt.Errorf("pack setup: migration %s to %s failed: %w", step.FromVersion, step.ToVersion, err)
			}
		}
		applied = append(applied, step.FromVersion+"->"+step.ToVersion)
	}
	if data.ConfigValues, err = normalizeMigratedConfig(manifest, data.ConfigValues); err != nil {
		return nil, fmt.Errorf("pack setup: migrated config is invalid: %w", err)
	}
	if data.CredentialSlots, err = normalizeMigratedCredentials(manifest, data.CredentialSlots); err != nil {
		return nil, fmt.Errorf("pack setup: migrated credential bindings are invalid: %w", err)
	}
	backupRelative, err := createMigrationBackup(dataDir, manifest.ID, fromVersion, manifest.Version, original)
	if err != nil {
		return nil, err
	}

	state := &MigrationState{
		SchemaVersion: MigrationSchemaVersion, PackID: manifest.ID,
		FromVersion: fromVersion, ToVersion: manifest.Version, Category: category,
		AppliedSteps: applied, BackupRelative: backupRelative,
		AppliedAt: options.Now.UTC().Format(time.RFC3339),
	}
	writes := []struct {
		name  string
		value interface{}
	}{
		{ConfigFileName, ConfigFile{PackID: manifest.ID, ConfigSchemaVersion: ConfigSchemaVersion, Values: data.ConfigValues}},
		{CredentialFileName, CredentialFile{PackID: manifest.ID, CredentialSchemaVersion: CredentialSchemaVersion, Slots: data.CredentialSlots}},
		{StateFileName, StateFile{PackID: manifest.ID, PackVersion: manifest.Version, StateSchemaVersion: StateSchemaVersion, Completed: false, UpdatedAt: state.AppliedAt}},
		{MigrationFileName, state},
	}
	for _, write := range writes {
		path := filepath.Join(dataDir, write.name)
		if err := writeJSONAtomic(path, write.value); err != nil {
			if rollbackErr := rollbackMigrationFiles(dataDir, original); rollbackErr != nil {
				return nil, errors.Join(fmt.Errorf("pack setup: persist migration: %w", err), rollbackErr)
			}
			return nil, fmt.Errorf("pack setup: persist migration: %w", err)
		}
		if options.AfterWrite != nil {
			if err := options.AfterWrite(write.name); err != nil {
				if rollbackErr := rollbackMigrationFiles(dataDir, original); rollbackErr != nil {
					return nil, errors.Join(fmt.Errorf("pack setup: persist migration: %w", err), rollbackErr)
				}
				return nil, fmt.Errorf("pack setup: persist migration: %w", err)
			}
		}
	}
	return &MigrationResult{Changed: true, Category: category, State: state}, nil
}

func (r *MigrationRegistry) path(packID, fromVersion, toVersion string) ([]MigrationStep, string, error) {
	current := fromVersion
	steps := []MigrationStep{}
	category := MigrationRevalidation
	visited := map[string]bool{}
	for current != toVersion {
		if visited[current] {
			return nil, "", fmt.Errorf("pack setup: migration chain contains a cycle")
		}
		visited[current] = true
		step, ok := r.steps[migrationStepKey(packID, current)]
		if !ok || compareVersions(step.ToVersion, toVersion) > 0 {
			// Unknown changes never mutate setup values. They require explicit
			// user review while preserving files and credential references.
			steps = append(steps, MigrationStep{
				PackID: packID, FromVersion: current, ToVersion: toVersion,
				Category: MigrationUserReview,
			})
			category = MigrationUserReview
			break
		}
		steps = append(steps, step)
		if step.Category == MigrationUserReview || (step.Category == MigrationConfig && category == MigrationRevalidation) {
			category = step.Category
		}
		current = step.ToVersion
	}
	return steps, category, nil
}

func loadMigrationData(dataDir, packID string) (*MigrationData, map[string][]byte, error) {
	original := map[string][]byte{}
	for _, name := range []string{ConfigFileName, CredentialFileName, StateFileName, MigrationFileName} {
		value, err := readFileLimit(filepath.Join(dataDir, name), MaxConfigStorageBytes)
		if err == nil {
			original[name] = value
		} else if !errors.Is(err, os.ErrNotExist) {
			return nil, nil, err
		}
	}
	data := &MigrationData{ConfigValues: map[string]interface{}{}, CredentialSlots: map[string]CredentialSlot{}}
	if raw, ok := original[ConfigFileName]; ok {
		var file ConfigFile
		if err := json.Unmarshal(raw, &file); err != nil || file.PackID != packID || file.ConfigSchemaVersion != ConfigSchemaVersion {
			return nil, nil, fmt.Errorf("pack setup: existing config cannot be migrated")
		}
		if file.Values != nil {
			data.ConfigValues = file.Values
		}
	}
	if raw, ok := original[CredentialFileName]; ok {
		var file CredentialFile
		if err := json.Unmarshal(raw, &file); err != nil || file.PackID != packID || file.CredentialSchemaVersion != CredentialSchemaVersion {
			return nil, nil, fmt.Errorf("pack setup: existing credential bindings cannot be migrated")
		}
		if file.Slots != nil {
			data.CredentialSlots = file.Slots
		}
	}
	return data, original, nil
}

func normalizeMigratedConfig(manifest pack.Manifest, values map[string]interface{}) (map[string]interface{}, error) {
	fields := map[string]pack.ConfigField{}
	for _, field := range manifest.ConfigSchema {
		fields[field.Key] = field
	}
	result := map[string]interface{}{}
	for key, value := range values {
		field, current := fields[key]
		if !current {
			if !safeStaleValue(value) {
				return nil, fmt.Errorf("stale field %q is not safe", key)
			}
			result[key] = value
			continue
		}
		normalized, err := validateValue(field, value)
		if err == nil {
			result[key] = normalized
		}
	}
	for _, field := range manifest.ConfigSchema {
		if _, exists := result[field.Key]; !exists && field.Default != nil {
			normalized, err := validateValue(field, field.Default)
			if err != nil {
				return nil, err
			}
			result[field.Key] = normalized
		}
	}
	return result, nil
}

func normalizeMigratedCredentials(manifest pack.Manifest, slots map[string]CredentialSlot) (map[string]CredentialSlot, error) {
	requirements := map[string]pack.CredentialRequirement{}
	for _, requirement := range manifest.CredentialRequirements {
		requirements[requirement.Key] = requirement
	}
	result := map[string]CredentialSlot{}
	for key, slot := range slots {
		if err := validateStoredCredentialSlotShape(key, slot); err != nil {
			return nil, err
		}
		requirement, current := requirements[key]
		if current && requirement.Type != slot.CredentialType {
			continue
		}
		result[key] = slot
	}
	return result, nil
}

func createMigrationBackup(dataDir, packID, fromVersion, toVersion string, files map[string][]byte) (string, error) {
	name := sanitizeVersion(fromVersion) + "-to-" + sanitizeVersion(toVersion)
	relative := filepath.Join("setup-backups", name)
	finalDir := filepath.Join(dataDir, relative)
	if _, err := os.Stat(finalDir); err == nil {
		return filepath.ToSlash(relative), nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return "", err
	}
	root := filepath.Dir(finalDir)
	if err := os.MkdirAll(root, 0700); err != nil {
		return "", err
	}
	tempDir, err := os.MkdirTemp(root, "."+name+".tmp-")
	if err != nil {
		return "", err
	}
	defer os.RemoveAll(tempDir)
	type entry struct {
		Name   string `json:"name"`
		SHA256 string `json:"sha256"`
	}
	inventory := struct {
		SchemaVersion int     `json:"schema_version"`
		PackID        string  `json:"pack_id"`
		FromVersion   string  `json:"from_version"`
		ToVersion     string  `json:"to_version"`
		Files         []entry `json:"files"`
	}{SchemaVersion: 1, PackID: packID, FromVersion: fromVersion, ToVersion: toVersion}
	names := make([]string, 0, len(files))
	for fileName := range files {
		if fileName != MigrationFileName {
			names = append(names, fileName)
		}
	}
	sort.Strings(names)
	for _, fileName := range names {
		value := files[fileName]
		if err := os.WriteFile(filepath.Join(tempDir, fileName), value, 0600); err != nil {
			return "", err
		}
		sum := sha256.Sum256(value)
		inventory.Files = append(inventory.Files, entry{Name: fileName, SHA256: hex.EncodeToString(sum[:])})
	}
	if err := writeJSONAtomic(filepath.Join(tempDir, "BACKUP_INFO.json"), inventory); err != nil {
		return "", err
	}
	if err := os.Rename(tempDir, finalDir); err != nil {
		return "", err
	}
	return filepath.ToSlash(relative), nil
}

func rollbackMigrationFiles(dataDir string, original map[string][]byte) error {
	var rollbackErrors []error
	for _, name := range []string{ConfigFileName, CredentialFileName, StateFileName, MigrationFileName} {
		path := filepath.Join(dataDir, name)
		if value, ok := original[name]; ok {
			if err := writeBytesAtomic(path, value); err != nil {
				rollbackErrors = append(rollbackErrors, fmt.Errorf("restore %s: %w", name, err))
			}
		} else {
			if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
				rollbackErrors = append(rollbackErrors, fmt.Errorf("remove %s: %w", name, err))
			}
		}
	}
	if len(rollbackErrors) > 0 {
		return fmt.Errorf("pack setup: migration rollback failed: %w", errors.Join(rollbackErrors...))
	}
	return nil
}

func writeBytesAtomic(path string, value []byte) error {
	temp, err := os.CreateTemp(filepath.Dir(path), "."+filepath.Base(path)+".tmp-")
	if err != nil {
		return err
	}
	tempPath := temp.Name()
	defer os.Remove(tempPath)
	if _, err := temp.Write(value); err != nil {
		temp.Close()
		return err
	}
	if err := temp.Chmod(0600); err != nil {
		temp.Close()
		return err
	}
	if err := temp.Close(); err != nil {
		return err
	}
	return os.Rename(tempPath, path)
}

func loadMigrationState(dataDir, packID string) (*MigrationState, error) {
	raw, err := readFileLimit(filepath.Join(dataDir, MigrationFileName), MaxMigrationStorageBytes)
	if err != nil {
		return nil, err
	}
	var state MigrationState
	if err := json.Unmarshal(raw, &state); err != nil {
		return nil, fmt.Errorf("pack setup: migration state must be JSON")
	}
	if state.SchemaVersion != MigrationSchemaVersion {
		return nil, fmt.Errorf("pack setup: unsupported migration schema_version %d", state.SchemaVersion)
	}
	if state.PackID != packID || !validVersion(state.FromVersion) || !validVersion(state.ToVersion) {
		return nil, fmt.Errorf("pack setup: migration state identity is invalid")
	}
	switch state.Category {
	case MigrationRevalidation, MigrationConfig, MigrationUserReview:
	default:
		return nil, fmt.Errorf("pack setup: migration state category is invalid")
	}
	return &state, nil
}

// LoadMigrationState returns only validated, bounded migration metadata for the
// current Pack version. It contains no config values or credential references.
func LoadMigrationState(dataDir string, manifest pack.Manifest) (*MigrationState, error) {
	state, err := loadMigrationState(dataDir, manifest.ID)
	if err != nil {
		return nil, err
	}
	if state.ToVersion != manifest.Version {
		return nil, fmt.Errorf("pack setup: migration state is not current")
	}
	return state, nil
}

func migrationStepKey(packID, fromVersion string) string { return packID + "\x00" + fromVersion }

func sanitizeVersion(value string) string {
	return strings.NewReplacer("+", "_", ".", "_", "-", "_").Replace(value)
}

func validVersion(value string) bool {
	return pack.IsValidSemVer(value)
}

func compareVersions(a, b string) int {
	parse := func(value string) ([3]string, []string) {
		var result [3]string
		base := strings.SplitN(value, "+", 2)[0]
		parts := strings.SplitN(base, "-", 2)
		for index, raw := range strings.Split(parts[0], ".") {
			result[index] = strings.TrimLeft(raw, "0")
			if result[index] == "" {
				result[index] = "0"
			}
		}
		if len(parts) == 2 {
			return result, strings.Split(parts[1], ".")
		}
		return result, nil
	}
	ac, ap := parse(a)
	bc, bp := parse(b)
	for index := range ac {
		if comparison := compareNumericText(ac[index], bc[index]); comparison != 0 {
			return comparison
		}
	}
	if len(ap) == 0 && len(bp) == 0 {
		return 0
	}
	if len(ap) == 0 {
		return 1
	}
	if len(bp) == 0 {
		return -1
	}
	for index := 0; index < len(ap) && index < len(bp); index++ {
		aNumeric := allDigits(ap[index])
		bNumeric := allDigits(bp[index])
		switch {
		case aNumeric && bNumeric:
			if comparison := compareNumericText(ap[index], bp[index]); comparison != 0 {
				return comparison
			}
		case aNumeric:
			return -1
		case bNumeric:
			return 1
		default:
			if comparison := strings.Compare(ap[index], bp[index]); comparison != 0 {
				return comparison
			}
		}
	}
	if len(ap) < len(bp) {
		return -1
	}
	if len(ap) > len(bp) {
		return 1
	}
	return 0
}

func compareNumericText(a, b string) int {
	a = strings.TrimLeft(a, "0")
	b = strings.TrimLeft(b, "0")
	if a == "" {
		a = "0"
	}
	if b == "" {
		b = "0"
	}
	if len(a) < len(b) {
		return -1
	}
	if len(a) > len(b) {
		return 1
	}
	return strings.Compare(a, b)
}

func allDigits(value string) bool {
	if value == "" {
		return false
	}
	for _, char := range value {
		if char < '0' || char > '9' {
			return false
		}
	}
	return true
}
