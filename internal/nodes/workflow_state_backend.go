package nodes

import (
	"fmt"
	"sync"

	"goflow/config"
	"goflow/internal/storage"
)

var (
	workflowStateOnce  sync.Once
	workflowStateStore *storage.StateStore
	workflowStateDB    *storage.DB
	workflowStateErr   error
)

// defaultWorkflowStateStore opens the same SQLite database configured for
// Goflow and reuses one process-lifetime store. ExecutionContext callbacks,
// when supplied, always take precedence so tests and embedded runtimes can
// inject an explicit backend without global state.
func defaultWorkflowStateStore() (*storage.StateStore, error) {
	workflowStateOnce.Do(func() {
		cfg := config.LoadConfig()
		workflowStateDB, workflowStateErr = storage.NewDB(cfg.DBPath)
		if workflowStateErr != nil {
			return
		}
		workflowStateStore = storage.NewStateStore(workflowStateDB)
	})
	if workflowStateErr != nil {
		return nil, fmt.Errorf("open persistent workflow state: %w", workflowStateErr)
	}
	if workflowStateStore == nil {
		return nil, fmt.Errorf("persistent workflow state is unavailable")
	}
	return workflowStateStore, nil
}

func workflowStateGet(ctx *ExecutionContext, scope, key string) (interface{}, bool, error) {
	if ctx.StateGet != nil {
		return ctx.StateGet(scope, key)
	}
	store, err := defaultWorkflowStateStore()
	if err != nil {
		return nil, false, err
	}
	return store.Get(scope, ctx.WorkflowID, key)
}

func workflowStateSet(ctx *ExecutionContext, scope, key string, value interface{}) error {
	if ctx.StateSet != nil {
		return ctx.StateSet(scope, key, value)
	}
	store, err := defaultWorkflowStateStore()
	if err != nil {
		return err
	}
	return store.Set(scope, ctx.WorkflowID, key, value)
}

func workflowStateDelete(ctx *ExecutionContext, scope, key string) (bool, error) {
	if ctx.StateDelete != nil {
		return ctx.StateDelete(scope, key)
	}
	store, err := defaultWorkflowStateStore()
	if err != nil {
		return false, err
	}
	return store.Delete(scope, ctx.WorkflowID, key)
}

func workflowStateIncrement(ctx *ExecutionContext, scope, key string, delta float64) (float64, error) {
	if ctx.StateIncrement != nil {
		return ctx.StateIncrement(scope, key, delta)
	}
	store, err := defaultWorkflowStateStore()
	if err != nil {
		return 0, err
	}
	return store.Increment(scope, ctx.WorkflowID, key, delta)
}
