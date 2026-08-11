package packrun

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"goflow/internal/crypto"
	"goflow/internal/pack"
	"goflow/internal/packsetup"
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

func TestPrepareReconstructsCompletedSetupAcrossRestartsAndPackUpgrade(t *testing.T) {
	packDir := t.TempDir()
	writeSetupRunPackFiles(t, packDir, "0.1.0", "v1")
	loaded := loadPack(t, packDir)
	dataDir := t.TempDir()

	first, err := prepare(context.Background(), loaded, dataDir)
	if err != nil {
		t.Fatalf("first prepare: %v", err)
	}
	firstWorkflow := loadManagedWorkflow(t, dataDir, first.WorkflowID)
	if firstWorkflow.IsActive {
		t.Fatalf("first-run workflow must be inactive even when the pack definition is active")
	}
	assertManagedBindings(t, firstWorkflow, "https://default.example.test/daily.json", "", "v1")

	credentialID := saveCompletedRunSetup(t, loaded, first)
	if _, err := packsetup.SaveState(dataDir, loaded.Manifest, false, time.Now()); err != nil {
		t.Fatalf("save incomplete setup state: %v", err)
	}
	if _, err := prepare(context.Background(), loaded, dataDir); err != nil {
		t.Fatalf("prepare incomplete setup: %v", err)
	}
	if workflow := loadManagedWorkflow(t, dataDir, first.WorkflowID); workflow.IsActive {
		t.Fatalf("incomplete setup became active")
	}

	completedAt := time.Date(2026, 8, 9, 10, 0, 0, 0, time.UTC)
	if _, err := packsetup.SaveState(dataDir, loaded.Manifest, true, completedAt); err != nil {
		t.Fatalf("save completed setup state: %v", err)
	}
	restarted, err := prepare(context.Background(), loaded, dataDir)
	if err != nil {
		t.Fatalf("prepare completed restart: %v", err)
	}
	if restarted.WorkflowID != first.WorkflowID {
		t.Fatalf("workflow ID changed after completed restart: %s -> %s", first.WorkflowID, restarted.WorkflowID)
	}
	bound := loadManagedWorkflow(t, dataDir, restarted.WorkflowID)
	if !bound.IsActive {
		t.Fatalf("completed restart did not restore active workflow")
	}
	assertManagedBindings(t, bound, "https://source.example.test/daily.json", credentialID, "v1")

	for restart := 0; restart < 3; restart++ {
		prepared, err := prepare(context.Background(), loaded, dataDir)
		if err != nil {
			t.Fatalf("idempotent restart %d: %v", restart, err)
		}
		if prepared.WorkflowID != first.WorkflowID {
			t.Fatalf("restart %d changed workflow ID", restart)
		}
		assertWorkflowCount(t, dataDir, 1, first.WorkflowID, "Setup Pack")
		assertCredentialCount(t, dataDir, 1)
		state, err := packsetup.LoadState(dataDir, loaded.Manifest)
		if err != nil || !state.Completed || state.UpdatedAt != completedAt.Format(time.RFC3339) {
			t.Fatalf("restart %d changed setup state: state=%#v err=%v", restart, state, err)
		}
	}

	writeSetupRunPackFiles(t, packDir, "0.2.0", "v2")
	upgraded := loadPack(t, packDir)
	upgradePrepared, err := prepare(context.Background(), upgraded, dataDir)
	if err != nil {
		t.Fatalf("prepare pack upgrade: %v", err)
	}
	if upgradePrepared.WorkflowID != first.WorkflowID {
		t.Fatalf("pack upgrade changed stable workflow ID")
	}
	upgradedWorkflow := loadManagedWorkflow(t, dataDir, first.WorkflowID)
	if !upgradedWorkflow.IsActive {
		t.Fatalf("valid completed setup was not active after pack upgrade")
	}
	assertManagedBindings(t, upgradedWorkflow, "https://source.example.test/daily.json", credentialID, "v2")
	assertWorkflowCount(t, dataDir, 1, first.WorkflowID, "Setup Pack")
	assertCredentialCount(t, dataDir, 1)
}

func TestPrepareInvalidPersistedSetupFailsClosed(t *testing.T) {
	for _, test := range []struct {
		name       string
		breakSetup func(t *testing.T, prepared *Prepared, credentialID string)
	}{
		{
			name: "missing config",
			breakSetup: func(t *testing.T, prepared *Prepared, _ string) {
				t.Helper()
				if err := os.Remove(filepath.Join(prepared.DataDir, packsetup.ConfigFileName)); err != nil {
					t.Fatalf("remove config: %v", err)
				}
			},
		},
		{
			name: "deleted credential",
			breakSetup: func(t *testing.T, prepared *Prepared, credentialID string) {
				t.Helper()
				db, err := storage.NewDB(prepared.DBPath)
				if err != nil {
					t.Fatalf("open db: %v", err)
				}
				defer db.Close()
				if err := storage.NewCredentialStore(db, nil).Delete(credentialID); err != nil {
					t.Fatalf("delete credential: %v", err)
				}
			},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			packDir := t.TempDir()
			writeSetupRunPackFiles(t, packDir, "0.1.0", "v1")
			loaded := loadPack(t, packDir)
			dataDir := t.TempDir()
			first, err := prepare(context.Background(), loaded, dataDir)
			if err != nil {
				t.Fatalf("first prepare: %v", err)
			}
			credentialID := saveCompletedRunSetup(t, loaded, first)
			if _, err := packsetup.SaveState(dataDir, loaded.Manifest, true, time.Now()); err != nil {
				t.Fatalf("save state: %v", err)
			}
			if _, err := prepare(context.Background(), loaded, dataDir); err != nil {
				t.Fatalf("activate completed setup: %v", err)
			}
			if !loadManagedWorkflow(t, dataDir, first.WorkflowID).IsActive {
				t.Fatalf("precondition: workflow was not active")
			}

			test.breakSetup(t, first, credentialID)
			if _, err := prepare(context.Background(), loaded, dataDir); err != nil {
				t.Fatalf("fail-closed prepare: %v", err)
			}
			workflow := loadManagedWorkflow(t, dataDir, first.WorkflowID)
			if workflow.IsActive {
				t.Fatalf("invalid persisted setup left workflow active")
			}
			assertManagedBindings(t, workflow, "https://default.example.test/daily.json", "", "v1")
			state, err := packsetup.LoadState(dataDir, loaded.Manifest)
			if err != nil || state.Completed {
				t.Fatalf("invalid setup was not downgraded: state=%#v err=%v", state, err)
			}
		})
	}
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

func writeSetupRunPackFiles(t *testing.T, dir, version, marker string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Join(dir, "workflows"), 0700); err != nil {
		t.Fatalf("mkdir workflows: %v", err)
	}
	manifest := map[string]interface{}{
		"schema_version":       1,
		"id":                   "example.setup-run",
		"name":                 "Setup Pack",
		"version":              version,
		"entry_workflow":       "workflows/main.json",
		"required_credentials": []string{},
		"supported_platforms":  []string{pack.CurrentPlatform()},
		"config_schema": []map[string]interface{}{
			{
				"key":      "source_url",
				"label":    "Source URL",
				"type":     "url",
				"required": true,
				"default":  "https://default.example.test/daily.json",
			},
		},
		"credential_requirements": []map[string]interface{}{
			{
				"key":       "telegram",
				"label":     "Telegram bot",
				"type":      "TELEGRAM_BOT",
				"required":  true,
				"test_kind": "telegram_get_me",
			},
		},
		"bindings": []map[string]interface{}{
			{
				"source": "config.source_url",
				"target": map[string]interface{}{"node_id": "fetch", "param": "url"},
			},
			{
				"source": "credential.telegram",
				"target": map[string]interface{}{"node_id": "send", "param": "credential_id"},
			},
		},
	}
	writeJSON(t, filepath.Join(dir, "pack.json"), manifest)
	workflowDef := map[string]interface{}{
		"name":        "Setup Pack",
		"description": "managed setup test workflow",
		"is_active":   true,
		"nodes": []map[string]interface{}{
			{
				"id":   "fetch",
				"type": "httpRequest",
				"params": map[string]interface{}{
					"method":  "GET",
					"url":     "https://default.example.test/daily.json",
					"headers": fmt.Sprintf(`{"X-Definition":%q}`, marker),
				},
			},
			{
				"id":   "send",
				"type": "telegramBot",
				"params": map[string]interface{}{
					"credential_id": "",
					"chat_id":       "@setup_test",
					"message":       "setup test",
				},
			},
		},
		"edges": []interface{}{},
	}
	writeJSON(t, filepath.Join(dir, "workflows", "main.json"), workflowDef)
}

func saveCompletedRunSetup(t *testing.T, loaded *pack.Pack, prepared *Prepared) string {
	t.Helper()
	if _, err := packsetup.SaveConfig(prepared.DataDir, loaded.Manifest, map[string]interface{}{
		"source_url": "https://source.example.test/daily.json",
	}); err != nil {
		t.Fatalf("save config: %v", err)
	}
	db, err := storage.NewDB(prepared.DBPath)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer db.Close()
	store := storage.NewCredentialStore(db, crypto.NewCryptoManager(prepared.MasterKey))
	credential, err := store.Create("Setup Telegram", "TELEGRAM_BOT", `{"token":"test-only-value"}`)
	if err != nil {
		t.Fatalf("create credential: %v", err)
	}
	if _, err := packsetup.SaveCredentialBindings(prepared.DataDir, loaded.Manifest, map[string]string{
		"telegram": credential.ID,
	}, packRunCredentialResolver(store)); err != nil {
		t.Fatalf("save credential bindings: %v", err)
	}
	return credential.ID
}

func loadManagedWorkflow(t *testing.T, dataDir, workflowID string) *storage.Workflow {
	t.Helper()
	db, err := storage.NewDB(filepath.Join(dataDir, "goflow.db"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer db.Close()
	workflow, err := storage.NewWorkflowStore(db).GetByID(workflowID)
	if err != nil {
		t.Fatalf("load managed workflow: %v", err)
	}
	return workflow
}

func assertManagedBindings(t *testing.T, workflow *storage.Workflow, wantURL, wantCredentialID, wantMarker string) {
	t.Helper()
	var nodeList []struct {
		ID     string                 `json:"id"`
		Params map[string]interface{} `json:"params"`
	}
	if err := json.Unmarshal([]byte(workflow.NodesJSON), &nodeList); err != nil {
		t.Fatalf("decode workflow nodes: %v", err)
	}
	paramsByID := make(map[string]map[string]interface{}, len(nodeList))
	for _, node := range nodeList {
		paramsByID[node.ID] = node.Params
	}
	if got := paramsByID["fetch"]["url"]; got != wantURL {
		t.Fatalf("source URL got %v want %q", got, wantURL)
	}
	if got := paramsByID["send"]["credential_id"]; got != wantCredentialID {
		t.Fatalf("credential binding got %v want %q", got, wantCredentialID)
	}
	wantHeaders := fmt.Sprintf(`{"X-Definition":%q}`, wantMarker)
	if got := paramsByID["fetch"]["headers"]; got != wantHeaders {
		t.Fatalf("latest definition marker got %v want %q", got, wantHeaders)
	}
}

func assertCredentialCount(t *testing.T, dataDir string, want int) {
	t.Helper()
	db, err := storage.NewDB(filepath.Join(dataDir, "goflow.db"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer db.Close()
	credentials, err := storage.NewCredentialStore(db, nil).ListAll()
	if err != nil {
		t.Fatalf("list credentials: %v", err)
	}
	if len(credentials) != want {
		t.Fatalf("credential count got %d want %d", len(credentials), want)
	}
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
