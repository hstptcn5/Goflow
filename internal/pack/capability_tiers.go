package pack

import (
	"encoding/json"
	"fmt"
	"strings"

	"goflow/internal/nodes"
)

const (
	ExecutionTierBounded         = "bounded"
	ExecutionTierTrustedExternal = "trusted_external"
	CapabilityTrustedExternalV1  = "goflow.execution.trusted-external.v1"
)

type ExecutionTierAssessment struct {
	Tier             string   `json:"tier"`
	TrustedNodeIDs   []string `json:"trusted_node_ids,omitempty"`
	TrustedNodeTypes []string `json:"trusted_node_types,omitempty"`
}

func normalizedExecutionTier(raw string) (string, error) {
	tier := strings.ToLower(strings.TrimSpace(raw))
	if tier == "" {
		return ExecutionTierBounded, nil
	}
	switch tier {
	case ExecutionTierBounded, ExecutionTierTrustedExternal:
		return tier, nil
	default:
		return "", fmt.Errorf("manifest: execution_tier must be %q or %q", ExecutionTierBounded, ExecutionTierTrustedExternal)
	}
}

func nodeRequiresTrustedExternal(nodeType nodes.NodeType) bool {
	value := string(nodeType)
	if strings.HasPrefix(value, "custom.") || strings.HasPrefix(value, "user.") {
		return true
	}
	switch nodeType {
	case nodes.TypePythonCode, nodes.TypeGoflowPlugin, nodes.TypeSSHRunner, nodes.TypeGitCommand:
		return true
	default:
		return false
	}
}

func AssessExecutionTier(nodesJSON string) (ExecutionTierAssessment, error) {
	var workflowNodes []nodes.Node
	if err := json.Unmarshal([]byte(nodesJSON), &workflowNodes); err != nil {
		return ExecutionTierAssessment{}, fmt.Errorf("workflow: invalid nodes_json for execution tier assessment: %w", err)
	}
	assessment := ExecutionTierAssessment{Tier: ExecutionTierBounded}
	seenTypes := map[string]bool{}
	for _, node := range workflowNodes {
		if !nodeRequiresTrustedExternal(node.Type) {
			continue
		}
		assessment.Tier = ExecutionTierTrustedExternal
		assessment.TrustedNodeIDs = append(assessment.TrustedNodeIDs, node.ID)
		if value := string(node.Type); !seenTypes[value] {
			seenTypes[value] = true
			assessment.TrustedNodeTypes = append(assessment.TrustedNodeTypes, value)
		}
	}
	return assessment, nil
}

func validatePackExecutionTier(manifest Manifest, nodesJSON string) error {
	declared, err := normalizedExecutionTier(manifest.ExecutionTier)
	if err != nil {
		return err
	}
	assessment, err := AssessExecutionTier(nodesJSON)
	if err != nil {
		return err
	}
	if assessment.Tier == ExecutionTierTrustedExternal && declared != ExecutionTierTrustedExternal {
		return fmt.Errorf("manifest: bounded pack contains trusted external execution nodes %v; declare execution_tier=%q explicitly", assessment.TrustedNodeTypes, ExecutionTierTrustedExternal)
	}
	if declared == ExecutionTierTrustedExternal && !containsCapability(manifest.RequiredCapabilities, CapabilityTrustedExternalV1) {
		return fmt.Errorf("manifest: trusted_external packs must require capability %q", CapabilityTrustedExternalV1)
	}
	return nil
}

func containsCapability(values []string, target string) bool {
	for _, value := range values {
		if strings.TrimSpace(value) == target {
			return true
		}
	}
	return false
}
