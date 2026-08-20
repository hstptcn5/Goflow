package packsetup

import (
	"encoding/json"
	"testing"

	"goflow/internal/client"
	"goflow/internal/pack"
)

func TestApplyBindingsLeavesMissingOptionalCredentialUnbound(t *testing.T) {
	workflow := client.Workflow{
		Name:      "optional credential",
		NodesJSON: `[{"id":"ai","type":"openAIGPT","name":"AI","params":{"model":"gpt-4o-mini","prompt":"hello","credential_id":"","api_key":""}}]`,
		EdgesJSON: "[]",
	}
	manifest := pack.Manifest{
		CredentialRequirements: []pack.CredentialRequirement{{
			Key:      "openai",
			Label:    "OpenAI API key",
			Type:     "OPENAI_API_KEY",
			Required: false,
		}},
		Bindings: []pack.Binding{{
			Source: "credential.openai",
			Target: pack.BindingTarget{NodeID: "ai", Param: "credential_id"},
		}},
	}

	bound, err := ApplyBindings(workflow, manifest, map[string]interface{}{}, map[string]CredentialSlot{})
	if err != nil {
		t.Fatalf("optional credential should not block binding: %v", err)
	}
	var nodes []map[string]interface{}
	if err := json.Unmarshal([]byte(bound.NodesJSON), &nodes); err != nil {
		t.Fatalf("decode nodes: %v", err)
	}
	params := nodes[0]["params"].(map[string]interface{})
	if got := params["credential_id"]; got != "" {
		t.Fatalf("expected original empty credential_id to remain, got %#v", got)
	}
}
