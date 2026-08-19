package nodes

import (
	"fmt"
	"net/url"
	"strings"
)

const (
	sourcePolicyCleared        = "cleared"
	sourcePolicyPilotOnly      = "pilot_only"
	sourcePolicyReviewRequired = "review_required"
	sourcePolicyBlocked        = "blocked"
)

type SourcePolicyExecutor struct{}

func NewSourcePolicyExecutor() *SourcePolicyExecutor { return &SourcePolicyExecutor{} }

func (e *SourcePolicyExecutor) Execute(_ *ExecutionContext, node *Node) (interface{}, error) {
	policy, warnings, err := buildSourcePolicy(node)
	if err != nil {
		return nil, err
	}
	policy["warnings"] = warnings
	policy["allowed"] = true
	policy["policy_enforced"] = true
	return policy, nil
}

func (e *SourcePolicyExecutor) Validate(node *Node) error {
	_, _, err := buildSourcePolicy(node)
	return err
}

func buildSourcePolicy(node *Node) (map[string]interface{}, []string, error) {
	stringParam := func(name, defaultValue string) string {
		value, _ := node.Params[name].(string)
		value = strings.TrimSpace(value)
		if value == "" {
			return defaultValue
		}
		return value
	}
	boolParam := func(name string, defaultValue bool) bool {
		value, ok := node.Params[name].(bool)
		if !ok {
			return defaultValue
		}
		return value
	}

	sourceID := stringParam("source_id", "")
	publisher := stringParam("publisher", "")
	sourceURL := stringParam("source_url", "")
	collectionMethod := stringParam("collection_method", "publisher_rss")
	usageContext := stringParam("usage_context", "personal_noncommercial")
	policyStatus := stringParam("policy_status", sourcePolicyReviewRequired)
	enforcement := stringParam("enforcement", "block")
	policyNote := stringParam("policy_note", "")

	if sourceID == "" {
		return nil, nil, fmt.Errorf("source policy requires source_id")
	}
	if publisher == "" {
		return nil, nil, fmt.Errorf("source policy requires publisher")
	}

	allowedMethods := map[string]bool{
		"official_api":  true,
		"publisher_rss": true,
		"public_web":    true,
		"public_file":   true,
		"manual_upload": true,
	}
	if !allowedMethods[collectionMethod] {
		return nil, nil, fmt.Errorf("unsupported source collection_method %q", collectionMethod)
	}

	canonicalHost := ""
	if sourceURL == "" {
		if collectionMethod != "manual_upload" {
			return nil, nil, fmt.Errorf("source policy requires source_url unless collection_method=manual_upload")
		}
	} else {
		parsed, err := url.Parse(sourceURL)
		if err != nil || !parsed.IsAbs() || parsed.Host == "" || (parsed.Scheme != "http" && parsed.Scheme != "https") {
			return nil, nil, fmt.Errorf("source policy requires an absolute http/https source_url")
		}
		canonicalHost = strings.ToLower(parsed.Hostname())
	}

	allowedUsage := map[string]bool{
		"personal_noncommercial":  true,
		"internal_business":       true,
		"commercial_distribution": true,
	}
	if !allowedUsage[usageContext] {
		return nil, nil, fmt.Errorf("unsupported source usage_context %q", usageContext)
	}
	allowedStatuses := map[string]bool{
		sourcePolicyCleared:        true,
		sourcePolicyPilotOnly:      true,
		sourcePolicyReviewRequired: true,
		sourcePolicyBlocked:        true,
	}
	if !allowedStatuses[policyStatus] {
		return nil, nil, fmt.Errorf("unsupported source policy_status %q", policyStatus)
	}
	if enforcement != "block" && enforcement != "warn" {
		return nil, nil, fmt.Errorf("source policy enforcement must be block or warn")
	}

	maxRequests, err := positiveIntParam(node.Params["max_requests_per_minute"], 6)
	if err != nil || maxRequests > 600 {
		return nil, nil, fmt.Errorf("source policy max_requests_per_minute must be between 1 and 600")
	}

	termsChecked := boolParam("terms_checked", false)
	robotsChecked := boolParam("robots_checked", false)
	attributionRequired := boolParam("attribution_required", true)
	linkToOriginal := boolParam("link_to_original", true)
	republishFullText := boolParam("republish_full_text", false)
	republishImages := boolParam("republish_images", false)
	articleBodyFetch := boolParam("article_body_fetch", false)

	warnings := make([]string, 0, 4)
	block := func(message string) error {
		if enforcement == "warn" {
			warnings = append(warnings, message)
			return nil
		}
		return fmt.Errorf("source policy blocked execution: %s", message)
	}

	if policyStatus == sourcePolicyBlocked {
		return nil, nil, fmt.Errorf("source policy blocked execution: source is marked blocked")
	}
	if usageContext == "commercial_distribution" && policyStatus != sourcePolicyCleared {
		if err := block("commercial distribution requires policy_status=cleared"); err != nil {
			return nil, nil, err
		}
	}
	if usageContext == "commercial_distribution" && !termsChecked {
		if err := block("commercial distribution requires terms_checked=true"); err != nil {
			return nil, nil, err
		}
	}
	if (republishFullText || republishImages) && policyStatus != sourcePolicyCleared {
		if err := block("republishing full text or images requires policy_status=cleared"); err != nil {
			return nil, nil, err
		}
	}
	if collectionMethod == "public_web" && !robotsChecked {
		warnings = append(warnings, "robots policy has not been checked for this public_web source")
	}
	if policyStatus == sourcePolicyReviewRequired {
		warnings = append(warnings, "source policy still requires review")
	}
	if policyStatus == sourcePolicyPilotOnly && usageContext != "personal_noncommercial" {
		if err := block("pilot_only sources are limited to personal_noncommercial use"); err != nil {
			return nil, nil, err
		}
	}

	return map[string]interface{}{
		"source_id":               sourceID,
		"publisher":               publisher,
		"source_url":              sourceURL,
		"canonical_host":          canonicalHost,
		"collection_method":       collectionMethod,
		"usage_context":           usageContext,
		"policy_status":           policyStatus,
		"terms_checked":           termsChecked,
		"robots_checked":          robotsChecked,
		"attribution_required":    attributionRequired,
		"link_to_original":        linkToOriginal,
		"republish_full_text":     republishFullText,
		"republish_images":        republishImages,
		"article_body_fetch":      articleBodyFetch,
		"max_requests_per_minute": maxRequests,
		"policy_note":             policyNote,
		"enforcement":             enforcement,
	}, warnings, nil
}

func positiveIntParam(value interface{}, defaultValue int) (int, error) {
	if value == nil {
		return defaultValue, nil
	}
	switch typed := value.(type) {
	case int:
		if typed > 0 {
			return typed, nil
		}
	case int64:
		if typed > 0 {
			return int(typed), nil
		}
	case float64:
		if typed > 0 && typed == float64(int(typed)) {
			return int(typed), nil
		}
	}
	return 0, fmt.Errorf("must be a positive integer")
}

func (e *SourcePolicyExecutor) GetDefinition() NodeDefinition {
	return NodeDefinition{
		Type:        TypeSourcePolicy,
		Name:        "Source Policy",
		Description: "Attaches source provenance and compliance metadata and blocks use outside declared policy boundaries",
		Icon:        "ShieldCheck",
		Category:    "ACTION",
		Retryable:   false,
		Params: []ParamDefinition{
			{Name: "source_id", Label: "Source ID", Type: "text", Required: true, Description: "Stable identifier for this source"},
			{Name: "publisher", Label: "Publisher / Owner", Type: "text", Required: true},
			{Name: "source_url", Label: "Canonical Source URL", Type: "text", Required: false, Description: "Required except for manual_upload sources"},
			{Name: "collection_method", Label: "Collection Method", Type: "select", Default: "publisher_rss", Options: []string{"official_api", "publisher_rss", "public_web", "public_file", "manual_upload"}, Required: true},
			{Name: "usage_context", Label: "Usage Context", Type: "select", Default: "personal_noncommercial", Options: []string{"personal_noncommercial", "internal_business", "commercial_distribution"}, Required: true},
			{Name: "policy_status", Label: "Policy Review Status", Type: "select", Default: sourcePolicyReviewRequired, Options: []string{sourcePolicyCleared, sourcePolicyPilotOnly, sourcePolicyReviewRequired, sourcePolicyBlocked}, Required: true},
			{Name: "terms_checked", Label: "Terms Checked", Type: "boolean", Default: false, Required: true},
			{Name: "robots_checked", Label: "Robots Policy Checked", Type: "boolean", Default: false, Required: true},
			{Name: "attribution_required", Label: "Attribution Required", Type: "boolean", Default: true, Required: true},
			{Name: "link_to_original", Label: "Link To Original", Type: "boolean", Default: true, Required: true},
			{Name: "republish_full_text", Label: "Full-text Republication Allowed", Type: "boolean", Default: false, Required: true},
			{Name: "republish_images", Label: "Image Republication Allowed", Type: "boolean", Default: false, Required: true},
			{Name: "article_body_fetch", Label: "Article Body Fetch Allowed", Type: "boolean", Default: false, Required: true},
			{Name: "max_requests_per_minute", Label: "Max Requests / Minute", Type: "integer", Default: 6, Required: true},
			{Name: "enforcement", Label: "Enforcement", Type: "select", Default: "block", Options: []string{"block", "warn"}, Required: true},
			{Name: "policy_note", Label: "Policy Note", Type: "textarea", Default: "", Required: false, Description: "Record the review source/date or unresolved questions. This is metadata, not legal advice."},
		},
	}
}
