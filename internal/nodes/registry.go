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
