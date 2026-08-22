package api

import (
	"strings"
	"testing"

	"goflow/internal/nodes"
)

func compactDefinitionByType(t *testing.T, defs []map[string]interface{}, nodeType nodes.NodeType) map[string]interface{} {
	t.Helper()
	for _, def := range defs {
		if def["type"] == nodeType {
			return def
		}
	}
	t.Fatalf("compact Agent definition %s not found", nodeType)
	return nil
}

func TestAgentDefinitionsIncludeGroundedRuntimeContracts(t *testing.T) {
	registry := nodes.NewBuiltinRegistry()
	handler := NewAIAgentHandler(nil, registry, nil, nil)
	defs := handler.compactAgentDefinitions()

	pythonDef := compactDefinitionByType(t, defs, nodes.TypePythonCode)
	pythonRef, ok := pythonDef["output_reference"].(*nodes.OutputReferenceContract)
	if !ok || pythonRef == nil {
		t.Fatalf("python output_reference missing or wrong type: %#v", pythonDef["output_reference"])
	}
	if pythonRef.RootMode != "direct" || !pythonRef.DynamicFields {
		t.Fatalf("python output reference = %#v", pythonRef)
	}
	pythonGuidance := pythonRef.Description + " " + strings.Join(pythonRef.Examples, " ")
	if !strings.Contains(pythonGuidance, "{{<node_id>.category}}") {
		t.Fatalf("python contract is missing direct-root example: %s", pythonGuidance)
	}
	if !strings.Contains(pythonGuidance, "{{<node_id>.output.category}}") {
		t.Fatalf("python contract is missing anti-envelope guidance: %s", pythonGuidance)
	}

	webhookDef := compactDefinitionByType(t, defs, nodes.TypeWebhookTrigger)
	webhookTrigger, ok := webhookDef["trigger_contract"].(*nodes.TriggerContract)
	if !ok || webhookTrigger == nil {
		t.Fatalf("webhook trigger_contract missing or wrong type: %#v", webhookDef["trigger_contract"])
	}
	if webhookTrigger.EndpointTemplate != "/webhook/{workflow_id}" || !webhookTrigger.RequiresActive {
		t.Fatalf("webhook trigger contract = %#v", webhookTrigger)
	}
	if !strings.Contains(strings.Join(webhookTrigger.Notes, " "), "Do not instruct users to POST directly to the node path parameter") {
		t.Fatalf("webhook invocation warning missing: %#v", webhookTrigger.Notes)
	}

	outputs, ok := webhookDef["outputs"].([]nodes.PluginOutputDefinition)
	if !ok || len(outputs) == 0 {
		t.Fatalf("webhook outputs missing from Agent compact definition: %#v", webhookDef["outputs"])
	}
	foundBody := false
	for _, output := range outputs {
		if output.Name == "body" {
			foundBody = true
			break
		}
	}
	if !foundBody {
		t.Fatalf("webhook body output not grounded: %#v", outputs)
	}
}
