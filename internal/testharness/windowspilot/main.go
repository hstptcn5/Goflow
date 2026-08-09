package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strings"
	"sync"
	"time"
)

const (
	expectedPackID  = "official.dailyops-rest-telegram"
	configSourceURL = "https://pilot.example.test/dailyops.json"
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
	CurrentConfigValues map[string]interface{} `json:"current_config_values"`
}

func main() {
	appDir := flag.String("app-dir", "", "path to the extracted DailyOps appliance")
	flag.Parse()
	if runtime.GOOS != "windows" {
		fatalf("native Windows smoke must run on Windows")
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

	if err := verifyTamperRejection(absAppDir, tempRoot); err != nil {
		return err
	}

	fmt.Println("WINDOWS_PILOT_SMOKE PASS")
	fmt.Println("target=windows-amd64")
	fmt.Printf("pack_id=%s\n", expectedPackID)
	fmt.Printf("workflow_id=%s\n", firstInfo.workflowID)
	fmt.Println("healthz=200 ui=200 bootstrap=200 status=200")
	fmt.Println("single_instance=reused-existing")
	fmt.Println("restart=stable-workflow-and-config")
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
	process := &managedProcess{done: make(chan error, 1)}
	process.cmd = exec.Command(exePath)
	process.cmd.Dir = appDir
	process.cmd.Env = replaceEnv(os.Environ(), "LOCALAPPDATA", localAppData)
	process.cmd.Stdout = &process.stdout
	process.cmd.Stderr = &process.stderr
	configureProcess(process.cmd)
	if err := process.cmd.Start(); err != nil {
		return nil, fmt.Errorf("start extracted appliance: %w", err)
	}
	go func() {
		process.done <- process.cmd.Wait()
		close(process.done)
	}()
	return process, nil
}

func waitForReady(process *managedProcess, timeout time.Duration) (*applianceInfo, error) {
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
				info, err := inspectAppliance(client, origin)
				if err == nil {
					return info, nil
				}
			}
		}
		time.Sleep(250 * time.Millisecond)
	}
	return nil, fmt.Errorf("appliance did not become ready: %s", combinedOutput(process))
}

func inspectAppliance(client *http.Client, origin string) (*applianceInfo, error) {
	if err := getOK(client, origin+"/"); err != nil {
		return nil, fmt.Errorf("UI probe: %w", err)
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
