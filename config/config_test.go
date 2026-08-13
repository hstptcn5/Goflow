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

func TestExecutionRetentionUsesSafeRanges(t *testing.T) {
	t.Setenv("GOFLOW_DB_PATH", ":memory:")
	t.Setenv("GOFLOW_MASTER_KEY", "test-master-key")
	tests := []struct {
		name    string
		days    string
		count   string
		wantDay int
		wantMax int
	}{
		{"minimum", "1", "1", 1, 1},
		{"maximum", "365", "10000", 365, 10000},
		{"zero falls back", "0", "0", 30, 1000},
		{"too large falls back", "366", "10001", 30, 1000},
		{"malformed falls back", "forever", "unlimited", 30, 1000},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Setenv("GOFLOW_EXECUTION_RETENTION_DAYS", test.days)
			t.Setenv("GOFLOW_MAX_EXECUTIONS_PER_WORKFLOW", test.count)
			cfg := LoadConfig()
			if cfg.ExecutionRetentionDays != test.wantDay || cfg.MaxExecutionsPerWorkflow != test.wantMax {
				t.Fatalf("retention = %d/%d; want %d/%d", cfg.ExecutionRetentionDays, cfg.MaxExecutionsPerWorkflow, test.wantDay, test.wantMax)
			}
		})
	}
}
