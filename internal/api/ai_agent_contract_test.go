package api

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"goflow/internal/nodes"
)

func TestAgentDefinitionsIncludeGroundedRuntimeContracts(t *testing.T) {
	registry := nodes.NewBuiltinRegistry()
	handler := NewAIAgentHandler(nil, registry, nil, nil)

	var encoded bytes.Buffer
	encoder := json.NewEncoder(&encoded)
	encoder.SetEscapeHTML(false)
	if err := encoder.Encode(handler.compactAgentDefinitions()); err != nil {
		t.Fatal(err)
	}
	text := encoded.String()

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
