package engine

import (
	"encoding/json"
	"fmt"
	"testing"

	"goflow/internal/crypto"
	"goflow/internal/nodes"
	"goflow/internal/storage"
)

type credentialMetadataProbe struct{}

func (p *credentialMetadataProbe) Execute(ctx *nodes.ExecutionContext, node *nodes.Node) (interface{}, error) {
	credentialID, _ := node.Params["credential_id"].(string)
	if ctx.Credentials[credentialID] != "provider-secret" {
		return nil, fmt.Errorf("credential secret was not hydrated")
	}
	metadata, ok := ctx.CredentialMetadata[credentialID]
	if !ok {
		return nil, fmt.Errorf("credential metadata was not hydrated")
	}
	if metadata.Kind != "API_KEY" || metadata.Provider != "deepseek" {
		return nil, fmt.Errorf("unexpected credential metadata: %#v", metadata)
	}
	return map[string]interface{}{"provider": metadata.Provider}, nil
}

func (p *credentialMetadataProbe) Validate(node *nodes.Node) error { return nil }
func (p *credentialMetadataProbe) GetDefinition() nodes.NodeDefinition {
	return nodes.NodeDefinition{Type: "credentialMetadataProbe", Retryable: false}
}

func TestEngineHydratesCredentialMetadataForReferencedCredential(t *testing.T) {
	db, err := storage.NewDB(":memory:")
	if err != nil {
		t.Fatalf("NewDB: %v", err)
	}
	defer db.Close()

	credentialStore := storage.NewCredentialStore(db, crypto.NewCryptoManager("credential-metadata-test-key"))
	credential, err := credentialStore.CreateWithMetadata("DeepSeek", "API_KEY", "deepseek", "provider-secret")
	if err != nil {
		t.Fatalf("CreateWithMetadata: %v", err)
	}

	registry := nodes.NewPluginRegistry()
	if err := registry.Register(&credentialMetadataProbe{}); err != nil {
		t.Fatalf("register probe: %v", err)
	}
	workflowStore := storage.NewWorkflowStore(db)
	engine := NewEngine(registry, storage.NewExecutionStore(db), credentialStore, NewEventBus(), workflowStore)

	nodeList := []nodes.Node{{
		ID:   "probe",
		Type: "credentialMetadataProbe",
		Params: map[string]interface{}{
			"credential_id": credential.ID,
		},
	}}
	nodesJSON, _ := json.Marshal(nodeList)
	workflow := &storage.Workflow{
		ID:        "wf-credential-metadata",
		Name:      "Credential metadata",
		NodesJSON: string(nodesJSON),
		EdgesJSON: "[]",
	}
	if err := workflowStore.Create(workflow); err != nil {
		t.Fatalf("create workflow: %v", err)
	}

	execution, err := engine.ExecuteWorkflow(workflow, nil)
	if err != nil {
		t.Fatalf("ExecuteWorkflow: %v", err)
	}
	if execution.Status != "SUCCESS" {
		t.Fatalf("execution status = %s, error = %s", execution.Status, execution.ErrorMessage)
	}
}
