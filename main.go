package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"goflow/config"
	"goflow/internal/api"
	"goflow/internal/crypto"
	"goflow/internal/engine"
	"goflow/internal/nodes"
	"goflow/internal/storage"

	"github.com/robfig/cron/v3"
)

func main() {
	log.Println("==================================================")
	log.Println("[INFO] Starting Goflow Workflow Automation Engine...")
	log.Println("==================================================")

	cfg := config.LoadConfig()

	// 1. Initialize Storage Layer & SQLite Connection Pool
	db, err := storage.NewDB(cfg.DBPath)
	if err != nil {
		log.Fatalf("[ERROR] Failed to initialize SQLite database: %v", err)
	}
	defer db.Close()
	log.Printf("[INFO] SQLite initialized at %s (WAL mode enabled)", cfg.DBPath)

	// 2. Initialize Crypto Manager for Credentials Encryption
	cm := crypto.NewCryptoManager(cfg.MasterKey)
	credStore := storage.NewCredentialStore(db, cm)
	wfStore := storage.NewWorkflowStore(db)
	execStore := storage.NewExecutionStore(db)

	// 3. Initialize Plugin Registry and Register All Built-in Node Executors
	registry := nodes.NewPluginRegistry()
	_ = registry.Register(nodes.NewWebhookTriggerExecutor())
	_ = registry.Register(nodes.NewCronTriggerExecutor())
	_ = registry.Register(nodes.NewHTTPRequestExecutor())
	_ = registry.Register(nodes.NewTelegramBotExecutor())
	_ = registry.Register(nodes.NewJSONTransformExecutor())
	_ = registry.Register(nodes.NewConditionIFExecutor())
	_ = registry.Register(nodes.NewEmailSMTPExecutor())
	_ = registry.Register(nodes.NewDelaySleepExecutor())
	_ = registry.Register(nodes.NewOpenAIGPTExecutor())
	_ = registry.Register(nodes.NewDeepSeekAIExecutor())
	_ = registry.Register(nodes.NewDiscordBotExecutor())
	_ = registry.Register(nodes.NewSlackBotExecutor())
	_ = registry.Register(nodes.NewJSCodeRunnerExecutor())
	log.Printf("[INFO] Plugin Registry initialized with %d built-in nodes", len(registry.ListDefinitions()))

	// 4. Initialize EventBus and DAG Execution Engine
	eventBus := engine.NewEventBus()
	eng := engine.NewEngine(registry, execStore, credStore, eventBus)

	// 5. Initialize Cron Scheduler for Timed Triggers
	cScheduler := cron.New()
	cScheduler.Start()
	defer cScheduler.Stop()

	// Track scheduled workflows and their entries
	type cronJob struct {
		entryID  cron.EntryID
		cronExpr string
	}
	scheduledJobs := make(map[string]cronJob)

	// Background task to scan active workflows and register cron schedules
	go func() {
		for {
			wfs, err := wfStore.ListAll()
			if err == nil {
				activeCronWfs := make(map[string]string) // workflowID -> cronExpr

				for _, wf := range wfs {
					if !wf.IsActive {
						continue
					}

					var nodeList []nodes.Node
					if err := json.Unmarshal([]byte(wf.NodesJSON), &nodeList); err != nil {
						continue
					}

					// Find if this workflow has a Cron Trigger node
					for _, node := range nodeList {
						if node.Type == nodes.TypeCronTrigger {
							if cronExpr, ok := node.Params["cron_expression"].(string); ok && cronExpr != "" {
								activeCronWfs[wf.ID] = cronExpr
								break
							}
						}
					}
				}

				// 1. Remove jobs that are no longer active, or have a different cron expression
				for wfID, job := range scheduledJobs {
					currentCronExpr, active := activeCronWfs[wfID]
					if !active || currentCronExpr != job.cronExpr {
						cScheduler.Remove(job.entryID)
						delete(scheduledJobs, wfID)
						log.Printf("[Cron] Removed scheduler for workflow %s", wfID)
					}
				}

				// 2. Add or update active cron workflows
				for wfID, cronExpr := range activeCronWfs {
					if _, scheduled := scheduledJobs[wfID]; !scheduled {
						wfIDCopy := wfID
						cronExprCopy := cronExpr
						entryID, err := cScheduler.AddFunc(cronExprCopy, func() {
							log.Printf("[Cron] Triggering workflow %s...", wfIDCopy)
							latestWf, err := wfStore.GetByID(wfIDCopy)
							if err != nil {
								log.Printf("[Cron] Error fetching latest workflow %s: %v", wfIDCopy, err)
								return
							}
							if !latestWf.IsActive {
								log.Printf("[Cron] Workflow %s is no longer active, skipping execution", wfIDCopy)
								return
							}
							payload := map[string]interface{}{
								"triggered_at": time.Now().Format(time.RFC3339),
								"schedule":     cronExprCopy,
							}
							_, err = eng.ExecuteWorkflow(latestWf, payload)
							if err != nil {
								log.Printf("[Cron] Error executing workflow %s: %v", wfIDCopy, err)
							}
						})
						if err == nil {
							scheduledJobs[wfID] = cronJob{
								entryID:  entryID,
								cronExpr: cronExpr,
							}
							log.Printf("[Cron] Scheduled workflow %s with pattern %s", wfID, cronExpr)
						} else {
							log.Printf("[Cron] Failed to schedule workflow %s with pattern %s: %v", wfID, cronExpr, err)
						}
					}
				}
			}
			time.Sleep(10 * time.Second)
		}
	}()

	// 6. Initialize REST API Router & Serve Static Embedded Web UI
	uiFS := getEmbeddedUI()
	router := api.NewRouter(wfStore, execStore, credStore, registry, eng, eventBus, uiFS)

	server := &http.Server{
		Addr:         fmt.Sprintf(":%s", cfg.Port),
		Handler:      router,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
	}

	// 7. Graceful Shutdown Handler
	go func() {
		log.Printf("[INFO] Goflow Web Server running on http://localhost:%s", cfg.Port)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("[ERROR] Server error: %v", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("[INFO] Shutting down Goflow gracefully...")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		log.Printf("[WARN] Server forced shutdown: %v", err)
	}
	log.Println("[INFO] Goflow stopped successfully.")
}
