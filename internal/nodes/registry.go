package nodes

import (
	"fmt"
	"sync"
)

type PluginRegistry struct {
	executors map[NodeType]NodeExecutor
	mu        sync.RWMutex
}

type validatingExecutor struct{ NodeExecutor }

func (e *validatingExecutor) Execute(ctx *ExecutionContext, node *Node) (interface{}, error) {
	if err := e.NodeExecutor.Validate(node); err != nil {
		return nil, fmt.Errorf("invalid %s node configuration: %w", e.NodeExecutor.GetDefinition().Type, err)
	}
	return e.NodeExecutor.Execute(ctx, node)
}

type credentialDefinitionOverrideExecutor struct {
	NodeExecutor
	paramName string
	kinds     []string
	providers []string
}

func (e *credentialDefinitionOverrideExecutor) GetDefinition() NodeDefinition {
	def := e.NodeExecutor.GetDefinition()
	for i := range def.Params {
		if def.Params[i].Name != e.paramName {
			continue
		}
		def.Params[i].CredentialKinds = append([]string(nil), e.kinds...)
		def.Params[i].CredentialProviders = append([]string(nil), e.providers...)
		break
	}
	return def
}

func withCredentialCompatibility(executor NodeExecutor, paramName string, kinds, providers []string) NodeExecutor {
	return &credentialDefinitionOverrideExecutor{
		NodeExecutor: executor,
		paramName:    paramName,
		kinds:        append([]string(nil), kinds...),
		providers:    append([]string(nil), providers...),
	}
}

func NewPluginRegistry() *PluginRegistry {
	return &PluginRegistry{executors: make(map[NodeType]NodeExecutor)}
}

func NewBuiltinRegistry() *PluginRegistry {
	return NewBuiltinRegistryWithTelegramExecutor(NewTelegramBotExecutor())
}

func NewBuiltinRegistryWithTelegramExecutor(telegramExecutor NodeExecutor) *PluginRegistry {
	if telegramExecutor == nil {
		telegramExecutor = NewTelegramBotExecutor()
	}
	registry := NewPluginRegistry()
	_ = registry.Register(NewWebhookTriggerExecutor())
	_ = registry.Register(NewCronTriggerExecutor())
	_ = registry.Register(NewManualTriggerExecutor())
	_ = registry.Register(NewHTTPRequestExecutor())
	_ = registry.Register(NewNormalizedHTTPSourceExecutor())
	_ = registry.Register(NewRSSFeedSourceExecutor())
	_ = registry.Register(NewSourcePolicyExecutor())
	_ = registry.Register(telegramExecutor)
	_ = registry.Register(NewZaloOAExecutor())
	_ = registry.Register(NewJSONTransformExecutor())
	_ = registry.Register(NewConditionIFExecutor())
	_ = registry.Register(NewSwitchExecutor())
	_ = registry.Register(NewWorkflowStateExecutor())
	_ = registry.Register(NewEmailSMTPExecutor())
	_ = registry.Register(NewDelaySleepExecutor())
	_ = registry.Register(NewOpenAIGPTExecutor())
	_ = registry.Register(NewDeepSeekAIExecutor())
	_ = registry.Register(NewProviderAIExtractExecutor())
	_ = registry.Register(NewDiscordBotExecutor())
	_ = registry.Register(NewSlackBotExecutor())
	_ = registry.Register(NewJSCodeRunnerExecutor())
	_ = registry.Register(NewPythonCodeExecutor())
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
	if executor == nil {
		return fmt.Errorf("node executor is nil")
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	nodeType := executor.GetDefinition().Type
	if nodeType == "" {
		return fmt.Errorf("node executor type is empty")
	}
	if _, exists := r.executors[nodeType]; exists {
		return fmt.Errorf("node type '%s' already registered", nodeType)
	}
	r.executors[nodeType] = &validatingExecutor{NodeExecutor: executor}
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
		defs = append(defs, DefinitionWithErrorPolicy(exec.GetDefinition()))
	}
	return defs
}
