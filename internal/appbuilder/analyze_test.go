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
	red, err := Analyze(`[{"id":"python","type":"pythonCode","name":"Python","params":{}}]`)
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
