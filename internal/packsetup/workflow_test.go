package packsetup

import (
	"path/filepath"
	"strings"
	"testing"
	"time"

	"goflow/internal/client"
	"goflow/internal/pack"
)

func TestReconstructManagedWorkflowRequiresCompletedValidSetup(t *testing.T) {
	dataDir := t.TempDir()
	manifest := reconstructionManifest()
	base := reconstructionWorkflow()
	resolver := CredentialLookupFunc(func(id string) (CredentialIdentity, error) {
		return CredentialIdentity{ID: id, Type: "TELEGRAM_BOT"}, nil
	})

	first, completed, err := ReconstructManagedWorkflow(base, manifest, dataDir, resolver)
	if err != nil {
		t.Fatalf("first reconstruction: %v", err)
	}
	if completed || first.IsActive {
		t.Fatalf("first-run workflow must remain inactive: completed=%v active=%v", completed, first.IsActive)
	}

	if _, err := SaveConfig(dataDir, manifest, map[string]interface{}{"source_url": "https://source.example.test/daily.json"}); err != nil {
		t.Fatalf("save config: %v", err)
	}
	if _, err := SaveCredentialBindings(dataDir, manifest, map[string]string{"telegram": "cred-telegram"}, resolver); err != nil {
		t.Fatalf("save credential bindings: %v", err)
	}
	if _, err := SaveState(dataDir, manifest, false, time.Now()); err != nil {
		t.Fatalf("save incomplete state: %v", err)
	}
	incomplete, completed, err := ReconstructManagedWorkflow(base, manifest, dataDir, resolver)
	if err != nil || completed || incomplete.IsActive {
		t.Fatalf("incomplete reconstruction completed=%v active=%v err=%v", completed, incomplete.IsActive, err)
	}

	if _, err := SaveState(dataDir, manifest, true, time.Now()); err != nil {
		t.Fatalf("save completed state: %v", err)
	}
	bound, completed, err := ReconstructManagedWorkflow(base, manifest, dataDir, resolver)
	if err != nil {
		t.Fatalf("completed reconstruction: %v", err)
	}
	if !completed || !bound.IsActive {
		t.Fatalf("completed workflow was not active: completed=%v active=%v", completed, bound.IsActive)
	}
	if !strings.Contains(bound.NodesJSON, "https://source.example.test/daily.json") || !strings.Contains(bound.NodesJSON, "cred-telegram") {
		t.Fatalf("completed workflow did not contain persisted bindings: %s", bound.NodesJSON)
	}
}

func TestReconstructManagedWorkflowRejectsMissingRequiredCredential(t *testing.T) {
	dataDir := t.TempDir()
	manifest := reconstructionManifest()
	if _, err := SaveConfig(dataDir, manifest, map[string]interface{}{"source_url": "https://source.example.test/daily.json"}); err != nil {
		t.Fatalf("save config: %v", err)
	}
	if _, err := SaveState(dataDir, manifest, true, time.Now()); err != nil {
		t.Fatalf("save completed state: %v", err)
	}
	writeJSONAtomicForTest(t, dataDir, CredentialFileName, CredentialFile{
		PackID:                  manifest.ID,
		CredentialSchemaVersion: CredentialSchemaVersion,
		Slots:                   map[string]CredentialSlot{},
	})

	workflow, completed, err := ReconstructManagedWorkflow(reconstructionWorkflow(), manifest, dataDir, nil)
	if err == nil || !strings.Contains(err.Error(), `credential slot "telegram" is required`) {
		t.Fatalf("expected required credential rejection, got %v", err)
	}
	if completed || workflow.IsActive {
		t.Fatalf("invalid setup must fail closed: completed=%v active=%v", completed, workflow.IsActive)
	}
}

func reconstructionManifest() pack.Manifest {
	return pack.Manifest{
		ID: "example.reconstruction",
		ConfigSchema: []pack.ConfigField{
			{Key: "source_url", Type: "url", Required: true},
		},
		CredentialRequirements: []pack.CredentialRequirement{
			{Key: "telegram", Type: "TELEGRAM_BOT", Required: true},
		},
		Bindings: []pack.Binding{
			{Source: "config.source_url", Target: pack.BindingTarget{NodeID: "fetch", Param: "url"}},
			{Source: "credential.telegram", Target: pack.BindingTarget{NodeID: "send", Param: "credential_id"}},
		},
	}
}

func reconstructionWorkflow() client.Workflow {
	return client.Workflow{
		ID:        "workflow-id",
		Name:      "Reconstruction",
		IsActive:  true,
		NodesJSON: `[{"id":"fetch","type":"httpRequest","params":{"url":"https://default.example.test/data.json"}},{"id":"send","type":"telegramBot","params":{"credential_id":""}}]`,
		EdgesJSON: "[]",
	}
}

func writeJSONAtomicForTest(t *testing.T, dataDir, name string, value interface{}) {
	t.Helper()
	if err := writeJSONAtomic(filepath.Join(dataDir, name), value); err != nil {
		t.Fatalf("write %s: %v", name, err)
	}
}
