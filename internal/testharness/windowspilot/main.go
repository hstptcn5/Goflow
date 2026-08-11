package main

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strings"
	"sync"
	"time"

	"goflow/internal/nodes"
	"goflow/internal/packrun"
	"goflow/internal/storage"
)

const (
	expectedPackID  = "official.dailyops-rest-telegram"
	configSourceURL = "https://pilot.example.test/dailyops.json"
	nativeChatID    = "@dailyops_native_smoke"
)

var expectedMessageFragments = []string{
	"2026-08-09",
	"48250.75",
	"314",
	"3 SKUs below threshold",
	"Revenue up 12.4% vs prior day",
}

var targetURLPattern = regexp.MustCompile(`(?m)^URL:\s+(http://127\.0\.0\.1:\d+)(?:/\S*)?\s*$`)

type lockedBuffer struct {
	mu sync.Mutex
	b  bytes.Buffer
}

func (b *lockedBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.b.Write(p)
}

func (b *lockedBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.b.String()
}

type managedProcess struct {
	cmd    *exec.Cmd
	stdout lockedBuffer
	stderr lockedBuffer
	done   chan error
}

type applianceInfo struct {
	origin     string
	workflowID string
	token      string
}

type bootstrapResponse struct {
	Token string `json:"token"`
	Pack  struct {
		ID string `json:"id"`
	} `json:"pack"`
}

type statusResponse struct {
	WorkflowID    string `json:"workflow_id"`
	Server        string `json:"server"`
	State         string `json:"state"`
	SetupComplete bool   `json:"setup_complete"`
}

type setupResponse struct {
	CurrentConfigValues    map[string]interface{} `json:"current_config_values"`
	SetupComplete          bool                   `json:"setup_complete"`
	CredentialRequirements []struct {
		Key      string `json:"key"`
		Assigned bool   `json:"assigned"`
	} `json:"credential_requirements"`
	DecryptedValuesReturned bool `json:"decrypted_values_returned"`
}

type executionSummary struct {
	ID     string `json:"id"`
	Status string `json:"status"`
}

type executionListResponse struct {
	Executions []executionSummary `json:"executions"`
}

type runResponse struct {
	ExecutionID string `json:"execution_id"`
}

type mockCallState struct {
	mu          sync.Mutex
	sourceCount int
	sourceCalls []string
	getMe       int
	sendMessage int
	unexpected  []string
}

func main() {
	appDir := flag.String("app-dir", "", "path to the extracted DailyOps appliance")
	serveBundle := flag.Bool("serve-bundle", false, "run the extracted bundle with test-only network seams")
	dataDir := flag.String("data-dir", "", "isolated pack data directory")
	telegramBaseURL := flag.String("telegram-base-url", "", "test-only Telegram API base URL")
	flag.Parse()
	if runtime.GOOS != "windows" {
		fatalf("native Windows smoke must run on Windows")
	}
	if strings.TrimSpace(*appDir) == "" {
		fatalf("--app-dir is required")
	}
	if *serveBundle {
		if strings.TrimSpace(*dataDir) == "" || strings.TrimSpace(*telegramBaseURL) == "" {
			fatalf("--data-dir and --telegram-base-url are required in bundle server mode")
		}
		if err := serveExtractedBundle(*appDir, *dataDir, *telegramBaseURL); err != nil {
			fatalf("bundle server failed")
		}
		return
	}
	if err := run(*appDir); err != nil {
		fatalf("%v", err)
	}
}

func serveExtractedBundle(appDir, dataDir, telegramBaseURL string) error {
	client := &http.Client{Timeout: 5 * time.Second}
	registry := nodes.NewBuiltinRegistryWithTelegramExecutor(nodes.NewTelegramBotExecutorWithClient(client, telegramBaseURL))
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	err := packrun.RunExtractedBundle(ctx, appDir, packrun.Options{
		DataDir:              dataDir,
		Port:                 0,
		NoOpen:               true,
		Stdout:               os.Stdout,
		Stderr:               os.Stderr,
		Registry:             registry,
		TelegramAPIBaseURL:   telegramBaseURL,
		ConnectionTestClient: client,
	})
	if err != nil && ctx.Err() == nil && err != http.ErrServerClosed {
		return err
	}
	return nil
}

func run(appDir string) error {
	absAppDir, err := filepath.Abs(appDir)
	if err != nil {
		return fmt.Errorf("resolve appliance directory: %w", err)
	}
	if err := verifyLayout(absAppDir); err != nil {
		return err
	}
	exePath := filepath.Join(absAppDir, "goflow.exe")
	if err := verifyPEAMD64(exePath); err != nil {
		return err
	}

	tempRoot, err := os.MkdirTemp("", "goflow-windows-pilot-smoke-")
	if err != nil {
		return fmt.Errorf("create isolated smoke directory: %w", err)
	}
	defer os.RemoveAll(tempRoot)
	localAppData := filepath.Join(tempRoot, "local-app-data")
	if err := os.MkdirAll(localAppData, 0700); err != nil {
		return fmt.Errorf("create isolated data root: %w", err)
	}
	expectedDataDir := filepath.Join(localAppData, "Goflow", "packs", expectedPackID)

	first, err := startAppliance(exePath, absAppDir, localAppData)
	if err != nil {
		return err
	}
	defer forceStop(first)
	firstInfo, err := waitForReady(first, 60*time.Second)
	if err != nil {
		return redactError(err, absAppDir, tempRoot)
	}
	if err := verifyExternalState(absAppDir, expectedDataDir, false); err != nil {
		return err
	}
	if err := savePilotConfig(firstInfo); err != nil {
		return err
	}
	if err := verifyExternalState(absAppDir, expectedDataDir, true); err != nil {
		return err
	}

	second, err := startAppliance(exePath, absAppDir, localAppData)
	if err != nil {
		return err
	}
	if err := waitForReuse(second, 20*time.Second); err != nil {
		forceStop(second)
		return redactError(err, absAppDir, tempRoot)
	}
	if err := probeHealth(firstInfo.origin); err != nil {
		return fmt.Errorf("primary instance was not healthy after reuse: %w", err)
	}

	if err := stopCleanly(first, 20*time.Second); err != nil {
		return err
	}
	first = nil

	restarted, err := startAppliance(exePath, absAppDir, localAppData)
	if err != nil {
		return err
	}
	defer forceStop(restarted)
	restartedInfo, err := waitForReady(restarted, 60*time.Second)
	if err != nil {
		return redactError(err, absAppDir, tempRoot)
	}
	if restartedInfo.workflowID != firstInfo.workflowID {
		return fmt.Errorf("workflow identity changed across restart")
	}
	if err := verifyPersistedConfig(restartedInfo.origin); err != nil {
		return err
	}
	if err := stopCleanly(restarted, 20*time.Second); err != nil {
		return err
	}
	restarted = nil

	callState := &mockCallState{}
	sourceServer := newSourceServer(callState)
	defer sourceServer.Close()
	telegramToken, err := randomTelegramToken()
	if err != nil {
		return err
	}
	telegramServer := newTelegramServer(callState, telegramToken)
	defer telegramServer.Close()

	firstRun, err := startBundleHarness(absAppDir, expectedDataDir, telegramServer.URL)
	if err != nil {
		return err
	}
	defer forceStop(firstRun)
	firstRunInfo, err := waitForAPIReady(firstRun, 60*time.Second)
	if err != nil {
		return redactError(err, absAppDir, tempRoot, telegramToken, sourceServer.URL, telegramServer.URL)
	}
	if firstRunInfo.workflowID != firstInfo.workflowID {
		return fmt.Errorf("bundle test seam changed the stable workflow identity")
	}
	if err := completeNativeSetup(firstRunInfo, sourceServer.URL, telegramToken); err != nil {
		return redactError(err, telegramToken, sourceServer.URL, telegramServer.URL)
	}
	if err := runWorkflowAndWait(firstRunInfo); err != nil {
		return err
	}
	if err := assertMockCalls(callState, 1, 1, 1); err != nil {
		return err
	}
	if err := stopCleanly(firstRun, 20*time.Second); err != nil {
		return err
	}
	firstRun = nil

	persistedRuntime, err := startAppliance(exePath, absAppDir, localAppData)
	if err != nil {
		return err
	}
	defer forceStop(persistedRuntime)
	persistedInfo, err := waitForReady(persistedRuntime, 60*time.Second)
	if err != nil {
		return redactError(err, absAppDir, tempRoot, telegramToken, sourceServer.URL, telegramServer.URL)
	}
	if persistedInfo.workflowID != firstRunInfo.workflowID {
		return fmt.Errorf("extracted runtime changed workflow identity after completed setup restart")
	}
	if err := verifyCompletedSetup(persistedInfo.origin); err != nil {
		return err
	}
	if err := assertMockCalls(callState, 1, 1, 1); err != nil {
		return fmt.Errorf("extracted runtime restart caused an unsolicited network call: %w", err)
	}
	if err := stopCleanly(persistedRuntime, 20*time.Second); err != nil {
		return err
	}
	persistedRuntime = nil

	secondRun, err := startBundleHarness(absAppDir, expectedDataDir, telegramServer.URL)
	if err != nil {
		return err
	}
	defer forceStop(secondRun)
	secondRunInfo, err := waitForAPIReady(secondRun, 60*time.Second)
	if err != nil {
		return redactError(err, absAppDir, tempRoot, telegramToken, sourceServer.URL, telegramServer.URL)
	}
	if secondRunInfo.workflowID != firstRunInfo.workflowID {
		return fmt.Errorf("workflow identity changed after completed setup restart")
	}
	if err := verifyCompletedSetup(secondRunInfo.origin); err != nil {
		return err
	}
	if err := assertMockCalls(callState, 1, 1, 1); err != nil {
		return fmt.Errorf("restart caused an unsolicited network call: %w", err)
	}
	if err := runWorkflowAndWait(secondRunInfo); err != nil {
		return err
	}
	if err := assertMockCalls(callState, 2, 1, 2); err != nil {
		return err
	}
	if err := stopCleanly(secondRun, 20*time.Second); err != nil {
		return err
	}
	secondRun = nil
	if err := verifyManagedStateCounts(expectedDataDir); err != nil {
		return err
	}
	if err := verifyExternalState(absAppDir, expectedDataDir, true); err != nil {
		return err
	}

	if err := verifyTamperRejection(absAppDir, tempRoot); err != nil {
		return err
	}

	fmt.Println("WINDOWS_PILOT_SMOKE PASS")
	fmt.Println("target=windows-amd64")
	fmt.Printf("pack_id=%s\n", expectedPackID)
	fmt.Printf("workflow_id=%s\n", firstInfo.workflowID)
	fmt.Println("healthz=200 ui=200 bootstrap=200 status=200")
	fmt.Println("single_instance=reused-existing")
	fmt.Println("restart=ready-and-run-without-reconfiguration")
	fmt.Println("extracted_runtime_restart=ready-stable-workflow")
	fmt.Println("source_requests=2 getMe=1 sendMessage=2")
	fmt.Println("setup=complete credential=assigned")
	fmt.Println("managed_state=one-workflow-one-credential")
	fmt.Println("state_location=external-local-app-data")
	fmt.Println("tamper=rejected-before-startup")
	return nil
}

func verifyLayout(appDir string) error {
	expected := []string{
		"PACK_INFO.json",
		"README.txt",
		"goflow.exe",
		"pack/pack.json",
		"pack/workflows/main.json",
	}
	var observed []string
	err := filepath.WalkDir(appDir, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			return nil
		}
		relative, err := filepath.Rel(appDir, path)
		if err != nil {
			return err
		}
		observed = append(observed, filepath.ToSlash(relative))
		return nil
	})
	if err != nil {
		return fmt.Errorf("inspect extracted layout: %w", err)
	}
	sort.Strings(observed)
	if strings.Join(observed, "\n") != strings.Join(expected, "\n") {
		return fmt.Errorf("unexpected extracted appliance layout: %v", observed)
	}
	return verifyNoRuntimeState(appDir)
}

func startAppliance(exePath, appDir, localAppData string) (*managedProcess, error) {
	command := exec.Command(exePath)
	command.Dir = appDir
	command.Env = replaceEnv(os.Environ(), "LOCALAPPDATA", localAppData)
	return startManagedProcess(command, "start extracted appliance")
}

func startBundleHarness(appDir, dataDir, telegramBaseURL string) (*managedProcess, error) {
	harnessPath, err := os.Executable()
	if err != nil {
		return nil, fmt.Errorf("resolve native smoke harness: %w", err)
	}
	command := exec.Command(harnessPath,
		"--serve-bundle",
		"--app-dir", appDir,
		"--data-dir", dataDir,
		"--telegram-base-url", telegramBaseURL,
	)
	command.Dir = appDir
	return startManagedProcess(command, "start extracted bundle test server")
}

func startManagedProcess(command *exec.Cmd, operation string) (*managedProcess, error) {
	process := &managedProcess{done: make(chan error, 1)}
	process.cmd = command
	process.cmd.Stdout = &process.stdout
	process.cmd.Stderr = &process.stderr
	configureProcess(process.cmd)
	if err := process.cmd.Start(); err != nil {
		return nil, fmt.Errorf("%s: %w", operation, err)
	}
	go func() {
		process.done <- process.cmd.Wait()
		close(process.done)
	}()
	return process, nil
}

func waitForReady(process *managedProcess, timeout time.Duration) (*applianceInfo, error) {
	return waitForProcessReady(process, timeout, true)
}

func waitForAPIReady(process *managedProcess, timeout time.Duration) (*applianceInfo, error) {
	return waitForProcessReady(process, timeout, false)
}

func waitForProcessReady(process *managedProcess, timeout time.Duration, requireUI bool) (*applianceInfo, error) {
	deadline := time.Now().Add(timeout)
	client := &http.Client{Timeout: time.Second}
	for time.Now().Before(deadline) {
		select {
		case err := <-process.done:
			return nil, fmt.Errorf("appliance exited before readiness: %v: %s", err, combinedOutput(process))
		default:
		}
		match := targetURLPattern.FindStringSubmatch(process.stdout.String())
		if len(match) == 2 {
			origin := match[1]
			if err := getOK(client, origin+"/healthz"); err == nil {
				info, err := inspectAppliance(client, origin, requireUI)
				if err == nil {
					return info, nil
				}
			}
		}
		time.Sleep(250 * time.Millisecond)
	}
	return nil, fmt.Errorf("appliance did not become ready: %s", combinedOutput(process))
}

func inspectAppliance(client *http.Client, origin string, requireUI bool) (*applianceInfo, error) {
	if requireUI {
		if err := getOK(client, origin+"/"); err != nil {
			return nil, fmt.Errorf("UI probe: %w", err)
		}
	}
	var bootstrap bootstrapResponse
	if err := getJSON(client, origin+"/api/appliance/bootstrap", &bootstrap); err != nil {
		return nil, fmt.Errorf("bootstrap probe: %w", err)
	}
	if bootstrap.Pack.ID != expectedPackID || bootstrap.Token == "" {
		return nil, fmt.Errorf("bootstrap identity or session token was missing")
	}
	var status statusResponse
	if err := getJSON(client, origin+"/api/appliance/status", &status); err != nil {
		return nil, fmt.Errorf("status probe: %w", err)
	}
	if status.Server != "ok" || status.WorkflowID == "" {
		return nil, fmt.Errorf("status did not report a healthy managed workflow")
	}
	return &applianceInfo{origin: origin, workflowID: status.WorkflowID, token: bootstrap.Token}, nil
}

func savePilotConfig(info *applianceInfo) error {
	payload := map[string]interface{}{
		"values": map[string]interface{}{
			"source_url":          configSourceURL,
			"chat_id":             "@dailyops_pilot",
			"report_title":        "DailyOps Pilot Report",
			"low_stock_threshold": 5,
		},
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	req, err := http.NewRequest(http.MethodPost, info.origin+"/api/appliance/setup/config", bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Origin", info.origin)
	req.Header.Set("X-Goflow-Appliance-Token", info.token)
	res, err := (&http.Client{Timeout: 5 * time.Second}).Do(req)
	if err != nil {
		return fmt.Errorf("save setup configuration: %w", err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return fmt.Errorf("save setup configuration returned HTTP %d", res.StatusCode)
	}
	return nil
}

func verifyPersistedConfig(origin string) error {
	var setup setupResponse
	if err := getJSON(&http.Client{Timeout: 5 * time.Second}, origin+"/api/appliance/setup", &setup); err != nil {
		return fmt.Errorf("read setup after restart: %w", err)
	}
	if setup.CurrentConfigValues["source_url"] != configSourceURL {
		return fmt.Errorf("setup configuration did not persist across restart")
	}
	return nil
}

func completeNativeSetup(info *applianceInfo, sourceURL, telegramToken string) error {
	if err := postJSON(info, "/api/appliance/setup/config", map[string]interface{}{
		"values": map[string]interface{}{
			"source_url":          sourceURL + "/dailyops.json",
			"chat_id":             nativeChatID,
			"report_title":        "DailyOps Daily Report",
			"low_stock_threshold": 5,
		},
	}, http.StatusOK, nil); err != nil {
		return fmt.Errorf("save native setup config: %w", err)
	}
	if err := postJSON(info, "/api/appliance/setup/credentials/create", map[string]interface{}{
		"key":   "telegram",
		"name":  "Native smoke Telegram",
		"value": telegramToken,
	}, http.StatusCreated, nil); err != nil {
		return fmt.Errorf("create native setup credential: %w", err)
	}
	if err := postJSON(info, "/api/appliance/setup/credentials/test", map[string]interface{}{
		"key": "telegram",
	}, http.StatusOK, nil); err != nil {
		return fmt.Errorf("test native setup credential: %w", err)
	}
	if err := postJSON(info, "/api/appliance/setup/complete", map[string]interface{}{}, http.StatusOK, nil); err != nil {
		return fmt.Errorf("complete native setup: %w", err)
	}
	return verifyCompletedSetup(info.origin)
}

func verifyCompletedSetup(origin string) error {
	client := &http.Client{Timeout: 5 * time.Second}
	var status statusResponse
	if err := getJSON(client, origin+"/api/appliance/status", &status); err != nil {
		return fmt.Errorf("read completed setup status: %w", err)
	}
	if status.State != "READY" || !status.SetupComplete {
		return fmt.Errorf("completed setup did not report READY")
	}
	var setup setupResponse
	if err := getJSON(client, origin+"/api/appliance/setup", &setup); err != nil {
		return fmt.Errorf("read completed setup details: %w", err)
	}
	if !setup.SetupComplete || setup.DecryptedValuesReturned {
		return fmt.Errorf("completed setup response was not safely redacted")
	}
	assigned := false
	for _, requirement := range setup.CredentialRequirements {
		if requirement.Key == "telegram" {
			assigned = requirement.Assigned
		}
	}
	if !assigned {
		return fmt.Errorf("persisted Telegram credential was not assigned")
	}
	return nil
}

func runWorkflowAndWait(info *applianceInfo) error {
	var started runResponse
	if err := postJSON(info, "/api/appliance/workflow/run", map[string]interface{}{
		"input": map[string]interface{}{},
	}, http.StatusAccepted, &started); err != nil {
		return fmt.Errorf("run managed workflow: %w", err)
	}
	if started.ExecutionID == "" {
		return fmt.Errorf("run response omitted execution identity")
	}
	client := &http.Client{Timeout: 5 * time.Second}
	deadline := time.Now().Add(30 * time.Second)
	for time.Now().Before(deadline) {
		var list executionListResponse
		if err := getJSON(client, info.origin+"/api/appliance/executions?limit=10", &list); err == nil {
			for _, execution := range list.Executions {
				if execution.ID != started.ExecutionID {
					continue
				}
				switch execution.Status {
				case "SUCCESS":
					return nil
				case "FAILED", "CANCELLED":
					return fmt.Errorf("managed workflow execution did not succeed")
				}
			}
		}
		time.Sleep(250 * time.Millisecond)
	}
	return fmt.Errorf("managed workflow execution timed out")
}

func postJSON(info *applianceInfo, path string, payload interface{}, expectedStatus int, destination interface{}) error {
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	parsed, err := url.Parse(info.origin)
	if err != nil || parsed.Scheme != "http" || parsed.Hostname() != "127.0.0.1" {
		return fmt.Errorf("refusing non-loopback appliance URL")
	}
	request, err := http.NewRequest(http.MethodPost, info.origin+path, bytes.NewReader(body))
	if err != nil {
		return err
	}
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Origin", info.origin)
	request.Header.Set("X-Goflow-Appliance-Token", info.token)
	response, err := (&http.Client{Timeout: 10 * time.Second}).Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	if response.StatusCode != expectedStatus {
		return fmt.Errorf("HTTP %d", response.StatusCode)
	}
	if destination == nil {
		_, err = io.Copy(io.Discard, io.LimitReader(response.Body, 1<<20))
		return err
	}
	return json.NewDecoder(io.LimitReader(response.Body, 1<<20)).Decode(destination)
}

func randomTelegramToken() (string, error) {
	secret := make([]byte, 24)
	if _, err := rand.Read(secret); err != nil {
		return "", fmt.Errorf("create test credential: %w", err)
	}
	return fmt.Sprintf("987654:%x", secret), nil
}

func newSourceServer(state *mockCallState) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		state.mu.Lock()
		state.sourceCount++
		state.sourceCalls = append(state.sourceCalls, request.Method+" "+request.URL.Path)
		if request.Method != http.MethodGet || request.URL.Path != "/dailyops.json" {
			state.unexpected = append(state.unexpected, "unexpected source request")
		}
		state.mu.Unlock()
		if request.Method != http.MethodGet || request.URL.Path != "/dailyops.json" {
			http.NotFound(response, request)
			return
		}
		response.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(response).Encode(map[string]interface{}{
			"report_date":              "2026-08-09",
			"timezone":                 "Asia/Bangkok",
			"revenue":                  48250.75,
			"order_count":              314,
			"cancelled_refunded_count": 7,
			"low_stock_summary":        "3 SKUs below threshold",
			"comparison_summary":       "Revenue up 12.4% vs prior day",
		})
	}))
}

func newTelegramServer(state *mockCallState, token string) *httptest.Server {
	expectedPrefix := "/bot" + token + "/"
	return httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		if !strings.HasPrefix(request.URL.Path, expectedPrefix) {
			recordUnexpected(state, "unexpected Telegram request path")
			writeTelegramResponse(response, http.StatusNotFound, map[string]interface{}{"ok": false})
			return
		}
		method := strings.TrimPrefix(request.URL.Path, expectedPrefix)
		switch {
		case request.Method == http.MethodGet && method == "getMe":
			state.mu.Lock()
			state.getMe++
			state.mu.Unlock()
			writeTelegramResponse(response, http.StatusOK, map[string]interface{}{
				"ok":     true,
				"result": map[string]interface{}{"id": 42, "is_bot": true, "username": "native_smoke_bot"},
			})
		case request.Method == http.MethodPost && method == "sendMessage":
			var payload struct {
				ChatID string `json:"chat_id"`
				Text   string `json:"text"`
			}
			if err := json.NewDecoder(io.LimitReader(request.Body, 64<<10)).Decode(&payload); err != nil {
				recordUnexpected(state, "invalid Telegram payload")
				writeTelegramResponse(response, http.StatusBadRequest, map[string]interface{}{"ok": false})
				return
			}
			valid := payload.ChatID == nativeChatID && strings.Contains(payload.Text, "DailyOps Daily Report")
			for _, fragment := range expectedMessageFragments {
				valid = valid && strings.Contains(payload.Text, fragment)
			}
			if !valid {
				recordUnexpected(state, "Telegram payload did not contain the expected source data")
				writeTelegramResponse(response, http.StatusBadRequest, map[string]interface{}{"ok": false})
				return
			}
			state.mu.Lock()
			state.sendMessage++
			state.mu.Unlock()
			writeTelegramResponse(response, http.StatusOK, map[string]interface{}{
				"ok":     true,
				"result": map[string]interface{}{"message_id": 101, "chat": map[string]interface{}{"id": nativeChatID}},
			})
		default:
			recordUnexpected(state, "unexpected Telegram method")
			writeTelegramResponse(response, http.StatusNotFound, map[string]interface{}{"ok": false})
		}
	}))
}

func writeTelegramResponse(response http.ResponseWriter, status int, payload interface{}) {
	response.Header().Set("Content-Type", "application/json")
	response.WriteHeader(status)
	_ = json.NewEncoder(response).Encode(payload)
}

func recordUnexpected(state *mockCallState, message string) {
	state.mu.Lock()
	defer state.mu.Unlock()
	state.unexpected = append(state.unexpected, message)
}

func assertMockCalls(state *mockCallState, source, getMe, sendMessage int) error {
	state.mu.Lock()
	defer state.mu.Unlock()
	if len(state.unexpected) > 0 {
		return fmt.Errorf("native mock observed unexpected requests")
	}
	if state.sourceCount != source || state.getMe != getMe || state.sendMessage != sendMessage {
		return fmt.Errorf("native mock call counts mismatch: source=%d getMe=%d sendMessage=%d", state.sourceCount, state.getMe, state.sendMessage)
	}
	for _, call := range state.sourceCalls {
		if call != "GET /dailyops.json" {
			return fmt.Errorf("native source request method or path was unexpected")
		}
	}
	return nil
}

func verifyManagedStateCounts(dataDir string) error {
	for _, name := range []string{"pack-config.json", "pack-credentials.json", "pack-setup-state.json"} {
		if info, err := os.Stat(filepath.Join(dataDir, name)); err != nil || !info.Mode().IsRegular() {
			return fmt.Errorf("completed setup state file %s was not persisted externally", name)
		}
	}
	db, err := storage.NewDB(filepath.Join(dataDir, "goflow.db"))
	if err != nil {
		return fmt.Errorf("open persisted data: %w", err)
	}
	defer db.Close()
	workflows, err := storage.NewWorkflowStore(db).ListAll()
	if err != nil {
		return fmt.Errorf("list persisted workflows: %w", err)
	}
	credentials, err := storage.NewCredentialStore(db, nil).ListAll()
	if err != nil {
		return fmt.Errorf("list persisted credentials: %w", err)
	}
	if len(workflows) != 1 || workflows[0].ID == "" || !workflows[0].IsActive {
		return fmt.Errorf("persisted managed workflow was duplicated or inactive")
	}
	if len(credentials) != 1 {
		return fmt.Errorf("persisted credential was duplicated or missing")
	}
	return nil
}

func verifyExternalState(appDir, dataDir string, expectConfig bool) error {
	for _, name := range []string{"goflow.db", "goflow.master.key", "pack-state.json"} {
		if info, err := os.Stat(filepath.Join(dataDir, name)); err != nil || !info.Mode().IsRegular() {
			return fmt.Errorf("expected external runtime state %s was not created", name)
		}
	}
	if expectConfig {
		if info, err := os.Stat(filepath.Join(dataDir, "pack-config.json")); err != nil || !info.Mode().IsRegular() {
			return fmt.Errorf("setup configuration was not stored in the external data directory")
		}
	}
	return verifyNoRuntimeState(appDir)
}

func verifyNoRuntimeState(appDir string) error {
	forbiddenNames := map[string]bool{
		"goflow.db":             true,
		"goflow.master.key":     true,
		"pack-config.json":      true,
		"pack-credentials.json": true,
		"pack-setup-state.json": true,
		"pack-state.json":       true,
		"run-state.json":        true,
	}
	return filepath.WalkDir(appDir, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !entry.IsDir() && forbiddenNames[strings.ToLower(entry.Name())] {
			return fmt.Errorf("runtime state leaked into the extracted appliance")
		}
		return nil
	})
}

func waitForReuse(process *managedProcess, timeout time.Duration) error {
	select {
	case err := <-process.done:
		if err != nil {
			return fmt.Errorf("second instance failed: %v: %s", err, combinedOutput(process))
		}
		if !strings.Contains(process.stdout.String(), "Reusing running pack instance") {
			return fmt.Errorf("second instance did not reuse the running appliance")
		}
		return nil
	case <-time.After(timeout):
		return fmt.Errorf("second instance did not exit after reusing the running appliance")
	}
}

func stopCleanly(process *managedProcess, timeout time.Duration) error {
	if process == nil || process.cmd == nil || process.cmd.Process == nil {
		return nil
	}
	restoreInterruptHandling, err := interruptProcess(process.cmd.Process.Pid)
	if err != nil {
		forceStop(process)
		return fmt.Errorf("request clean appliance shutdown: %w", err)
	}
	defer restoreInterruptHandling()
	select {
	case err := <-process.done:
		if err != nil {
			return fmt.Errorf("appliance did not stop cleanly: %v", err)
		}
		return nil
	case <-time.After(timeout):
		forceStop(process)
		return fmt.Errorf("timed out waiting for clean appliance shutdown")
	}
}

func forceStop(process *managedProcess) {
	if process == nil || process.cmd == nil || process.cmd.Process == nil || process.cmd.ProcessState != nil {
		return
	}
	_ = process.cmd.Process.Kill()
	select {
	case <-process.done:
	case <-time.After(5 * time.Second):
	}
}

func verifyTamperRejection(appDir, tempRoot string) error {
	tamperedDir := filepath.Join(tempRoot, "tampered-app")
	if err := copyDirectory(appDir, tamperedDir); err != nil {
		return fmt.Errorf("prepare tamper fixture: %w", err)
	}
	workflowPath := filepath.Join(tamperedDir, "pack", "workflows", "main.json")
	file, err := os.OpenFile(workflowPath, os.O_APPEND|os.O_WRONLY, 0)
	if err != nil {
		return fmt.Errorf("tamper controlled pack file: %w", err)
	}
	if _, err := file.WriteString("\n"); err != nil {
		_ = file.Close()
		return fmt.Errorf("tamper controlled pack file: %w", err)
	}
	if err := file.Close(); err != nil {
		return err
	}
	tamperedData := filepath.Join(tempRoot, "tampered-local-app-data")
	process, err := startAppliance(filepath.Join(tamperedDir, "goflow.exe"), tamperedDir, tamperedData)
	if err != nil {
		return err
	}
	select {
	case exitErr := <-process.done:
		if exitErr == nil {
			return fmt.Errorf("tampered appliance unexpectedly started")
		}
		output := combinedOutput(process)
		if !strings.Contains(output, "extracted bundle verification failed") {
			return fmt.Errorf("tampered appliance failed without a verification rejection")
		}
	case <-time.After(20 * time.Second):
		forceStop(process)
		return fmt.Errorf("tampered appliance did not reject startup")
	}
	if entries, err := os.ReadDir(tamperedData); err == nil && len(entries) > 0 {
		return fmt.Errorf("tampered appliance created runtime state before rejection")
	} else if err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return nil
}

func copyDirectory(source, destination string) error {
	return filepath.WalkDir(source, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		relative, err := filepath.Rel(source, path)
		if err != nil {
			return err
		}
		target := filepath.Join(destination, relative)
		if entry.IsDir() {
			return os.MkdirAll(target, 0700)
		}
		input, err := os.Open(path)
		if err != nil {
			return err
		}
		defer input.Close()
		output, err := os.OpenFile(target, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0600)
		if err != nil {
			return err
		}
		_, copyErr := io.Copy(output, input)
		closeErr := output.Close()
		if copyErr != nil {
			return copyErr
		}
		return closeErr
	})
}

func probeHealth(origin string) error {
	return getOK(&http.Client{Timeout: 2 * time.Second}, origin+"/healthz")
}

func getOK(client *http.Client, address string) error {
	res, err := client.Get(address)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return fmt.Errorf("HTTP %d", res.StatusCode)
	}
	return nil
}

func getJSON(client *http.Client, address string, destination interface{}) error {
	parsed, err := url.Parse(address)
	if err != nil || parsed.Scheme != "http" || parsed.Hostname() != "127.0.0.1" {
		return fmt.Errorf("refusing non-loopback appliance URL")
	}
	res, err := client.Get(address)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return fmt.Errorf("HTTP %d", res.StatusCode)
	}
	return json.NewDecoder(io.LimitReader(res.Body, 1<<20)).Decode(destination)
}

func replaceEnv(environment []string, key, value string) []string {
	prefix := strings.ToUpper(key) + "="
	result := make([]string, 0, len(environment)+1)
	for _, item := range environment {
		if !strings.HasPrefix(strings.ToUpper(item), prefix) {
			result = append(result, item)
		}
	}
	return append(result, key+"="+value)
}

func combinedOutput(process *managedProcess) string {
	return strings.TrimSpace(process.stdout.String() + "\n" + process.stderr.String())
}

func redactError(err error, values ...string) error {
	message := err.Error()
	for _, value := range values {
		if value == "" {
			continue
		}
		message = strings.ReplaceAll(message, value, "[REDACTED_PATH]")
		message = strings.ReplaceAll(message, filepath.ToSlash(value), "[REDACTED_PATH]")
	}
	return errors.New(message)
}

func fatalf(format string, args ...interface{}) {
	fmt.Fprintf(os.Stderr, "windows pilot smoke: "+format+"\n", args...)
	os.Exit(1)
}
