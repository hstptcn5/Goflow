package main

import (
	"context"
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
	log.Println("🚀 Starting Goflow Workflow Automation Engine...")
	log.Println("==================================================")

	cfg := config.LoadConfig()

	// 1. Khởi tạo Storage Layer & SQLite Pool
	db, err := storage.NewDB(cfg.DBPath)
	if err != nil {
		log.Fatalf("❌ Failed to initialize SQLite database: %v", err)
	}
	defer db.Close()
	log.Printf("📦 SQLite initialized at %s (WAL mode enabled)", cfg.DBPath)

	// 2. Khởi tạo Crypto Manager cho Credentials
	cm := crypto.NewCryptoManager(cfg.MasterKey)
	credStore := storage.NewCredentialStore(db, cm)
	wfStore := storage.NewWorkflowStore(db)
	execStore := storage.NewExecutionStore(db)

	// 3. Khởi tạo Plugin Registry và Đăng ký các Built-in Node Executors
	registry := nodes.NewPluginRegistry()
	_ = registry.Register(nodes.NewWebhookTriggerExecutor())
	_ = registry.Register(nodes.NewCronTriggerExecutor())
	_ = registry.Register(nodes.NewHTTPRequestExecutor())
	_ = registry.Register(nodes.NewTelegramBotExecutor())
	_ = registry.Register(nodes.NewJSONTransformExecutor())
	_ = registry.Register(nodes.NewConditionIFExecutor())
	_ = registry.Register(nodes.NewEmailSMTPExecutor())
	_ = registry.Register(nodes.NewDelaySleepExecutor())
	log.Printf("🧩 Plugin Registry initialized with %d built-in nodes", len(registry.ListDefinitions()))

	// 4. Khởi tạo EventBus và DAG Engine
	eventBus := engine.NewEventBus()
	eng := engine.NewEngine(registry, execStore, credStore, eventBus)

	// 5. Khởi tạo Cron Scheduler cho Cron Triggers
	cScheduler := cron.New()
	cScheduler.Start()
	defer cScheduler.Stop()

	// Task tự động quét active workflows và đăng ký cron schedule
	go func() {
		for {
			wfs, err := wfStore.ListAll()
			if err == nil {
				for _, wf := range wfs {
					if wf.IsActive {
						// Logic kích hoạt cron scheduler nếu có Cron Node
					}
				}
			}
			time.Sleep(1 * time.Minute)
		}
	}()

	// 6. Khởi tạo REST API Router & Serve Static Embedded UI
	uiFS := getEmbeddedUI()
	router := api.NewRouter(wfStore, execStore, credStore, registry, eng, eventBus, uiFS)

	server := &http.Server{
		Addr:         fmt.Sprintf(":%s", cfg.Port),
		Handler:      router,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
	}

	// 7. Graceful Shutdown
	go func() {
		log.Printf("🌐 Goflow Web Server running on http://localhost:%s", cfg.Port)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("❌ Server error: %v", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("⚡ Shutting down Goflow gracefully...")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		log.Printf("⚠️ Server forced shutdown: %v", err)
	}
	log.Println("👋 Goflow stopped successfully.")
}
