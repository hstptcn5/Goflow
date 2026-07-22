package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"

	"goflow/internal/crypto"
	"goflow/internal/engine"
	"goflow/internal/nodes"
	"goflow/internal/storage"
)

type WorkflowJSON struct {
	Name        string       `json:"name"`
	Description string       `json:"description"`
	Nodes       []nodes.Node `json:"nodes"`
	Edges       []nodes.Edge `json:"edges"`
}

func main() {
	// 1. Read the updated daily.json from disk
	content, err := ioutil.ReadFile("d:\\build2026\\Goflow\\daily.json")
	if err != nil {
		log.Fatalf("failed to read daily.json: %v", err)
	}

	var wfJSON WorkflowJSON
	if err := json.Unmarshal(content, &wfJSON); err != nil {
		log.Fatalf("failed to parse daily.json: %v", err)
	}

	nodesBytes, _ := json.Marshal(wfJSON.Nodes)
	edgesBytes, _ := json.Marshal(wfJSON.Edges)

	// 2. Initialize DB and stores
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

	// Find the workflow or update the first one
	wfs, err := wfStore.ListAll()
	if err != nil {
		log.Fatalf("list workflows failed: %v", err)
	}

	var wf *storage.Workflow
	if len(wfs) == 0 {
		// Create new
		wf = &storage.Workflow{
			Name:        wfJSON.Name,
			Description: wfJSON.Description,
			IsActive:    true,
			NodesJSON:   string(nodesBytes),
			EdgesJSON:   string(edgesBytes),
		}
		if err := wfStore.Create(wf); err != nil {
			log.Fatalf("create workflow failed: %v", err)
		}
	} else {
		wf = &wfs[0]
		wf.Name = wfJSON.Name
		wf.Description = wfJSON.Description
		wf.NodesJSON = string(nodesBytes)
		wf.EdgesJSON = string(edgesBytes)
		if err := wfStore.Update(wf); err != nil {
			log.Fatalf("update workflow failed: %v", err)
		}
	}

	fmt.Printf("Running Workflow: %s (ID: %s)\n", wf.Name, wf.ID)

	// Execute
	exec, err := eng.ExecuteWorkflow(wf, nil)
	if err != nil {
		log.Fatalf("execution failed: %v", err)
	}

	fmt.Printf("Execution Status: %s\n", exec.Status)
	fmt.Printf("Execution Duration: %d ms\n", exec.DurationMs)
	fmt.Printf("Execution Logs: %s\n", exec.LogsJSON)
}
