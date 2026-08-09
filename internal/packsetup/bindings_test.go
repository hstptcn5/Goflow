package packsetup

import (
	"encoding/json"
	"strings"
	"testing"

	"goflow/internal/client"
	"goflow/internal/nodes"
	"goflow/internal/pack"
)

func TestApplyBindingsClonesWorkflowAndBindsSetupValues(t *testing.T) {
	workflow := testBindingWorkflow(t)
	manifest := pack.Manifest{
		ID: "example.bindings",
		ConfigSchema: []pack.ConfigField{
			{Key: "source_url", Label: "Source URL", Type: "url", Required: true},
			{Key: "chat_id", Label: "Chat ID", Type: "string", Required: true},
		},
		CredentialRequirements: []pack.CredentialRequirement{
			{Key: "telegram", Label: "Telegram bot", Type: "TELEGRAM_BOT", Required: true},
		},
		Bindings: []pack.Binding{
			{Source: "config.source_url", Target: pack.BindingTarget{NodeID: "fetch", Param: "url"}},
			{Source: "config.chat_id", Target: pack.BindingTarget{NodeID: "send", Param: "chat_id"}},
			{Source: "credential.telegram", Target: pack.BindingTarget{NodeID: "send", Param: "credential_id"}},
		},
	}
	bound, err := ApplyBindings(workflow, manifest, map[string]interface{}{
		"source_url": "https://example.test/sales.json",
		"chat_id":    "12345",
	}, map[string]CredentialSlot{
		"telegram": {CredentialID: "cred-telegram", CredentialType: "TELEGRAM_BOT"},
	})
	if err != nil {
		t.Fatalf("ApplyBindings failed: %v", err)
	}
	if bound.NodesJSON == workflow.NodesJSON {
		t.Fatalf("expected cloned workflow nodes to change")
	}
	if strings.Contains(workflow.NodesJSON, "cred-telegram") {
		t.Fatalf("source workflow was mutated: %s", workflow.NodesJSON)
	}
	var nodeList []nodes.Node
	if err := json.Unmarshal([]byte(bound.NodesJSON), &nodeList); err != nil {
		t.Fatalf("unmarshal bound nodes: %v", err)
	}
	params := paramsByNode(nodeList)
	if params["fetch"]["url"] != "https://example.test/sales.json" {
		t.Fatalf("source_url was not bound: %#v", params["fetch"])
	}
	if params["send"]["chat_id"] != "12345" || params["send"]["credential_id"] != "cred-telegram" {
		t.Fatalf("telegram params were not bound: %#v", params["send"])
	}
}

func TestApplyBindingsRejectsMissingSetupSources(t *testing.T) {
	tests := []struct {
		name        string
		binding     pack.Binding
		config      map[string]interface{}
		credentials map[string]CredentialSlot
		want        string
	}{
		{
			name:    "missing config",
			binding: pack.Binding{Source: "config.source_url", Target: pack.BindingTarget{NodeID: "fetch", Param: "url"}},
			want:    "config source",
		},
		{
			name:    "missing credential",
			binding: pack.Binding{Source: "credential.telegram", Target: pack.BindingTarget{NodeID: "send", Param: "credential_id"}},
			want:    "credential source",
		},
		{
			name:    "missing node",
			binding: pack.Binding{Source: "config.source_url", Target: pack.BindingTarget{NodeID: "missing", Param: "url"}},
			config:  map[string]interface{}{"source_url": "https://example.test"},
			want:    "does not exist",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ApplyBindings(testBindingWorkflow(t), pack.Manifest{Bindings: []pack.Binding{tt.binding}}, tt.config, tt.credentials)
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("expected %q error, got %v", tt.want, err)
			}
		})
	}
}

func testBindingWorkflow(t *testing.T) client.Workflow {
	t.Helper()
	nodeList := []nodes.Node{
		{ID: "fetch", Type: nodes.TypeHTTPRequest, Name: "Fetch", Params: map[string]interface{}{"method": "GET", "url": "https://placeholder.test", "headers": "{}", "body": ""}},
		{ID: "send", Type: nodes.TypeTelegramBot, Name: "Send", Params: map[string]interface{}{"credential_id": "", "chat_id": "", "message": "Report"}},
	}
	nodesJSON, err := json.Marshal(nodeList)
	if err != nil {
		t.Fatalf("marshal nodes: %v", err)
	}
	return client.Workflow{Name: "Binding test", NodesJSON: string(nodesJSON), EdgesJSON: "[]"}
}

func paramsByNode(nodeList []nodes.Node) map[string]map[string]interface{} {
	result := map[string]map[string]interface{}{}
	for _, node := range nodeList {
		result[node.ID] = node.Params
	}
	return result
}
