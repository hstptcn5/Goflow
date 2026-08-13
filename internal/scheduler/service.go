package scheduler

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"sync"
	"time"

	"goflow/internal/application"
	"goflow/internal/engine"
	"goflow/internal/storage"
)

const (
	DefaultTickInterval = 30 * time.Second
	DefaultDueGrace     = time.Minute

	CategoryScheduleInvalid  = storage.ScheduleErrorInvalid
	CategoryMissedSkipped    = storage.ScheduleErrorMissedSkipped
	CategorySetupIncomplete  = storage.ScheduleErrorSetup
	CategoryRevalidation     = storage.ScheduleErrorRevalidation
	CategoryWorkflowInactive = storage.ScheduleErrorInactive
	CategoryAlreadyRunning   = storage.ScheduleErrorAlreadyRunning
	CategoryInternal         = storage.ScheduleErrorInternal
)

type Clock interface {
	Now() time.Time
}

type SystemClock struct{}

func (SystemClock) Now() time.Time { return time.Now() }

type ScheduleStore interface {
	GetByWorkflow(workflowID string) (*storage.WorkflowSchedule, error)
	Advance(workflowID, packID string, advance storage.ScheduleAdvance) (bool, error)
}

type Triggerer interface {
	Trigger(ctx context.Context, req application.TriggerRequest) (*application.TriggerResult, error)
}

type ReadinessFunc func() (bool, string)

type Logger interface {
	Printf(format string, values ...interface{})
}

type Service struct {
	store      ScheduleStore
	triggerer  Triggerer
	clock      Clock
	readiness  ReadinessFunc
	logger     Logger
	packID     string
	workflowID string
	dueGrace   time.Duration
	mu         sync.Mutex
}

type Options struct {
	Store      ScheduleStore
	Triggerer  Triggerer
	Clock      Clock
	Readiness  ReadinessFunc
	Logger     Logger
	PackID     string
	WorkflowID string
	DueGrace   time.Duration
}

type TickResult struct {
	State        string
	Category     string
	ExecutionID  string
	ScheduledFor *time.Time
	NextRunAt    *time.Time
	Triggered    bool
	Deduplicated bool
}

func NewService(options Options) (*Service, error) {
	if options.Store == nil || options.Triggerer == nil {
		return nil, fmt.Errorf("scheduler store and triggerer are required")
	}
	if options.PackID == "" || options.WorkflowID == "" {
		return nil, fmt.Errorf("scheduler pack_id and workflow_id are required")
	}
	if options.Clock == nil {
		options.Clock = SystemClock{}
	}
	if options.Readiness == nil {
		options.Readiness = func() (bool, string) { return true, "" }
	}
	if options.DueGrace <= 0 {
		options.DueGrace = DefaultDueGrace
	}
	return &Service{
		store: options.Store, triggerer: options.Triggerer, clock: options.Clock,
		readiness: options.Readiness, logger: options.Logger, packID: options.PackID,
		workflowID: options.WorkflowID, dueGrace: options.DueGrace,
	}, nil
}

func (s *Service) Tick(ctx context.Context) (TickResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := ctx.Err(); err != nil {
		return TickResult{}, err
	}
	schedule, err := s.store.GetByWorkflow(s.workflowID)
	if errors.Is(err, storage.ErrWorkflowScheduleNotFound) {
		return TickResult{State: storage.ScheduleStateDisabled}, nil
	}
	if errors.Is(err, storage.ErrInvalidWorkflowSchedule) {
		return TickResult{State: storage.ScheduleStateNeedsAttention, Category: CategoryScheduleInvalid}, nil
	}
	if err != nil {
		return TickResult{}, fmt.Errorf("scheduler load: %w", err)
	}
	if schedule.PackID != s.packID {
		return TickResult{State: storage.ScheduleStateNeedsAttention, Category: CategoryScheduleInvalid}, nil
	}
	if !schedule.Enabled {
		return resultForSchedule(schedule), nil
	}
	now := s.clock.Now().UTC()
	if schedule.NextRunAt == nil {
		next, err := NextDailyAfter(schedule.LocalTime, schedule.Timezone, now)
		if err != nil {
			return TickResult{State: storage.ScheduleStateNeedsAttention, Category: CategoryScheduleInvalid}, nil
		}
		return s.advance(schedule, nil, next, "", storage.ScheduleStateOK, "", false, false)
	}
	due := schedule.NextRunAt.UTC()
	if due.After(now) {
		return resultForSchedule(schedule), nil
	}
	if schedule.LastScheduledFor != nil && !due.After(schedule.LastScheduledFor.UTC()) {
		next, err := NextDailyAfter(schedule.LocalTime, schedule.Timezone, now)
		if err != nil {
			return TickResult{}, err
		}
		return s.advance(schedule, nil, next, "", storage.ScheduleStateOK, "", false, false)
	}
	next, err := NextDailyAfter(schedule.LocalTime, schedule.Timezone, now)
	if err != nil {
		return TickResult{}, err
	}
	if now.Sub(due) > s.dueGrace {
		return s.advance(schedule, &due, next, "", storage.ScheduleStateNeedsAttention, CategoryMissedSkipped, false, false)
	}
	if ready, category := s.readiness(); !ready {
		category = safeReadinessCategory(category)
		return s.advance(schedule, &due, next, "", storage.ScheduleStateNeedsAttention, category, false, false)
	}
	key, requestID := scheduledIdentifiers(s.packID, s.workflowID, due)
	triggerResult, err := s.triggerer.Trigger(ctx, application.TriggerRequest{
		WorkflowID: s.workflowID,
		Input: map[string]interface{}{
			"triggered_at":  now.Format(time.RFC3339),
			"scheduled_for": due.Format(time.RFC3339),
			"schedule_kind": storage.ScheduleKindDaily,
			"timezone":      schedule.Timezone,
		},
		Mode:           application.ModeAsync,
		IdempotencyKey: key,
		Source:         application.SourceSchedule,
		Principal:      "appliance-scheduler",
		RequestID:      requestID,
	})
	if errors.Is(err, engine.ErrWorkflowConcurrencyLimit) || errors.Is(err, engine.ErrConcurrencyLimit) {
		return s.advance(schedule, &due, next, "", storage.ScheduleStateNeedsAttention, CategoryAlreadyRunning, false, false)
	}
	if errors.Is(err, application.ErrWorkflowInactive) {
		return s.advance(schedule, &due, next, "", storage.ScheduleStateNeedsAttention, CategoryWorkflowInactive, false, false)
	}
	if err != nil {
		return TickResult{}, fmt.Errorf("scheduler trigger: %w", err)
	}
	if triggerResult == nil || triggerResult.Execution == nil {
		return TickResult{}, fmt.Errorf("scheduler trigger returned no execution")
	}
	return s.advance(schedule, &due, next, triggerResult.Execution.ID, storage.ScheduleStateOK, "", true, triggerResult.Deduplicated)
}

func (s *Service) Run(ctx context.Context, interval time.Duration) {
	if interval <= 0 {
		interval = DefaultTickInterval
	}
	s.tickAndLog(ctx)
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.tickAndLog(ctx)
		}
	}
}

func (s *Service) tickAndLog(ctx context.Context) {
	result, err := s.Tick(ctx)
	if err != nil && !errors.Is(err, context.Canceled) && s.logger != nil {
		s.logger.Printf("[Scheduler] Tick failed for workflow %s: %v", s.workflowID, err)
		return
	}
	if result.Triggered && s.logger != nil {
		s.logger.Printf("[Scheduler] Admitted scheduled execution %s for workflow %s", result.ExecutionID, s.workflowID)
	}
}

func (s *Service) advance(schedule *storage.WorkflowSchedule, scheduledFor *time.Time, next time.Time, executionID, state, category string, triggered, deduplicated bool) (TickResult, error) {
	advanced, err := s.store.Advance(schedule.WorkflowID, schedule.PackID, storage.ScheduleAdvance{
		ExpectedRevision: schedule.Revision,
		ScheduledFor:     scheduledFor,
		NextRunAt:        next,
		ExecutionID:      executionID,
		State:            state,
		ErrorCategory:    category,
		UpdatedAt:        s.clock.Now().UTC(),
	})
	if err != nil {
		return TickResult{}, err
	}
	if !advanced {
		return TickResult{State: state, Category: category, NextRunAt: &next}, nil
	}
	return TickResult{
		State: state, Category: category, ExecutionID: executionID,
		ScheduledFor: scheduledFor, NextRunAt: &next, Triggered: triggered,
		Deduplicated: deduplicated,
	}, nil
}

func resultForSchedule(schedule *storage.WorkflowSchedule) TickResult {
	return TickResult{
		State: schedule.State, Category: schedule.ErrorCategory,
		ExecutionID: schedule.LastExecutionID, ScheduledFor: schedule.LastScheduledFor,
		NextRunAt: schedule.NextRunAt,
	}
}

func scheduledIdentifiers(packID, workflowID string, scheduledFor time.Time) (string, string) {
	payload := "goflow-schedule-v1\x00" + packID + "\x00" + workflowID + "\x00" + scheduledFor.UTC().Format(time.RFC3339)
	digest := sha256.Sum256([]byte(payload))
	hexDigest := hex.EncodeToString(digest[:])
	return "schedule:v1:" + hexDigest, "schedule-" + hexDigest[:24]
}

func safeReadinessCategory(category string) string {
	switch category {
	case CategorySetupIncomplete, CategoryRevalidation:
		return category
	default:
		return CategorySetupIncomplete
	}
}
