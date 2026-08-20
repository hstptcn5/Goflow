package pack_test

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"goflow/internal/pack"
	"goflow/internal/workflow"
)

const vietnamMorningBriefPackDir = "../../examples/packs/vietnam-morning-brief"

func TestVietnamMorningBriefPackDeclaresMinimalPilotSetup(t *testing.T) {
	loaded, err := pack.Load(vietnamMorningBriefPackDir)
	if err != nil {
		t.Fatalf("load pack: %v", err)
	}
	if loaded.Manifest.ID != "official.vietnam-morning-brief" || loaded.Manifest.Version != "0.1.0" {
		t.Fatalf("unexpected pack identity: %s@%s", loaded.Manifest.ID, loaded.Manifest.Version)
	}
	fields := map[string]pack.ConfigField{}
	for _, field := range loaded.Manifest.ConfigSchema {
		fields[field.Key] = field
	}
	if len(fields) != 2 {
		t.Fatalf("morning brief setup should remain minimal, got %d config fields", len(fields))
	}
	if chat := fields["chat_id"]; !chat.Required || chat.Type != "string" {
		t.Fatalf("unexpected chat_id setup: %#v", chat)
	}
	ai := fields["ai_provider"]
	if ai.Type != "select" || fmt.Sprint(ai.Default) != "none" || len(ai.Options) != 3 {
		t.Fatalf("unexpected AI provider setup: %#v", ai)
	}
	creds := map[string]pack.CredentialRequirement{}
	for _, req := range loaded.Manifest.CredentialRequirements {
		creds[req.Key] = req
	}
	if !creds["telegram"].Required || creds["telegram"].TestKind != "telegram_get_me" {
		t.Fatalf("Telegram must remain the required tested destination credential")
	}
	if creds["openai"].Required || creds["deepseek"].Required {
		t.Fatalf("AI credentials must remain optional")
	}
	fixture, err := pack.LoadOfflineTestFixture(loaded)
	if err != nil {
		t.Fatalf("load offline fixture: %v", err)
	}
	if fixture == nil || fmt.Sprint(fixture.Config["ai_provider"]) != "none" {
		t.Fatalf("offline fixture must exercise the no-AI first-run path: %#v", fixture)
	}
	wf, err := workflow.ReadFileLimit(loaded.EntryWorkflowPath, pack.MaxWorkflowBytes)
	if err != nil {
		t.Fatalf("read workflow: %v", err)
	}
	if wf.MaxConcurrentRuns != 1 || wf.ConcurrencyPolicy != "reject" {
		t.Fatalf("morning brief must reject duplicate active runs, got max=%d policy=%q", wf.MaxConcurrentRuns, wf.ConcurrencyPolicy)
	}
	workflowData, err := os.ReadFile(loaded.EntryWorkflowPath)
	if err != nil {
		t.Fatalf("read workflow source: %v", err)
	}
	workflowText := string(workflowData)
	for _, want := range []string{"rssFeedSource", "vnexpress.net/rss/tin-moi-nhat.rss", "tuoitre.vn/home.rss", "thanhnien.vn/rss/home.rss", "prepare_brief", "compile_openai", "compile_deepseek"} {
		if !strings.Contains(workflowText, want) {
			t.Fatalf("workflow missing %q", want)
		}
	}
}

func TestVietnamMorningBriefSourceManifestFailsCommerciallyOpenByDefault(t *testing.T) {
	path := filepath.Join(vietnamMorningBriefPackDir, "sources.json")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read source manifest: %v", err)
	}
	var manifest struct {
		DistributionStatus string `json:"distribution_status"`
		Sources            []struct {
			ID                string `json:"id"`
			CollectionMethod  string `json:"collection_method"`
			ArticleBodyFetch  bool   `json:"article_body_fetch"`
			RepublishFullText bool   `json:"republish_full_text"`
			LinkToOriginal    bool   `json:"link_to_original"`
			CommercialStatus  string `json:"commercial_status"`
		} `json:"sources"`
	}
	if err := json.Unmarshal(data, &manifest); err != nil {
		t.Fatalf("parse source manifest: %v", err)
	}
	if manifest.DistributionStatus != "personal_noncommercial_pilot" || len(manifest.Sources) != 3 {
		t.Fatalf("unexpected pilot source manifest: %#v", manifest)
	}
	for _, source := range manifest.Sources {
		if source.CollectionMethod != "publisher_rss" || source.ArticleBodyFetch || source.RepublishFullText || !source.LinkToOriginal {
			t.Fatalf("source %s violates RSS-only attribution boundary: %#v", source.ID, source)
		}
		if source.CommercialStatus != "not_cleared" {
			t.Fatalf("source %s must fail commercially closed until permissions are reviewed", source.ID)
		}
	}
}
