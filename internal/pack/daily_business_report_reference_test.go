package pack_test

import (
	"archive/zip"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
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

const dailyBusinessReportPackDir = "../../examples/packs/daily-business-report"

func TestDailyBusinessReportPackDeclaresProductSetup(t *testing.T) {
	loaded, err := pack.Load(dailyBusinessReportPackDir)
	if err != nil {
		t.Fatalf("load pack: %v", err)
	}
	if loaded.Manifest.ID != "official.daily-business-report" || loaded.Manifest.Version != "0.9.0" {
		t.Fatalf("unexpected pack identity: %s@%s", loaded.Manifest.ID, loaded.Manifest.Version)
	}
	fields := map[string]pack.ConfigField{}
	for _, field := range loaded.Manifest.ConfigSchema {
		fields[field.Key] = field
	}
	if fields["source_url"].TestKind != "http_json_contract" {
		t.Fatalf("source_url must declare contract testing")
	}
	ai := fields["ai_provider"]
	if ai.Type != "select" || fmt.Sprint(ai.Default) != "none" || len(ai.Options) != 3 {
		t.Fatalf("unexpected AI provider setup: %#v", ai)
	}
	creds := map[string]pack.CredentialRequirement{}
	for _, req := range loaded.Manifest.CredentialRequirements {
		creds[req.Key] = req
	}
	if !creds["telegram"].Required || creds["telegram"].TestKind != "telegram_get_me" {
		t.Fatalf("Telegram must remain the tested required destination credential")
	}
	if creds["openai"].Required || creds["deepseek"].Required {
		t.Fatalf("AI credentials must remain optional")
	}
	wf, err := workflow.ReadFileLimit(loaded.EntryWorkflowPath, pack.MaxWorkflowBytes)
	if err != nil {
		t.Fatalf("read workflow: %v", err)
	}
	if wf.MaxConcurrentRuns != 1 || wf.ConcurrencyPolicy != "reject" {
		t.Fatalf("flagship pack must reject duplicate active runs, got max=%d policy=%q", wf.MaxConcurrentRuns, wf.ConcurrencyPolicy)
	}
}

func TestDailyBusinessReportDefaultPathNeedsNoAIAndSendsOnce(t *testing.T) {
	result := executeDailyBusinessReportPack(t, "none")
	if result.execution.Status != "SUCCESS" {
		t.Fatalf("expected SUCCESS, got %s error=%s logs=%s", result.execution.Status, result.execution.ErrorMessage, result.execution.LogsJSON)
	}
	if result.telegramRequests != 1 {
		t.Fatalf("expected exactly one Telegram send, got %d", result.telegramRequests)
	}
	if result.openAICalls != 0 || result.deepSeekCalls != 0 {
		t.Fatalf("default path must not call AI, openai=%d deepseek=%d", result.openAICalls, result.deepSeekCalls)
	}
	for _, want := range []string{"Daily Business Report", "2026-08-18", "48250.75", "Orders: 314", "3 SKUs below threshold", "12.4%"} {
		if !strings.Contains(result.telegramBody, want) {
			t.Fatalf("Telegram body missing %q: %s", want, result.telegramBody)
		}
	}
}

func TestDailyBusinessReportOpenAIPathCallsOnlyOpenAI(t *testing.T) {
	result := executeDailyBusinessReportPack(t, "openai")
	if result.execution.Status != "SUCCESS" || result.telegramRequests != 1 {
		t.Fatalf("unexpected OpenAI path result: status=%s telegram=%d error=%s", result.execution.Status, result.telegramRequests, result.execution.ErrorMessage)
	}
	if result.openAICalls != 1 || result.deepSeekCalls != 0 {
		t.Fatalf("wrong AI branch calls: openai=%d deepseek=%d", result.openAICalls, result.deepSeekCalls)
	}
	if !strings.Contains(result.telegramBody, "AI note") || !strings.Contains(result.telegramBody, "OpenAI attention note") {
		t.Fatalf("OpenAI commentary missing: %s", result.telegramBody)
	}
}

func TestDailyBusinessReportDeepSeekPathCallsOnlyDeepSeek(t *testing.T) {
	result := executeDailyBusinessReportPack(t, "deepseek")
	if result.execution.Status != "SUCCESS" || result.telegramRequests != 1 {
		t.Fatalf("unexpected DeepSeek path result: status=%s telegram=%d error=%s", result.execution.Status, result.telegramRequests, result.execution.ErrorMessage)
	}
	if result.openAICalls != 0 || result.deepSeekCalls != 1 {
		t.Fatalf("wrong AI branch calls: openai=%d deepseek=%d", result.openAICalls, result.deepSeekCalls)
	}
	if !strings.Contains(result.telegramBody, "AI note") || !strings.Contains(result.telegramBody, "DeepSeek attention note") {
		t.Fatalf("DeepSeek commentary missing: %s", result.telegramBody)
	}
}

func TestDailyBusinessReportPackBuildsDeterministicallyAndExcludesRuntimeState(t *testing.T) {
	loaded, err := pack.Load(dailyBusinessReportPackDir)
	if err != nil {
		t.Fatalf("load pack: %v", err)
	}
	runtimePath := filepath.Join(t.TempDir(), "goflow-runtime")
	if err := os.WriteFile(runtimePath, []byte("runtime"), 0600); err != nil {
		t.Fatalf("write runtime fixture: %v", err)
	}
	archives := make([]string, 0, 2)
	for i := 0; i < 2; i++ {
		result, err := pack.Build(pack.BuildOptions{PackDir: loaded.Root, OutputDir: t.TempDir(), RuntimePath: runtimePath})
		if err != nil {
			t.Fatalf("build %d: %v", i, err)
		}
		if err := pack.VerifyBundleArchiveFile(result.ArchivePath); err != nil {
			t.Fatalf("verify archive %d: %v", i, err)
		}
		archives = append(archives, result.ArchivePath)
	}
	if first, second := sha256BusinessReportFile(t, archives[0]), sha256BusinessReportFile(t, archives[1]); first != second {
		t.Fatalf("expected deterministic archive hash, got %s and %s", first, second)
	}
	assertDailyBusinessReportArchiveSafe(t, archives[0])
}

type dailyBusinessReportResult struct {
	execution        *storage.Execution
	telegramRequests int
	telegramBody     string
	openAICalls      int32
	deepSeekCalls    int32
}

func executeDailyBusinessReportPack(t *testing.T, provider string) dailyBusinessReportResult {
	t.Helper()
	loaded, err := pack.Load(dailyBusinessReportPackDir)
	if err != nil {
		t.Fatalf("load pack: %v", err)
	}
	source := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"report_date":"2026-08-18","timezone":"Asia/Ho_Chi_Minh","revenue":48250.75,"order_count":314,"cancelled_refunded_count":7,"low_stock_summary":"3 SKUs below threshold","comparison_summary":"Revenue up 12.4% vs prior day"}`))
	}))
	defer source.Close()

	telegramRequests := 0
	telegramBody := ""
	telegram := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		telegramRequests++
		body, _ := io.ReadAll(r.Body)
		telegramBody = string(body)
		_, _ = w.Write([]byte(`{"ok":true,"result":{"message_id":1}}`))
	}))
	defer telegram.Close()

	db, err := storage.NewDB(filepath.Join(t.TempDir(), "goflow.db"))
	if err != nil {
		t.Fatalf("new db: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	wfStore := storage.NewWorkflowStore(db)
	execStore := storage.NewExecutionStore(db)
	credStore := storage.NewCredentialStore(db, crypto.NewCryptoManager("daily-business-report-test-master-key"))
	telegramCred, err := credStore.Create("Daily Business Report Telegram", "TELEGRAM_BOT", "fake-telegram-value")
	if err != nil {
		t.Fatalf("create telegram credential: %v", err)
	}

	dataDir := t.TempDir()
	cfg, err := packsetup.SaveConfig(dataDir, loaded.Manifest, map[string]interface{}{
		"source_url":  source.URL,
		"chat_id":     "@daily_business_report_test",
		"ai_provider": provider,
	})
	if err != nil {
		t.Fatalf("save config: %v", err)
	}
	credentialIDs := map[string]string{"telegram": telegramCred.ID}
	identities := map[string]packsetup.CredentialIdentity{
		telegramCred.ID: {ID: telegramCred.ID, Type: "TELEGRAM_BOT"},
	}
	if provider == "openai" {
		cred, createErr := credStore.Create("Daily Business Report OpenAI", "OPENAI_API_KEY", "fake-openai-value")
		if createErr != nil {
			t.Fatalf("create OpenAI credential: %v", createErr)
		}
		credentialIDs["openai"] = cred.ID
		identities[cred.ID] = packsetup.CredentialIdentity{ID: cred.ID, Type: "OPENAI_API_KEY"}
	}
	if provider == "deepseek" {
		cred, createErr := credStore.Create("Daily Business Report DeepSeek", "DEEPSEEK_API_KEY", "fake-deepseek-value")
		if createErr != nil {
			t.Fatalf("create DeepSeek credential: %v", createErr)
		}
		credentialIDs["deepseek"] = cred.ID
		identities[cred.ID] = packsetup.CredentialIdentity{ID: cred.ID, Type: "DEEPSEEK_API_KEY"}
	}
	creds, err := packsetup.SaveCredentialBindings(dataDir, loaded.Manifest, credentialIDs, packsetup.CredentialLookupFunc(func(id string) (packsetup.CredentialIdentity, error) {
		identity, ok := identities[id]
		if !ok {
			return packsetup.CredentialIdentity{}, fmt.Errorf("credential not found")
		}
		return identity, nil
	}))
	if err != nil {
		t.Fatalf("save credentials: %v", err)
	}

	wfDef, err := workflow.ReadFileLimit(loaded.EntryWorkflowPath, pack.MaxWorkflowBytes)
	if err != nil {
		t.Fatalf("read workflow: %v", err)
	}
	bound, err := packsetup.ApplyBindings(client.Workflow{
		ID:                "daily-business-report",
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
		ID:                "daily-business-report",
		Name:              bound.Name,
		Description:       bound.Description,
		IsActive:          true,
		NodesJSON:         bound.NodesJSON,
		EdgesJSON:         bound.EdgesJSON,
		InputSchemaJSON:   "{}",
		OutputSchemaJSON:  "{}",
		RiskLevel:         "low",
		MaxConcurrentRuns: 1,
		ConcurrencyPolicy: "reject",
	}
	if err := wfStore.Create(wf); err != nil {
		t.Fatalf("create workflow: %v", err)
	}

	var openAICalls int32
	var deepSeekCalls int32
	registry := nodes.NewPluginRegistry()
	for _, executor := range []nodes.NodeExecutor{
		nodes.NewHTTPRequestExecutorWithClient(source.Client()),
		nodes.NewConditionIFExecutor(),
		nodes.NewJSONTransformExecutor(),
		nodes.NewTelegramBotExecutorWithClient(telegram.Client(), telegram.URL),
		&fakeBusinessReportAIExecutor{nodeType: nodes.TypeOpenAIGPT, response: "OpenAI attention note", calls: &openAICalls},
		&fakeBusinessReportAIExecutor{nodeType: nodes.TypeDeepSeekAI, response: "DeepSeek attention note", calls: &deepSeekCalls},
	} {
		if err := registry.Register(executor); err != nil {
			t.Fatalf("register %s: %v", executor.GetDefinition().Type, err)
		}
	}
	eng := engine.NewEngine(registry, execStore, credStore, engine.NewEventBus(), wfStore)
	execution, err := eng.ExecuteWorkflow(wf, map[string]interface{}{})
	if err != nil && execution == nil {
		t.Fatalf("execute workflow: %v", err)
	}
	reloaded, err := execStore.GetByID(execution.ID)
	if err != nil {
		t.Fatalf("reload execution: %v", err)
	}
	return dailyBusinessReportResult{
		execution:        reloaded,
		telegramRequests: telegramRequests,
		telegramBody:     telegramBody,
		openAICalls:      atomic.LoadInt32(&openAICalls),
		deepSeekCalls:    atomic.LoadInt32(&deepSeekCalls),
	}
}

type fakeBusinessReportAIExecutor struct {
	nodeType nodes.NodeType
	response string
	calls    *int32
}

func (f *fakeBusinessReportAIExecutor) Execute(_ *nodes.ExecutionContext, _ *nodes.Node) (interface{}, error) {
	atomic.AddInt32(f.calls, 1)
	return map[string]interface{}{"ai_response": f.response}, nil
}

func (f *fakeBusinessReportAIExecutor) Validate(_ *nodes.Node) error { return nil }
func (f *fakeBusinessReportAIExecutor) GetDefinition() nodes.NodeDefinition {
	return nodes.NodeDefinition{Type: f.nodeType, Name: "Fake AI", Retryable: true}
}

func sha256BusinessReportFile(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read archive: %v", err)
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func assertDailyBusinessReportArchiveSafe(t *testing.T, path string) {
	t.Helper()
	reader, err := zip.OpenReader(path)
	if err != nil {
		t.Fatalf("open archive: %v", err)
	}
	defer reader.Close()
	for _, file := range reader.File {
		lower := strings.ToLower(file.Name)
		for _, forbidden := range []string{"goflow.db", "master.key", "pack-config.json", "pack-credentials.json", "pack-state.json", "run-state.json"} {
			if strings.Contains(lower, forbidden) {
				t.Fatalf("archive contains runtime state file %s", file.Name)
			}
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
		for _, secret := range []string{"fake-telegram-value", "fake-openai-value", "fake-deepseek-value", "daily-business-report-test-master-key"} {
			if strings.Contains(string(data), secret) {
				t.Fatalf("archive file %s contains test secret material", file.Name)
			}
		}
	}
}
