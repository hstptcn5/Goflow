package application

import (
	"context"
	"fmt"
	"strings"

	"goflow/internal/engine"
	"goflow/internal/storage"
)

type TriggerSource string

const (
	SourceUI      TriggerSource = "ui"
	SourceAPI     TriggerSource = "api"
	SourceWebhook TriggerSource = "webhook"
	SourceCron    TriggerSource = "cron"
	SourceCLI     TriggerSource = "cli"
	SourceMCP     TriggerSource = "mcp"
)

type TriggerMode string

const (
	ModeSync  TriggerMode = "sync"
	ModeAsync TriggerMode = "async"
)

type TriggerRequest struct {
	WorkflowID     string
	Input          interface{}
	Mode           TriggerMode
	IdempotencyKey string
	Source         TriggerSource
	Principal      string
	RequestID      string
}

type TriggerResult struct {
	Execution    *storage.Execution
	Deduplicated bool
}

type TriggerService struct {
	wfStore *storage.WorkflowStore
	engine  *engine.Engine
}

func NewTriggerService(wfStore *storage.WorkflowStore, eng *engine.Engine) *TriggerService {
	return &TriggerService{wfStore: wfStore, engine: eng}
}

func (s *TriggerService) Trigger(ctx context.Context, req TriggerRequest) (*TriggerResult, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	req.normalize()
	if req.WorkflowID == "" {
		return nil, fmt.Errorf("workflow_id is required")
	}

	wf, err := s.wfStore.GetByID(req.WorkflowID)
	if err != nil {
		return nil, err
	}

	opts := engine.TriggerOptions{
		Source:         string(req.Source),
		Principal:      req.Principal,
		RequestID:      req.RequestID,
		IdempotencyKey: req.IdempotencyKey,
	}

	if req.Mode == ModeAsync {
		exec, deduplicated, err := s.engine.StartWorkflowAsync(wf, req.Input, opts)
		if err != nil {
			return nil, err
		}
		return &TriggerResult{Execution: exec, Deduplicated: deduplicated}, nil
	}

	exec, err := s.engine.ExecuteWorkflowWithOptions(wf, req.Input, opts)
	if err != nil {
		return nil, err
	}
	return &TriggerResult{Execution: exec, Deduplicated: false}, nil
}

func (r *TriggerRequest) normalize() {
	if r.Mode == "" {
		r.Mode = ModeSync
	}
	if r.Source == "" {
		r.Source = SourceAPI
	}
	r.WorkflowID = strings.TrimSpace(r.WorkflowID)
	r.IdempotencyKey = strings.TrimSpace(r.IdempotencyKey)
	r.Principal = strings.TrimSpace(r.Principal)
	r.RequestID = strings.TrimSpace(r.RequestID)
}
