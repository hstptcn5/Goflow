package packrun

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"goflow/internal/pack"
	"goflow/internal/storage"
)

func TestPrepareIdempotentlyUpsertsManagedWorkflow(t *testing.T) {
	packDir := writeRunPack(t, "example.packrun", "0.1.0", "Hello Pack", false, nil)
	loaded := loadPack(t, packDir)
	dataDir := t.TempDir()

	first, err := prepare(context.Background(), loaded, dataDir)
	if err != nil {
		t.Fatalf("first prepare failed: %v", err)
	}
	assertWorkflowCount(t, dataDir, 1, first.WorkflowID, "Hello Pack")

	second, err := prepare(context.Background(), loaded, dataDir)
	if err != nil {
		t.Fatalf("second prepare failed: %v", err)
	}
	if second.WorkflowID != first.WorkflowID {
		t.Fatalf("workflow ID changed: %s -> %s", first.WorkflowID, second.WorkflowID)
	}
	assertWorkflowCount(t, dataDir, 1, first.WorkflowID, "Hello Pack")

	writeRunPackFiles(t, packDir, "example.packrun", "0.2.0", "Hello Pack Updated", false, nil)
	upgraded := loadPack(t, packDir)
	third, err := prepare(context.Background(), upgraded, dataDir)
	if err != nil {
		t.Fatalf("upgrade prepare failed: %v", err)
	}
	if third.WorkflowID != first.WorkflowID {
		t.Fatalf("upgrade changed workflow ID: %s -> %s", first.WorkflowID, third.WorkflowID)
	}
	assertWorkflowCount(t, dataDir, 1, first.WorkflowID, "Hello Pack Updated")
}

func TestRunRejectsPackagedPluginsInMVP(t *testing.T) {
	packDir := writeRunPack(t, "example.plugins", "0.1.0", "Plugin Pack", false, []string{"plugins/tool.exe"})
	var stdout, stderr bytes.Buffer
	err := Run(context.Background(), Options{
		PackDir: packDir,
		DataDir: t.TempDir(),
		NoOpen:  true,
		Stdout:  &stdout,
		Stderr:  &stderr,
	})
	if err == nil || !strings.Contains(err.Error(), "packaged plugin execution is not supported") {
		t.Fatalf("expected plugin policy error, got %v", err)
	}
}

func TestDataLockExcludesSecondHolderAndReleases(t *testing.T) {
	dataDir := t.TempDir()
	first, acquired, err := acquireDataLock(dataDir)
	if err != nil || !acquired {
		t.Fatalf("first lock acquired=%t err=%v", acquired, err)
	}
	_, acquired, err = acquireDataLock(dataDir)
	if err != nil {
		t.Fatalf("second lock returned unexpected error: %v", err)
	}
	if acquired {
		t.Fatalf("second lock unexpectedly acquired")
	}
	first.Release()
	third, acquired, err := acquireDataLock(dataDir)
	if err != nil || !acquired {
		t.Fatalf("third lock after release acquired=%t err=%v", acquired, err)
	}
	third.Release()
}

func TestRunStateURLMustBeLoopback(t *testing.T) {
	for _, raw := range []string{"http://127.0.0.1:1234/workflows", "http://localhost:1234/credentials"} {
		if !isLoopbackURL(raw) {
			t.Fatalf("expected loopback URL: %s", raw)
		}
	}
	for _, raw := range []string{"https://127.0.0.1:1234", "http://192.168.1.2:1234", "http://example.com"} {
		if isLoopbackURL(raw) {
			t.Fatalf("expected non-loopback URL rejection: %s", raw)
		}
	}
}

func TestOpenURLUsesInjectedOpener(t *testing.T) {
	called := ""
	err := openURL(Options{Opener: func(rawURL string) error {
		called = rawURL
		return nil
	}}, "http://127.0.0.1:1234/workflows")
	if err != nil {
		t.Fatalf("openURL failed: %v", err)
	}
	if called != "http://127.0.0.1:1234/workflows" {
		t.Fatalf("opener called with %q", called)
	}
}

func TestReuseExistingInstanceRetriesUntilStateAppears(t *testing.T) {
	var stdout bytes.Buffer
	attempts := 0
	err := reuseExistingInstance(context.Background(), Options{
		Stdout: &stdout,
		NoOpen: true,
	}, "data-dir", "example.pack", testRetryOptions(t, func(string) (RunState, error) {
		attempts++
		if attempts < 3 {
			return RunState{}, os.ErrNotExist
		}
		return RunState{PackID: "example.pack", URL: "http://127.0.0.1:1234/workflows"}, nil
	}, func(context.Context, string) error { return nil }))
	if err != nil {
		t.Fatalf("reuseExistingInstance failed: %v", err)
	}
	if attempts != 3 || !strings.Contains(stdout.String(), "http://127.0.0.1:1234/workflows") {
		t.Fatalf("unexpected attempts/stdout: %d %q", attempts, stdout.String())
	}
}

func TestReuseExistingInstanceRetriesStaleStateUntilReplaced(t *testing.T) {
	attempts := 0
	err := reuseExistingInstance(context.Background(), Options{Stdout: ioDiscard(), NoOpen: true}, "data-dir", "example.pack", testRetryOptions(t, func(string) (RunState, error) {
		attempts++
		if attempts == 1 {
			return RunState{PackID: "example.pack", URL: "http://127.0.0.1:1/workflows"}, nil
		}
		return RunState{PackID: "example.pack", URL: "http://127.0.0.1:1234/workflows"}, nil
	}, func(_ context.Context, rawURL string) error {
		if strings.Contains(rawURL, ":1/") {
			return errors.New("connection refused")
		}
		return nil
	}))
	if err != nil {
		t.Fatalf("reuseExistingInstance failed: %v", err)
	}
	if attempts != 2 {
		t.Fatalf("expected 2 attempts, got %d", attempts)
	}
}

func TestReuseExistingInstanceRetriesUntilHealthReady(t *testing.T) {
	probes := 0
	err := reuseExistingInstance(context.Background(), Options{Stdout: ioDiscard(), NoOpen: true}, "data-dir", "example.pack", testRetryOptions(t, func(string) (RunState, error) {
		return RunState{PackID: "example.pack", URL: "http://127.0.0.1:1234/workflows"}, nil
	}, func(context.Context, string) error {
		probes++
		if probes < 4 {
			return errors.New("not ready")
		}
		return nil
	}))
	if err != nil {
		t.Fatalf("reuseExistingInstance failed: %v", err)
	}
	if probes != 4 {
		t.Fatalf("expected 4 probes, got %d", probes)
	}
}

func TestReuseExistingInstanceTimeoutAndContextCancel(t *testing.T) {
	err := reuseExistingInstance(context.Background(), Options{Stdout: ioDiscard(), NoOpen: true}, "data-dir", "example.pack", testRetryOptions(t, func(string) (RunState, error) {
		return RunState{}, os.ErrNotExist
	}, func(context.Context, string) error { return nil }))
	if err == nil || !strings.Contains(err.Error(), "timed out") || !strings.Contains(err.Error(), "data-dir") || !strings.Contains(err.Error(), "run-state") {
		t.Fatalf("expected actionable timeout, got %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	err = reuseExistingInstance(ctx, Options{Stdout: ioDiscard(), NoOpen: true}, "data-dir", "example.pack", testRetryOptions(t, func(string) (RunState, error) {
		return RunState{}, os.ErrNotExist
	}, func(context.Context, string) error { return nil }))
	if err == nil || !strings.Contains(err.Error(), "cancelled") {
		t.Fatalf("expected cancellation error, got %v", err)
	}
}

func TestReuseExistingInstanceOpenerAtMostOnceAndNoOpen(t *testing.T) {
	calls := 0
	opts := Options{
		Stdout: ioDiscard(),
		Opener: func(string) error {
			calls++
			return nil
		},
	}
	err := reuseExistingInstance(context.Background(), opts, "data-dir", "example.pack", testRetryOptions(t, func(string) (RunState, error) {
		return RunState{PackID: "example.pack", URL: "http://127.0.0.1:1234/workflows"}, nil
	}, func(context.Context, string) error { return nil }))
	if err != nil {
		t.Fatalf("reuseExistingInstance failed: %v", err)
	}
	if calls != 1 {
		t.Fatalf("expected one opener call, got %d", calls)
	}
	opts.NoOpen = true
	err = reuseExistingInstance(context.Background(), opts, "data-dir", "example.pack", testRetryOptions(t, func(string) (RunState, error) {
		return RunState{PackID: "example.pack", URL: "http://127.0.0.1:1234/workflows"}, nil
	}, func(context.Context, string) error { return nil }))
	if err != nil {
		t.Fatalf("reuseExistingInstance no-open failed: %v", err)
	}
	if calls != 1 {
		t.Fatalf("no-open invoked opener, calls=%d", calls)
	}
}

func TestReuseExistingInstanceRetriesMismatchedPackID(t *testing.T) {
	attempts := 0
	calls := 0
	err := reuseExistingInstance(context.Background(), Options{
		Stdout: ioDiscard(),
		Opener: func(string) error {
			calls++
			return nil
		},
	}, "shared-data", "example.expected", testRetryOptions(t, func(string) (RunState, error) {
		attempts++
		if attempts == 1 {
			return RunState{PackID: "example.other", URL: "http://127.0.0.1:1234/workflows"}, nil
		}
		return RunState{PackID: "example.expected", URL: "http://127.0.0.1:1234/workflows"}, nil
	}, func(context.Context, string) error { return nil }))
	if err != nil {
		t.Fatalf("reuseExistingInstance failed: %v", err)
	}
	if attempts != 2 {
		t.Fatalf("expected retry after mismatched pack id, got attempts=%d", attempts)
	}
	if calls != 1 {
		t.Fatalf("expected opener once after correct pack id, got %d", calls)
	}
}

func TestReuseExistingInstancePermanentMismatchedPackIDTimeout(t *testing.T) {
	calls := 0
	err := reuseExistingInstance(context.Background(), Options{
		Stdout: ioDiscard(),
		Opener: func(string) error {
			calls++
			return nil
		},
	}, "shared-data", "example.expected", testRetryOptions(t, func(string) (RunState, error) {
		return RunState{PackID: "example.other", URL: "http://127.0.0.1:1234/workflows"}, nil
	}, func(context.Context, string) error { return nil }))
	if err == nil || !strings.Contains(err.Error(), "example.expected") || !strings.Contains(err.Error(), "example.other") {
		t.Fatalf("expected pack-id timeout with expected/observed IDs, got %v", err)
	}
	if calls != 0 {
		t.Fatalf("opener called for mismatched pack id: %d", calls)
	}
}

func TestTwoDifferentPacksSharingCustomDataDirDoNotReuseEachOther(t *testing.T) {
	err := reuseExistingInstance(context.Background(), Options{Stdout: ioDiscard(), NoOpen: true}, "shared-data", "example.pack-b", testRetryOptions(t, func(string) (RunState, error) {
		return RunState{PackID: "example.pack-a", URL: "http://127.0.0.1:1234/workflows"}, nil
	}, func(context.Context, string) error { return nil }))
	if err == nil || !strings.Contains(err.Error(), "example.pack-b") || !strings.Contains(err.Error(), "example.pack-a") {
		t.Fatalf("expected cross-pack reuse rejection, got %v", err)
	}
}

func TestRemoveRunStateAfterPrimaryLock(t *testing.T) {
	dataDir := t.TempDir()
	if err := writeRunState(dataDir, RunState{PackID: "old", URL: "http://127.0.0.1:1"}); err != nil {
		t.Fatalf("write run-state: %v", err)
	}
	if err := removeRunState(dataDir); err != nil {
		t.Fatalf("removeRunState failed: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dataDir, "run-state.json")); !os.IsNotExist(err) {
		t.Fatalf("run-state still exists or unexpected stat error: %v", err)
	}
}

func TestPrimaryLaunchClearsStaleHealthyRunState(t *testing.T) {
	dataDir := t.TempDir()
	if err := writeRunState(dataDir, RunState{PackID: "example.pack", URL: "http://127.0.0.1:1234/workflows"}); err != nil {
		t.Fatalf("write stale run-state: %v", err)
	}
	if err := removeRunState(dataDir); err != nil {
		t.Fatalf("removeRunState failed: %v", err)
	}
	probes := 0
	err := reuseExistingInstance(context.Background(), Options{Stdout: ioDiscard(), NoOpen: true}, dataDir, "example.pack", testRetryOptions(t, readRunState, func(context.Context, string) error {
		probes++
		return nil
	}))
	if err == nil || !strings.Contains(err.Error(), "run-state") {
		t.Fatalf("expected missing run-state after stale cleanup, got %v", err)
	}
	if probes != 0 {
		t.Fatalf("stale healthy URL was probed after cleanup, probes=%d", probes)
	}
}

func TestProbeHealthContextCancellationAndDelayedSuccess(t *testing.T) {
	block := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-r.Context().Done():
			return
		case <-block:
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"status":"ok"}`))
		}
	}))
	defer server.Close()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := probeHealth(ctx, server.URL+"/workflows"); err == nil {
		t.Fatalf("expected cancelled probe error")
	}

	ctx, cancel = context.WithTimeout(context.Background(), time.Millisecond)
	defer cancel()
	if err := probeHealth(ctx, server.URL+"/workflows"); err == nil {
		t.Fatalf("expected deadline probe error")
	}

	close(block)
	if err := probeHealth(context.Background(), server.URL+"/workflows"); err != nil {
		t.Fatalf("expected delayed health success, got %v", err)
	}
}

func TestReuseExistingInstanceDeadlineCancelsBlockedProbe(t *testing.T) {
	err := reuseExistingInstance(context.Background(), Options{Stdout: ioDiscard(), NoOpen: true}, "data-dir", "example.pack", reuseRetryOptions{
		Timeout: 10 * time.Millisecond,
		Backoff: time.Millisecond,
		ReadState: func(string) (RunState, error) {
			return RunState{PackID: "example.pack", URL: "http://127.0.0.1:1234/workflows"}, nil
		},
		Probe: func(ctx context.Context, _ string) error {
			<-ctx.Done()
			return ctx.Err()
		},
		After: func(context.Context, time.Duration) error {
			return nil
		},
	})
	if err == nil || !strings.Contains(err.Error(), "context deadline exceeded") {
		t.Fatalf("expected deadline cancellation, got %v", err)
	}
}

func testRetryOptions(t *testing.T, read func(string) (RunState, error), probe func(context.Context, string) error) reuseRetryOptions {
	t.Helper()
	now := time.Now()
	return reuseRetryOptions{
		Timeout:     time.Hour,
		Backoff:     time.Millisecond,
		MaxAttempts: 5,
		ReadState:   read,
		Probe:       probe,
		Now: func() time.Time {
			return now
		},
		After: func(context.Context, time.Duration) error {
			return nil
		},
	}
}

func ioDiscard() *bytes.Buffer {
	return &bytes.Buffer{}
}

func writeRunPack(t *testing.T, id, version, name string, active bool, plugins []string) string {
	t.Helper()
	dir := t.TempDir()
	writeRunPackFiles(t, dir, id, version, name, active, plugins)
	return dir
}

func writeRunPackFiles(t *testing.T, dir, id, version, name string, active bool, plugins []string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Join(dir, "workflows"), 0700); err != nil {
		t.Fatalf("mkdir workflows: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(dir, "plugins"), 0700); err != nil {
		t.Fatalf("mkdir plugins: %v", err)
	}
	for _, plugin := range plugins {
		path := filepath.Join(dir, filepath.FromSlash(plugin))
		if err := os.WriteFile(path, []byte("plugin"), 0700); err != nil {
			t.Fatalf("write plugin: %v", err)
		}
	}
	manifest := map[string]interface{}{
		"schema_version":       1,
		"id":                   id,
		"name":                 name,
		"version":              version,
		"entry_workflow":       "workflows/main.json",
		"required_credentials": []string{},
		"supported_platforms":  []string{pack.CurrentPlatform()},
	}
	if len(plugins) > 0 {
		manifest["plugins"] = plugins
	}
	writeJSON(t, filepath.Join(dir, "pack.json"), manifest)
	workflowDef := map[string]interface{}{
		"name":        name,
		"description": "managed test workflow",
		"is_active":   active,
		"nodes": []map[string]interface{}{
			{"id": "trigger", "type": "webhookTrigger", "params": map[string]interface{}{}},
		},
		"edges": []interface{}{},
	}
	writeJSON(t, filepath.Join(dir, "workflows", "main.json"), workflowDef)
}

func writeJSON(t *testing.T, path string, value interface{}) {
	t.Helper()
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		t.Fatalf("marshal json: %v", err)
	}
	if err := os.WriteFile(path, append(data, '\n'), 0600); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

func loadPack(t *testing.T, dir string) *pack.Pack {
	t.Helper()
	loaded, err := pack.Load(dir)
	if err != nil {
		t.Fatalf("load pack: %v", err)
	}
	return loaded
}

func assertWorkflowCount(t *testing.T, dataDir string, wantCount int, wantID, wantName string) {
	t.Helper()
	db, err := storage.NewDB(filepath.Join(dataDir, "goflow.db"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer db.Close()
	wfs, err := storage.NewWorkflowStore(db).ListAll()
	if err != nil {
		t.Fatalf("list workflows: %v", err)
	}
	if len(wfs) != wantCount {
		t.Fatalf("workflow count got %d want %d: %#v", len(wfs), wantCount, wfs)
	}
	if wfs[0].ID != wantID || wfs[0].Name != wantName {
		t.Fatalf("workflow got id=%s name=%s, want id=%s name=%s", wfs[0].ID, wfs[0].Name, wantID, wantName)
	}
}
