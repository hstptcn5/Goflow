package packsetup

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"goflow/internal/pack"
)

const (
	CredentialFileName        = "pack-credentials.json"
	CredentialSchemaVersion   = 1
	MaxCredentialStorageBytes = 64 << 10
	MaxCredentialIDLength     = 128
)

type CredentialIdentity struct {
	ID   string
	Type string
}

type CredentialResolver interface {
	LookupCredential(id string) (CredentialIdentity, error)
}

type CredentialLookupFunc func(id string) (CredentialIdentity, error)

func (fn CredentialLookupFunc) LookupCredential(id string) (CredentialIdentity, error) {
	return fn(id)
}

type CredentialFile struct {
	PackID                  string                    `json:"pack_id"`
	CredentialSchemaVersion int                       `json:"credential_schema_version"`
	Slots                   map[string]CredentialSlot `json:"slots"`
}

type CredentialSlot struct {
	CredentialID   string `json:"credential_id"`
	CredentialType string `json:"credential_type"`
}

type LoadedCredentials struct {
	Credentials CredentialFile
	Stale       map[string]CredentialSlot
}

func LoadCredentialBindings(dataDir string, manifest pack.Manifest, resolver CredentialResolver) (*LoadedCredentials, error) {
	path := filepath.Join(dataDir, CredentialFileName)
	data, err := readFileLimit(path, MaxCredentialStorageBytes)
	if err != nil {
		return nil, err
	}
	var file CredentialFile
	if err := json.Unmarshal(data, &file); err != nil {
		return nil, fmt.Errorf("pack setup: credential file must be JSON: %w", err)
	}
	if file.PackID != manifest.ID {
		return nil, fmt.Errorf("pack setup: credential pack_id mismatch: expected %q got %q", manifest.ID, file.PackID)
	}
	if file.CredentialSchemaVersion != CredentialSchemaVersion {
		return nil, fmt.Errorf("pack setup: unsupported credential_schema_version %d", file.CredentialSchemaVersion)
	}
	if file.Slots == nil {
		file.Slots = map[string]CredentialSlot{}
	}
	current, stale, err := validateStoredCredentialSlots(manifest, file.Slots, resolver, false)
	if err != nil {
		return nil, err
	}
	file.Slots = current
	return &LoadedCredentials{Credentials: file, Stale: stale}, nil
}

func SaveCredentialBindings(dataDir string, manifest pack.Manifest, slots map[string]string, resolver CredentialResolver) (*CredentialFile, error) {
	if err := os.MkdirAll(dataDir, 0700); err != nil {
		return nil, fmt.Errorf("pack setup: create data directory: %w", err)
	}
	existing := map[string]CredentialSlot{}
	if loaded, err := LoadCredentialBindings(dataDir, manifest, resolver); err == nil {
		for key, slot := range loaded.Stale {
			existing[key] = slot
		}
	} else if !os.IsNotExist(err) {
		return nil, err
	}
	for key, credentialID := range slots {
		req, ok := credentialRequirementByKey(manifest, key)
		if !ok {
			return nil, fmt.Errorf("pack setup: credential slot %q is not declared by manifest", key)
		}
		existing[key] = CredentialSlot{CredentialID: credentialID, CredentialType: req.Type}
	}
	current, stale, err := validateStoredCredentialSlots(manifest, existing, resolver, true)
	if err != nil {
		return nil, err
	}
	allSlots := make(map[string]CredentialSlot, len(current)+len(stale))
	for key, slot := range stale {
		allSlots[key] = slot
	}
	for key, slot := range current {
		allSlots[key] = slot
	}
	file := CredentialFile{
		PackID:                  manifest.ID,
		CredentialSchemaVersion: CredentialSchemaVersion,
		Slots:                   allSlots,
	}
	if err := writeJSONAtomic(filepath.Join(dataDir, CredentialFileName), file); err != nil {
		return nil, err
	}
	return &file, nil
}

func validateStoredCredentialSlots(manifest pack.Manifest, slots map[string]CredentialSlot, resolver CredentialResolver, requireRequired bool) (map[string]CredentialSlot, map[string]CredentialSlot, error) {
	requirements := map[string]pack.CredentialRequirement{}
	for _, req := range manifest.CredentialRequirements {
		requirements[req.Key] = req
	}
	current := map[string]CredentialSlot{}
	stale := map[string]CredentialSlot{}
	for key, slot := range slots {
		if err := validateStoredCredentialSlotShape(key, slot); err != nil {
			return nil, nil, err
		}
		req, ok := requirements[key]
		if !ok {
			stale[key] = slot
			continue
		}
		if slot.CredentialType != req.Type {
			return nil, nil, fmt.Errorf("pack setup: credential slot %q has type %q, expected %q", key, slot.CredentialType, req.Type)
		}
		if resolver != nil {
			identity, err := resolver.LookupCredential(slot.CredentialID)
			if err != nil {
				return nil, nil, fmt.Errorf("pack setup: credential slot %q: %w", key, err)
			}
			if identity.ID != slot.CredentialID {
				return nil, nil, fmt.Errorf("pack setup: credential slot %q resolved a different credential id", key)
			}
			if identity.Type != req.Type {
				return nil, nil, fmt.Errorf("pack setup: credential slot %q references credential type %q, expected %q", key, identity.Type, req.Type)
			}
		}
		current[key] = slot
	}
	if requireRequired {
		for _, req := range manifest.CredentialRequirements {
			if !req.Required {
				continue
			}
			if _, ok := current[req.Key]; !ok {
				return nil, nil, fmt.Errorf("pack setup: credential slot %q is required", req.Key)
			}
		}
	}
	return current, stale, nil
}

func validateStoredCredentialSlotShape(key string, slot CredentialSlot) error {
	if err := validateSetupStorageKey(key, "credential slot key"); err != nil {
		return err
	}
	if secretWord(key) {
		return fmt.Errorf("pack setup: credential slot key %q is not safe to retain", key)
	}
	if strings.TrimSpace(slot.CredentialID) == "" {
		return fmt.Errorf("pack setup: credential slot %q requires credential_id", key)
	}
	if len(slot.CredentialID) > MaxCredentialIDLength {
		return fmt.Errorf("pack setup: credential slot %q credential_id exceeds %d character limit", key, MaxCredentialIDLength)
	}
	if looksSecretLike(slot.CredentialID) || secretWord(slot.CredentialID) {
		return fmt.Errorf("pack setup: credential slot %q credential_id is not safe to retain", key)
	}
	if !isKnownStoredCredentialType(slot.CredentialType) {
		return fmt.Errorf("pack setup: credential slot %q has unsupported credential_type", key)
	}
	return nil
}

func validateSetupStorageKey(key, field string) error {
	if strings.TrimSpace(key) == "" {
		return fmt.Errorf("%s is required", field)
	}
	if len(key) > pack.MaxSetupKeyLength {
		return fmt.Errorf("%s exceeds %d character limit", field, pack.MaxSetupKeyLength)
	}
	for _, r := range key {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '_' {
			continue
		}
		return fmt.Errorf("%s must use lowercase letters, numbers, and underscores", field)
	}
	return nil
}

func credentialRequirementByKey(manifest pack.Manifest, key string) (pack.CredentialRequirement, bool) {
	for _, req := range manifest.CredentialRequirements {
		if req.Key == key {
			return req, true
		}
	}
	return pack.CredentialRequirement{}, false
}

func isKnownStoredCredentialType(value string) bool {
	switch value {
	case "API_KEY", "TELEGRAM_BOT", "BEARER_TOKEN", "BASIC_AUTH", "OPENAI_API_KEY", "DEEPSEEK_API_KEY", "GOOGLE_SERVICE_ACCOUNT", "DATABASE_URL", "SSH_KEY", "SMTP_ACCOUNT":
		return true
	default:
		return false
	}
}
