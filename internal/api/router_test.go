package api

import (
	"bytes"
	"encoding/json"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"goflow/internal/crypto"
	"goflow/internal/engine"
	"goflow/internal/nodes"
	"goflow/internal/pack"
	"goflow/internal/packsetup"
	"goflow/internal/storage"
)

func TestRouterAllowsConfiguredMCPOriginInGlobalCORS(t *testing.T) {
	router := NewRouter(nil, nil, nil, nil, nil, nil, nil, nil, nil, "", 60, "http://127.0.0.1:8080", []string{"https://agent.example.com"}, 2, 30)
	req := httptest.NewRequest(http.MethodOptions, "/mcp", nil)
	req.Header.Set("Origin", "https://agent.example.com")
	req.Header.Set("Access-Control-Request-Method", "POST")
	req.Header.Set("Access-Control-Request-Headers", "Authorization, Content-Type")
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)
	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "https://agent.example.com" {
		t.Fatalf("expected custom MCP CORS origin, got %q; status=%d body=%s", got, rec.Code, rec.Body.String())
	}
}

func TestApplianceRoutesAreAbsentInGenericMode(t *testing.T) {
	router := NewRouter(nil, nil, nil, nil, nil, nil, nil, nil, nil, "", 60, "http://127.0.0.1:8080", nil, 2, 30)
	req := httptest.NewRequest(http.MethodGet, "/api/appliance/bootstrap", nil)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected generic appliance route 404, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestApplianceBootstrapAndMutationGuard(t *testing.T) {
	appliance := &ApplianceContext{
		Enabled:      true,
		Origin:       "http://example.com",
		SessionToken: "test-session-token",
		PackID:       "example.appliance",
		PackName:     "Example Appliance",
		PackVersion:  "0.1.0",
		Description:  "Example",
		WorkflowID:   "wf-1",
		DataDir:      t.TempDir(),
	}
	router := NewRouter(nil, nil, nil, nil, nil, nil, nil, nil, nil, "", 60, "http://127.0.0.1:8080", nil, 2, 30, appliance)

	req := httptest.NewRequest(http.MethodGet, "/api/appliance/bootstrap", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected bootstrap 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var bootstrap map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &bootstrap); err != nil {
		t.Fatalf("decode bootstrap: %v", err)
	}
	if bootstrap["token"] != "test-session-token" {
		t.Fatalf("unexpected bootstrap response: %#v", bootstrap)
	}

	mutation := func(origin, token, contentType, host string) int {
		req := httptest.NewRequest(http.MethodPost, "/api/appliance/setup/complete", bytes.NewBufferString(`{}`))
		req.Host = host
		if origin != "" {
			req.Header.Set("Origin", origin)
		}
		if token != "" {
			req.Header.Set(applianceTokenHeader, token)
		}
		if contentType != "" {
			req.Header.Set("Content-Type", contentType)
		}
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		return rec.Code
	}
	if got := mutation("http://example.com", "test-session-token", "application/json", "example.com"); got != http.StatusOK {
		t.Fatalf("expected guarded mutation to reach handler, got %d", got)
	}
	if got := mutation("http://evil.example", "test-session-token", "application/json", "example.com"); got != http.StatusForbidden {
		t.Fatalf("expected bad origin 403, got %d", got)
	}
	if got := mutation("http://example.com", "wrong", "application/json", "example.com"); got != http.StatusForbidden {
		t.Fatalf("expected bad token 403, got %d", got)
	}
	if got := mutation("http://example.com", "test-session-token", "text/plain", "example.com"); got != http.StatusUnsupportedMediaType {
		t.Fatalf("expected bad content type 415, got %d", got)
	}
	if got := mutation("http://example.com", "test-session-token", "application/json", "evil.example"); got != http.StatusNotFound {
		t.Fatalf("expected bad host 404, got %d", got)
	}
}

func TestRouterServesSPAIndexForFrontendRoutes(t *testing.T) {
	uiFS := fstest.MapFS{
		"index.html": &fstest.MapFile{Data: []byte(`<div id="app"></div>`)},
		"NODES.md":   &fstest.MapFile{Data: []byte(`# Nodes`)},
	}
	router := NewRouter(nil, nil, nil, nil, nil, nil, nil, nil, fs.FS(uiFS), "", 60, "http://127.0.0.1:8080", nil, 2, 30)

	req := httptest.NewRequest(http.MethodGet, "/workflows/wf-1", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected frontend route to serve index, got %d: %s", rec.Code, rec.Body.String())
	}
	if rec.Body.String() != `<div id="app"></div>` {
		t.Fatalf("expected index content, got %q", rec.Body.String())
	}

	req = httptest.NewRequest(http.MethodGet, "/NODES.md", nil)
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Body.String() != `# Nodes` {
		t.Fatalf("expected real static asset to be served, got %q", rec.Body.String())
	}
}

func TestApplianceSetupReadinessAndRedaction(t *testing.T) {
	db, err := storage.NewDB(filepath.Join(t.TempDir(), "goflow.db"))
	if err != nil {
		t.Fatalf("NewDB: %v", err)
	}
	defer db.Close()
	credStore := storage.NewCredentialStore(db, crypto.NewCryptoManager("test-master-key"))
	cred, err := credStore.Create("Telegram", "TELEGRAM_BOT", "123:secret-token")
	if err != nil {
		t.Fatalf("create credential: %v", err)
	}
	appliance := &ApplianceContext{
		Enabled:      true,
		Origin:       "http://example.com",
		SessionToken: "test-session-token",
		PackID:       "example.appliance",
		PackName:     "Example Appliance",
		PackVersion:  "0.1.0",
		Description:  "Example",
		WorkflowID:   "wf-1",
		DataDir:      t.TempDir(),
		ConfigSchema: []pack.ConfigField{
			{Key: "source_url", Label: "Source URL", Type: "url", Required: true},
		},
		CredentialRequirements: []pack.CredentialRequirement{
			{Key: "telegram", Label: "Telegram", Type: "TELEGRAM_BOT", Required: true},
		},
	}
	router := NewRouter(nil, nil, credStore, nil, nil, nil, nil, nil, nil, "", 60, "http://127.0.0.1:8080", nil, 2, 30, appliance)

	statusBody := applianceRequest(t, router, http.MethodGet, "/api/appliance/status", nil, nil)
	if !strings.Contains(statusBody, "NEEDS_SETUP") || !strings.Contains(statusBody, "config.source_url") || !strings.Contains(statusBody, "credential.telegram") {
		t.Fatalf("expected missing setup status, got %s", statusBody)
	}

	applianceRequest(t, router, http.MethodPost, "/api/appliance/setup/config", []byte(`{"values":{"source_url":"https://example.test/data.json"}}`), map[string]string{
		"Origin":             "http://example.com",
		applianceTokenHeader: "test-session-token",
		"Content-Type":       "application/json",
	})
	credentialBody := applianceRequest(t, router, http.MethodPost, "/api/appliance/setup/credentials", []byte(`{"slots":{"telegram":"`+cred.ID+`"}}`), map[string]string{
		"Origin":             "http://example.com",
		applianceTokenHeader: "test-session-token",
		"Content-Type":       "application/json",
	})
	if strings.Contains(credentialBody, cred.ID) || strings.Contains(credentialBody, "secret-token") {
		t.Fatalf("credential response leaked secret material: %s", credentialBody)
	}
	statusBody = applianceRequest(t, router, http.MethodGet, "/api/appliance/status", nil, nil)
	if !strings.Contains(statusBody, "NEEDS_SETUP") || !strings.Contains(statusBody, `"can_complete":true`) {
		t.Fatalf("expected completable setup status, got %s", statusBody)
	}
	applianceRequest(t, router, http.MethodPost, "/api/appliance/setup/complete", []byte(`{}`), map[string]string{
		"Origin":             "http://example.com",
		applianceTokenHeader: "test-session-token",
		"Content-Type":       "application/json",
	})
	statusBody = applianceRequest(t, router, http.MethodGet, "/api/appliance/status", nil, nil)
	if !strings.Contains(statusBody, "READY") || !strings.Contains(statusBody, `"setup_complete":true`) {
		t.Fatalf("expected ready status, got %s", statusBody)
	}
	setupBody := applianceRequest(t, router, http.MethodGet, "/api/appliance/setup", nil, nil)
	if !strings.Contains(setupBody, `"assigned":true`) || strings.Contains(setupBody, cred.ID) || strings.Contains(setupBody, "secret-token") {
		t.Fatalf("setup response was not redacted: %s", setupBody)
	}
}

func TestApplianceCreateCredentialStoresEncryptedAndRedactsResponse(t *testing.T) {
	db, err := storage.NewDB(filepath.Join(t.TempDir(), "goflow.db"))
	if err != nil {
		t.Fatalf("NewDB: %v", err)
	}
	defer db.Close()
	credStore := storage.NewCredentialStore(db, crypto.NewCryptoManager("test-master-key"))
	appliance := &ApplianceContext{
		Enabled:      true,
		Origin:       "http://example.com",
		SessionToken: "test-session-token",
		PackID:       "example.appliance",
		PackName:     "Example Appliance",
		PackVersion:  "0.1.0",
		WorkflowID:   "wf-1",
		DataDir:      t.TempDir(),
		CredentialRequirements: []pack.CredentialRequirement{
			{Key: "telegram", Label: "Telegram", Type: "TELEGRAM_BOT", Required: true},
		},
	}
	router := NewRouter(nil, nil, credStore, nil, nil, nil, nil, nil, nil, "", 60, "http://127.0.0.1:8080", nil, 2, 30, appliance)
	body := applianceRequest(t, router, http.MethodPost, "/api/appliance/setup/credentials/create", []byte(`{"key":"telegram","name":"Telegram","value":"123:secret-token"}`), map[string]string{
		"Origin":             "http://example.com",
		applianceTokenHeader: "test-session-token",
		"Content-Type":       "application/json",
	})
	if strings.Contains(body, "secret-token") {
		t.Fatalf("create response leaked credential value: %s", body)
	}
	credentials, err := credStore.ListAll()
	if err != nil {
		t.Fatalf("list credentials: %v", err)
	}
	if len(credentials) != 1 || credentials[0].Type != "TELEGRAM_BOT" {
		t.Fatalf("unexpected credentials: %#v", credentials)
	}
	if strings.Contains(body, credentials[0].ID) {
		t.Fatalf("create response leaked credential id: %s", body)
	}
	decrypted, err := credStore.GetDecryptedData(credentials[0].ID)
	if err != nil {
		t.Fatalf("decrypt credential: %v", err)
	}
	if decrypted != "123:secret-token" {
		t.Fatalf("unexpected decrypted credential")
	}
}

func TestApplianceTelegramConnectionTestUsesGetMeAndRedactsFailure(t *testing.T) {
	db, err := storage.NewDB(filepath.Join(t.TempDir(), "goflow.db"))
	if err != nil {
		t.Fatalf("NewDB: %v", err)
	}
	defer db.Close()
	credStore := storage.NewCredentialStore(db, crypto.NewCryptoManager("test-master-key"))
	cred, err := credStore.Create("Telegram", "TELEGRAM_BOT", "123:secret-token")
	if err != nil {
		t.Fatalf("create credential: %v", err)
	}
	var observedPath string
	telegram := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		observedPath = r.URL.Path
		if strings.Contains(r.URL.Path, "sendMessage") {
			t.Fatalf("connection test must not send messages")
		}
		_, _ = w.Write([]byte(`{"ok":true,"result":{"id":123}}`))
	}))
	defer telegram.Close()
	appliance := &ApplianceContext{
		Enabled:              true,
		Origin:               "http://example.com",
		SessionToken:         "test-session-token",
		PackID:               "example.appliance",
		PackName:             "Example Appliance",
		PackVersion:          "0.1.0",
		WorkflowID:           "wf-1",
		DataDir:              t.TempDir(),
		TelegramAPIBaseURL:   telegram.URL,
		ConnectionTestClient: telegram.Client(),
		CredentialRequirements: []pack.CredentialRequirement{
			{Key: "telegram", Label: "Telegram", Type: "TELEGRAM_BOT", Required: true, TestKind: "telegram_get_me"},
		},
	}
	if _, err := packsetup.SaveCredentialBindings(appliance.DataDir, applianceManifest(appliance), map[string]string{"telegram": cred.ID}, applianceCredentialResolver(credStore)); err != nil {
		t.Fatalf("save credential binding: %v", err)
	}
	router := NewRouter(nil, nil, credStore, nil, nil, nil, nil, nil, nil, "", 60, "http://127.0.0.1:8080", nil, 2, 30, appliance)
	body := applianceRequest(t, router, http.MethodPost, "/api/appliance/setup/credentials/test", []byte(`{"key":"telegram"}`), map[string]string{
		"Origin":             "http://example.com",
		applianceTokenHeader: "test-session-token",
		"Content-Type":       "application/json",
	})
	if !strings.Contains(body, `"status":"OK"`) {
		t.Fatalf("expected OK test response, got %s", body)
	}
	if observedPath != "/bot123:secret-token/getMe" {
		t.Fatalf("expected getMe path, got %q", observedPath)
	}

	failingTelegram := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"ok":false,"description":"bad bot123:secret-token"}`))
	}))
	defer failingTelegram.Close()
	appliance.TelegramAPIBaseURL = failingTelegram.URL
	appliance.ConnectionTestClient = failingTelegram.Client()
	body = applianceRequestStatus(t, router, http.MethodPost, "/api/appliance/setup/credentials/test", []byte(`{"key":"telegram"}`), map[string]string{
		"Origin":             "http://example.com",
		applianceTokenHeader: "test-session-token",
		"Content-Type":       "application/json",
	}, http.StatusBadGateway)
	if strings.Contains(body, "secret-token") {
		t.Fatalf("failure response leaked token: %s", body)
	}
}

func TestApplianceWorkflowRunAndExecutionSummariesAreRedacted(t *testing.T) {
	db, err := storage.NewDB(filepath.Join(t.TempDir(), "goflow.db"))
	if err != nil {
		t.Fatalf("NewDB: %v", err)
	}
	defer db.Close()
	wfStore := storage.NewWorkflowStore(db)
	execStore := storage.NewExecutionStore(db)
	credStore := storage.NewCredentialStore(db, crypto.NewCryptoManager("test-master-key"))
	registry := nodes.NewPluginRegistry()
	if err := registry.Register(applianceTestExecutor{}); err != nil {
		t.Fatalf("register executor: %v", err)
	}
	eventBus := engine.NewEventBus()
	eng := engine.NewEngine(registry, execStore, credStore, eventBus, wfStore)
	appliance := &ApplianceContext{
		Enabled:      true,
		Origin:       "http://example.com",
		SessionToken: "test-session-token",
		PackID:       "example.appliance",
		PackName:     "Example Appliance",
		PackVersion:  "0.1.0",
		WorkflowID:   "wf-run",
		DataDir:      t.TempDir(),
	}
	if _, err := packsetup.SaveState(appliance.DataDir, applianceManifest(appliance), true, time.Now()); err != nil {
		t.Fatalf("save state: %v", err)
	}
	if err := wfStore.Create(&storage.Workflow{
		ID:          "wf-run",
		Name:        "Run Me",
		IsActive:    true,
		NodesJSON:   `[{"id":"n1","type":"applianceTestAction","name":"Action","params":{}}]`,
		EdgesJSON:   `[]`,
		RiskLevel:   "low",
		ExposeCLI:   false,
		ExposeMCP:   false,
		Description: "Managed workflow",
	}); err != nil {
		t.Fatalf("create workflow: %v", err)
	}
	router := NewRouter(wfStore, execStore, credStore, nil, nil, registry, eng, eventBus, nil, "", 60, "http://127.0.0.1:8080", nil, 2, 30, appliance)
	runBody := applianceRequestStatus(t, router, http.MethodPost, "/api/appliance/workflow/run", []byte(`{"input":{"api_token":"secret-canary"},"idempotency_key":"secret-idem"}`), map[string]string{
		"Origin":             "http://example.com",
		applianceTokenHeader: "test-session-token",
		"Content-Type":       "application/json; charset=utf-8",
	}, http.StatusAccepted)
	if strings.Contains(runBody, "secret-canary") || strings.Contains(runBody, "secret-idem") {
		t.Fatalf("run response leaked sensitive input: %s", runBody)
	}
	deadline := time.Now().Add(2 * time.Second)
	for {
		latestBody := applianceRequest(t, router, http.MethodGet, "/api/appliance/executions/latest", nil, nil)
		if strings.Contains(latestBody, `"status":"SUCCESS"`) {
			if strings.Contains(latestBody, "secret-canary") || strings.Contains(latestBody, "secret-idem") || strings.Contains(latestBody, "secret-output") || strings.Contains(latestBody, "logs_json") {
				t.Fatalf("latest execution response leaked sensitive material: %s", latestBody)
			}
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("execution did not finish; latest=%s", latestBody)
		}
		time.Sleep(10 * time.Millisecond)
	}
	recentBody := applianceRequest(t, router, http.MethodGet, "/api/appliance/executions?limit=100", nil, nil)
	if !strings.Contains(recentBody, `"limit":50`) || strings.Contains(recentBody, "secret-output") || strings.Contains(recentBody, "logs_json") {
		t.Fatalf("recent executions response was not bounded/redacted: %s", recentBody)
	}
	diagnosticsBody := applianceRequest(t, router, http.MethodGet, "/api/appliance/diagnostics", nil, nil)
	if strings.Contains(diagnosticsBody, "secret-canary") || strings.Contains(diagnosticsBody, "secret-idem") || strings.Contains(diagnosticsBody, "secret-output") || strings.Contains(diagnosticsBody, "goflow.db") {
		t.Fatalf("diagnostics leaked sensitive material: %s", diagnosticsBody)
	}
}

func TestApplianceRunRejectsMissingSetupWithLogicalRequirements(t *testing.T) {
	appliance := &ApplianceContext{
		Enabled:      true,
		Origin:       "http://example.com",
		SessionToken: "test-session-token",
		PackID:       "example.appliance",
		PackName:     "Example Appliance",
		PackVersion:  "0.1.0",
		WorkflowID:   "wf-1",
		DataDir:      t.TempDir(),
		ConfigSchema: []pack.ConfigField{
			{Key: "source_url", Label: "Source URL", Type: "url", Required: true},
		},
	}
	router := NewRouter(nil, nil, nil, nil, nil, nil, nil, nil, nil, "", 60, "http://127.0.0.1:8080", nil, 2, 30, appliance)
	body := applianceRequestStatus(t, router, http.MethodPost, "/api/appliance/workflow/run", []byte(`{"input":{"password":"secret-canary"}}`), map[string]string{
		"Origin":             "http://example.com",
		applianceTokenHeader: "test-session-token",
		"Content-Type":       "application/json",
	}, http.StatusConflict)
	if !strings.Contains(body, "config.source_url") || strings.Contains(body, "secret-canary") {
		t.Fatalf("missing setup response was not logical/redacted: %s", body)
	}
}

func TestApplianceCredentialTestRateLimit(t *testing.T) {
	db, err := storage.NewDB(filepath.Join(t.TempDir(), "goflow.db"))
	if err != nil {
		t.Fatalf("NewDB: %v", err)
	}
	defer db.Close()
	credStore := storage.NewCredentialStore(db, crypto.NewCryptoManager("test-master-key"))
	appliance := &ApplianceContext{
		Enabled:      true,
		Origin:       "http://example.com",
		SessionToken: "test-session-token",
		PackID:       "example.appliance",
		PackName:     "Example Appliance",
		PackVersion:  "0.1.0",
		WorkflowID:   "wf-1",
		DataDir:      t.TempDir(),
		CredentialRequirements: []pack.CredentialRequirement{
			{Key: "telegram", Label: "Telegram", Type: "TELEGRAM_BOT", Required: true},
		},
	}
	router := NewRouter(nil, nil, credStore, nil, nil, nil, nil, nil, nil, "", 60, "http://127.0.0.1:8080", nil, 2, 30, appliance)
	headers := map[string]string{
		"Origin":             "http://example.com",
		applianceTokenHeader: "test-session-token",
		"Content-Type":       "application/json",
	}
	for i := 0; i < 10; i++ {
		body := applianceRequest(t, router, http.MethodPost, "/api/appliance/setup/credentials/test", []byte(`{"key":"telegram"}`), headers)
		if !strings.Contains(body, `"status":"SKIPPED"`) {
			t.Fatalf("expected skipped response, got %s", body)
		}
	}
	applianceRequestStatus(t, router, http.MethodPost, "/api/appliance/setup/credentials/test", []byte(`{"key":"telegram"}`), headers, http.StatusTooManyRequests)
}

func TestApplianceStatusDetectsDeletedCredentialAfterCompletion(t *testing.T) {
	db, err := storage.NewDB(filepath.Join(t.TempDir(), "goflow.db"))
	if err != nil {
		t.Fatalf("NewDB: %v", err)
	}
	defer db.Close()
	credStore := storage.NewCredentialStore(db, crypto.NewCryptoManager("test-master-key"))
	cred, err := credStore.Create("Telegram", "TELEGRAM_BOT", "123:secret-token")
	if err != nil {
		t.Fatalf("create credential: %v", err)
	}
	appliance := &ApplianceContext{
		Enabled:      true,
		Origin:       "http://example.com",
		SessionToken: "test-session-token",
		PackID:       "example.appliance",
		PackName:     "Example Appliance",
		PackVersion:  "0.1.0",
		WorkflowID:   "wf-1",
		DataDir:      t.TempDir(),
		CredentialRequirements: []pack.CredentialRequirement{
			{Key: "telegram", Label: "Telegram", Type: "TELEGRAM_BOT", Required: true},
		},
	}
	if _, err := packsetup.SaveCredentialBindings(appliance.DataDir, applianceManifest(appliance), map[string]string{"telegram": cred.ID}, applianceCredentialResolver(credStore)); err != nil {
		t.Fatalf("save credential binding: %v", err)
	}
	if _, err := packsetup.SaveState(appliance.DataDir, applianceManifest(appliance), true, time.Now()); err != nil {
		t.Fatalf("save state: %v", err)
	}
	if err := credStore.Delete(cred.ID); err != nil {
		t.Fatalf("delete credential: %v", err)
	}
	router := NewRouter(nil, nil, credStore, nil, nil, nil, nil, nil, nil, "", 60, "http://127.0.0.1:8080", nil, 2, 30, appliance)
	body := applianceRequest(t, router, http.MethodGet, "/api/appliance/status", nil, nil)
	if !strings.Contains(body, "NEEDS_SETUP") || !strings.Contains(body, `"credential"`) || strings.Contains(body, cred.ID) || strings.Contains(body, "secret-token") {
		t.Fatalf("deleted credential was not surfaced safely: %s", body)
	}
}

func TestApplianceWorkflowStatusReportsManagedWorkflowRecoveryStates(t *testing.T) {
	db, err := storage.NewDB(filepath.Join(t.TempDir(), "goflow.db"))
	if err != nil {
		t.Fatalf("NewDB: %v", err)
	}
	defer db.Close()
	wfStore := storage.NewWorkflowStore(db)
	execStore := storage.NewExecutionStore(db)
	appliance := &ApplianceContext{
		Enabled:      true,
		Origin:       "http://example.com",
		SessionToken: "test-session-token",
		PackID:       "example.appliance",
		PackName:     "Example Appliance",
		PackVersion:  "0.1.0",
		WorkflowID:   "wf-recovery",
		DataDir:      t.TempDir(),
	}
	if _, err := packsetup.SaveState(appliance.DataDir, applianceManifest(appliance), true, time.Now()); err != nil {
		t.Fatalf("save state: %v", err)
	}
	router := NewRouter(wfStore, execStore, nil, nil, nil, nil, nil, nil, nil, "", 60, "http://127.0.0.1:8080", nil, 2, 30, appliance)
	body := applianceRequest(t, router, http.MethodGet, "/api/appliance/workflow/status", nil, nil)
	if !strings.Contains(body, `"state":"ERROR"`) || !strings.Contains(body, "managed workflow is not available") {
		t.Fatalf("expected deleted workflow error state, got %s", body)
	}
	if err := wfStore.Create(&storage.Workflow{
		ID:        "wf-recovery",
		Name:      "Recovery",
		IsActive:  false,
		NodesJSON: `[]`,
		EdgesJSON: `[]`,
	}); err != nil {
		t.Fatalf("create workflow: %v", err)
	}
	body = applianceRequest(t, router, http.MethodGet, "/api/appliance/workflow/status", nil, nil)
	if !strings.Contains(body, `"state":"ERROR"`) || !strings.Contains(body, `"is_active":false`) {
		t.Fatalf("expected inactive workflow error state, got %s", body)
	}
}

func TestApplianceMutationGuardCoversStateChangingRoutes(t *testing.T) {
	db, err := storage.NewDB(filepath.Join(t.TempDir(), "goflow.db"))
	if err != nil {
		t.Fatalf("NewDB: %v", err)
	}
	defer db.Close()
	credStore := storage.NewCredentialStore(db, crypto.NewCryptoManager("test-master-key"))
	appliance := &ApplianceContext{
		Enabled:      true,
		Origin:       "http://example.com",
		SessionToken: "test-session-token",
		PackID:       "example.appliance",
		PackName:     "Example Appliance",
		PackVersion:  "0.1.0",
		WorkflowID:   "wf-1",
		DataDir:      t.TempDir(),
	}
	router := NewRouter(nil, nil, credStore, nil, nil, nil, nil, nil, nil, "", 60, "http://127.0.0.1:8080", nil, 2, 30, appliance)
	for _, path := range []string{
		"/api/appliance/setup/config",
		"/api/appliance/setup/credentials",
		"/api/appliance/setup/credentials/create",
		"/api/appliance/setup/credentials/test",
		"/api/appliance/setup/complete",
		"/api/appliance/setup/reopen",
		"/api/appliance/workflow/run",
	} {
		t.Run(path, func(t *testing.T) {
			applianceRequestStatus(t, router, http.MethodPost, path, []byte(`{}`), map[string]string{
				"Origin":       "http://example.com",
				"Content-Type": "application/json",
			}, http.StatusForbidden)
			applianceRequestStatus(t, router, http.MethodPost, path, []byte(`{}`), map[string]string{
				"Origin":             "http://evil.example",
				applianceTokenHeader: "test-session-token",
				"Content-Type":       "application/json",
			}, http.StatusForbidden)
			applianceRequestStatus(t, router, http.MethodPost, path, bytes.Repeat([]byte{'{'}, int(applianceMaxJSONBody)+1), map[string]string{
				"Origin":             "http://example.com",
				applianceTokenHeader: "test-session-token",
				"Content-Type":       "application/json",
			}, http.StatusBadRequest)
		})
	}
}

type applianceTestExecutor struct{}

func (applianceTestExecutor) Execute(ctx *nodes.ExecutionContext, node *nodes.Node) (interface{}, error) {
	return map[string]interface{}{"message": "ok", "token": "secret-output"}, nil
}

func (applianceTestExecutor) Validate(node *nodes.Node) error {
	return nil
}

func (applianceTestExecutor) GetDefinition() nodes.NodeDefinition {
	return nodes.NodeDefinition{
		Type:        nodes.NodeType("applianceTestAction"),
		Name:        "Appliance Test Action",
		Description: "Test action",
		Category:    "test",
	}
}

func applianceRequest(t *testing.T, router http.Handler, method, path string, body []byte, headers map[string]string) string {
	t.Helper()
	return applianceRequestStatus(t, router, method, path, body, headers, 0)
}

func applianceRequestStatus(t *testing.T, router http.Handler, method, path string, body []byte, headers map[string]string, wantStatus int) string {
	t.Helper()
	reader := bytes.NewReader(body)
	req := httptest.NewRequest(method, path, reader)
	req.Host = "example.com"
	for key, value := range headers {
		req.Header.Set(key, value)
	}
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if wantStatus != 0 && rec.Code != wantStatus {
		t.Fatalf("%s %s returned %d, want %d: %s", method, path, rec.Code, wantStatus, rec.Body.String())
	}
	if wantStatus == 0 && (rec.Code < 200 || rec.Code >= 300) {
		t.Fatalf("%s %s returned %d: %s", method, path, rec.Code, rec.Body.String())
	}
	return rec.Body.String()
}
