package main

import (
	"os"
	"runtime"
	"testing"
	"time"
)

func TestNativeWindowsPilot(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("native Morning Brief pilot runs only on Windows")
	}
	appDir := os.Getenv("GOFLOW_MORNING_BRIEF_APP_DIR")
	if appDir == "" {
		t.Fatal("GOFLOW_MORNING_BRIEF_APP_DIR is required")
	}

	dataDir := t.TempDir()
	state := &mockState{}
	rssClient := newRSSClient(state)
	telegramToken, err := randomTelegramToken()
	if err != nil {
		t.Fatalf("create Telegram token: %v", err)
	}
	telegramServer := newTelegramServer(state, telegramToken)
	defer telegramServer.Close()

	first, err := startBundle(appDir, dataDir, rssClient, telegramServer.URL)
	if err != nil {
		t.Fatalf("start first bundle: %v", err)
	}
	firstInfo, err := waitForReady(first, 60*time.Second)
	if err != nil {
		_ = stopBundle(first)
		t.Fatalf("wait first bundle: %v", err)
	}
	if err := assertNativeCounts(state, 0, 0, 0, 0); err != nil {
		_ = stopBundle(first)
		t.Fatalf("startup network boundary: %v", err)
	}
	if err := completeSetup(firstInfo, telegramToken); err != nil {
		_ = stopBundle(first)
		t.Fatalf("complete setup: %v", err)
	}
	if err := runWorkflowAndWait(firstInfo); err != nil {
		_ = stopBundle(first)
		t.Fatalf("first workflow run: %v", err)
	}
	if err := assertNativeCounts(state, 3, 2, 2, 1); err != nil {
		_ = stopBundle(first)
		t.Fatalf("first run calls: %v", err)
	}
	if err := assertLatestMessage(state); err != nil {
		_ = stopBundle(first)
		t.Fatalf("first Telegram brief: %v", err)
	}
	if err := stopBundle(first); err != nil {
		t.Fatalf("stop first bundle: %v", err)
	}

	second, err := startBundle(appDir, dataDir, rssClient, telegramServer.URL)
	if err != nil {
		t.Fatalf("restart bundle: %v", err)
	}
	secondInfo, err := waitForReady(second, 60*time.Second)
	if err != nil {
		_ = stopBundle(second)
		t.Fatalf("wait restarted bundle: %v", err)
	}
	if secondInfo.workflowID != firstInfo.workflowID {
		_ = stopBundle(second)
		t.Fatalf("workflow identity changed across restart: %s -> %s", firstInfo.workflowID, secondInfo.workflowID)
	}
	if err := verifyCompletedSetup(secondInfo.origin); err != nil {
		_ = stopBundle(second)
		t.Fatalf("verify persisted setup: %v", err)
	}
	if err := assertNativeCounts(state, 3, 2, 2, 1); err != nil {
		_ = stopBundle(second)
		t.Fatalf("restart network boundary: %v", err)
	}
	if err := runWorkflowAndWait(secondInfo); err != nil {
		_ = stopBundle(second)
		t.Fatalf("second workflow run: %v", err)
	}
	if err := assertNativeCounts(state, 6, 2, 2, 2); err != nil {
		_ = stopBundle(second)
		t.Fatalf("second run calls: %v", err)
	}
	if err := assertLatestMessage(state); err != nil {
		_ = stopBundle(second)
		t.Fatalf("second Telegram brief: %v", err)
	}
	if err := stopBundle(second); err != nil {
		t.Fatalf("stop restarted bundle: %v", err)
	}

	t.Log("VIETNAM_MORNING_BRIEF_WINDOWS_E2E PASS")
	t.Logf("pack_id=%s workflow_id=%s", expectedPackID, firstInfo.workflowID)
	t.Log("setup=persisted-across-restart ai_provider=none")
	t.Log("rss_requests=6 telegram_getMe=2 telegram_getChat=2 telegram_sendMessage=2")
	t.Log("startup_network_calls=0 restart_network_calls=0 source_links=publisher-originals")
}

func assertNativeCounts(state *mockState, rss, getMe, getChat, sendMessage int) error {
	state.mu.Lock()
	defer state.mu.Unlock()
	if len(state.unexpected) > 0 {
		return &nativeCountError{message: "mock observed unexpected requests", detail: state.unexpected}
	}
	if len(state.rssCalls) != rss || state.getMe != getMe || state.getChat != getChat || state.sendMessage != sendMessage {
		return &nativeCountError{
			message:  "mock call counts mismatch",
			counts:   [4]int{len(state.rssCalls), state.getMe, state.getChat, state.sendMessage},
			expected: [4]int{rss, getMe, getChat, sendMessage},
		}
	}
	return nil
}

type nativeCountError struct {
	message  string
	detail   []string
	counts   [4]int
	expected [4]int
}

func (e *nativeCountError) Error() string {
	if len(e.detail) > 0 {
		return e.message + ": " + joinNativeDetails(e.detail)
	}
	return e.message + ": got rss/getMe/getChat/sendMessage=" + formatNativeCounts(e.counts) +
		" expected=" + formatNativeCounts(e.expected)
}

func joinNativeDetails(values []string) string {
	result := ""
	for i, value := range values {
		if i > 0 {
			result += "; "
		}
		result += value
	}
	return result
}

func formatNativeCounts(values [4]int) string {
	return itoaNative(values[0]) + "/" + itoaNative(values[1]) + "/" + itoaNative(values[2]) + "/" + itoaNative(values[3])
}

func itoaNative(value int) string {
	if value == 0 {
		return "0"
	}
	result := ""
	for value > 0 {
		result = string(rune('0'+value%10)) + result
		value /= 10
	}
	return result
}
