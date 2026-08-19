package nodes

import "testing"

func TestSourcePolicyAllowsReviewedPersonalSourceWithWarnings(t *testing.T) {
	executor := NewSourcePolicyExecutor()
	node := &Node{Params: map[string]interface{}{
		"source_id":                "publisher-rss",
		"publisher":                "Publisher",
		"source_url":               "https://example.com/rss",
		"collection_method":        "publisher_rss",
		"usage_context":            "personal_noncommercial",
		"policy_status":            "review_required",
		"terms_checked":            false,
		"robots_checked":           false,
		"attribution_required":     true,
		"link_to_original":         true,
		"republish_full_text":      false,
		"republish_images":         false,
		"article_body_fetch":       false,
		"max_requests_per_minute": 6,
		"enforcement":              "block",
	}}

	result, err := executor.Execute(NewExecutionContext("wf", "exec"), node)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	output := result.(map[string]interface{})
	if output["allowed"] != true || output["canonical_host"] != "example.com" {
		t.Fatalf("unexpected source policy output: %#v", output)
	}
	warnings := output["warnings"].([]string)
	if len(warnings) != 1 || warnings[0] != "source policy still requires review" {
		t.Fatalf("unexpected warnings: %#v", warnings)
	}
}

func TestSourcePolicyBlocksCommercialUseUntilCleared(t *testing.T) {
	executor := NewSourcePolicyExecutor()
	node := &Node{Params: map[string]interface{}{
		"source_id":         "publisher-rss",
		"publisher":         "Publisher",
		"source_url":        "https://example.com/rss",
		"collection_method": "publisher_rss",
		"usage_context":     "commercial_distribution",
		"policy_status":     "review_required",
		"terms_checked":     false,
		"enforcement":       "block",
	}}
	if _, err := executor.Execute(NewExecutionContext("wf", "exec"), node); err == nil {
		t.Fatal("expected uncleared commercial source to be blocked")
	}
}

func TestSourcePolicyBlocksRepublicationUnlessCleared(t *testing.T) {
	executor := NewSourcePolicyExecutor()
	node := &Node{Params: map[string]interface{}{
		"source_id":           "public-file",
		"publisher":           "Agency",
		"source_url":          "https://example.com/document.pdf",
		"collection_method":   "public_file",
		"usage_context":       "personal_noncommercial",
		"policy_status":       "pilot_only",
		"republish_full_text": true,
		"enforcement":         "block",
	}}
	if err := executor.Validate(node); err == nil {
		t.Fatal("expected full-text republication on uncleared source to fail validation")
	}
}

func TestSourcePolicyWarnModeSurfacesBoundaryWithoutPretendingClearance(t *testing.T) {
	executor := NewSourcePolicyExecutor()
	node := &Node{Params: map[string]interface{}{
		"source_id":         "web-source",
		"publisher":         "Publisher",
		"source_url":        "https://example.com/news",
		"collection_method": "public_web",
		"usage_context":     "commercial_distribution",
		"policy_status":     "review_required",
		"terms_checked":     false,
		"robots_checked":    false,
		"enforcement":       "warn",
	}}
	result, err := executor.Execute(NewExecutionContext("wf", "exec"), node)
	if err != nil {
		t.Fatalf("warn enforcement should not block: %v", err)
	}
	warnings := result.(map[string]interface{})["warnings"].([]string)
	if len(warnings) < 3 {
		t.Fatalf("expected commercial, terms, robots, and review warnings: %#v", warnings)
	}
}
