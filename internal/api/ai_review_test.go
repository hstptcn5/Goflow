package api

import (
	"encoding/json"
	"strings"
	"testing"

	"goflow/internal/nodes"
	"goflow/internal/storage"
)

func TestStrictAIReviewProviderRequiresExplicitProviderMetadata(t *testing.T) {
	tests := []struct {
		name     string
		cred     *storage.Credential
		wantOK   bool
		provider string
	}{
		{name: "openai metadata", cred: &storage.Credential{Kind: "API_KEY", Provider: "openai", Type: "API_KEY"}, wantOK: true, provider: "openai"},
		{name: "deepseek metadata", cred: &storage.Credential{Kind: "API_KEY", Provider: "deepseek", Type: "API_KEY"}, wantOK: true, provider: "deepseek"},
		{name: "legacy openai", cred: &storage.Credential{Type: "OpenAI"}, wantOK: true, provider: "openai"},
		{name: "legacy deepseek", cred: &storage.Credential{Type: "DeepSeek"}, wantOK: true, provider: "deepseek"},
		{name: "generic key rejected", cred: &storage.Credential{Kind: "API_KEY", Provider: "custom", Type: "API_KEY"}, wantOK: false},
		{name: "bearer rejected", cred: &storage.Credential{Kind: "BEARER_TOKEN", Provider: "openai", Type: "BEARER_TOKEN"}, wantOK: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, _, provider, ok := strictAIReviewProvider(tt.cred)
			if ok != tt.wantOK {
				t.Fatalf("ok = %v, want %v", ok, tt.wantOK)
			}
			if ok && provider != tt.provider {
				t.Fatalf("provider = %q, want %q", provider, tt.provider)
			}
		})
	}
}

func TestBoundedReviewJSONRedactsSensitiveValues(t *testing.T) {
	input := map[string]interface{}{
		"credential_id": "cred-real-id",
		"params": map[string]interface{}{
			"api_key": "sk-this-should-never-leave-goflow",
			"headers": map[string]interface{}{
				"Authorization": "Bearer secret-token-value",
			},
			"url": "https://example.com/data",
		},
	}
	encoded := boundedReviewJSON(input)
	for _, secret := range []string{"cred-real-id", "sk-this-should-never-leave-goflow", "secret-token-value"} {
		if strings.Contains(encoded, secret) {
			t.Fatalf("review context leaked %q: %s", secret, encoded)
		}
	}
	if !strings.Contains(encoded, "[REDACTED]") {
		t.Fatalf("expected redaction marker, got %s", encoded)
	}
}

func TestBuildAIReviewMessagesRedactsHumanFocusAndIncludesAuthoritativeFacts(t *testing.T) {
	handler := newTestAIHandler(t)
	messages := handler.buildAIReviewMessages(aiReviewRequest{
		Mode:     "workflow",
		Focus:    "Optimize this output; api_key=super-secret-review-focus",
		Workflow: validReviewerWorkflow(),
	})
	encoded, _ := json.Marshal(messages)
	text := string(encoded)
	if strings.Contains(text, "super-secret-review-focus") {
		t.Fatalf("review focus leaked a secret: %s", text)
	}
	if !strings.Contains(text, "[REDACTED]") {
		t.Fatalf("expected redaction marker in reviewer focus: %s", text)
	}
	if !strings.Contains(text, "status_code, headers và data") {
		t.Fatalf("expected authoritative HTTP output contract in reviewer context: %s", text)
	}
	if !strings.Contains(text, "TẤT CẢ text dành cho người dùng phải viết bằng tiếng Việt") {
		t.Fatalf("expected Vietnamese response requirement: %s", text)
	}
}

func validReviewerWorkflow() workflowDraft {
	return workflowDraft{
		Name: "Review target",
		Nodes: []nodes.Node{
			{ID: "webhook_1", Type: nodes.TypeWebhookTrigger, Name: "Webhook", Params: map[string]interface{}{}},
			{ID: "http_1", Type: nodes.TypeHTTPRequest, Name: "HTTP", Params: map[string]interface{}{
				"method": "GET",
				"url":    "https://example.com",
			}},
		},
		Edges: []nodes.Edge{{ID: "edge_1", Source: "webhook_1", Target: "http_1"}},
	}
}

func score(t *testing.T, result aiReviewResult, key string) int {
	t.Helper()
	value, ok := result.Scores[key]
	if !ok || value == nil {
		t.Fatalf("score %q is unavailable: %#v", key, result.Scores)
	}
	return *value
}

func TestParseAIReviewResultClampsScoresUsesNAAndKeepsValidatedProposal(t *testing.T) {
	handler := newTestAIHandler(t)
	proposal := validReviewerWorkflow()
	proposal.Name = "Improved"
	payload := map[string]interface{}{
		"summary": "Workflow cần một vài cải thiện.",
		"scores": map[string]interface{}{
			"reliability":      120,
			"security":         -4,
			"data_correctness": 80,
			"output_quality":   77,
		},
		"findings": []map[string]interface{}{
			{"severity": "urgent", "category": "reliability", "title": "Kiểm tra", "why": "Lý do", "impact": "Tác động", "suggested_change": "Thay đổi", "evidence": "node http_1", "confidence": 120},
		},
		"proposal_summary":  "Giữ graph hợp lệ.",
		"proposed_workflow": proposal,
	}
	raw, _ := json.Marshal(payload)
	result, err := handler.parseAIReviewResult(string(raw), aiReviewRequest{Mode: "workflow", Workflow: validReviewerWorkflow()}, "openai", "gpt-4o")
	if err != nil {
		t.Fatalf("parse review: %v", err)
	}
	if score(t, result, "reliability") != 100 || score(t, result, "security") != 0 {
		t.Fatalf("scores were not clamped: %#v", result.Scores)
	}
	if result.Scores["output_quality"] != nil {
		t.Fatalf("workflow-only output quality must be unavailable, got %#v", result.Scores["output_quality"])
	}
	if len(result.Findings) != 1 || result.Findings[0].Severity != "medium" || result.Findings[0].Confidence != 100 {
		t.Fatalf("finding was not normalized: %#v", result.Findings)
	}
	if result.ProposedWorkflow == nil || !result.ProposalValidated {
		t.Fatalf("valid proposal should remain available: %#v", result)
	}
}

func TestParseAIReviewResultDropsInvalidProposalButKeepsFindings(t *testing.T) {
	handler := newTestAIHandler(t)
	payload := `{
		"summary":"Có một vấn đề cần sửa.",
		"scores":{"reliability":60},
		"findings":[{"id":"f1","severity":"high","category":"reliability","title":"Proposal không hợp lệ","why":"Patch sai","impact":"Không thể áp dụng an toàn","suggested_change":"Kiểm tra thủ công","evidence":"node bad","confidence":95}],
		"proposed_workflow":{"name":"Broken","nodes":[{"id":"bad","type":"missingNode","name":"Bad","params":{}}],"edges":[]}
	}`
	result, err := handler.parseAIReviewResult(payload, aiReviewRequest{Mode: "workflow", Workflow: validReviewerWorkflow()}, "deepseek", "deepseek-v4-flash")
	if err != nil {
		t.Fatalf("parse review: %v", err)
	}
	if len(result.Findings) != 1 {
		t.Fatalf("findings should still be returned: %#v", result.Findings)
	}
	if result.ProposedWorkflow != nil || result.ProposalValidated {
		t.Fatalf("invalid proposal must not be exposed for apply: %#v", result.ProposedWorkflow)
	}
	if len(result.ProposalValidationIssues) == 0 {
		t.Fatal("expected proposal validation issues")
	}
}

func TestStripProposalSecretsRemovesCredentialAndSecretParams(t *testing.T) {
	handler := newTestAIHandler(t)
	proposal := validReviewerWorkflow()
	proposal.Nodes[1].Params["api_key"] = "sk-secret-value"
	proposal.Nodes[1].Params["credential_id"] = "credential-id"
	handler.stripProposalSecrets(&proposal)
	if _, ok := proposal.Nodes[1].Params["api_key"]; ok {
		t.Fatal("api_key must be removed from review proposal")
	}
	if _, ok := proposal.Nodes[1].Params["credential_id"]; ok {
		t.Fatal("credential_id must be removed from review proposal")
	}
}

func TestValidateReviewProposalSupportsExplicitParamDeletionButProtectsCredentials(t *testing.T) {
	handler := newTestAIHandler(t)
	current := workflowDraft{
		Name: "Sheets",
		Nodes: []nodes.Node{{
			ID:   "sheet_1",
			Type: nodes.TypeGoogleSheets,
			Name: "Sheets",
			Params: map[string]interface{}{
				"credential_id":        "cred-1",
				"service_account_json": `{"private_key":"secret"}`,
				"spreadsheet_id":       "sheet-id",
				"sheet_name":           "Sheet1",
				"action":               "APPEND",
			},
		}},
	}
	proposal := cloneWorkflowDraft(current)
	delete(proposal.Nodes[0].Params, "service_account_json")
	delete(proposal.Nodes[0].Params, "credential_id")

	issues, deletes := handler.validateReviewProposal(current, &proposal, map[string][]string{
		"sheet_1": {"service_account_json"},
	})
	if len(issues) != 0 {
		t.Fatalf("expected explicit non-credential deletion to validate, got %#v", issues)
	}
	if len(deletes["sheet_1"]) != 1 || deletes["sheet_1"][0] != "service_account_json" {
		t.Fatalf("unexpected delete params: %#v", deletes)
	}

	_, protectedDeletes := handler.validateReviewProposal(current, &proposal, map[string][]string{
		"sheet_1": {"credential_id"},
	})
	if len(protectedDeletes) != 0 {
		t.Fatalf("credential deletion must not be exposed: %#v", protectedDeletes)
	}
}
