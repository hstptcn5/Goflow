package appbuilder

import (
	"strings"
	"testing"
)

func TestAnalyzePortabilityLevels(t *testing.T) {
	green, err := Analyze(`[{"id":"start","type":"manualTrigger","name":"Start","params":{}},{"id":"map","type":"jsonTransform","name":"Map","params":{"mapping":"{}"}}]`)
	if err != nil || green.Level != Green || !green.CanBuild {
		t.Fatalf("expected green report, got %#v, %v", green, err)
	}
	yellow, err := Analyze(`[{"id":"ai","type":"deepseekAI","name":"DeepSeek","params":{}}]`)
	if err != nil || yellow.Level != Yellow || !yellow.CanBuild || len(yellow.Warnings) != 1 {
		t.Fatalf("expected yellow report, got %#v, %v", yellow, err)
	}
	python, err := Analyze(`[{"id":"python","type":"pythonCode","name":"Python","params":{}}]`)
	if err != nil || python.Level != Yellow || !python.CanBuild || len(python.Warnings) != 1 || !strings.Contains(python.Warnings[0], "Python 3") {
		t.Fatalf("expected buildable yellow Python report, got %#v, %v", python, err)
	}
	red, err := Analyze(`[{"id":"file","type":"localFile","name":"Local file","params":{}}]`)
	if err != nil || red.Level != Red || red.CanBuild || len(red.Blockers) != 1 {
		t.Fatalf("expected red report, got %#v, %v", red, err)
	}
}

func TestExternalizeCredentials(t *testing.T) {
	raw, requirements, bindings, err := ExternalizeCredentials(`[{"id":"deepseek-main","type":"deepseekAI","name":"Phân tích","params":{"credential_id":"machine-id","api_key":"must-not-ship"}}]`)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(raw, "machine-id") || strings.Contains(raw, "must-not-ship") {
		t.Fatalf("portable workflow retained local credential material: %s", raw)
	}
	if len(requirements) != 1 || requirements[0].Type != "DEEPSEEK_API_KEY" || len(bindings) != 1 || bindings[0].Target.NodeID != "deepseek-main" {
		t.Fatalf("unexpected setup metadata: %#v %#v", requirements, bindings)
	}
}

func TestExternalizeCredentialsCreatesMissingAIExtractSlot(t *testing.T) {
	raw, requirements, bindings, err := ExternalizeCredentials(`[{"id":"extract","type":"aiExtract","name":"AI Extract","params":{"provider":"deepseek","model":"auto","input_type":"text","input":"hello","instructions":"extract","json_schema":"{}","schema_name":"result","max_media_bytes":1024}}]`)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(raw, `"credential_id":"__goflow_setup_extract_credential"`) {
		t.Fatalf("portable workflow did not receive a setup credential marker: %s", raw)
	}
	if len(requirements) != 1 || requirements[0].Type != "DEEPSEEK_API_KEY" || len(bindings) != 1 || bindings[0].Target.NodeID != "extract" || bindings[0].Target.Param != "credential_id" {
		t.Fatalf("unexpected AI Extract setup metadata: %#v %#v", requirements, bindings)
	}
}

func TestExternalizeCredentialsSkipsUnauthenticatedHTTP(t *testing.T) {
	raw, requirements, bindings, err := ExternalizeCredentials(`[{"id":"fetch","type":"httpRequest","name":"Fetch","params":{"auth_mode":"none","credential_id":""}}]`)
	if err != nil || len(requirements) != 0 || len(bindings) != 0 || !strings.Contains(raw, `"credential_id":""`) {
		t.Fatalf("unexpected unauthenticated HTTP externalization: %s %#v %#v %v", raw, requirements, bindings, err)
	}
}

func TestExternalizeCredentialsPreservesProviderMetadata(t *testing.T) {
	_, requirements, _, err := ExternalizeCredentials(`[{"id":"notify","type":"slackBot","name":"Slack","params":{"credential_id":"old"}}]`)
	if err != nil || len(requirements) != 1 || requirements[0].Kind != "API_KEY" || requirements[0].Provider != "slack" {
		t.Fatalf("unexpected provider metadata: %#v %v", requirements, err)
	}
}
