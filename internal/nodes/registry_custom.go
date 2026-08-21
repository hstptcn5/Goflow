package nodes

import (
	"fmt"
	"strings"
)

// RegisterOrReplaceCustom registers a discovered/promoted custom node and allows
// replacing only user.* or custom.* definitions. Built-in node types remain immutable.
func (r *PluginRegistry) RegisterOrReplaceCustom(executor NodeExecutor) error {
	if executor == nil {
		return fmt.Errorf("node executor is nil")
	}
	nodeType := executor.GetDefinition().Type
	value := string(nodeType)
	if !strings.HasPrefix(value, "user.") && !strings.HasPrefix(value, "custom.") {
		return fmt.Errorf("only user.* or custom.* node types can be replaced")
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.executors[nodeType] = &validatingExecutor{NodeExecutor: executor}
	return nil
}
