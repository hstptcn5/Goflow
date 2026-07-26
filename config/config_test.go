package config

import "testing"

func TestDefaultMaxParallelNodesPerRun(t *testing.T) {
	t.Setenv("GOFLOW_MAX_PARALLEL_NODES_PER_EXECUTION", "")
	t.Setenv("GOFLOW_DB_PATH", ":memory:")
	t.Setenv("GOFLOW_MASTER_KEY", "test-master-key")

	cfg := LoadConfig()
	if cfg.MaxParallelNodesPerRun != 4 {
		t.Fatalf("expected default node concurrency 4, got %d", cfg.MaxParallelNodesPerRun)
	}
}

func TestDefaultMCPRateLimitPerMinute(t *testing.T) {
	t.Setenv("GOFLOW_DB_PATH", ":memory:")
	t.Setenv("GOFLOW_MASTER_KEY", "test-master-key")

	cfg := LoadConfig()
	if cfg.MCPRateLimitPerMinute != 30 {
		t.Fatalf("expected default MCP rate limit 30, got %d", cfg.MCPRateLimitPerMinute)
	}
}
