package nodes

import (
	"context"
	"strings"
	"testing"
)

func TestBuiltinRegistryContainsEveryPublicNodeType(t *testing.T) {
	registry := NewBuiltinRegistry()
	expected := []NodeType{
		TypeWebhookTrigger, TypeCronTrigger, TypeManualTrigger, TypeGithubWebhook,
		TypeHTTPRequest, TypeNormalizedHTTPSource, TypeRSSFeedSource, TypeSourcePolicy,
		TypeTelegramBot, TypeZaloOA, TypeDiscordBot, TypeSlackBot, TypeEmailSMTP,
		TypeGoogleSheets, TypeGoogleDrive, TypeGmailREST, TypeNotionPage,
		TypeJSONTransform, TypeConditionIF, TypeDelaySleep, TypeJSCodeRunner, TypeSubWorkflow,
		TypeOpenAIGPT, TypeDeepSeekAI, TypeAIExtract,
		TypePostgresQuery, TypeMySQLQuery, TypeMongoDBCommand, TypeRedisCommand,
		TypeSSHRunner, TypeGitCommand, TypeGoflowPlugin,
	}
	for _, nodeType := range expected {
		if _, ok := registry.Get(nodeType); !ok {
			t.Fatalf("built-in registry is missing node type %q", nodeType)
		}
	}
}

func TestRegistryValidatesResolvedParamsBeforeExecute(t *testing.T) {
	registry := NewBuiltinRegistry()
	executor, ok := registry.Get(TypeConditionIF)
	if !ok {
		t.Fatal("condition executor missing")
	}
	_, err := executor.Execute(NewExecutionContext("wf", "exec"), &Node{Type: TypeConditionIF, Params: map[string]interface{}{
		"field": "x", "operator": "unsupported", "value": "x",
	}})
	if err == nil || !strings.Contains(err.Error(), "invalid conditionIf node configuration") {
		t.Fatalf("expected registry validation failure, got %v", err)
	}
}

func TestManualTriggerIsExecutableAndReturnsTriggerPayload(t *testing.T) {
	registry := NewBuiltinRegistry()
	executor, ok := registry.Get(TypeManualTrigger)
	if !ok {
		t.Fatal("manual trigger executor missing")
	}
	ctx := NewExecutionContext("wf", "exec")
	ctx.SetOutput("$trigger", map[string]interface{}{"hello": "world"})
	result, err := executor.Execute(ctx, &Node{Type: TypeManualTrigger, Params: map[string]interface{}{}})
	if err != nil {
		t.Fatalf("manual trigger execute failed: %v", err)
	}
	payload, ok := result.(map[string]interface{})
	if !ok || payload["hello"] != "world" {
		t.Fatalf("unexpected manual payload: %#v", result)
	}
}

func TestResolveNodeCredentialFailsClosedWhenSelectedCredentialIsMissing(t *testing.T) {
	ctx := NewExecutionContext("wf", "exec")
	node := &Node{Params: map[string]interface{}{
		"credential_id": "missing", "api_key": "plaintext-fallback",
	}}
	secret, err := resolveNodeCredential(ctx, node, "api_key", "test credential")
	if err == nil {
		t.Fatalf("expected selected missing credential to fail closed, got secret %q", secret)
	}
	if secret != "" {
		t.Fatalf("selected credential unexpectedly fell back to plaintext: %q", secret)
	}
}

func TestJSONTransformAcceptsStructuredResolvedObject(t *testing.T) {
	executor := NewJSONTransformExecutor()
	ctx := NewExecutionContext("wf", "exec")
	result, err := executor.Execute(ctx, &Node{Params: map[string]interface{}{
		"json_template": map[string]interface{}{"price": 123.0},
	}})
	if err != nil {
		t.Fatalf("structured JSON transform failed: %v", err)
	}
	out := result.(map[string]interface{})["transformed"].(map[string]interface{})
	if out["price"] != 123.0 {
		t.Fatalf("unexpected transform output: %#v", out)
	}
}

func TestMongoDesignValidationAllowsUnresolvedJSONExpression(t *testing.T) {
	executor := NewMongoDBCommandExecutor()
	node := &Node{Params: map[string]interface{}{
		"database":      "app",
		"collection":    "events",
		"command":       "UPDATE_ONE",
		"filter_json":   `{{source.data.filter}}`,
		"document_json": `{{source.data.update}}`,
	}}
	if err := executor.Validate(node); err != nil {
		t.Fatalf("unresolved MongoDB JSON expressions should be valid at design time: %v", err)
	}
}

func TestConditionRejectsUnknownOperator(t *testing.T) {
	executor := NewConditionIFExecutor()
	if err := executor.Validate(&Node{Params: map[string]interface{}{"operator": "typo"}}); err == nil {
		t.Fatal("expected invalid IF operator to be rejected")
	}
}

func TestSubWorkflowRejectsExcessiveConcurrency(t *testing.T) {
	_, err := parseSubWorkflowConcurrency(maxSubWorkflowConcurrency + 1)
	if err == nil {
		t.Fatal("expected excessive sub-workflow concurrency to fail")
	}
}

func TestJSRunnerRejectsExcessiveTimeout(t *testing.T) {
	_, err := parseJSTimeout(maxJSTimeoutSeconds + 1)
	if err == nil {
		t.Fatal("expected excessive JavaScript timeout to fail")
	}
}

func TestRedisDBParserDoesNotAcceptNumericPrefix(t *testing.T) {
	if _, err := parseRedisDB("1junk"); err == nil {
		t.Fatal("expected malformed Redis DB index to fail")
	}
}

func TestGoogleSheetsRejectsMalformedValuesInsteadOfStringFallback(t *testing.T) {
	if _, err := parseSheetsValues("not-json"); err == nil {
		t.Fatal("expected malformed values_json to fail")
	}
}

func TestGmailRejectsHeaderInjection(t *testing.T) {
	node := &Node{Params: map[string]interface{}{
		"credential_id": "cred", "to": "victim@example.com\r\nBcc: attacker@example.com", "subject": "hello", "body": "body",
	}}
	if err := validateGmailNode(node); err == nil {
		t.Fatal("expected Gmail CRLF header injection to fail")
	}
}

func TestWebhookURLsAreProviderBound(t *testing.T) {
	if err := validateSlackWebhookURL("https://example.com/services/T/B/X"); err == nil {
		t.Fatal("expected non-Slack webhook host to fail")
	}
	if err := validateDiscordWebhookURL("https://example.com/api/webhooks/1/token"); err == nil {
		t.Fatal("expected non-Discord webhook host to fail")
	}
}

func TestSSHRequiresHostKeyFingerprint(t *testing.T) {
	node := &Node{Params: map[string]interface{}{
		"address": "example.com:22", "username": "user", "password": "secret", "command": "uptime", "timeout": "30",
	}}
	if err := validateSSHNode(node); err == nil || !strings.Contains(err.Error(), "host_key_sha256") {
		t.Fatalf("expected missing SSH fingerprint error, got %v", err)
	}
}

func TestGitRejectsCredentialInRepositoryURL(t *testing.T) {
	if err := validateGitRepositoryURL("https://user:secret@example.com/repo.git"); err == nil {
		t.Fatal("expected embedded Git credential URL to fail")
	}
}

func TestZaloWorkflowEndpointCannotExfiltrateToken(t *testing.T) {
	node := &Node{Params: map[string]interface{}{
		"access_token": "secret", "user_id": "123", "message": "hello", "endpoint": "https://evil.example/steal",
	}}
	_, _, _, _, err := zaloOARequestParams(nil, node, defaultZaloOAMessageEndpoint)
	if err == nil {
		t.Fatal("expected non-Zalo endpoint override to fail")
	}
}

func TestCronRejectsInvalidExpression(t *testing.T) {
	executor := NewCronTriggerExecutor()
	if err := executor.Validate(&Node{Params: map[string]interface{}{"cron_expression": "not a cron"}}); err == nil {
		t.Fatal("expected invalid cron expression to fail")
	}
}

func TestPluginPathRejectsTraversal(t *testing.T) {
	if _, err := resolvePluginPath("plugins", "../evil"); err == nil {
		t.Fatal("expected plugin path traversal to fail")
	}
}

func TestBoundedBufferDiscardsExcessWithoutBlockingWriter(t *testing.T) {
	buf := newBoundedBuffer(4)
	if n, err := buf.Write([]byte("123456")); err != nil || n != 6 {
		t.Fatalf("unexpected bounded buffer write result n=%d err=%v", n, err)
	}
	if buf.String() != "1234" || !buf.Exceeded() {
		t.Fatalf("unexpected bounded buffer state: %q exceeded=%v", buf.String(), buf.Exceeded())
	}
}

func TestCancelledContextReachesJSRunner(t *testing.T) {
	executor := NewJSCodeRunnerExecutor()
	parent, cancel := context.WithCancel(context.Background())
	cancel()
	ctx := NewExecutionContextWithContext(parent, "wf", "exec")
	_, err := executor.Execute(ctx, &Node{Params: map[string]interface{}{
		"code": "while (true) {}", "timeout": "30",
	}})
	if err == nil {
		t.Fatal("expected cancelled JavaScript execution to fail")
	}
}
