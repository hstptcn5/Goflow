package pack_test

import (
	"archive/zip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"goflow/internal/client"
	"goflow/internal/crypto"
	"goflow/internal/engine"
	"goflow/internal/nodes"
	"goflow/internal/pack"
	"goflow/internal/packsetup"
	"goflow/internal/storage"
	"goflow/internal/workflow"
)

const dailyOpsPackDir = "../../examples/packs/dailyops-rest-telegram"

func TestDailyOpsReferencePackMockRunSendsExpectedTelegramOnce(t *testing.T) {
	source := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/dailyops.json" {
			t.Fatalf("unexpected source path %s", r.URL.Path)
		}
		_, _ = w.Write([]byte(`{
			"report_date":"2026-08-09",
			"timezone":"Asia/Bangkok",
			"revenue":1250000,
			"order_count":42,
			"cancelled_refunded_count":3,
			"low_stock_summary":"2 items under threshold",
			"comparison_summary":"Revenue up 8% vs previous day"
		}`))
	}))
	defer source.Close()

	var telegramRequests int
	var telegramBody string
	telegram := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		telegramRequests++
		if !strings.HasSuffix(r.URL.Path, "/sendMessage") {
			t.Fatalf("unexpected telegram path %s", r.URL.Path)
		}
		body, _ := io.ReadAll(r.Body)
		telegramBody = string(body)
		_, _ = w.Write([]byte(`{"ok":true,"result":{"message_id":1}}`))
	}))
	defer telegram.Close()

	exec := executeDailyOpsPack(t, source.URL+"/dailyops.json", telegram.URL, telegram.Client())
	if exec.Status != "SUCCESS" {
		t.Fatalf("expected SUCCESS execution, got %s error=%s logs=%s", exec.Status, exec.ErrorMessage, exec.LogsJSON)
	}
	if telegramRequests != 1 {
		t.Fatalf("expected one telegram request, got %d", telegramRequests)
	}
	for _, want := range []string{"DailyOps Daily Report", "2026-08-09", "1250000", "Orders: 42", "2 items under threshold", "Revenue up 8%"} {
		if !strings.Contains(telegramBody, want) {
			t.Fatalf("telegram body missing %q: %s", want, telegramBody)
		}
	}
	if strings.Contains(telegramBody, "credential_id") {
		t.Fatalf("telegram body leaked credential id: %s", telegramBody)
	}
}

func TestDailyOpsReferencePackTelegramFailureDoesNotDuplicateSend(t *testing.T) {
	source := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{
			"report_date":"2026-08-09",
			"timezone":"Asia/Bangkok",
			"revenue":100,
			"order_count":1,
			"cancelled_refunded_count":0,
			"low_stock_summary":"none",
			"comparison_summary":"flat"
		}`))
	}))
	defer source.Close()

	var telegramRequests int
	telegram := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		telegramRequests++
		w.WriteHeader(http.StatusBadGateway)
		_, _ = w.Write([]byte(`{"ok":false,"description":"temporary rejection"}`))
	}))
	defer telegram.Close()

	exec := executeDailyOpsPack(t, source.URL, telegram.URL, telegram.Client())
	if exec.Status != "FAILED" {
		t.Fatalf("expected FAILED execution, got %s", exec.Status)
	}
	if telegramRequests != 1 {
		t.Fatalf("non-idempotent telegram send should not retry, got %d requests", telegramRequests)
	}
	if strings.Contains(exec.ErrorMessage, "fake-telegram-value") {
		t.Fatalf("execution error leaked credential value: %s", exec.ErrorMessage)
	}
}

func TestDailyOpsReferencePackBuildsDeterministicallyAndVerifies(t *testing.T) {
	loaded, err := pack.Load(dailyOpsPackDir)
	if err != nil {
		t.Fatalf("load pack: %v", err)
	}
	runtimePath := filepath.Join(t.TempDir(), "goflow-runtime")
	if err := os.WriteFile(runtimePath, []byte("runtime"), 0600); err != nil {
		t.Fatalf("write runtime fixture: %v", err)
	}
	archives := make([]string, 0, 2)
	for i := 0; i < 2; i++ {
		outputDir := t.TempDir()
		result, err := pack.Build(pack.BuildOptions{
			PackDir:     loaded.Root,
			OutputDir:   outputDir,
			RuntimePath: runtimePath,
		})
		if err != nil {
			t.Fatalf("build %d: %v", i, err)
		}
		if err := pack.VerifyBundleArchiveFile(result.ArchivePath); err != nil {
			t.Fatalf("verify archive %d: %v", i, err)
		}
		archives = append(archives, result.ArchivePath)
	}
	first := sha256File(t, archives[0])
	second := sha256File(t, archives[1])
	if first != second {
		t.Fatalf("expected deterministic archive hash, got %s and %s", first, second)
	}
	assertArchiveExcludesDailyOpsState(t, archives[0])
}

func executeDailyOpsPack(t *testing.T, sourceURL, telegramURL string, telegramClient *http.Client) *storage.Execution {
	t.Helper()
	loaded, err := pack.Load(dailyOpsPackDir)
	if err != nil {
		t.Fatalf("load pack: %v", err)
	}
	db, err := storage.NewDB(filepath.Join(t.TempDir(), "goflow.db"))
	if err != nil {
		t.Fatalf("new db: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	wfStore := storage.NewWorkflowStore(db)
	execStore := storage.NewExecutionStore(db)
	credStore := storage.NewCredentialStore(db, crypto.NewCryptoManager("dailyops-test-master-key"))
	cred, err := credStore.Create("DailyOps Telegram", "TELEGRAM_BOT", "fake-telegram-value")
	if err != nil {
		t.Fatalf("create credential: %v", err)
	}
	dataDir := t.TempDir()
	cfg, err := packsetup.SaveConfig(dataDir, loaded.Manifest, map[string]interface{}{
		"source_url":          sourceURL,
		"chat_id":             "@dailyops_demo",
		"report_title":        "DailyOps Daily Report",
		"low_stock_threshold": 5,
	})
	if err != nil {
		t.Fatalf("save config: %v", err)
	}
	creds, err := packsetup.SaveCredentialBindings(dataDir, loaded.Manifest, map[string]string{"telegram": cred.ID}, packsetup.CredentialLookupFunc(func(id string) (packsetup.CredentialIdentity, error) {
		return packsetup.CredentialIdentity{ID: cred.ID, Type: "TELEGRAM_BOT"}, nil
	}))
	if err != nil {
		t.Fatalf("save credentials: %v", err)
	}
	wfDef, err := workflow.ReadFileLimit(loaded.EntryWorkflowPath, pack.MaxWorkflowBytes)
	if err != nil {
		t.Fatalf("read workflow: %v", err)
	}
	bound, err := packsetup.ApplyBindings(client.Workflow{
		ID:                "dailyops",
		Name:              wfDef.Name,
		Description:       wfDef.Description,
		IsActive:          true,
		NodesJSON:         wfDef.NodesJSON,
		EdgesJSON:         wfDef.EdgesJSON,
		InputSchemaJSON:   wfDef.InputSchemaJSON,
		OutputSchemaJSON:  wfDef.OutputSchemaJSON,
		RiskLevel:         wfDef.RiskLevel,
		MaxConcurrentRuns: wfDef.MaxConcurrentRuns,
		ConcurrencyPolicy: wfDef.ConcurrencyPolicy,
	}, loaded.Manifest, cfg.Values, creds.Slots)
	if err != nil {
		t.Fatalf("apply bindings: %v", err)
	}
	wf := &storage.Workflow{
		ID:                "dailyops",
		Name:              bound.Name,
		Description:       bound.Description,
		IsActive:          true,
		NodesJSON:         bound.NodesJSON,
		EdgesJSON:         bound.EdgesJSON,
		InputSchemaJSON:   "{}",
		OutputSchemaJSON:  "{}",
		RiskLevel:         "low",
		MaxConcurrentRuns: 1,
		ConcurrencyPolicy: "global",
	}
	if err := wfStore.Create(wf); err != nil {
		t.Fatalf("create workflow: %v", err)
	}
	registry := nodes.NewPluginRegistry()
	if err := registry.Register(nodes.NewHTTPRequestExecutorWithClient(http.DefaultClient)); err != nil {
		t.Fatalf("register http: %v", err)
	}
	if err := registry.Register(nodes.NewJSONTransformExecutor()); err != nil {
		t.Fatalf("register transform: %v", err)
	}
	if err := registry.Register(nodes.NewTelegramBotExecutorWithClient(telegramClient, telegramURL)); err != nil {
		t.Fatalf("register telegram: %v", err)
	}
	eng := engine.NewEngine(registry, execStore, credStore, engine.NewEventBus(), wfStore)
	exec, err := eng.ExecuteWorkflow(wf, map[string]interface{}{})
	if err != nil && exec == nil {
		t.Fatalf("execute workflow: %v", err)
	}
	if exec == nil {
		t.Fatalf("missing execution")
	}
	reloaded, err := execStore.GetByID(exec.ID)
	if err != nil {
		t.Fatalf("reload execution: %v", err)
	}
	return reloaded
}

func sha256File(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read archive: %v", err)
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func assertArchiveExcludesDailyOpsState(t *testing.T, path string) {
	t.Helper()
	reader, err := zip.OpenReader(path)
	if err != nil {
		t.Fatalf("open archive: %v", err)
	}
	defer reader.Close()
	for _, file := range reader.File {
		if strings.Contains(file.Name, "goflow.db") || strings.Contains(file.Name, "master.key") || strings.Contains(file.Name, "pack-config.json") || strings.Contains(file.Name, "pack-state.json") {
			t.Fatalf("archive contains runtime state file %s", file.Name)
		}
		handle, err := file.Open()
		if err != nil {
			t.Fatalf("open %s: %v", file.Name, err)
		}
		data, err := io.ReadAll(io.LimitReader(handle, 1<<20))
		_ = handle.Close()
		if err != nil {
			t.Fatalf("read %s: %v", file.Name, err)
		}
		if strings.Contains(string(data), "fake-telegram-value") || strings.Contains(string(data), "dailyops-test-master-key") {
			t.Fatalf("archive file %s contains test secret material", file.Name)
		}
		var decoded map[string]interface{}
		_ = json.Unmarshal(data, &decoded)
	}
}
