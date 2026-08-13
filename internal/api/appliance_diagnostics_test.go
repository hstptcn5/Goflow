package api

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"goflow/internal/packsetup"
	"goflow/internal/storage"
)

func TestApplianceDiagnosticsGoldenRedactedAndRestartStable(t *testing.T) {
	dataDir := t.TempDir()
	db, err := storage.NewDB(filepath.Join(dataDir, "goflow.db"))
	if err != nil {
		t.Fatalf("NewDB: %v", err)
	}
	defer db.Close()
	wfStore := storage.NewWorkflowStore(db)
	if err := wfStore.Create(&storage.Workflow{
		ID: "wf-secret-id", Name: "DailyOps", IsActive: true, NodesJSON: "[]", EdgesJSON: "[]",
	}); err != nil {
		t.Fatalf("create workflow: %v", err)
	}
	appliance := &ApplianceContext{
		Enabled: true, AppVersion: "0.5.0-test", IntegrityState: "verified",
		PackID: "official.dailyops-rest-telegram", PackVersion: "0.3.0",
		WorkflowID: "wf-secret-id", DataDir: dataDir,
		ScheduleStore: storage.NewWorkflowScheduleStore(db),
	}
	if _, err := packsetup.SaveState(dataDir, applianceManifest(appliance), true, time.Now()); err != nil {
		t.Fatalf("save setup state: %v", err)
	}
	started := time.Date(2026, 8, 9, 1, 2, 3, 0, time.UTC)
	rawError := "goflow-safe-error:source_invalid_json:token-canary https://mock.invalid/feed?token=query-secret chat_id=-100314 payload-secret response-secret " + dataDir
	if _, err := db.WriteDB.Exec(`
		INSERT INTO executions (id, workflow_id, status, duration_ms, logs_json, started_at, finished_at, input_json, error_message)
		VALUES ('exec-secret-id', ?, 'FAILED', 1250, ?, ?, ?, ?, ?)
	`, appliance.WorkflowID, `[{"secret":"log-canary"}]`, started, started.Add(1250*time.Millisecond), `{"payload":"input-canary"}`, rawError); err != nil {
		t.Fatalf("insert execution: %v", err)
	}

	first, err := json.MarshalIndent(buildApplianceDiagnostics(appliance, wfStore, storage.NewExecutionStore(db), nil), "", "  ")
	if err != nil {
		t.Fatalf("marshal diagnostics: %v", err)
	}
	restarted := *appliance
	second, err := json.MarshalIndent(buildApplianceDiagnostics(&restarted, wfStore, storage.NewExecutionStore(db), nil), "", "  ")
	if err != nil {
		t.Fatalf("marshal restarted diagnostics: %v", err)
	}
	if !bytes.Equal(first, second) {
		t.Fatalf("diagnostics changed after restart:\nfirst=%s\nsecond=%s", first, second)
	}
	for _, forbidden := range []string{
		"wf-secret-id", "exec-secret-id", "token-canary", "query-secret", "-100314",
		"payload-secret", "response-secret", "log-canary", "input-canary", dataDir,
	} {
		if strings.Contains(string(first), forbidden) {
			t.Fatalf("diagnostics contain forbidden value %q: %s", forbidden, first)
		}
	}
	normalized := bytes.ReplaceAll(first, []byte(runtime.GOOS+"/"+runtime.GOARCH), []byte("<platform>"))
	want, err := os.ReadFile(filepath.Join("testdata", "appliance-diagnostics.golden.json"))
	if err != nil {
		t.Fatalf("read golden diagnostics: %v", err)
	}
	want = bytes.ReplaceAll(want, []byte("\r\n"), []byte("\n"))
	if !bytes.Equal(bytes.TrimSpace(normalized), bytes.TrimSpace(want)) {
		t.Fatalf("diagnostics golden mismatch:\nwant=%s\ngot=%s", want, normalized)
	}
}

func TestAppliancePublicExecutionErrorRejectsMalformedLegacySecrets(t *testing.T) {
	category, message := appliancePublicExecutionError("FAILED", "goflow-safe-error:legacy-secret:token-canary C:\\Users\\Pilot\\AppData")
	if category != "internal_error" || strings.Contains(message, "token-canary") || strings.Contains(message, "Pilot") {
		t.Fatalf("unsafe legacy error was exposed: category=%q message=%q", category, message)
	}
}
