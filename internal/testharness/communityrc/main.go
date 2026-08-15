package main

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/subtle"
	"database/sql"
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
	"strconv"
	"strings"
	"time"

	"goflow/internal/buildinfo"
	"goflow/internal/communityartifact"
	"goflow/internal/crypto"
	"goflow/internal/storage"

	_ "modernc.org/sqlite"
)

const (
	secretCanary       = "community-stable-secret-canary-7d95d4"
	maxStartupLogBytes = 64 << 10
	rcSourceVersion    = "1.0.0-rc.1"
	rcSourceChannel    = "community-rc"
	rcSourceCommit     = "0fdf961ecf67a6ec903d6555b48f67d937728a08"
)

func main() {
	if len(os.Args) < 2 {
		fatal("mode is required: build, verify, smoke, upgrade, or backup-restore")
	}
	var err error
	switch os.Args[1] {
	case "build":
		err = build(os.Args[2:])
	case "verify":
		err = verify(os.Args[2:])
	case "smoke":
		err = smoke(os.Args[2:])
	case "upgrade":
		err = upgrade(os.Args[2:])
	case "backup-restore":
		err = backupRestore(os.Args[2:])
	default:
		err = fmt.Errorf("unknown mode %q", os.Args[1])
	}
	if err != nil {
		fatal(err.Error())
	}
}

func build(args []string) error {
	fs := flag.NewFlagSet("build", flag.ContinueOnError)
	runtimePath := fs.String("runtime", "", "runtime executable")
	license := fs.String("license", "LICENSE", "license path")
	output := fs.String("output", "", "output directory")
	commit := fs.String("commit", "", "full source commit")
	target := fs.String("target", "", "target")
	if err := fs.Parse(args); err != nil {
		return err
	}
	result, err := communityartifact.Build(communityartifact.BuildOptions{RuntimePath: *runtimePath, LicensePath: *license, OutputDir: *output, Version: communityartifact.ReleaseVersion, Channel: communityartifact.ReleaseChannel, Commit: *commit, Target: *target})
	if err != nil {
		return err
	}
	if err := communityartifact.VerifyChecksumFile(result.ArchivePath, result.ChecksumPath); err != nil {
		return err
	}
	fmt.Printf("archive=%s\nchecksum=%s\nsha256=%s\n", result.ArchivePath, result.ChecksumPath, result.SHA256)
	return nil
}

func verify(args []string) error {
	fs := flag.NewFlagSet("verify", flag.ContinueOnError)
	archive := fs.String("archive", "", "archive path")
	commit := fs.String("commit", "", "full source commit")
	target := fs.String("target", "", "target")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if err := communityartifact.VerifyChecksumFile(*archive, *archive+".sha256"); err != nil {
		return err
	}
	metadata, err := communityartifact.Verify(*archive, communityartifact.VerifyOptions{Version: communityartifact.ReleaseVersion, Channel: communityartifact.ReleaseChannel, Commit: *commit, Target: *target})
	if err != nil {
		return err
	}
	fmt.Printf("verified=%s target=%s commit=%s\n", filepath.Base(*archive), metadata.Target, metadata.Commit)
	return nil
}

func smoke(args []string) error {
	archive, expected, err := acceptanceFlags("smoke", args)
	if err != nil {
		return err
	}
	if err := communityartifact.VerifyChecksumFile(archive, archive+".sha256"); err != nil {
		return err
	}
	metadata, err := communityartifact.Verify(archive, expected)
	if err != nil {
		return err
	}
	appDir, cleanup, err := extractArtifact(archive)
	if err != nil {
		return err
	}
	defer cleanup()
	if err := assertExactAppDir(appDir, metadata.RuntimePath()); err != nil {
		return err
	}
	runtimePath := filepath.Join(appDir, metadata.RuntimePath())
	if err := verifyVersionCommand(runtimePath, *metadata); err != nil {
		return err
	}
	dataDir, err := os.MkdirTemp("", "goflow-community-data-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(dataDir)
	workflowID, credentialID, err := exerciseRuntime(runtimePath, dataDir, true)
	if err != nil {
		return err
	}
	if workflowID == "" || credentialID == "" {
		return fmt.Errorf("representative state identity was empty")
	}
	if err := exerciseRestart(runtimePath, dataDir, workflowID, credentialID); err != nil {
		return err
	}
	if err := assertExactAppDir(appDir, metadata.RuntimePath()); err != nil {
		return err
	}
	if err := rejectPlaintext(dataDir, secretCanary); err != nil {
		return err
	}
	fmt.Printf("smoke=success restart=success credential=decrypted ui=200 healthz=200 workflow=%s data_dir=external\n", workflowID)
	return nil
}

func upgrade(args []string) error {
	fs := flag.NewFlagSet("upgrade", flag.ContinueOnError)
	baseRuntime := fs.String("base-runtime", "", "runtime built from exact RC commit")
	baseCommit := fs.String("base-commit", "", "exact RC commit")
	archive := fs.String("archive", "", "candidate archive")
	commit := fs.String("commit", "", "candidate commit")
	target := fs.String("target", "linux-amd64", "candidate target")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *baseCommit != rcSourceCommit {
		return fmt.Errorf("base commit must be exact RC source %s", rcSourceCommit)
	}
	if err := verifyExactVersionCommand(*baseRuntime, buildinfo.Info{Version: rcSourceVersion, Channel: rcSourceChannel, Commit: rcSourceCommit, Target: *target}); err != nil {
		return fmt.Errorf("RC runtime identity: %w", err)
	}
	if err := communityartifact.VerifyChecksumFile(*archive, *archive+".sha256"); err != nil {
		return err
	}
	metadata, err := communityartifact.Verify(*archive, communityartifact.VerifyOptions{Version: communityartifact.ReleaseVersion, Channel: communityartifact.ReleaseChannel, Commit: *commit, Target: *target})
	if err != nil {
		return err
	}
	appDir, cleanup, err := extractArtifact(*archive)
	if err != nil {
		return err
	}
	defer cleanup()
	candidate := filepath.Join(appDir, metadata.RuntimePath())
	dataDir, err := os.MkdirTemp("", "goflow-community-upgrade-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(dataDir)
	workflowID, credentialID, err := exerciseRuntime(*baseRuntime, dataDir, true)
	if err != nil {
		return fmt.Errorf("RC runtime: %w", err)
	}
	if err := exerciseRestart(candidate, dataDir, workflowID, credentialID); err != nil {
		return fmt.Errorf("Stable runtime over RC data: %w", err)
	}
	if err := exerciseRestart(candidate, dataDir, workflowID, credentialID); err != nil {
		return fmt.Errorf("Stable second restart over RC data: %w", err)
	}
	if err := rejectPlaintext(dataDir, secretCanary); err != nil {
		return err
	}
	corruptDir, err := copyDataDir(dataDir)
	if err != nil {
		return err
	}
	defer os.RemoveAll(corruptDir)
	if err := injectFutureMigration(filepath.Join(corruptDir, "goflow.db")); err != nil {
		return err
	}
	logs, err := expectStartupFailure(candidate, corruptDir)
	if err != nil {
		return err
	}
	if strings.Contains(logs, secretCanary) {
		return fmt.Errorf("failed startup leaked credential plaintext")
	}
	fmt.Printf("upgrade=success base_commit=%s workflow=%s credential=decrypted corrupt_migration=failed-closed\n", *baseCommit, workflowID)
	return nil
}

func backupRestore(args []string) error {
	archive, expected, err := acceptanceFlags("backup-restore", args)
	if err != nil {
		return err
	}
	if err := communityartifact.VerifyChecksumFile(archive, archive+".sha256"); err != nil {
		return err
	}
	metadata, err := communityartifact.Verify(archive, expected)
	if err != nil {
		return err
	}
	appDir, cleanup, err := extractArtifact(archive)
	if err != nil {
		return err
	}
	defer cleanup()
	runtimePath := filepath.Join(appDir, metadata.RuntimePath())
	if err := verifyVersionCommand(runtimePath, *metadata); err != nil {
		return err
	}
	sourceDir, err := os.MkdirTemp("", "goflow-community-backup-source-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(sourceDir)
	workflowID, credentialID, err := exerciseRuntime(runtimePath, sourceDir, true)
	if err != nil {
		return fmt.Errorf("create stopped backup source: %w", err)
	}
	backupDir, err := os.MkdirTemp("", "goflow-community-backup-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(backupDir)
	for _, name := range []string{"goflow.db", "goflow.master.key"} {
		if err := copyRegularFile(filepath.Join(sourceDir, name), filepath.Join(backupDir, name)); err != nil {
			return fmt.Errorf("back up stopped state: %w", err)
		}
	}
	restoredDir, err := restoreFiles(backupDir, "goflow.db", "goflow.master.key")
	if err != nil {
		return err
	}
	defer os.RemoveAll(restoredDir)
	if err := exerciseRestart(runtimePath, restoredDir, workflowID, credentialID); err != nil {
		return fmt.Errorf("restored state: %w", err)
	}
	if err := exerciseRestart(runtimePath, restoredDir, workflowID, credentialID); err != nil {
		return fmt.Errorf("restored state second restart: %w", err)
	}
	dbOnlyDir, err := restoreFiles(backupDir, "goflow.db")
	if err != nil {
		return err
	}
	defer os.RemoveAll(dbOnlyDir)
	if err := verifyPartialRestore(runtimePath, dbOnlyDir, workflowID, credentialID, true); err != nil {
		return fmt.Errorf("database-only restore: %w", err)
	}
	keyOnlyDir, err := restoreFiles(backupDir, "goflow.master.key")
	if err != nil {
		return err
	}
	defer os.RemoveAll(keyOnlyDir)
	if err := verifyPartialRestore(runtimePath, keyOnlyDir, workflowID, credentialID, false); err != nil {
		return fmt.Errorf("key-only restore: %w", err)
	}
	if err := rejectPlaintext(appDir, secretCanary); err != nil {
		return err
	}
	fmt.Printf("backup_restore=success workflow=%s credential=decrypted database_only=failed-safe key_only=failed-safe stopped_runtime=true\n", workflowID)
	return nil
}

func acceptanceFlags(name string, args []string) (string, communityartifact.VerifyOptions, error) {
	fs := flag.NewFlagSet(name, flag.ContinueOnError)
	archive := fs.String("archive", "", "archive")
	commit := fs.String("commit", "", "commit")
	target := fs.String("target", "linux-amd64", "target")
	if err := fs.Parse(args); err != nil {
		return "", communityartifact.VerifyOptions{}, err
	}
	return *archive, communityartifact.VerifyOptions{Version: communityartifact.ReleaseVersion, Channel: communityartifact.ReleaseChannel, Commit: *commit, Target: *target}, nil
}

func extractArtifact(path string) (string, func(), error) {
	dir, err := os.MkdirTemp("", "goflow-community-app-*")
	if err != nil {
		return "", func() {}, err
	}
	cleanup := func() { os.RemoveAll(dir) }
	r, err := zip.OpenReader(path)
	if err != nil {
		cleanup()
		return "", func() {}, err
	}
	defer r.Close()
	for _, f := range r.File {
		rc, e := f.Open()
		if e != nil {
			cleanup()
			return "", func() {}, e
		}
		data, e := io.ReadAll(rc)
		rc.Close()
		if e != nil {
			cleanup()
			return "", func() {}, e
		}
		out := filepath.Join(dir, f.Name)
		if e = os.WriteFile(out, data, f.Mode().Perm()); e != nil {
			cleanup()
			return "", func() {}, e
		}
	}
	return dir, cleanup, nil
}

func assertExactAppDir(dir, runtimeName string) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return err
	}
	got := make([]string, 0, len(entries))
	for _, e := range entries {
		if e.IsDir() {
			return fmt.Errorf("application directory contains unexpected directory %q", e.Name())
		}
		got = append(got, e.Name())
	}
	sort.Strings(got)
	want := []string{"COMMUNITY_ARTIFACT.json", "LICENSE", "README.txt", runtimeName}
	sort.Strings(want)
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		return fmt.Errorf("application directory changed: got %v want %v", got, want)
	}
	return nil
}

func verifyVersionCommand(path string, metadata communityartifact.Metadata) error {
	return verifyExactVersionCommand(path, buildinfo.Info{Version: metadata.Version, Channel: metadata.Channel, Commit: metadata.Commit, Target: metadata.Target})
}

func verifyExactVersionCommand(path string, expected buildinfo.Info) error {
	cmd := exec.Command(path, "version", "--output", "json")
	out, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("version command: %w", err)
	}
	var info buildinfo.Info
	if err := json.Unmarshal(out, &info); err != nil {
		return err
	}
	if info.Version != expected.Version || info.Channel != expected.Channel || info.Commit != expected.Commit || info.Target != expected.Target {
		return fmt.Errorf("runtime identity mismatch: got version=%q channel=%q commit=%q target=%q", info.Version, info.Channel, info.Commit, info.Target)
	}
	return nil
}

type process struct {
	cmd     *exec.Cmd
	done    chan error
	logPath string
	logFile *os.File
}

var (
	serverURLPattern     = regexp.MustCompile(`(?m)^.*Goflow Web Server running on (\S+)\r?$`)
	errServerURLNotFound = errors.New("server URL not found")
)

func startRuntime(path, dataDir string) (*process, string, error) {
	logFile, err := os.CreateTemp("", "goflow-community-log-*.txt")
	if err != nil {
		return nil, "", err
	}
	cmd := exec.Command(path, "serve")
	cmd.Env = append(os.Environ(), "GOFLOW_HOST=127.0.0.1", "GOFLOW_PORT=0", "GOFLOW_DB_PATH="+filepath.Join(dataDir, "goflow.db"), "GOFLOW_MASTER_KEY_FILE="+filepath.Join(dataDir, "goflow.master.key"), "GOFLOW_LOG_LEVEL=info")
	cmd.Stdout = logFile
	cmd.Stderr = logFile
	if err := cmd.Start(); err != nil {
		logFile.Close()
		os.Remove(logFile.Name())
		return nil, "", err
	}
	p := &process{cmd: cmd, done: make(chan error, 1), logPath: logFile.Name(), logFile: logFile}
	go func() { p.done <- cmd.Wait(); close(p.done) }()
	origin, err := waitReady(p, 30*time.Second)
	if err != nil {
		_ = stopRuntime(p)
		return nil, "", err
	}
	return p, origin, nil
}

func stopRuntime(p *process) error {
	if p == nil {
		return nil
	}
	if runtime.GOOS == "windows" {
		if err := p.cmd.Process.Kill(); err != nil {
			return err
		}
		<-p.done
		p.logFile.Close()
		os.Remove(p.logPath)
		return nil
	}
	_ = p.cmd.Process.Signal(os.Interrupt)
	select {
	case err := <-p.done:
		p.logFile.Close()
		os.Remove(p.logPath)
		if err != nil {
			return fmt.Errorf("runtime exit: %w", err)
		}
		return nil
	case <-time.After(8 * time.Second):
		_ = p.cmd.Process.Kill()
		<-p.done
		p.logFile.Close()
		logs, _ := os.ReadFile(p.logPath)
		os.Remove(p.logPath)
		return fmt.Errorf("runtime did not stop gracefully: %s", logs)
	}
}

func waitReady(p *process, timeout time.Duration) (string, error) {
	deadline := time.Now().Add(timeout)
	client := &http.Client{Timeout: time.Second}
	for time.Now().Before(deadline) {
		select {
		case err := <-p.done:
			return "", fmt.Errorf("runtime exited before readiness: %v", err)
		default:
		}
		logs, err := readStartupLog(p.logPath)
		if err != nil {
			return "", err
		}
		origin, err := parseServerURL(logs)
		if err == nil {
			if res, probeErr := client.Get(origin + "/healthz"); probeErr == nil {
				res.Body.Close()
				if res.StatusCode == http.StatusOK {
					return origin, nil
				}
			}
		} else if !errors.Is(err, errServerURLNotFound) {
			return "", err
		}
		time.Sleep(100 * time.Millisecond)
	}
	return "", fmt.Errorf("runtime readiness timeout")
}

func readStartupLog(path string) ([]byte, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, fmt.Errorf("inspect startup log: %w", err)
	}
	if info.Size() > maxStartupLogBytes {
		return nil, fmt.Errorf("startup log exceeded bounded size")
	}
	return os.ReadFile(path)
}

func parseServerURL(logs []byte) (string, error) {
	if len(logs) > maxStartupLogBytes {
		return "", fmt.Errorf("startup log exceeded bounded size")
	}
	matches := serverURLPattern.FindAllSubmatch(logs, -1)
	if len(matches) == 0 {
		return "", errServerURLNotFound
	}
	if len(matches) != 1 {
		return "", fmt.Errorf("startup log contains multiple server URLs")
	}
	raw := string(matches[0][1])
	parsed, err := url.Parse(raw)
	if err != nil || parsed.Scheme != "http" || parsed.User != nil || parsed.Hostname() != "127.0.0.1" || parsed.Path != "" || parsed.RawQuery != "" || parsed.Fragment != "" {
		return "", fmt.Errorf("startup log contains invalid loopback server URL")
	}
	port, err := strconv.Atoi(parsed.Port())
	if err != nil || port < 1 || port > 65535 || parsed.Host != "127.0.0.1:"+strconv.Itoa(port) {
		return "", fmt.Errorf("startup log contains invalid loopback server port")
	}
	return "http://127.0.0.1:" + strconv.Itoa(port), nil
}

func exerciseRuntime(path, dataDir string, create bool) (string, string, error) {
	p, origin, err := startRuntime(path, dataDir)
	if err != nil {
		return "", "", err
	}
	var workflowID string
	var credentialID string
	defer func() {
		if p != nil {
			_ = stopRuntime(p)
		}
	}()
	if err := getStatus(origin+"/", http.StatusOK); err != nil {
		return "", "", fmt.Errorf("UI: %w", err)
	}
	if create {
		var wf struct {
			ID string `json:"id"`
		}
		if err := postJSON(origin+"/api/v1/workflows", map[string]any{"name": "Community upgrade fixture", "description": "Phase 1 compatibility fixture", "nodes_json": "[]", "edges_json": "[]"}, http.StatusCreated, &wf); err != nil {
			return "", "", err
		}
		workflowID = wf.ID
		var credential struct {
			ID string `json:"id"`
		}
		if err := postJSON(origin+"/api/v1/credentials", map[string]any{"name": "Compatibility credential", "type": "API_KEY", "data": secretCanary}, http.StatusCreated, &credential); err != nil {
			return "", "", err
		}
		credentialID = credential.ID
	}
	if err := stopRuntime(p); err != nil {
		return "", "", err
	}
	p = nil
	if create {
		if err := verifyCredentialAtRest(dataDir, credentialID); err != nil {
			return "", "", err
		}
	}
	return workflowID, credentialID, nil
}

func exerciseRestart(path, dataDir, workflowID, credentialID string) error {
	p, origin, err := startRuntime(path, dataDir)
	if err != nil {
		return err
	}
	defer func() {
		if p != nil {
			_ = stopRuntime(p)
		}
	}()
	if err := getStatus(origin+"/api/v1/workflows/"+workflowID, http.StatusOK); err != nil {
		return err
	}
	var workflows []map[string]any
	if err := getJSON(origin+"/api/v1/workflows", &workflows); err != nil {
		return err
	}
	if len(workflows) != 1 {
		return fmt.Errorf("workflow duplicated or missing after restart: count=%d", len(workflows))
	}
	if id, _ := workflows[0]["id"].(string); id != workflowID {
		return fmt.Errorf("workflow identity changed after restart")
	}
	var credentials []map[string]any
	if err := getJSON(origin+"/api/v1/credentials", &credentials); err != nil {
		return err
	}
	if len(credentials) != 1 {
		return fmt.Errorf("credential duplicated or missing after restart: count=%d", len(credentials))
	}
	if id, _ := credentials[0]["id"].(string); id != credentialID {
		return fmt.Errorf("credential identity changed after restart")
	}
	if err := stopRuntime(p); err != nil {
		return err
	}
	p = nil
	return verifyCredentialAtRest(dataDir, credentialID)
}

func verifyCredentialAtRest(dataDir, credentialID string) error {
	keyFile, err := os.Open(filepath.Join(dataDir, "goflow.master.key"))
	if err != nil {
		return fmt.Errorf("credential decryption evidence could not load the test key")
	}
	keyData, readErr := io.ReadAll(io.LimitReader(keyFile, 1025))
	closeErr := keyFile.Close()
	if readErr != nil || closeErr != nil || len(keyData) == 0 || len(keyData) > 1024 {
		return fmt.Errorf("credential decryption evidence could not load the test key")
	}
	db, err := storage.NewDB(filepath.Join(dataDir, "goflow.db"))
	if err != nil {
		return fmt.Errorf("credential decryption evidence could not open test state")
	}
	defer db.Close()
	store := storage.NewCredentialStore(db, crypto.NewCryptoManager(strings.TrimSpace(string(keyData))))
	credentials, err := store.ListAll()
	if err != nil || len(credentials) != 1 || credentials[0].ID != credentialID {
		return fmt.Errorf("credential decryption evidence found unexpected test state")
	}
	decrypted, err := store.GetDecryptedData(credentialID)
	if err != nil || subtle.ConstantTimeCompare([]byte(decrypted), []byte(secretCanary)) != 1 {
		return fmt.Errorf("credential decryption evidence did not match")
	}
	return nil
}

func getStatus(url string, want int) error {
	res, err := (&http.Client{Timeout: 5 * time.Second}).Get(url)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	if res.StatusCode != want {
		return fmt.Errorf("GET %s returned %d", url, res.StatusCode)
	}
	return nil
}
func getJSON(url string, out any) error {
	res, err := (&http.Client{Timeout: 5 * time.Second}).Get(url)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return fmt.Errorf("GET %s returned %d", url, res.StatusCode)
	}
	return json.NewDecoder(io.LimitReader(res.Body, 1<<20)).Decode(out)
}
func postJSON(url string, payload any, want int, out any) error {
	data, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	res, err := (&http.Client{Timeout: 5 * time.Second}).Post(url, "application/json", bytes.NewReader(data))
	if err != nil {
		return err
	}
	defer res.Body.Close()
	body, err := io.ReadAll(io.LimitReader(res.Body, 1<<20))
	if err != nil {
		return err
	}
	if res.StatusCode != want {
		return fmt.Errorf("POST %s returned %d: %s", url, res.StatusCode, body)
	}
	if out != nil {
		return json.Unmarshal(body, out)
	}
	return nil
}

func rejectPlaintext(root, canary string) error {
	return filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		if bytes.Contains(data, []byte(canary)) {
			return fmt.Errorf("credential plaintext leaked into %s", filepath.Base(path))
		}
		return nil
	})
}

func restoreFiles(backupDir string, names ...string) (string, error) {
	dest, err := os.MkdirTemp("", "goflow-community-restore-*")
	if err != nil {
		return "", err
	}
	for _, name := range names {
		if err := copyRegularFile(filepath.Join(backupDir, name), filepath.Join(dest, name)); err != nil {
			os.RemoveAll(dest)
			return "", err
		}
	}
	return dest, nil
}

func copyRegularFile(source, destination string) error {
	info, err := os.Lstat(source)
	if err != nil {
		return err
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("backup source %q is not a regular file", filepath.Base(source))
	}
	data, err := os.ReadFile(source)
	if err != nil {
		return err
	}
	return os.WriteFile(destination, data, 0600)
}

func verifyPartialRestore(runtimePath, dataDir, workflowID, credentialID string, databaseOnly bool) error {
	p, origin, err := startRuntime(runtimePath, dataDir)
	if err != nil {
		return fmt.Errorf("runtime did not start safely for inspection: %w", err)
	}
	defer func() {
		if p != nil {
			_ = stopRuntime(p)
		}
	}()
	var workflows []map[string]any
	if err := getJSON(origin+"/api/v1/workflows", &workflows); err != nil {
		return err
	}
	var credentials []map[string]any
	if err := getJSON(origin+"/api/v1/credentials", &credentials); err != nil {
		return err
	}
	if databaseOnly {
		if len(workflows) != 1 || workflows[0]["id"] != workflowID {
			return fmt.Errorf("database-only restore changed workflow identity")
		}
		if len(credentials) != 1 || credentials[0]["id"] != credentialID {
			return fmt.Errorf("database-only restore changed credential identity")
		}
	} else {
		if len(workflows) != 0 || len(credentials) != 0 {
			return fmt.Errorf("key-only restore unexpectedly recovered state")
		}
		if err := getStatus(origin+"/api/v1/workflows/"+workflowID, http.StatusNotFound); err != nil {
			return fmt.Errorf("key-only restore exposed original workflow: %w", err)
		}
	}
	if err := stopRuntime(p); err != nil {
		return err
	}
	p = nil
	if databaseOnly {
		if err := verifyCredentialAtRest(dataDir, credentialID); err == nil {
			return fmt.Errorf("database-only restore decrypted credential without the backed-up key")
		}
	}
	return rejectPlaintext(dataDir, secretCanary)
}

func copyDataDir(source string) (string, error) {
	dest, err := os.MkdirTemp("", "goflow-community-corrupt-*")
	if err != nil {
		return "", err
	}
	entries, err := os.ReadDir(source)
	if err != nil {
		os.RemoveAll(dest)
		return "", err
	}
	for _, entry := range entries {
		if !entry.Type().IsRegular() {
			continue
		}
		data, err := os.ReadFile(filepath.Join(source, entry.Name()))
		if err != nil {
			os.RemoveAll(dest)
			return "", err
		}
		if err := os.WriteFile(filepath.Join(dest, entry.Name()), data, 0600); err != nil {
			os.RemoveAll(dest)
			return "", err
		}
	}
	return dest, nil
}

func injectFutureMigration(path string) error {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return err
	}
	defer db.Close()
	_, err = db.Exec(`INSERT INTO schema_migrations (version,name,applied_at) VALUES (999,'future-unsupported',CURRENT_TIMESTAMP)`)
	return err
}

func expectStartupFailure(path, dataDir string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, path, "serve")
	cmd.Env = append(os.Environ(), "GOFLOW_HOST=127.0.0.1", "GOFLOW_PORT=0", "GOFLOW_DB_PATH="+filepath.Join(dataDir, "goflow.db"), "GOFLOW_MASTER_KEY_FILE="+filepath.Join(dataDir, "goflow.master.key"))
	output, err := cmd.CombinedOutput()
	if err == nil {
		return string(output), fmt.Errorf("runtime accepted unsupported migration")
	}
	if errors.Is(ctx.Err(), context.DeadlineExceeded) {
		return string(output), fmt.Errorf("runtime hung instead of failing closed")
	}
	if !strings.Contains(string(output), "unsupported") {
		return string(output), fmt.Errorf("runtime failed without explicit unsupported-migration error: %s", output)
	}
	return string(output), nil
}

func fatal(message string) {
	fmt.Fprintln(os.Stderr, "community release harness:", message)
	os.Exit(1)
}
