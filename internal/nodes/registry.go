package nodes

import (
	"fmt"
	"sync"
)

type PluginRegistry struct {
	executors map[NodeType]NodeExecutor
	mu        sync.RWMutex
}

func NewPluginRegistry() *PluginRegistry {
	return &PluginRegistry{
		executors: make(map[NodeType]NodeExecutor),
	}
}

func NewBuiltinRegistry() *PluginRegistry {
	registry := NewPluginRegistry()
	_ = registry.Register(NewWebhookTriggerExecutor())
	_ = registry.Register(NewCronTriggerExecutor())
	_ = registry.Register(NewHTTPRequestExecutor())
	_ = registry.Register(NewTelegramBotExecutor())
	_ = registry.Register(NewJSONTransformExecutor())
	_ = registry.Register(NewConditionIFExecutor())
	_ = registry.Register(NewEmailSMTPExecutor())
	_ = registry.Register(NewDelaySleepExecutor())
	_ = registry.Register(NewOpenAIGPTExecutor())
	_ = registry.Register(NewDeepSeekAIExecutor())
	_ = registry.Register(NewDiscordBotExecutor())
	_ = registry.Register(NewSlackBotExecutor())
	_ = registry.Register(NewJSCodeRunnerExecutor())
	_ = registry.Register(NewSubWorkflowExecutor())
	_ = registry.Register(NewPostgresQueryExecutor())
	_ = registry.Register(NewRedisCommandExecutor())
	_ = registry.Register(NewGoogleSheetsExecutor())
	_ = registry.Register(NewMySQLQueryExecutor())
	_ = registry.Register(NewMongoDBCommandExecutor())
	_ = registry.Register(NewGoogleDriveExecutor())
	_ = registry.Register(NewGmailRESTExecutor())
	_ = registry.Register(NewNotionPageExecutor())
	_ = registry.Register(NewSSHRunnerExecutor())
	_ = registry.Register(NewGitCommandExecutor())
	_ = registry.Register(NewGithubWebhookExecutor())
	_ = registry.Register(NewGoflowPluginExecutor())
	return registry
}

func (r *PluginRegistry) Register(executor NodeExecutor) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	nodeType := executor.GetDefinition().Type
	if _, exists := r.executors[nodeType]; exists {
		return fmt.Errorf("node type '%s' already registered", nodeType)
	}

	r.executors[nodeType] = executor
	return nil
}

func (r *PluginRegistry) Get(nodeType NodeType) (NodeExecutor, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	executor, exists := r.executors[nodeType]
	return executor, exists
}

func (r *PluginRegistry) ListDefinitions() []NodeDefinition {
	r.mu.RLock()
	defer r.mu.RUnlock()

	defs := make([]NodeDefinition, 0, len(r.executors))
	for _, exec := range r.executors {
		defs = append(defs, exec.GetDefinition())
	}
	return defs
}
