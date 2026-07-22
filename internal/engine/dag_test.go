package engine

import (
	"testing"

	"goflow/internal/nodes"
)

func TestDAGTopologicalSort(t *testing.T) {
	nodeList := []nodes.Node{
		{ID: "node1", Type: nodes.TypeWebhookTrigger, Name: "Webhook"},
		{ID: "node2", Type: nodes.TypeJSONTransform, Name: "Transform"},
		{ID: "node3", Type: nodes.TypeHTTPRequest, Name: "HTTP Request 1"},
		{ID: "node4", Type: nodes.TypeTelegramBot, Name: "Telegram Bot"},
	}

	edgeList := []nodes.Edge{
		{ID: "e1", Source: "node1", Target: "node2"},
		{ID: "e2", Source: "node2", Target: "node3"},
		{ID: "e3", Source: "node2", Target: "node4"},
	}

	plan, err := BuildDAGPlan(nodeList, edgeList)
	if err != nil {
		t.Fatalf("Failed to build DAG plan: %v", err)
	}

	if len(plan.ExecutionLayers) != 3 {
		t.Fatalf("Expected 3 execution layers, got %d", len(plan.ExecutionLayers))
	}

	// Layer 0: node1
	if len(plan.ExecutionLayers[0]) != 1 || plan.ExecutionLayers[0][0] != "node1" {
		t.Fatalf("Layer 0 should contain node1, got %v", plan.ExecutionLayers[0])
	}

	// Layer 1: node2
	if len(plan.ExecutionLayers[1]) != 1 || plan.ExecutionLayers[1][0] != "node2" {
		t.Fatalf("Layer 1 should contain node2, got %v", plan.ExecutionLayers[1])
	}

	// Layer 2: node3 và node4 (chạy song song)
	if len(plan.ExecutionLayers[2]) != 2 {
		t.Fatalf("Layer 2 should contain 2 parallel nodes (node3, node4), got %d", len(plan.ExecutionLayers[2]))
	}
}

func TestDAGCycleDetection(t *testing.T) {
	nodeList := []nodes.Node{
		{ID: "nodeA", Type: nodes.TypeWebhookTrigger},
		{ID: "nodeB", Type: nodes.TypeHTTPRequest},
	}

	// Tạo chu trình A -> B -> A
	edgeList := []nodes.Edge{
		{ID: "e1", Source: "nodeA", Target: "nodeB"},
		{ID: "e2", Source: "nodeB", Target: "nodeA"},
	}

	_, err := BuildDAGPlan(nodeList, edgeList)
	if err == nil {
		t.Fatalf("Expected error for cyclic dependency, but got nil")
	}
}
