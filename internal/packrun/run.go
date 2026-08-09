package packrun

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	"goflow/config"
	"goflow/internal/api"
	"goflow/internal/pack"
	"goflow/internal/serverapp"
	"goflow/internal/storage"
	"goflow/internal/workflow"

	"github.com/google/uuid"
	_ "modernc.org/sqlite"
)

type Options struct {
	PackDir string
	DataDir string
	Port    int
	NoOpen  bool
	UIFS    fs.FS
	Stdout  io.Writer
	Stderr  io.Writer
	Opener  func(string) error
}

type State struct {
	PackID              string `json:"pack_id"`
	PackVersion         string `json:"pack_version"`
	WorkflowID          string `json:"workflow_id"`
	WorkflowContentHash string `json:"workflow_content_sha256"`
	UpdatedAt           string `json:"updated_at"`
}

type RunState struct {
	PackID    string `json:"pack_id"`
	URL       string `json:"url"`
	PID       int    `json:"pid"`
	StartedAt string `json:"started_at"`
}

type Prepared struct {
	Pack        *pack.Pack
	DataDir     string
	DBPath      string
	MasterKey   string
	WorkflowID  string
	WorkflowURL string
	State       State
}

func RunExtractedBundle(ctx context.Context, bundleDir string, opts Options) error {
	if _, err := pack.VerifyExtractedBundle(bundleDir); err != nil {
		return fmt.Errorf("pack run: extracted bundle verification failed: %w", err)
	}
	opts.PackDir = filepath.Join(bundleDir, "pack")
	return Run(ctx, opts)
}

func Run(ctx context.Context, opts Options) error {
	if ctx == nil {
		ctx = context.Background()
	}
	if opts.Stdout == nil {
		opts.Stdout = io.Discard
	}
	if opts.Stderr == nil {
		opts.Stderr = io.Discard
	}
	loaded, err := pack.Load(opts.PackDir)
	if err != nil {
		return err
	}
	if len(loaded.Manifest.Plugins) > 0 {
		return fmt.Errorf("pack run: packaged plugin execution is not supported in Pack Run MVP")
	}
	dataDir, err := resolveDataDir(opts.DataDir, loaded.Manifest.ID)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dataDir, 0700); err != nil {
		return fmt.Errorf("pack run: create data directory: %w", err)
	}
	lock, acquired, err := acquireDataLock(dataDir)
	if err != nil {
		return err
	}
	if !acquired {
		return reuseExistingInstance(ctx, opts, dataDir, loaded.Manifest.ID, defaultReuseRetryOptions())
	}
	defer lock.Release()
	if err := removeRunState(dataDir); err != nil {
		return err
	}

	prepared, err := prepare(ctx, loaded, dataDir)
	if err != nil {
		return err
	}
	listener, err := net.Listen("tcp", "127.0.0.1:"+strconv.Itoa(opts.Port))
	if err != nil {
		return fmt.Errorf("pack run: listen on loopback: %w", err)
	}
	origin := "http://" + listener.Addr().String()
	sessionToken, err := generateSessionToken()
	if err != nil {
		_ = listener.Close()
		return fmt.Errorf("pack run: create appliance session token: %w", err)
	}
	serverCfg := &config.Config{
		Host:                      "127.0.0.1",
		Port:                      strconv.Itoa(opts.Port),
		DBPath:                    prepared.DBPath,
		MasterKey:                 prepared.MasterKey,
		MaxConcurrentExecutions:   10,
		MaxParallelNodesPerRun:    4,
		WebhookRateLimitPerMinute: 60,
		ExecutionRetentionDays:    30,
		MaxExecutionsPerWorkflow:  1000,
		MCPAllowedOrigins:         []string{"http://127.0.0.1"},
		MCPMaxInflightPerClient:   2,
		MCPRateLimitPerMinute:     30,
	}
	app, err := serverapp.Start(ctx, serverapp.Options{
		Config:   serverCfg,
		UIFS:     opts.UIFS,
		Listener: listener,
		Appliance: &api.ApplianceContext{
			Enabled:                true,
			Origin:                 origin,
			SessionToken:           sessionToken,
			PackID:                 loaded.Manifest.ID,
			PackName:               loaded.Manifest.Name,
			PackVersion:            loaded.Manifest.Version,
			Description:            loaded.Manifest.Description,
			WorkflowID:             prepared.WorkflowID,
			DataDir:                dataDir,
			ConfigSchema:           loaded.Manifest.ConfigSchema,
			CredentialRequirements: loaded.Manifest.CredentialRequirements,
			LegacyRequiredCreds:    loaded.Manifest.RequiredCredentials,
		},
	})
	if err != nil {
		return err
	}
	targetURL := app.URL + prepared.WorkflowURL
	if err := writeRunState(dataDir, RunState{
		PackID:    loaded.Manifest.ID,
		URL:       targetURL,
		PID:       os.Getpid(),
		StartedAt: time.Now().UTC().Format(time.RFC3339),
	}); err != nil {
		_ = app.Shutdown(context.Background())
		return err
	}
	fmt.Fprintf(opts.Stdout, "Pack running\nURL: %s\nData directory: %s\nWorkflow ID: %s\n", targetURL, dataDir, prepared.WorkflowID)
	printCredentialRequirements(opts.Stdout, loaded.Manifest.RequiredCredentials)
	if !opts.NoOpen {
		if err := openURL(opts, targetURL); err != nil {
			fmt.Fprintf(opts.Stderr, "warning: could not open browser: %v\n", err)
		}
	}
	return <-app.Done()
}

func prepare(ctx context.Context, loaded *pack.Pack, dataDir string) (*Prepared, error) {
	dbPath := filepath.Join(dataDir, "goflow.db")
	keyPath := filepath.Join(dataDir, "goflow.master.key")
	masterKey, err := loadOrCreateMasterKey(keyPath)
	if err != nil {
		return nil, err
	}
	db, err := storage.NewDB(dbPath)
	if err != nil {
		return nil, fmt.Errorf("pack run: initialize data database: %w", err)
	}
	defer db.Close()
	wfStore := storage.NewWorkflowStore(db)
	workflowID := StableWorkflowID(loaded.Manifest.ID)
	workflowHash, err := fileSHA256Limited(loaded.EntryWorkflowPath, pack.MaxWorkflowBytes)
	if err != nil {
		return nil, err
	}
	workflowDef, err := workflow.ReadFileLimit(loaded.EntryWorkflowPath, pack.MaxWorkflowBytes)
	if err != nil {
		return nil, fmt.Errorf("pack run: read workflow: %w", err)
	}
	if err := workflow.ValidateDefinition(workflowDef); err != nil {
		return nil, fmt.Errorf("pack run: validate workflow: %w", err)
	}
	managed := &storage.Workflow{
		ID:                workflowID,
		Name:              workflowDef.Name,
		Description:       managedDescription(loaded.Manifest.Description, workflowDef.Description),
		IsActive:          workflowDef.IsActive,
		NodesJSON:         workflowDef.NodesJSON,
		EdgesJSON:         workflowDef.EdgesJSON,
		Slug:              workflowDef.Slug,
		InputSchemaJSON:   workflowDef.InputSchemaJSON,
		OutputSchemaJSON:  workflowDef.OutputSchemaJSON,
		ExposeCLI:         workflowDef.ExposeCLI,
		ExposeMCP:         workflowDef.ExposeMCP,
		MCPToolName:       workflowDef.MCPToolName,
		MCPDescription:    workflowDef.MCPDescription,
		RiskLevel:         workflowDef.RiskLevel,
		RequiresApproval:  workflowDef.RequiresApproval,
		MaxConcurrentRuns: workflowDef.MaxConcurrentRuns,
		ConcurrencyPolicy: workflowDef.ConcurrencyPolicy,
	}
	if existing, err := wfStore.GetByID(workflowID); err == nil {
		managed.CreatedAt = existing.CreatedAt
		if !sameManagedWorkflow(existing, managed) {
			if err := wfStore.Update(managed); err != nil {
				return nil, fmt.Errorf("pack run: update managed workflow: %w", err)
			}
		}
	} else if strings.Contains(err.Error(), "not found") {
		if err := wfStore.Create(managed); err != nil {
			return nil, fmt.Errorf("pack run: create managed workflow: %w", err)
		}
	} else {
		return nil, fmt.Errorf("pack run: read managed workflow: %w", err)
	}
	state := State{
		PackID:              loaded.Manifest.ID,
		PackVersion:         loaded.Manifest.Version,
		WorkflowID:          workflowID,
		WorkflowContentHash: workflowHash,
		UpdatedAt:           time.Now().UTC().Format(time.RFC3339),
	}
	if err := writePackState(dataDir, state); err != nil {
		return nil, err
	}
	return &Prepared{
		Pack:        loaded,
		DataDir:     dataDir,
		DBPath:      dbPath,
		MasterKey:   masterKey,
		WorkflowID:  workflowID,
		WorkflowURL: workflowTargetPath(loaded),
		State:       state,
	}, nil
}

func StableWorkflowID(packID string) string {
	return uuid.NewSHA1(uuid.NameSpaceURL, []byte("goflow-pack:"+packID)).String()
}

func workflowTargetPath(loaded *pack.Pack) string {
	if len(loaded.Manifest.RequiredCredentials) > 0 {
		return "/credentials"
	}
	return "/workflows"
}

func managedDescription(packDescription, workflowDescription string) string {
	if strings.TrimSpace(packDescription) != "" {
		return packDescription
	}
	return workflowDescription
}

func sameManagedWorkflow(a *storage.Workflow, b *storage.Workflow) bool {
	return a.Name == b.Name &&
		a.Description == b.Description &&
		a.IsActive == b.IsActive &&
		a.NodesJSON == b.NodesJSON &&
		a.EdgesJSON == b.EdgesJSON &&
		a.Slug == b.Slug &&
		a.InputSchemaJSON == b.InputSchemaJSON &&
		a.OutputSchemaJSON == b.OutputSchemaJSON &&
		a.ExposeCLI == b.ExposeCLI &&
		a.ExposeMCP == b.ExposeMCP &&
		a.MCPToolName == b.MCPToolName &&
		a.MCPDescription == b.MCPDescription &&
		a.RiskLevel == b.RiskLevel &&
		a.RequiresApproval == b.RequiresApproval &&
		a.MaxConcurrentRuns == b.MaxConcurrentRuns &&
		a.ConcurrencyPolicy == b.ConcurrencyPolicy
}

func resolveDataDir(explicit, packID string) (string, error) {
	if explicit != "" {
		return filepath.Abs(explicit)
	}
	if strings.ContainsAny(packID, `/\`) || strings.Contains(packID, "..") {
		return "", fmt.Errorf("pack run: unsafe pack id for data directory")
	}
	var base string
	switch runtime.GOOS {
	case "windows":
		base = os.Getenv("LOCALAPPDATA")
		if base == "" {
			return "", fmt.Errorf("pack run: LOCALAPPDATA is not set")
		}
		base = filepath.Join(base, "Goflow", "packs")
	case "darwin":
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		base = filepath.Join(home, "Library", "Application Support", "Goflow", "packs")
	default:
		base = os.Getenv("XDG_DATA_HOME")
		if base == "" {
			home, err := os.UserHomeDir()
			if err != nil {
				return "", err
			}
			base = filepath.Join(home, ".local", "share")
		}
		base = filepath.Join(base, "Goflow", "packs")
	}
	return filepath.Join(base, packID), nil
}

func loadOrCreateMasterKey(path string) (string, error) {
	if data, err := os.ReadFile(path); err == nil {
		key := strings.TrimSpace(string(data))
		if key == "" {
			return "", fmt.Errorf("pack run: master key file is empty")
		}
		return key, nil
	} else if !os.IsNotExist(err) {
		return "", err
	}
	keyBytes := make([]byte, 32)
	if _, err := rand.Read(keyBytes); err != nil {
		return "", err
	}
	key := base64.RawURLEncoding.EncodeToString(keyBytes)
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return "", err
	}
	if err := os.WriteFile(path, []byte(key+"\n"), 0600); err != nil {
		return "", err
	}
	return key, nil
}

func generateSessionToken() (string, error) {
	tokenBytes := make([]byte, 32)
	if _, err := rand.Read(tokenBytes); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(tokenBytes), nil
}

func acquireDataLock(dataDir string) (*dataLock, bool, error) {
	db, err := sql.Open("sqlite", filepath.Join(dataDir, "pack-run.lock.db")+"?_busy_timeout=100")
	if err != nil {
		return nil, false, err
	}
	tx, err := db.Begin()
	if err != nil {
		_ = db.Close()
		return nil, false, err
	}
	if _, err := tx.Exec(`CREATE TABLE IF NOT EXISTS lock_holder (id INTEGER PRIMARY KEY, pid INTEGER NOT NULL, updated_at TEXT NOT NULL)`); err != nil {
		_ = tx.Rollback()
		_ = db.Close()
		if isSQLiteLocked(err) {
			return nil, false, nil
		}
		return nil, false, fmt.Errorf("pack run: acquire data lock: %w", err)
	}
	if _, err := tx.Exec(`INSERT OR REPLACE INTO lock_holder (id, pid, updated_at) VALUES (1, ?, ?)`, os.Getpid(), time.Now().UTC().Format(time.RFC3339)); err != nil {
		_ = tx.Rollback()
		_ = db.Close()
		if isSQLiteLocked(err) {
			return nil, false, nil
		}
		return nil, false, fmt.Errorf("pack run: acquire data lock: %w", err)
	}
	return &dataLock{db: db, tx: tx}, true, nil
}

type dataLock struct {
	db *sql.DB
	tx *sql.Tx
}

func (l *dataLock) Release() {
	if l == nil {
		return
	}
	_ = l.tx.Rollback()
	_ = l.db.Close()
}

func isSQLiteLocked(err error) bool {
	lower := strings.ToLower(err.Error())
	return strings.Contains(lower, "locked") || strings.Contains(lower, "busy")
}

type reuseRetryOptions struct {
	Timeout     time.Duration
	Backoff     time.Duration
	MaxAttempts int
	ReadState   func(string) (RunState, error)
	Probe       func(context.Context, string) error
	After       func(context.Context, time.Duration) error
	Now         func() time.Time
}

func defaultReuseRetryOptions() reuseRetryOptions {
	return reuseRetryOptions{
		Timeout:   4 * time.Second,
		Backoff:   100 * time.Millisecond,
		ReadState: readRunState,
		Probe:     probeHealth,
		Now:       time.Now,
		After: func(ctx context.Context, d time.Duration) error {
			timer := time.NewTimer(d)
			defer timer.Stop()
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-timer.C:
				return nil
			}
		},
	}
}

func reuseExistingInstance(ctx context.Context, opts Options, dataDir, expectedPackID string, retry reuseRetryOptions) error {
	if retry.Timeout <= 0 {
		retry.Timeout = 4 * time.Second
	}
	if retry.Backoff <= 0 {
		retry.Backoff = 100 * time.Millisecond
	}
	if retry.ReadState == nil {
		retry.ReadState = readRunState
	}
	if retry.Probe == nil {
		retry.Probe = probeHealth
	}
	if retry.Now == nil {
		retry.Now = time.Now
	}
	if retry.After == nil {
		retry.After = defaultReuseRetryOptions().After
	}
	if ctx == nil {
		ctx = context.Background()
	}
	retryCtx, cancel := context.WithTimeout(ctx, retry.Timeout)
	defer cancel()
	deadline := retry.Now().Add(retry.Timeout)
	var lastErr error
	var observedPackID string
	for attempt := 1; ; attempt++ {
		if err := retryCtx.Err(); err != nil {
			return fmt.Errorf("pack run: cancelled while waiting for existing instance in %s: %w", dataDir, err)
		}
		state, err := retry.ReadState(dataDir)
		if err != nil {
			lastErr = fmt.Errorf("read run-state: %w", err)
		} else if state.PackID != expectedPackID {
			observedPackID = state.PackID
			lastErr = fmt.Errorf("run-state pack_id mismatch: expected %q observed %q", expectedPackID, state.PackID)
		} else if !isLoopbackURL(state.URL) {
			lastErr = fmt.Errorf("refusing non-loopback URL in run-state")
		} else if err := retry.Probe(retryCtx, state.URL); err != nil {
			lastErr = fmt.Errorf("probe %s: %w", state.URL, err)
		} else {
			fmt.Fprintf(opts.Stdout, "Reusing running pack instance\nURL: %s\n", state.URL)
			if !opts.NoOpen {
				if err := openURL(opts, state.URL); err != nil {
					fmt.Fprintf(opts.Stderr, "warning: could not open browser: %v\n", err)
				}
			}
			return nil
		}
		if retry.MaxAttempts > 0 && attempt >= retry.MaxAttempts {
			return reuseTimeoutError(dataDir, expectedPackID, observedPackID, lastErr)
		}
		if retry.Now().Add(retry.Backoff).After(deadline) {
			return reuseTimeoutError(dataDir, expectedPackID, observedPackID, lastErr)
		}
		if err := retry.After(retryCtx, retry.Backoff); err != nil {
			return fmt.Errorf("pack run: cancelled while waiting for existing instance in %s: %w", dataDir, err)
		}
	}
}

func reuseTimeoutError(dataDir, expectedPackID, observedPackID string, lastErr error) error {
	if observedPackID != "" {
		return fmt.Errorf("pack run: timed out waiting for existing instance in %s for pack %q; last observed pack %q; last error: %v", dataDir, expectedPackID, observedPackID, lastErr)
	}
	return fmt.Errorf("pack run: timed out waiting for existing instance in %s for pack %q; last error: %v", dataDir, expectedPackID, lastErr)
}

func probeHealth(ctx context.Context, rawURL string) error {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return err
	}
	healthURL := *parsed
	healthURL.Path = "/healthz"
	healthURL.RawQuery = ""
	probeCtx := ctx
	if deadline, ok := ctx.Deadline(); !ok || time.Until(deadline) > 2*time.Second {
		var cancel context.CancelFunc
		probeCtx, cancel = context.WithTimeout(ctx, 2*time.Second)
		defer cancel()
	}
	req, err := http.NewRequestWithContext(probeCtx, http.MethodGet, healthURL.String(), nil)
	if err != nil {
		return err
	}
	client := http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("healthz status %d", resp.StatusCode)
	}
	return nil
}

func removeRunState(dataDir string) error {
	path := filepath.Join(dataDir, "run-state.json")
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("pack run: remove stale run-state: %w", err)
	}
	return nil
}

func isLoopbackURL(rawURL string) bool {
	parsed, err := url.Parse(rawURL)
	if err != nil || parsed.Scheme != "http" {
		return false
	}
	host := parsed.Hostname()
	ip := net.ParseIP(host)
	return strings.EqualFold(host, "localhost") || (ip != nil && ip.IsLoopback())
}

func openURL(opts Options, rawURL string) error {
	if opts.Opener != nil {
		return opts.Opener(rawURL)
	}
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", rawURL)
	case "darwin":
		cmd = exec.Command("open", rawURL)
	default:
		cmd = exec.Command("xdg-open", rawURL)
	}
	return cmd.Start()
}

func printCredentialRequirements(w io.Writer, credentials []string) {
	if len(credentials) == 0 {
		return
	}
	fmt.Fprintln(w, "Required credentials:")
	for _, credential := range credentials {
		fmt.Fprintf(w, "- %s\n", credential)
	}
}

func writePackState(dataDir string, state State) error {
	return writeJSONAtomic(filepath.Join(dataDir, "pack-state.json"), state)
}

func writeRunState(dataDir string, state RunState) error {
	return writeJSONAtomic(filepath.Join(dataDir, "run-state.json"), state)
}

func readRunState(dataDir string) (RunState, error) {
	var state RunState
	data, err := os.ReadFile(filepath.Join(dataDir, "run-state.json"))
	if err != nil {
		return state, err
	}
	if err := json.Unmarshal(data, &state); err != nil {
		return state, err
	}
	return state, nil
}

func writeJSONAtomic(path string, value interface{}) error {
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	tmp, err := os.CreateTemp(filepath.Dir(path), "."+filepath.Base(path)+".tmp-*")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	defer func() {
		_ = os.Remove(tmpPath)
	}()
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmpPath, path)
}

func fileSHA256Limited(path string, maxBytes int64) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()
	h := sha256.New()
	n, err := io.Copy(h, io.LimitReader(file, maxBytes+1))
	if err != nil {
		return "", err
	}
	if n > maxBytes {
		return "", fmt.Errorf("file exceeds %d byte limit", maxBytes)
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

var ErrNotRunning = errors.New("pack run: not running")
