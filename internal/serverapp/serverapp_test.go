package serverapp

import (
	"context"
	"io"
	"log"
	"net"
	"net/http"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"goflow/config"
	"goflow/internal/nodes"
)

func TestStartWithEphemeralListenerHealthAndExistingRoute(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	listener := testListener(t)
	app, err := Start(ctx, Options{
		Config:   testConfig(t),
		Listener: listener,
		Logger:   testLogger(t),
	})
	if err != nil {
		t.Fatalf("Start failed: %v", err)
	}
	defer func() {
		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer shutdownCancel()
		if err := app.Shutdown(shutdownCtx); err != nil {
			t.Fatalf("Shutdown failed: %v", err)
		}
	}()

	body := httpGet(t, app.URL+"/healthz", http.StatusOK)
	if !strings.Contains(body, `"status":"ok"`) {
		t.Fatalf("unexpected health body: %s", body)
	}
	body = httpGet(t, app.URL+"/api/v1/workflows", http.StatusOK)
	if strings.TrimSpace(body) != "[]" {
		t.Fatalf("existing route returned unexpected body: %s", body)
	}
}

func TestRunReturnsWhenContextCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	listener := testListener(t)
	done := make(chan error, 1)
	go func() {
		done <- Run(ctx, Options{
			Config:   testConfig(t),
			Listener: listener,
			Logger:   testLogger(t),
		})
	}()

	waitForHTTP(t, "http://"+listener.Addr().String()+"/healthz")
	cancel()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("Run returned error after context cancel: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatalf("Run did not return after context cancel")
	}
}

func TestRunReturnsStartupServeError(t *testing.T) {
	listener := testListener(t)
	addr := listener.Addr().String()
	if err := listener.Close(); err != nil {
		t.Fatalf("close listener: %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	err := Run(ctx, Options{
		Config:   testConfig(t),
		Listener: listener,
		Logger:   testLogger(t),
	})
	if err == nil || !strings.Contains(err.Error(), "closed") {
		t.Fatalf("expected closed listener error for %s, got %v", addr, err)
	}
}

func TestRegistryFromOptionsDefaultsToBuiltin(t *testing.T) {
	registry := registryFromOptions(Options{})
	if _, ok := registry.Get(nodes.TypeTelegramBot); !ok {
		t.Fatalf("default registry is missing Telegram executor")
	}
}

func TestRegistryFromOptionsUsesInjectedRegistry(t *testing.T) {
	injected := nodes.NewPluginRegistry()
	registry := registryFromOptions(Options{Registry: injected})
	if registry != injected {
		t.Fatalf("expected injected registry")
	}
	if _, ok := registry.Get(nodes.TypeTelegramBot); ok {
		t.Fatalf("empty injected registry should not be replaced with builtin executors")
	}
}

func testConfig(t *testing.T) *config.Config {
	t.Helper()
	dir := t.TempDir()
	return &config.Config{
		Host:                      "127.0.0.1",
		Port:                      "0",
		DBPath:                    filepath.Join(dir, "goflow.db"),
		MasterKey:                 "test-master-key",
		MaxConcurrentExecutions:   10,
		MaxParallelNodesPerRun:    4,
		WebhookRateLimitPerMinute: 60,
		ExecutionRetentionDays:    30,
		MaxExecutionsPerWorkflow:  1000,
		MCPAllowedOrigins:         []string{"http://127.0.0.1"},
		MCPMaxInflightPerClient:   2,
		MCPRateLimitPerMinute:     30,
	}
}

func testListener(t *testing.T) net.Listener {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	return listener
}

func testLogger(t *testing.T) *log.Logger {
	t.Helper()
	return log.New(io.Discard, "", 0)
}

func httpGet(t *testing.T, url string, wantStatus int) string {
	t.Helper()
	client := http.Client{Timeout: 5 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		t.Fatalf("GET %s: %v", url, err)
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read response: %v", err)
	}
	if resp.StatusCode != wantStatus {
		t.Fatalf("GET %s status %d body %s", url, resp.StatusCode, string(data))
	}
	return string(data)
}

func waitForHTTP(t *testing.T, url string) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	var lastErr error
	for time.Now().Before(deadline) {
		client := http.Client{Timeout: 500 * time.Millisecond}
		resp, err := client.Get(url)
		if err == nil {
			_ = resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				return
			}
		}
		lastErr = err
		time.Sleep(25 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for %s: %v", url, lastErr)
}
