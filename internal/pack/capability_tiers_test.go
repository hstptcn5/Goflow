package pack

import (
	"encoding/json"
	"strings"
	"testing"

	"goflow/internal/nodes"
)

func tierNodesJSON(t *testing.T, nodeType nodes.NodeType) string {
	t.Helper()
	data, err := json.Marshal([]nodes.Node{{ID: "n1", Type: nodeType, Name: "node", Params: map[string]interface{}{}}})
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func TestAssessExecutionTier(t *testing.T) {
	bounded, err := AssessExecutionTier(tierNodesJSON(t, nodes.TypeHTTPRequest))
	if err != nil || bounded.Tier != ExecutionTierBounded {
		t.Fatalf("bounded=%#v err=%v", bounded, err)
	}
	trusted, err := AssessExecutionTier(tierNodesJSON(t, nodes.TypePythonCode))
	if err != nil || trusted.Tier != ExecutionTierTrustedExternal || len(trusted.TrustedNodeIDs) != 1 {
		t.Fatalf("trusted=%#v err=%v", trusted, err)
	}
}

func TestBoundedPackRejectsTrustedExternalNode(t *testing.T) {
	err := validatePackExecutionTier(Manifest{}, tierNodesJSON(t, nodes.TypePythonCode))
	if err == nil || !strings.Contains(err.Error(), "trusted external") {
		t.Fatalf("expected trusted external rejection, got %v", err)
	}
}

func TestTrustedExternalPackRequiresCapability(t *testing.T) {
	manifest := Manifest{ExecutionTier: ExecutionTierTrustedExternal}
	if err := validatePackExecutionTier(manifest, tierNodesJSON(t, nodes.TypePythonCode)); err == nil {
		t.Fatal("trusted_external pack without capability was accepted")
	}
	manifest.RequiredCapabilities = []string{CapabilityTrustedExternalV1}
	if err := validatePackExecutionTier(manifest, tierNodesJSON(t, nodes.TypePythonCode)); err != nil {
		t.Fatal(err)
	}
}
