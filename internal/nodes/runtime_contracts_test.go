package nodes

import (
	"strings"
	"testing"
)

func definitionByType(t *testing.T, registry *PluginRegistry, nodeType NodeType) NodeDefinition {
	t.Helper()
	for _, def := range registry.ListDefinitions() {
		if def.Type == nodeType {
			return def
		}
	}
	t.Fatalf("definition %s not found", nodeType)
	return NodeDefinition{}
}

func hasParam(def NodeDefinition, name string) bool {
	for _, param := range def.Params {
		if param.Name == name {
			return true
		}
	}
	return false
}

func hasOutput(def NodeDefinition, name string) bool {
	for _, output := range def.Outputs {
		if output.Name == name {
			return true
		}
	}
	return false
}

func hasCapabilityContaining(def NodeDefinition, fragment string) bool {
	for _, capability := range def.Capabilities {
		if strings.Contains(capability, fragment) {
			return true
		}
	}
	return false
}

func TestBuiltinRuntimeContractsGroundPythonWebhookAndSwitch(t *testing.T) {
	registry := NewBuiltinRegistry()

	pythonDef := definitionByType(t, registry, TypePythonCode)
	if pythonDef.OutputReference == nil {
		t.Fatal("python output_reference contract is missing")
	}
	if pythonDef.OutputReference.RootMode != "direct" {
		t.Fatalf("python root mode = %q, want direct", pythonDef.OutputReference.RootMode)
	}
	pythonContract := pythonDef.OutputReference.Description + " " + strings.Join(pythonDef.OutputReference.Examples, " ")
	if !strings.Contains(pythonContract, "{{<node_id>.category}}") {
		t.Fatalf("python contract does not teach direct category reference: %s", pythonContract)
	}
	if !strings.Contains(pythonContract, "not {{<node_id>.output.category}}") {
		t.Fatalf("python contract does not explicitly reject the invented output envelope: %s", pythonContract)
	}
	if strings.Contains(pythonDef.Description, "Runtime contract:") {
		t.Fatalf("runtime hints must not bloat UI description: %s", pythonDef.Description)
	}
	if !hasCapabilityContaining(pythonDef, "output_example:{{<node_id>.category}}") {
		t.Fatalf("python Agent capability hints are missing direct reference example: %#v", pythonDef.Capabilities)
	}

	webhookDef := definitionByType(t, registry, TypeWebhookTrigger)
	if webhookDef.TriggerContract == nil {
		t.Fatal("webhook trigger contract is missing")
	}
	if webhookDef.TriggerContract.EndpointTemplate != "/webhook/{workflow_id}" {
		t.Fatalf("webhook endpoint = %q", webhookDef.TriggerContract.EndpointTemplate)
	}
	if !webhookDef.TriggerContract.RequiresActive {
		t.Fatal("webhook contract must state that the workflow must be active")
	}
	if !hasOutput(webhookDef, "body") || !hasOutput(webhookDef, "query") {
		t.Fatalf("webhook outputs are incomplete: %#v", webhookDef.Outputs)
	}
	if !hasCapabilityContaining(webhookDef, "Do not instruct users to POST directly to the node path parameter") {
		t.Fatalf("webhook Agent hints do not carry invocation warning: %#v", webhookDef.Capabilities)
	}
	if strings.Contains(webhookDef.Description, "/webhook/{workflow_id}") {
		t.Fatalf("webhook runtime endpoint should remain structured/hinted, not appended to UI description: %s", webhookDef.Description)
	}

	switchDef := definitionByType(t, registry, TypeSwitch)
	for _, outputName := range []string{"matched", "matched_handle", "matched_index", "target_handle", "value"} {
		if !hasOutput(switchDef, outputName) {
			t.Fatalf("switch output %q missing", outputName)
		}
	}
}

func TestRegistryGetAndListDefinitionsExposeSameCommonSchema(t *testing.T) {
	registry := NewBuiltinRegistry()

	listed := definitionByType(t, registry, TypePythonCode)
	executor, ok := registry.Get(TypePythonCode)
	if !ok {
		t.Fatal("python executor missing")
	}
	got := executor.GetDefinition()

	if !hasParam(listed, "on_error") {
		t.Fatal("listed python definition should expose common on_error policy")
	}
	if !hasParam(got, "on_error") {
		t.Fatal("registry.Get definition must expose the same on_error policy used by validation")
	}
	if got.OutputReference == nil || listed.OutputReference == nil || got.OutputReference.Pattern != listed.OutputReference.Pattern {
		t.Fatalf("runtime contracts differ between Get and ListDefinitions: get=%#v list=%#v", got.OutputReference, listed.OutputReference)
	}
}

func TestPluginDeclaredOutputsReceiveDirectReferenceContract(t *testing.T) {
	def := NodeDefinition{
		Type:        "custom.example",
		Name:        "Example",
		Description: "Example custom node",
		Outputs: []PluginOutputDefinition{
			{Name: "result", Type: "string"},
		},
	}

	grounded := DefinitionWithRuntimeContract(def)
	if grounded.OutputReference == nil {
		t.Fatal("custom node with declared outputs should receive a default direct output contract")
	}
	if grounded.OutputReference.RootMode != "direct" {
		t.Fatalf("custom node output root mode = %q", grounded.OutputReference.RootMode)
	}
	if !hasCapabilityContaining(grounded, "output_field:result:string") {
		t.Fatalf("custom output field should be present in Agent capability hints: %#v", grounded.Capabilities)
	}
}
