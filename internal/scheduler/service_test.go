package scheduler

import (
	"context"
	"errors"
	"io"
	"log"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"goflow/internal/application"
	"goflow/internal/crypto"
	"goflow/internal/engine"
	"goflow/internal/nodes"
	"goflow/internal/storage"
)

func TestServiceScheduledTickCreatesOneExecutionWithStableMetadata(t *testing.T) {
	fixture := newServiceFixture(t)
	service := fixture.service(t, fixture.store)

	first, err := service.Tick(context.Background())
	if err != nil {
		t.Fatalf("first Tick failed: %v", err)
	}
	second, err := service.Tick(context.Background())
	if err != nil {
		t.Fatalf("second Tick failed: %v", err)
	}
	if !first.Triggered || first.Deduplicated || second.Triggered {
		t.Fatalf("unexpected tick results: first=%+v second=%+v", first, second)
	}
	executions, err := fixture.execStore.ListByWorkflow(fixture.workflow.ID, 10)
	if err != nil {
		t.Fatalf("ListByWorkflow failed: %v", err)
	}
	if len(executions) != 1 {
		t.Fatalf("execution count = %d, want 1", len(executions))
	}
	execution := executions[0]
	if execution.TriggerSource != string(application.SourceSchedule) || execution.TriggerPrincipal != "appliance-scheduler" {
		t.Fatalf("unexpected trigger metadata: %+v", execution)
	}
	if execution.RequestID == "" || execution.IdempotencyKey == "" {
		t.Fatalf("missing deterministic identifiers: %+v", execution)
	}
}

func TestServiceInitializesFutureNextRunWithoutTrigger(t *testing.T) {
	beforeDue := time.Date(2026, 8, 13, 8, 0, 0, 0, time.UTC)
	fixture := newServiceFixtureAt(t, beforeDue)
	schedule, err := fixture.store.GetByWorkflow(fixture.workflow.ID)
	if err != nil {
		t.Fatalf("load schedule: %v", err)
	}
	schedule.NextRunAt = nil
	if err := fixture.store.Upsert(schedule, beforeDue); err != nil {
		t.Fatalf("clear next run: %v", err)
	}
	result, err := fixture.service(t, fixture.store).Tick(context.Background())
	if err != nil {
		t.Fatalf("Tick failed: %v", err)
	}
	want := time.Date(2026, 8, 13, 12, 0, 0, 0, time.UTC)
	if result.Triggered || result.NextRunAt == nil || !result.NextRunAt.Equal(want) {
		t.Fatalf("Tick = %+v, want future next run %s without trigger", result, want)
	}
	executions, err := fixture.execStore.ListByWorkflow(fixture.workflow.ID, 10)
	if err != nil || len(executions) != 0 {
		t.Fatalf("executions = %d, %v; want 0", len(executions), err)
	}
}

func TestServiceClockRollbackDoesNotRepeatScheduledInstant(t *testing.T) {
	fixture := newServiceFixture(t)
	if _, err := fixture.service(t, fixture.store).Tick(context.Background()); err != nil {
		t.Fatalf("due Tick failed: %v", err)
	}
	rolledBack := fixture
	rolledBack.clock = fixedClock{now: time.Date(2026, 8, 13, 11, 0, 0, 0, time.UTC)}
	result, err := rolledBack.service(t, fixture.store).Tick(context.Background())
	if err != nil {
		t.Fatalf("rollback Tick failed: %v", err)
	}
	if result.Triggered {
		t.Fatalf("clock rollback retriggered schedule: %+v", result)
	}
	executions, err := fixture.execStore.ListByWorkflow(fixture.workflow.ID, 10)
	if err != nil || len(executions) != 1 {
		t.Fatalf("executions = %d, %v; want 1", len(executions), err)
	}
}

func TestServiceRestartAfterMetadataFailureDeduplicatesExecution(t *testing.T) {
	fixture := newServiceFixture(t)
	failing := &failAdvanceStore{ScheduleStore: fixture.store, remaining: 1}
	firstService := fixture.service(t, failing)
	if _, err := firstService.Tick(context.Background()); err == nil {
		t.Fatal("expected metadata persistence failure")
	}

	restarted := fixture.service(t, fixture.store)
	result, err := restarted.Tick(context.Background())
	if err != nil {
		t.Fatalf("restart Tick failed: %v", err)
	}
	if !result.Triggered || !result.Deduplicated {
		t.Fatalf("restart result = %+v, want deduplicated trigger", result)
	}
	executions, err := fixture.execStore.ListByWorkflow(fixture.workflow.ID, 10)
	if err != nil {
		t.Fatalf("ListByWorkflow failed: %v", err)
	}
	if len(executions) != 1 {
		t.Fatalf("execution count after restart = %d, want 1", len(executions))
	}
}

func TestServiceConcurrentTicksProduceOneExecution(t *testing.T) {
	fixture := newServiceFixture(t)
	service := fixture.service(t, fixture.store)
	var wg sync.WaitGroup
	errorsSeen := make(chan error, 8)
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := service.Tick(context.Background())
			errorsSeen <- err
		}()
	}
	wg.Wait()
	close(errorsSeen)
	for err := range errorsSeen {
		if err != nil {
			t.Fatalf("concurrent Tick failed: %v", err)
		}
	}
	executions, err := fixture.execStore.ListByWorkflow(fixture.workflow.ID, 10)
	if err != nil || len(executions) != 1 {
		t.Fatalf("executions = %d, %v; want 1", len(executions), err)
	}
}

func TestServiceSkipsMissedAndBlockedSchedulesWithoutTrigger(t *testing.T) {
	tests := []struct {
		name     string
		now      time.Time
		ready    bool
		category string
		want     string
	}{
		{
			name: "missed", now: time.Date(2026, 8, 13, 12, 5, 0, 0, time.UTC),
			ready: true, want: CategoryMissedSkipped,
		},
		{
			name: "setup incomplete", now: time.Date(2026, 8, 13, 12, 0, 0, 0, time.UTC),
			ready: false, category: CategorySetupIncomplete, want: CategorySetupIncomplete,
		},
		{
			name: "revalidation", now: time.Date(2026, 8, 13, 12, 0, 0, 0, time.UTC),
			ready: false, category: CategoryRevalidation, want: CategoryRevalidation,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			fixture := newServiceFixtureAt(t, test.now)
			var calls atomic.Int32
			trigger := triggerFunc(func(context.Context, application.TriggerRequest) (*application.TriggerResult, error) {
				calls.Add(1)
				return nil, errors.New("must not trigger")
			})
			service, err := NewService(Options{
				Store: fixture.store, Triggerer: trigger, Clock: fixture.clock,
				PackID: fixture.packID, WorkflowID: fixture.workflow.ID,
				Readiness: func() (bool, string) { return test.ready, test.category },
			})
			if err != nil {
				t.Fatalf("NewService failed: %v", err)
			}
			result, err := service.Tick(context.Background())
			if err != nil {
				t.Fatalf("Tick failed: %v", err)
			}
			if result.Category != test.want || calls.Load() != 0 {
				t.Fatalf("result=%+v calls=%d", result, calls.Load())
			}
		})
	}
}

func TestServiceMapsInactiveAndConcurrencyWithoutCrashing(t *testing.T) {
	for _, test := range []struct {
		name string
		err  error
		want string
	}{
		{name: "inactive", err: application.ErrWorkflowInactive, want: CategoryWorkflowInactive},
		{name: "already running", err: engine.ErrWorkflowConcurrencyLimit, want: CategoryAlreadyRunning},
	} {
		t.Run(test.name, func(t *testing.T) {
			fixture := newServiceFixture(t)
			service, err := NewService(Options{
				Store: fixture.store,
				Triggerer: triggerFunc(func(context.Context, application.TriggerRequest) (*application.TriggerResult, error) {
					return nil, test.err
				}),
				Clock: fixture.clock, PackID: fixture.packID, WorkflowID: fixture.workflow.ID,
			})
			if err != nil {
				t.Fatalf("NewService failed: %v", err)
			}
			result, err := service.Tick(context.Background())
			if err != nil || result.Category != test.want || result.Triggered {
				t.Fatalf("Tick = %+v, %v", result, err)
			}
		})
	}
}

func TestServiceCorruptScheduleFailsClosed(t *testing.T) {
	fixture := newServiceFixture(t)
	if _, err := fixture.db.WriteDB.Exec(`UPDATE workflow_schedules SET timezone = 'Mars/Olympus' WHERE workflow_id = ?`, fixture.workflow.ID); err != nil {
		t.Fatalf("corrupt schedule: %v", err)
	}
	var calls atomic.Int32
	service, err := NewService(Options{
		Store: fixture.store,
		Triggerer: triggerFunc(func(context.Context, application.TriggerRequest) (*application.TriggerResult, error) {
			calls.Add(1)
			return nil, nil
		}),
		Clock: fixture.clock, PackID: fixture.packID, WorkflowID: fixture.workflow.ID,
	})
	if err != nil {
		t.Fatalf("NewService failed: %v", err)
	}
	result, err := service.Tick(context.Background())
	if err != nil || result.State != storage.ScheduleStateNeedsAttention || result.Category != CategoryScheduleInvalid || calls.Load() != 0 {
		t.Fatalf("Tick = %+v, %v calls=%d", result, err, calls.Load())
	}
}

func TestServiceContextCancellationAndRunShutdown(t *testing.T) {
	fixture := newServiceFixture(t)
	service := fixture.service(t, fixture.store)
	cancelled, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := service.Tick(cancelled); !errors.Is(err, context.Canceled) {
		t.Fatalf("Tick error = %v, want context.Canceled", err)
	}
	done := make(chan struct{})
	go func() {
		service.Run(cancelled, time.Hour)
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("Run did not stop after cancellation")
	}
}

type serviceFixture struct {
	db        *storage.DB
	store     *storage.WorkflowScheduleStore
	execStore *storage.ExecutionStore
	triggerer *application.TriggerService
	workflow  *storage.Workflow
	clock     fixedClock
	packID    string
}

func newServiceFixture(t *testing.T) serviceFixture {
	return newServiceFixtureAt(t, time.Date(2026, 8, 13, 12, 0, 0, 0, time.UTC))
}

func newServiceFixtureAt(t *testing.T, now time.Time) serviceFixture {
	t.Helper()
	db, err := storage.NewDB(filepath.Join(t.TempDir(), "scheduler.db"))
	if err != nil {
		t.Fatalf("NewDB failed: %v", err)
	}
	t.Cleanup(db.Close)
	wfStore := storage.NewWorkflowStore(db)
	workflow := &storage.Workflow{
		ID: "workflow-1", Name: "DailyOps", IsActive: true,
		NodesJSON: `[]`, EdgesJSON: `[]`, MaxConcurrentRuns: 1, ConcurrencyPolicy: "reject",
	}
	if err := wfStore.Create(workflow); err != nil {
		t.Fatalf("create workflow: %v", err)
	}
	execStore := storage.NewExecutionStore(db)
	credStore := storage.NewCredentialStore(db, crypto.NewCryptoManager("test-master-key"))
	eng := engine.NewEngine(nodes.NewBuiltinRegistry(), execStore, credStore, engine.NewEventBus(), wfStore, 10, 4)
	store := storage.NewWorkflowScheduleStore(db)
	due := time.Date(2026, 8, 13, 12, 0, 0, 0, time.UTC)
	if err := store.Upsert(&storage.WorkflowSchedule{
		WorkflowID: workflow.ID, PackID: "official.dailyops-rest-telegram",
		SchemaVersion: storage.WorkflowScheduleSchemaVersion, Enabled: true,
		Kind: storage.ScheduleKindDaily, LocalTime: "19:00", Timezone: "Asia/Bangkok",
		MissedRunPolicy: storage.ScheduleMissedRunSkip, NextRunAt: &due,
		State: storage.ScheduleStateOK,
	}, now.Add(-time.Hour)); err != nil {
		t.Fatalf("save schedule: %v", err)
	}
	return serviceFixture{
		db: db, store: store, execStore: execStore,
		triggerer: application.NewTriggerService(wfStore, eng), workflow: workflow,
		clock: fixedClock{now: now}, packID: "official.dailyops-rest-telegram",
	}
}

func (f serviceFixture) service(t *testing.T, store ScheduleStore) *Service {
	t.Helper()
	service, err := NewService(Options{
		Store: store, Triggerer: f.triggerer, Clock: f.clock,
		PackID: f.packID, WorkflowID: f.workflow.ID,
		Logger: log.New(io.Discard, "", 0),
	})
	if err != nil {
		t.Fatalf("NewService failed: %v", err)
	}
	return service
}

type fixedClock struct{ now time.Time }

func (c fixedClock) Now() time.Time { return c.now }

type triggerFunc func(context.Context, application.TriggerRequest) (*application.TriggerResult, error)

func (fn triggerFunc) Trigger(ctx context.Context, req application.TriggerRequest) (*application.TriggerResult, error) {
	return fn(ctx, req)
}

type failAdvanceStore struct {
	ScheduleStore
	remaining int
}

func (s *failAdvanceStore) Advance(workflowID, packID string, advance storage.ScheduleAdvance) (bool, error) {
	if s.remaining > 0 {
		s.remaining--
		return false, errors.New("injected metadata failure")
	}
	return s.ScheduleStore.Advance(workflowID, packID, advance)
}
