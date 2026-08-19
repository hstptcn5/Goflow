package main

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"sync"
	"time"

	"goflow/internal/nodes"
	"goflow/internal/packrun"
)

const (
	expectedPackID = "official.vietnam-morning-brief"
	pilotChatID    = "@vietnam_morning_brief_native_pilot"
)

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

type mockState struct {
	mu          sync.Mutex
	rssCalls    []string
	getMe       int
	getChat     int
	sendMessage int
	messages    []string
	unexpected  []string
}

type roundTripperFunc func(*http.Request) (*http.Response, error)

func (f roundTripperFunc) RoundTrip(req *http.Request) (*http.Response, error) { return f(req) }

type runningBundle struct {
	cancel context.CancelFunc
	done   chan error
	stdout *lockedBuffer
	stderr *lockedBuffer
}

func main() {
	appDir := flag.String("app-dir", "", "path to extracted Vietnam Morning Brief appliance")
	flag.Parse()
	if runtime.GOOS != "windows" {
		fatalf("native Vietnam Morning Brief pilot must run on Windows")
	}
	if strings.TrimSpace(*appDir) == "" {
		fatalf("--app-dir is required")
	}
	if err := run(*appDir); err != nil {
		fatalf("%v", err)
	}
}

func run(appDir string) error {
	absAppDir, err := filepath.Abs(appDir)
	if err != nil {
		return fmt.Errorf("resolve app directory: %w", err)
	}
	dataDir, err := os.MkdirTemp("", "goflow-morning-brief-native-pilot-")
	if err != nil {
		return fmt.Errorf("create isolated data directory: %w", err)
	}
	defer os.RemoveAll(dataDir)

	state := &mockState{}
	rssClient := newRSSClient(state)
	telegramToken, err := randomTelegramToken()
	if err != nil {
		return err
	}
	telegramServer := newTelegramServer(state, telegramToken)
	defer telegramServer.Close()

	first, err := startBundle(absAppDir, dataDir, rssClient, telegramServer.URL)
	if err != nil {
		return err
	}
	firstInfo, err := waitForReady(first, 60*time.Second)
	if err != nil {
		stopBundle(first)
		return err
	}
	if err := assertCounts(state, 0, 0, 0); err != nil {
		stopBundle(first)
		return fmt.Errorf("startup caused an unsolicited network call: %w", err)
	}
	if err := completeSetup(firstInfo, telegramToken); err != nil {
		stopBundle(first)
		return err
	}
	if err := runWorkflowAndWait(firstInfo); err != nil {
		stopBundle(first)
		return err
	}
	if err := assertCounts(state, 3, 1, 1); err != nil {
		stopBundle(first)
		return err
	}
	if err := assertLatestMessage(state); err != nil {
		stopBundle(first)
		return err
	}
	if err := stopBundle(first); err != nil {
		return err
	}

	second, err := startBundle(absAppDir, dataDir, rssClient, telegramServer.URL)
	if err != nil {
		return err
	}
	secondInfo, err := waitForReady(second, 60*time.Second)
	if err != nil {
		stopBundle(second)
		return err
	}
	if secondInfo.workflowID != firstInfo.workflowID {
		stopBundle(second)
		return fmt.Errorf("managed workflow identity changed across restart")
	}
	if err := verifyCompletedSetup(secondInfo.origin); err != nil {
		stopBundle(second)
		return err
	}
	if err := assertCounts(state, 3, 1, 1); err != nil {
		stopBundle(second)
		return fmt.Errorf("restart caused an unsolicited network call: %w", err)
	}
	if err := runWorkflowAndWait(secondInfo); err != nil {
		stopBundle(second)
		return err
	}
	if err := assertCounts(state, 6, 1, 2); err != nil {
		stopBundle(second)
		return err
	}
	if err := assertLatestMessage(state); err != nil {
		stopBundle(second)
		return err
	}
	if err := stopBundle(second); err != nil {
		return err
	}

	fmt.Println("VIETNAM_MORNING_BRIEF_WINDOWS_E2E PASS")
	fmt.Printf("pack_id=%s\n", expectedPackID)
	fmt.Printf("workflow_id=%s\n", firstInfo.workflowID)
	fmt.Println("setup=persisted-across-restart")
	fmt.Println("ai_provider=none")
	fmt.Println("rss_requests=6")
	fmt.Println("telegram_getMe=1 telegram_sendMessage=2")
	fmt.Println("startup_network_calls=0 restart_network_calls=0")
	fmt.Println("source_links=publisher-originals")
	return nil
}

func startBundle(appDir, dataDir string, rssClient *http.Client, telegramBaseURL string) (*runningBundle, error) {
	registry := nodes.NewPluginRegistry()
	for _, executor := range []nodes.NodeExecutor{
		nodes.NewRSSFeedSourceExecutorWithClient(rssClient, func() time.Time {
			return time.Date(2026, 8, 19, 7, 0, 0, 0, time.UTC)
		}),
		nodes.NewJSCodeRunnerExecutor(),
		nodes.NewConditionIFExecutor(),
		nodes.NewTelegramBotExecutorWithClient(&http.Client{Timeout: 5 * time.Second}, telegramBaseURL),
		nodes.NewOpenAIGPTExecutor(),
		nodes.NewDeepSeekAIExecutor(),
	} {
		if err := registry.Register(executor); err != nil {
			return nil, fmt.Errorf("register pilot executor: %w", err)
		}
	}

	ctx, cancel := context.WithCancel(context.Background())
	stdout := &lockedBuffer{}
	stderr := &lockedBuffer{}
	done := make(chan error, 1)
	go func() {
		done <- packrun.RunExtractedBundle(ctx, appDir, packrun.Options{
			DataDir:              dataDir,
			Port:                 0,
			NoOpen:               true,
			Stdout:               stdout,
			Stderr:               stderr,
			Registry:             registry,
			TelegramAPIBaseURL:   telegramBaseURL,
			ConnectionTestClient: &http.Client{Timeout: 5 * time.Second},
		})
		close(done)
	}()
	return &runningBundle{cancel: cancel, done: done, stdout: stdout, stderr: stderr}, nil
}

func stopBundle(bundle *runningBundle) error {
	if bundle == nil {
		return nil
	}
	bundle.cancel()
	select {
	case err := <-bundle.done:
		if err != nil && err != context.Canceled && err != http.ErrServerClosed {
			return fmt.Errorf("bundle shutdown failed: %v: %s", err, bundle.stderr.String())
		}
		return nil
	case <-time.After(15 * time.Second):
		return fmt.Errorf("timed out waiting for bundle shutdown")
	}
}

func waitForReady(bundle *runningBundle, timeout time.Duration) (*applianceInfo, error) {
	deadline := time.Now().Add(timeout)
	client := &http.Client{Timeout: time.Second}
	for time.Now().Before(deadline) {
		select {
		case err := <-bundle.done:
			return nil, fmt.Errorf("bundle exited before readiness: %v: %s", err, bundle.stderr.String())
		default:
		}
		match := targetURLPattern.FindStringSubmatch(bundle.stdout.String())
		if len(match) == 2 {
			origin := match[1]
			if err := getOK(client, origin+"/healthz"); err == nil {
				var bootstrap bootstrapResponse
				if err := getJSON(client, origin+"/api/appliance/bootstrap", &bootstrap); err != nil {
					time.Sleep(200 * time.Millisecond)
					continue
				}
				if bootstrap.Pack.ID != expectedPackID || bootstrap.Token == "" {
					return nil, fmt.Errorf("bootstrap returned unexpected pack identity")
				}
				var status statusResponse
				if err := getJSON(client, origin+"/api/appliance/status", &status); err != nil {
					return nil, err
				}
				if status.Server != "ok" || status.WorkflowID == "" {
					return nil, fmt.Errorf("managed workflow was not healthy")
				}
				return &applianceInfo{origin: origin, workflowID: status.WorkflowID, token: bootstrap.Token}, nil
			}
		}
		time.Sleep(200 * time.Millisecond)
	}
	return nil, fmt.Errorf("bundle did not become ready: stdout=%s stderr=%s", bundle.stdout.String(), bundle.stderr.String())
}

func completeSetup(info *applianceInfo, telegramToken string) error {
	if err := postJSON(info, "/api/appliance/setup/config", map[string]interface{}{
		"values": map[string]interface{}{
			"chat_id":     pilotChatID,
			"ai_provider": "none",
		},
	}, http.StatusOK, nil); err != nil {
		return fmt.Errorf("save setup config: %w", err)
	}
	if err := postJSON(info, "/api/appliance/setup/credentials/create", map[string]interface{}{
		"key":   "telegram",
		"name":  "Morning Brief native pilot Telegram",
		"value": telegramToken,
	}, http.StatusCreated, nil); err != nil {
		return fmt.Errorf("create Telegram credential: %w", err)
	}
	if err := postJSON(info, "/api/appliance/setup/credentials/test", map[string]interface{}{
		"key": "telegram",
	}, http.StatusOK, nil); err != nil {
		return fmt.Errorf("test Telegram credential: %w", err)
	}
	if err := postJSON(info, "/api/appliance/setup/complete", map[string]interface{}{}, http.StatusOK, nil); err != nil {
		return fmt.Errorf("complete setup: %w", err)
	}
	return verifyCompletedSetup(info.origin)
}

func verifyCompletedSetup(origin string) error {
	client := &http.Client{Timeout: 5 * time.Second}
	var status statusResponse
	if err := getJSON(client, origin+"/api/appliance/status", &status); err != nil {
		return fmt.Errorf("read setup status: %w", err)
	}
	if status.State != "READY" || !status.SetupComplete {
		return fmt.Errorf("completed setup did not report READY")
	}
	var setup setupResponse
	if err := getJSON(client, origin+"/api/appliance/setup", &setup); err != nil {
		return fmt.Errorf("read setup details: %w", err)
	}
	if !setup.SetupComplete || setup.DecryptedValuesReturned {
		return fmt.Errorf("setup response was not safely redacted")
	}
	if setup.CurrentConfigValues["chat_id"] != pilotChatID || setup.CurrentConfigValues["ai_provider"] != "none" {
		return fmt.Errorf("setup configuration did not persist")
	}
	assigned := false
	for _, requirement := range setup.CredentialRequirements {
		if requirement.Key == "telegram" {
			assigned = requirement.Assigned
		}
	}
	if !assigned {
		return fmt.Errorf("Telegram credential was not assigned")
	}
	return nil
}

func runWorkflowAndWait(info *applianceInfo) error {
	var started runResponse
	if err := postJSON(info, "/api/appliance/workflow/run", map[string]interface{}{"input": map[string]interface{}{}}, http.StatusAccepted, &started); err != nil {
		return fmt.Errorf("run workflow: %w", err)
	}
	if started.ExecutionID == "" {
		return fmt.Errorf("run response omitted execution ID")
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
					return fmt.Errorf("workflow execution ended as %s", execution.Status)
				}
			}
		}
		time.Sleep(250 * time.Millisecond)
	}
	return fmt.Errorf("workflow execution timed out")
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
	req, err := http.NewRequest(http.MethodPost, info.origin+path, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Origin", info.origin)
	req.Header.Set("X-Goflow-Appliance-Token", info.token)
	res, err := (&http.Client{Timeout: 10 * time.Second}).Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	if res.StatusCode != expectedStatus {
		data, _ := io.ReadAll(io.LimitReader(res.Body, 4096))
		return fmt.Errorf("HTTP %d: %s", res.StatusCode, strings.TrimSpace(string(data)))
	}
	if destination == nil {
		_, err = io.Copy(io.Discard, io.LimitReader(res.Body, 1<<20))
		return err
	}
	return json.NewDecoder(io.LimitReader(res.Body, 1<<20)).Decode(destination)
}

func newRSSClient(state *mockState) *http.Client {
	return &http.Client{
		Timeout: 5 * time.Second,
		Transport: roundTripperFunc(func(req *http.Request) (*http.Response, error) {
			key := req.URL.Host + req.URL.Path
			state.mu.Lock()
			state.rssCalls = append(state.rssCalls, req.Method+" "+key)
			state.mu.Unlock()
			if req.Method != http.MethodGet {
				recordUnexpected(state, "RSS request was not GET")
				return responseFor(req, http.StatusMethodNotAllowed, "text/plain", "method not allowed"), nil
			}
			var feed string
			switch key {
			case "vnexpress.net/rss/tin-moi-nhat.rss":
				feed = rssDocument("VnExpress", "Tin thử nghiệm VnExpress", "https://vnexpress.net/tin-thu-nghiem-vnexpress-1001.html", "Wed, 19 Aug 2026 06:40:00 +0000", "Tóm tắt từ VnExpress")
			case "tuoitre.vn/home.rss":
				feed = rssDocument("Tuổi Trẻ", "Tin thử nghiệm Tuổi Trẻ", "https://tuoitre.vn/tin-thu-nghiem-tuoi-tre-1002.htm", "Wed, 19 Aug 2026 06:20:00 +0000", "Tóm tắt từ Tuổi Trẻ")
			case "thanhnien.vn/rss/home.rss":
				feed = rssDocument("Thanh Niên", "Tin thử nghiệm Thanh Niên", "https://thanhnien.vn/tin-thu-nghiem-thanh-nien-1003.htm", "Wed, 19 Aug 2026 06:00:00 +0000", "Tóm tắt từ Thanh Niên")
			default:
				recordUnexpected(state, "unexpected RSS URL: "+key)
				return responseFor(req, http.StatusNotFound, "text/plain", "not found"), nil
			}
			return responseFor(req, http.StatusOK, "application/rss+xml", feed), nil
		}),
	}
}

func rssDocument(publisher, title, link, published, summary string) string {
	return fmt.Sprintf(`<?xml version="1.0"?><rss version="2.0"><channel><title>%s</title><item><title>%s</title><link>%s</link><pubDate>%s</pubDate><description>%s</description></item></channel></rss>`, publisher, title, link, published, summary)
}

func responseFor(req *http.Request, status int, contentType, body string) *http.Response {
	return &http.Response{
		StatusCode: status,
		Status:     fmt.Sprintf("%d %s", status, http.StatusText(status)),
		Header:     http.Header{"Content-Type": []string{contentType}},
		Body:       io.NopCloser(strings.NewReader(body)),
		Request:    req,
	}
}

func newTelegramServer(state *mockState, token string) *httptest.Server {
	expectedPrefix := "/bot" + token + "/"
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		if !strings.HasPrefix(req.URL.Path, expectedPrefix) {
			recordUnexpected(state, "unexpected Telegram request path")
			writeTelegram(w, http.StatusNotFound, map[string]interface{}{"ok": false})
			return
		}
		method := strings.TrimPrefix(req.URL.Path, expectedPrefix)
		switch {
		case req.Method == http.MethodGet && method == "getMe":
			state.mu.Lock()
			state.getMe++
			state.mu.Unlock()
			writeTelegram(w, http.StatusOK, map[string]interface{}{"ok": true, "result": map[string]interface{}{"id": 42, "is_bot": true, "username": "morning_brief_pilot_bot"}})
		case req.Method == http.MethodGet && method == "getChat":
			if req.URL.Query().Get("chat_id") != pilotChatID {
				recordUnexpected(state, "Telegram getChat used unexpected chat")
			}
			state.mu.Lock()
			state.getChat++
			state.mu.Unlock()
			writeTelegram(w, http.StatusOK, map[string]interface{}{"ok": true, "result": map[string]interface{}{"id": pilotChatID, "type": "channel"}})
		case req.Method == http.MethodPost && method == "sendMessage":
			var payload struct {
				ChatID string `json:"chat_id"`
				Text   string `json:"text"`
			}
			if err := json.NewDecoder(io.LimitReader(req.Body, 64<<10)).Decode(&payload); err != nil {
				recordUnexpected(state, "invalid Telegram payload")
				writeTelegram(w, http.StatusBadRequest, map[string]interface{}{"ok": false})
				return
			}
			if payload.ChatID != pilotChatID {
				recordUnexpected(state, "Telegram sendMessage used unexpected chat")
			}
			state.mu.Lock()
			state.sendMessage++
			state.messages = append(state.messages, payload.Text)
			state.mu.Unlock()
			writeTelegram(w, http.StatusOK, map[string]interface{}{"ok": true, "result": map[string]interface{}{"message_id": 101}})
		default:
			recordUnexpected(state, "unexpected Telegram method: "+method)
			writeTelegram(w, http.StatusNotFound, map[string]interface{}{"ok": false})
		}
	}))
}

func assertLatestMessage(state *mockState) error {
	state.mu.Lock()
	defer state.mu.Unlock()
	if len(state.messages) == 0 {
		return fmt.Errorf("Telegram received no brief")
	}
	message := state.messages[len(state.messages)-1]
	for _, want := range []string{
		"Điểm tin Việt Nam buổi sáng",
		"Tin thử nghiệm VnExpress",
		"Tin thử nghiệm Tuổi Trẻ",
		"Tin thử nghiệm Thanh Niên",
		"https://vnexpress.net/tin-thu-nghiem-vnexpress-1001.html",
		"https://tuoitre.vn/tin-thu-nghiem-tuoi-tre-1002.htm",
		"https://thanhnien.vn/tin-thu-nghiem-thanh-nien-1003.htm",
	} {
		if !strings.Contains(message, want) {
			return fmt.Errorf("Telegram brief missing %q", want)
		}
	}
	return nil
}

func assertCounts(state *mockState, rss, getMe, sendMessage int) error {
	state.mu.Lock()
	defer state.mu.Unlock()
	if len(state.unexpected) > 0 {
		return fmt.Errorf("mock observed unexpected requests: %s", strings.Join(state.unexpected, "; "))
	}
	if len(state.rssCalls) != rss || state.getMe != getMe || state.sendMessage != sendMessage {
		return fmt.Errorf("mock counts mismatch: rss=%d getMe=%d getChat=%d sendMessage=%d", len(state.rssCalls), state.getMe, state.getChat, state.sendMessage)
	}
	return nil
}

func recordUnexpected(state *mockState, message string) {
	state.mu.Lock()
	defer state.mu.Unlock()
	state.unexpected = append(state.unexpected, message)
}

func writeTelegram(w http.ResponseWriter, status int, payload interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func randomTelegramToken() (string, error) {
	secret := make([]byte, 24)
	if _, err := rand.Read(secret); err != nil {
		return "", fmt.Errorf("create test credential: %w", err)
	}
	return fmt.Sprintf("987654:%x", secret), nil
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

func fatalf(format string, args ...interface{}) {
	fmt.Fprintf(os.Stderr, format+"\n", args...)
	os.Exit(1)
}
