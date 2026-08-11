package packsetup

import (
	"errors"
	"fmt"
	"os"

	"goflow/internal/client"
	"goflow/internal/pack"
)

// PrepareBoundWorkflow reconstructs an active managed workflow from the
// current pack definition and validated persisted setup values.
func PrepareBoundWorkflow(base client.Workflow, manifest pack.Manifest, dataDir string, resolver CredentialResolver) (client.Workflow, error) {
	configValues := map[string]interface{}{}
	if len(manifest.ConfigSchema) > 0 {
		loaded, err := LoadConfig(dataDir, manifest)
		if err != nil {
			return client.Workflow{}, fmt.Errorf("pack setup: load config: %w", err)
		}
		configValues = loaded.Config.Values
	}

	credentialSlots := map[string]CredentialSlot{}
	if len(manifest.CredentialRequirements) > 0 {
		loaded, err := LoadCredentialBindings(dataDir, manifest, resolver)
		if err != nil {
			return client.Workflow{}, fmt.Errorf("pack setup: load credential bindings: %w", err)
		}
		if err := requireCredentialSlots(manifest, loaded.Credentials.Slots); err != nil {
			return client.Workflow{}, err
		}
		credentialSlots = loaded.Credentials.Slots
	}

	base.IsActive = false
	bound, err := ApplyBindings(base, manifest, configValues, credentialSlots)
	if err != nil {
		return client.Workflow{}, err
	}
	bound.IsActive = true
	return bound, nil
}

// ReconstructManagedWorkflow keeps first-run and incomplete setup workflows
// inactive. Completed setup is restored only from validated persisted values.
func ReconstructManagedWorkflow(base client.Workflow, manifest pack.Manifest, dataDir string, resolver CredentialResolver) (client.Workflow, bool, error) {
	base.IsActive = false
	state, err := LoadState(dataDir, manifest)
	if errors.Is(err, os.ErrNotExist) {
		return base, false, nil
	}
	if err != nil {
		return base, false, fmt.Errorf("pack setup: load completion state: %w", err)
	}
	if !state.Completed {
		return base, false, nil
	}
	bound, err := PrepareBoundWorkflow(base, manifest, dataDir, resolver)
	if err != nil {
		return base, false, err
	}
	return bound, true, nil
}

func requireCredentialSlots(manifest pack.Manifest, slots map[string]CredentialSlot) error {
	for _, requirement := range manifest.CredentialRequirements {
		if !requirement.Required {
			continue
		}
		if _, ok := slots[requirement.Key]; !ok {
			return fmt.Errorf("pack setup: credential slot %q is required", requirement.Key)
		}
	}
	return nil
}
