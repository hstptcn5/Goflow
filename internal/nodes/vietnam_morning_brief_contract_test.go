package nodes

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type morningBriefWorkflowFixture struct {
	Nodes []Node `json:"nodes"`
}

func loadMorningBriefWorkflow(t *testing.T) morningBriefWorkflowFixture {
	t.Helper()
	path := filepath.Join("..", "..", "examples", "packs", "vietnam-morning-brief", "workflows", "main.json")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read morning brief workflow: %v", err)
	}
	var workflow morningBriefWorkflowFixture
	if err := json.Unmarshal(data, &workflow); err != nil {
		t.Fatalf("parse morning brief workflow: %v", err)
	}
	return workflow
}

func morningBriefNode(t *testing.T, workflow morningBriefWorkflowFixture, id string) *Node {
	t.Helper()
	for i := range workflow.Nodes {
		if workflow.Nodes[i].ID == id {
			return &workflow.Nodes[i]
		}
	}
	t.Fatalf("morning brief node %q not found", id)
	return nil
}

func TestVietnamMorningBriefAICompilerReattachesOnlyOriginalSourceLinks(t *testing.T) {
	workflow := loadMorningBriefWorkflow(t)
	runner := NewJSCodeRunnerExecutor()
	ctx := NewExecutionContext("morning-brief", "test")

	originalURL := "https://publisher.example/story-a"
	ctx.SetOutput("collect_news", map[string]interface{}{
		"items": []interface{}{
			map[string]interface{}{
				"publisher":    "Publisher A",
				"title":        "Story A",
				"summary":      "Original feed summary.",
				"url":          originalURL,
				"published_at": "2026-08-19T06:00:00Z",
			},
		},
		"sources_ok":    1,
		"source_errors": []interface{}{},
	})

	prepared, err := runner.Execute(ctx, morningBriefNode(t, workflow, "prepare_brief"))
	if err != nil {
		t.Fatalf("prepare brief: %v", err)
	}
	ctx.SetOutput("prepare_brief", prepared)
	ctx.SetOutput("openai_summary", map[string]interface{}{
		"ai_response": `{"stories":[{"headline":"AI headline","summary":"AI summary","ids":["N1"],"url":"https://attacker.example/fake"}]}`,
	})

	compiled, err := runner.Execute(ctx, morningBriefNode(t, workflow, "compile_openai"))
	if err != nil {
		t.Fatalf("compile OpenAI brief: %v", err)
	}
	result, ok := compiled.(map[string]interface{})
	if !ok {
		t.Fatalf("unexpected compiled result type: %T", compiled)
	}
	message, _ := result["message"].(string)
	if !strings.Contains(message, originalURL) {
		t.Fatalf("compiled message lost original source URL: %q", message)
	}
	if strings.Contains(message, "attacker.example") {
		t.Fatalf("compiled message trusted an AI-supplied URL: %q", message)
	}
	if result["used_ai"] != true {
		t.Fatalf("expected valid AI result to be used: %#v", result)
	}
}

func TestVietnamMorningBriefInvalidAIIDsFallBackToSourceLinkedDigest(t *testing.T) {
	workflow := loadMorningBriefWorkflow(t)
	runner := NewJSCodeRunnerExecutor()
	ctx := NewExecutionContext("morning-brief", "test")

	originalURL := "https://publisher.example/story-a"
	ctx.SetOutput("collect_news", map[string]interface{}{
		"items": []interface{}{
			map[string]interface{}{
				"publisher":    "Publisher A",
				"title":        "Story A",
				"summary":      "Original feed summary.",
				"url":          originalURL,
				"published_at": "2026-08-19T06:00:00Z",
			},
		},
		"sources_ok":    1,
		"source_errors": []interface{}{},
	})

	prepared, err := runner.Execute(ctx, morningBriefNode(t, workflow, "prepare_brief"))
	if err != nil {
		t.Fatalf("prepare brief: %v", err)
	}
	ctx.SetOutput("prepare_brief", prepared)
	ctx.SetOutput("openai_summary", map[string]interface{}{
		"ai_response": `{"stories":[{"headline":"Unknown","summary":"Should not be trusted","ids":["N999"]}]}`,
	})

	compiled, err := runner.Execute(ctx, morningBriefNode(t, workflow, "compile_openai"))
	if err != nil {
		t.Fatalf("compile OpenAI brief: %v", err)
	}
	result, ok := compiled.(map[string]interface{})
	if !ok {
		t.Fatalf("unexpected compiled result type: %T", compiled)
	}
	message, _ := result["message"].(string)
	if !strings.Contains(message, originalURL) || !strings.Contains(message, "Story A") {
		t.Fatalf("fallback is not source-linked: %q", message)
	}
	if result["used_ai"] != false {
		t.Fatalf("invalid AI IDs should force fallback: %#v", result)
	}
}
