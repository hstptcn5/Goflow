package serverapp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"log"
	"net"
	"net/http"
	"strconv"
	"sync"
	"time"

	"goflow/config"
	"goflow/internal/api"
	"goflow/internal/application"
	"goflow/internal/crypto"
	"goflow/internal/engine"
	"goflow/internal/nodes"
	"goflow/internal/storage"

	"github.com/robfig/cron/v3"
)

type Options struct {
	Config   *config.Config
	UIFS     fs.FS
	Listener net.Listener
	Logger   *log.Logger
}

type App struct {
	URL string

	server        *http.Server
	listener      net.Listener
	db            *storage.DB
	cronScheduler *cron.Cron
	cronCancel    context.CancelFunc
	cleanupCancel context.CancelFunc
	goroutines    sync.WaitGroup
	done          chan error
	shutdownOnce  sync.Once
	logger        *log.Logger
}

func Run(ctx context.Context, opts Options) error {
	app, err := Start(ctx, opts)
	if err != nil {
		return err
	}
	return <-app.Done()
}

func Start(ctx context.Context, opts Options) (*App, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	cfg := opts.Config
	if cfg == nil {
		cfg = config.LoadConfig()
	}
	logger := opts.Logger
	if logger == nil {
		logger = log.Default()
	}
	cfg = effectiveConfig(cfg, opts.Listener)
	if cfg.IsPublicBind() && cfg.APIKey == "" {
		return nil, fmt.Errorf("refusing to bind %s:%s without GOFLOW_API_KEY; set GOFLOW_API_KEY or bind to 127.0.0.1", cfg.Host, cfg.Port)
	}

	listener := opts.Listener
	closeListenerOnError := false
	if listener == nil {
		var err error
		listener, err = net.Listen("tcp", fmt.Sprintf("%s:%s", cfg.Host, cfg.Port))
		if err != nil {
			return nil, fmt.Errorf("listen on %s:%s: %w", cfg.Host, cfg.Port, err)
		}
		closeListenerOnError = true
		cfg = effectiveConfig(cfg, listener)
	}
	defer func() {
		if closeListenerOnError {
			_ = listener.Close()
		}
	}()

	db, err := storage.NewDB(cfg.DBPath)
	if err != nil {
		return nil, fmt.Errorf("initialize SQLite database: %w", err)
	}
	closeDBOnError := true
	defer func() {
		if closeDBOnError {
			db.Close()
		}
	}()

	cm := crypto.NewCryptoManager(cfg.MasterKey)
	credStore := storage.NewCredentialStore(db, cm)
	wfStore := storage.NewWorkflowStore(db)
	execStore := storage.NewExecutionStore(db)
	tokenStore := storage.NewAccessTokenStore(db)
	auditStore := storage.NewAuditStore(db)
	if interrupted, err := execStore.MarkRunningInterrupted(); err != nil {
		logger.Printf("[WARN] Failed to mark interrupted executions: %v", err)
	} else if interrupted > 0 {
		logger.Printf("[INFO] Marked %d previously running executions as INTERRUPTED", interrupted)
	}
	if deleted, err := execStore.Cleanup(cfg.ExecutionRetentionDays, cfg.MaxExecutionsPerWorkflow); err != nil {
		logger.Printf("[WARN] Initial execution cleanup failed: %v", err)
	} else if deleted > 0 {
		logger.Printf("[INFO] Cleaned up %d old execution records", deleted)
	}

	registry := nodes.NewBuiltinRegistry()
	eventBus := engine.NewEventBus()
	eng := engine.NewEngine(registry, execStore, credStore, eventBus, wfStore, cfg.MaxConcurrentExecutions, cfg.MaxParallelNodesPerRun)
	triggerService := application.NewTriggerService(wfStore, eng)
	cronScheduler := cron.New()
	cronScheduler.Start()

	cronCtx, cronCancel := context.WithCancel(ctx)
	cleanupCtx, cleanupCancel := context.WithCancel(ctx)

	router := api.NewRouter(
		wfStore,
		execStore,
		credStore,
		tokenStore,
		auditStore,
		registry,
		eng,
		eventBus,
		opts.UIFS,
		cfg.APIKey,
		cfg.WebhookRateLimitPerMinute,
		mcpBaseURL(cfg),
		cfg.MCPAllowedOrigins,
		cfg.MCPMaxInflightPerClient,
		cfg.MCPRateLimitPerMinute,
	)

	server := &http.Server{
		Handler:      router,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 180 * time.Second,
	}
	app := &App{
		URL:           "http://" + listener.Addr().String(),
		server:        server,
		listener:      listener,
		db:            db,
		cronScheduler: cronScheduler,
		cronCancel:    cronCancel,
		cleanupCancel: cleanupCancel,
		done:          make(chan error, 1),
		logger:        logger,
	}

	startCleanupLoop(cleanupCtx, &app.goroutines, execStore, cfg, logger)
	startCronScanner(cronCtx, &app.goroutines, cronScheduler, wfStore, triggerService, logger)
	go app.serve(ctx)

	closeDBOnError = false
	closeListenerOnError = false
	logger.Printf("[INFO] Goflow Web Server running on %s", app.URL)
	return app, nil
}

func (app *App) Done() <-chan error {
	return app.done
}

func (app *App) Shutdown(ctx context.Context) error {
	var err error
	app.shutdownOnce.Do(func() {
		app.logger.Println("[INFO] Shutting down Goflow gracefully...")
		app.cronCancel()
		app.cleanupCancel()
		if ctx == nil {
			var cancel context.CancelFunc
			ctx, cancel = context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
		}
		err = app.server.Shutdown(ctx)
		if app.cronScheduler != nil {
			<-app.cronScheduler.Stop().Done()
		}
		app.goroutines.Wait()
		if app.db != nil {
			app.db.Close()
		}
	})
	return err
}

func (app *App) serve(ctx context.Context) {
	serveErr := make(chan error, 1)
	go func() {
		err := app.server.Serve(app.listener)
		if errors.Is(err, http.ErrServerClosed) {
			err = nil
		}
		serveErr <- err
	}()

	select {
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		err := app.Shutdown(shutdownCtx)
		cancel()
		if err != nil {
			app.done <- err
			return
		}
		if err := <-serveErr; err != nil {
			app.done <- err
			return
		}
		app.done <- nil
	case err := <-serveErr:
		if err != nil {
			_ = app.Shutdown(context.Background())
		}
		app.done <- err
	}
}

func startCleanupLoop(ctx context.Context, wg *sync.WaitGroup, execStore *storage.ExecutionStore, cfg *config.Config, logger *log.Logger) {
	wg.Add(1)
	go func() {
		defer wg.Done()
		ticker := time.NewTicker(6 * time.Hour)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				logger.Println("[Cleanup] Execution cleanup goroutine stopped gracefully")
				return
			case <-ticker.C:
				deleted, err := execStore.Cleanup(cfg.ExecutionRetentionDays, cfg.MaxExecutionsPerWorkflow)
				if err != nil {
					logger.Printf("[Cleanup] Failed to clean execution records: %v", err)
				} else if deleted > 0 {
					logger.Printf("[Cleanup] Removed %d old execution records", deleted)
				}
			}
		}
	}()
}

func startCronScanner(ctx context.Context, wg *sync.WaitGroup, cScheduler *cron.Cron, wfStore *storage.WorkflowStore, triggerService *application.TriggerService, logger *log.Logger) {
	type cronJob struct {
		entryID  cron.EntryID
		cronExpr string
	}
	scheduledJobs := make(map[string]cronJob)

	wg.Add(1)
	go func() {
		defer wg.Done()
		ticker := time.NewTicker(10 * time.Second)
		defer ticker.Stop()
		scan := func() {
			wfs, err := wfStore.ListAll()
			if err != nil {
				return
			}
			activeCronWfs := make(map[string]string)
			for _, wf := range wfs {
				if !wf.IsActive {
					continue
				}
				var nodeList []nodes.Node
				if err := json.Unmarshal([]byte(wf.NodesJSON), &nodeList); err != nil {
					continue
				}
				for _, node := range nodeList {
					if node.Type == nodes.TypeCronTrigger {
						if cronExpr, ok := node.Params["cron_expression"].(string); ok && cronExpr != "" {
							activeCronWfs[wf.ID] = cronExpr
							break
						}
					}
				}
			}
			for wfID, job := range scheduledJobs {
				currentCronExpr, active := activeCronWfs[wfID]
				if !active || currentCronExpr != job.cronExpr {
					cScheduler.Remove(job.entryID)
					delete(scheduledJobs, wfID)
					logger.Printf("[Cron] Removed scheduler for workflow %s", wfID)
				}
			}
			for wfID, cronExpr := range activeCronWfs {
				if _, scheduled := scheduledJobs[wfID]; scheduled {
					continue
				}
				wfIDCopy := wfID
				cronExprCopy := cronExpr
				entryID, err := cScheduler.AddFunc(cronExprCopy, func() {
					logger.Printf("[Cron] Triggering workflow %s...", wfIDCopy)
					latestWf, err := wfStore.GetByID(wfIDCopy)
					if err != nil {
						logger.Printf("[Cron] Error fetching latest workflow %s: %v", wfIDCopy, err)
						return
					}
					if !latestWf.IsActive {
						logger.Printf("[Cron] Workflow %s is no longer active, skipping execution", wfIDCopy)
						return
					}
					payload := map[string]interface{}{
						"triggered_at": time.Now().Format(time.RFC3339),
						"schedule":     cronExprCopy,
					}
					_, err = triggerService.Trigger(context.Background(), application.TriggerRequest{
						WorkflowID: latestWf.ID,
						Input:      payload,
						Mode:       application.ModeSync,
						Source:     application.SourceCron,
						Principal:  "cron",
					})
					if err != nil {
						logger.Printf("[Cron] Error executing workflow %s: %v", wfIDCopy, err)
					}
				})
				if err == nil {
					scheduledJobs[wfID] = cronJob{entryID: entryID, cronExpr: cronExpr}
					logger.Printf("[Cron] Scheduled workflow %s with pattern %s", wfID, cronExpr)
				} else {
					logger.Printf("[Cron] Failed to schedule workflow %s with pattern %s: %v", wfID, cronExpr, err)
				}
			}
		}

		scan()
		for {
			select {
			case <-ctx.Done():
				logger.Println("[Cron] Scanner goroutine stopped gracefully")
				return
			case <-ticker.C:
				scan()
			}
		}
	}()
}

func effectiveConfig(cfg *config.Config, listener net.Listener) *config.Config {
	copy := *cfg
	if tcpAddr, ok := listenerAddr(listener); ok {
		copy.Host = tcpAddr.IP.String()
		copy.Port = strconv.Itoa(tcpAddr.Port)
	}
	return &copy
}

func listenerAddr(listener net.Listener) (*net.TCPAddr, bool) {
	if listener == nil {
		return nil, false
	}
	tcpAddr, ok := listener.Addr().(*net.TCPAddr)
	return tcpAddr, ok
}

func mcpBaseURL(cfg *config.Config) string {
	if cfg.MCPBaseURL != "" {
		return cfg.MCPBaseURL
	}
	host := cfg.Host
	if host == "" || host == "0.0.0.0" || host == "::" || host == "*" {
		host = "127.0.0.1"
	}
	return fmt.Sprintf("http://%s:%s", host, cfg.Port)
}
