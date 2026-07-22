package main

import (
	"fmt"
	"log"

	"goflow/internal/crypto"
	"goflow/internal/engine"
	"goflow/internal/nodes"
	"goflow/internal/storage"
)

func main() {
	// Initialize DB
	db, err := storage.NewDB("d:\\build2026\\Goflow\\goflow.db")
	if err != nil {
		log.Fatalf("failed to init db: %v", err)
	}
	defer db.Close()

	wfStore := storage.NewWorkflowStore(db)
	execStore := storage.NewExecutionStore(db)
	cm := crypto.NewCryptoManager("goflow-master-secret-key-32bytes!")
	credStore := storage.NewCredentialStore(db, cm)

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

	eventBus := engine.NewEventBus()
	eng := engine.NewEngine(registry, execStore, credStore, eventBus)

	// Fetch all workflows to find our workflow
	wfs, err := wfStore.ListAll()
	if err != nil {
		log.Fatalf("list workflows failed: %v", err)
	}

	if len(wfs) == 0 {
		log.Fatalf("no workflows found in db")
	}

	wf := &wfs[0]
	fmt.Printf("Executing Workflow: %s (ID: %s)\n", wf.Name, wf.ID)

	// Execute
	exec, err := eng.ExecuteWorkflow(wf, nil)
	if err != nil {
		log.Fatalf("execution failed: %v", err)
	}

	fmt.Printf("Execution Status: %s\n", exec.Status)
	fmt.Printf("Execution Duration: %d ms\n", exec.DurationMs)
	fmt.Printf("Execution Logs length: %d\n", len(exec.LogsJSON))
	fmt.Printf("Execution Logs: %s\n", exec.LogsJSON)
}
