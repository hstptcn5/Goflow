package nodes

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"strings"
	"testing"
)

func TestGithubWebhookExecutor(t *testing.T) {
	executor := NewGithubWebhookExecutor()
	ctx := NewExecutionContext("wf-1", "exec-1")

	// Test 1: Missing trigger data
	_, err := executor.Execute(ctx, &Node{})
	if err == nil || !strings.Contains(err.Error(), "requires a trigger payload") {
		t.Errorf("Expected missing trigger payload error, got: %v", err)
	}

	// Test 2: Valid signature check
	secret := "my-secret-key"
	bodyRaw := `{"ref": "refs/heads/main", "commits": []}`
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(bodyRaw))
	sig := "sha256=" + hex.EncodeToString(mac.Sum(nil))

	ctx.SetOutput("$trigger", map[string]interface{}{
		"body_raw": bodyRaw,
		"headers": map[string]interface{}{
			"X-Hub-Signature-256": sig,
		},
	})

	node := &Node{
		Params: map[string]interface{}{
			"secret": secret,
		},
	}

	res, err := executor.Execute(ctx, node)
	if err != nil {
		t.Fatalf("Expected signature check to succeed, got error: %v", err)
	}

	resMap, ok := res.(map[string]interface{})
	if !ok || resMap["body_raw"] != bodyRaw {
		t.Errorf("Expected result to match input map structure, got: %v", res)
	}
}
