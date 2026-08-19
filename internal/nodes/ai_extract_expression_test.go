package nodes

import (
	"encoding/json"
	"testing"
)

func TestAIExtractAcceptsStructuredValueFromExactExpression(t *testing.T) {
	ctx := NewExecutionContext("wf", "exec")
	ctx.SetOutput("http", map[string]interface{}{
		"status_code": float64(200),
		"data": map[string]interface{}{
			"symbol": "XAUUSD",
			"price":  float64(2654.25),
		},
	})

	resolved := ResolveParams(ctx, map[string]interface{}{
		"provider":      "openai",
		"model":         "auto",
		"input_type":    "text",
		"input":         "{{http.data}}",
		"credential_id": "openai-cred",
	})
	if _, ok := resolved["input"].(map[string]interface{}); !ok {
		t.Fatalf("exact expression should preserve the upstream object before AI Extract normalization: %#v", resolved["input"])
	}

	prepared, err := prepareProviderAIExtractNode(&Node{ID: "extract", Params: resolved}, "openai")
	if err != nil {
		t.Fatalf("prepare AI Extract: %v", err)
	}
	serialized, ok := prepared.Params["input"].(string)
	if !ok {
		t.Fatalf("AI Extract should serialize structured text input, got %#v", prepared.Params["input"])
	}
	var decoded map[string]interface{}
	if err := json.Unmarshal([]byte(serialized), &decoded); err != nil {
		t.Fatalf("serialized AI Extract input is not JSON: %v", err)
	}
	if decoded["symbol"] != "XAUUSD" || decoded["price"] != 2654.25 {
		t.Fatalf("unexpected serialized upstream object: %#v", decoded)
	}
}
