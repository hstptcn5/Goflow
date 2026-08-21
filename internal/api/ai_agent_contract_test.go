package api

import (
	"encoding/json"
	"strings"
	"testing"

	"goflow/internal/nodes"
)

func TestAgentDefinitionsIncludeGroundedRuntimeContracts(t *testing.T) {
	registry := nodes.NewBuiltinRegistry()
	handler := NewAIAgentHandler(nil, registry, nil, nil)

	encoded, err := json.Marshal(handler.compactAgentDefinitions())
	if err != nil {
		t.Fatal(err)
	}
	text := string(encoded)

	for _, expected := range []string{
		"{{<node_id>.category}}",
		"not {{<node_id>.output.category}}",
		"/webhook/{workflow_id}",
		"Do not instruct users to POST directly to the node path parameter",
		"output_reference:direct",
	} {
		if !strings.Contains(text, expected) {
			t.Fatalf("Agent Lab compact node definitions are missing %q: %s", expected, text)
		}
	}
}
