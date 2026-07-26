package engine

import "testing"

func TestRedactSensitiveOutput(t *testing.T) {
	input := map[string]interface{}{
		"status":      "sent",
		"webhook_url": "https://discord.com/api/webhooks/123/secret-token",
		"headers": map[string]interface{}{
			"Authorization": "Bearer very-secret-token",
			"Content-Type":  "application/json",
		},
		"nested": []interface{}{
			map[string]interface{}{
				"api_key": "sk-test-secret-1234567890",
			},
			"posted to https://hooks.slack.com/services/T000/B000/SECRET",
		},
	}

	got := redactSensitive(input).(map[string]interface{})
	if got["webhook_url"] != redactedValue {
		t.Fatalf("expected webhook_url to be redacted, got %#v", got["webhook_url"])
	}

	headers := got["headers"].(map[string]interface{})
	if headers["Authorization"] != redactedValue {
		t.Fatalf("expected Authorization to be redacted, got %#v", headers["Authorization"])
	}
	if headers["Content-Type"] != "application/json" {
		t.Fatalf("expected non-sensitive header to remain, got %#v", headers["Content-Type"])
	}

	nested := got["nested"].([]interface{})
	first := nested[0].(map[string]interface{})
	if first["api_key"] != redactedValue {
		t.Fatalf("expected api_key to be redacted, got %#v", first["api_key"])
	}
	if nested[1] == input["nested"].([]interface{})[1] {
		t.Fatalf("expected Slack webhook URL inside string to be redacted")
	}
}

func TestRedactSensitiveString(t *testing.T) {
	got := redactSensitiveString("request failed with Authorization: Bearer abc.def.ghi and key sk-test-secret-1234567890")
	if got == "" || got == "request failed with Authorization: Bearer abc.def.ghi and key sk-test-secret-1234567890" {
		t.Fatalf("expected sensitive string to be redacted, got %q", got)
	}
}

func TestRedactSensitiveHandlesMapCycle(t *testing.T) {
	input := map[string]interface{}{"name": "root"}
	child := map[string]interface{}{"parent": input}
	input["child"] = child

	got := redactSensitive(input).(map[string]interface{})
	redactedChild := got["child"].(map[string]interface{})
	if redactedChild["parent"] != redactionCycleValue {
		t.Fatalf("expected cycle marker, got %#v", redactedChild["parent"])
	}
}

func TestRedactSensitiveHandlesSliceCycle(t *testing.T) {
	input := []interface{}{nil}
	input[0] = input

	got := redactSensitive(input).([]interface{})
	if got[0] != redactionCycleValue {
		t.Fatalf("expected cycle marker, got %#v", got[0])
	}
}

func TestRedactSensitiveDepthLimit(t *testing.T) {
	var value interface{} = "leaf"
	for i := 0; i < maxRedactionDepth+2; i++ {
		value = map[string]interface{}{"next": value}
	}

	got := redactSensitive(value)
	current := got
	for i := 0; i < maxRedactionDepth+1; i++ {
		m, ok := current.(map[string]interface{})
		if !ok {
			if current == redactionDepthValue {
				return
			}
			t.Fatalf("unexpected value before depth marker: %#v", current)
		}
		current = m["next"]
	}
	if current != redactionDepthValue {
		t.Fatalf("expected depth marker, got %#v", current)
	}
}
