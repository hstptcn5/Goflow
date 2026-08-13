package storage

import (
	"errors"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestWorkflowScheduleStoreRoundTripAndOptimisticAdvance(t *testing.T) {
	db := scheduleTestDB(t)
	store := NewWorkflowScheduleStore(db)
	now := time.Date(2026, 8, 13, 12, 0, 0, 0, time.UTC)
	next := now.Add(time.Hour)
	schedule := validWorkflowSchedule(next)
	if err := store.Upsert(schedule, now); err != nil {
		t.Fatalf("Upsert failed: %v", err)
	}

	loaded, err := store.GetByWorkflow(schedule.WorkflowID)
	if err != nil {
		t.Fatalf("GetByWorkflow failed: %v", err)
	}
	if loaded.Revision != 1 || !loaded.Enabled || loaded.NextRunAt == nil || !loaded.NextRunAt.Equal(next) {
		t.Fatalf("unexpected loaded schedule: %+v", loaded)
	}

	scheduledFor := next
	advanced, err := store.Advance(schedule.WorkflowID, schedule.PackID, ScheduleAdvance{
		ExpectedRevision: loaded.Revision,
		ScheduledFor:     &scheduledFor,
		NextRunAt:        next.Add(24 * time.Hour),
		ExecutionID:      "execution-1",
		State:            ScheduleStateOK,
		UpdatedAt:        now.Add(time.Minute),
	})
	if err != nil || !advanced {
		t.Fatalf("Advance = %t, %v", advanced, err)
	}
	advanced, err = store.Advance(schedule.WorkflowID, schedule.PackID, ScheduleAdvance{
		ExpectedRevision: loaded.Revision,
		NextRunAt:        next.Add(48 * time.Hour),
		State:            ScheduleStateOK,
		UpdatedAt:        now.Add(2 * time.Minute),
	})
	if err != nil || advanced {
		t.Fatalf("stale Advance = %t, %v", advanced, err)
	}

	loaded, err = store.GetByWorkflow(schedule.WorkflowID)
	if err != nil {
		t.Fatalf("GetByWorkflow after advance failed: %v", err)
	}
	if loaded.Revision != 2 || loaded.LastExecutionID != "execution-1" || loaded.LastScheduledFor == nil || !loaded.LastScheduledFor.Equal(next) {
		t.Fatalf("unexpected advanced schedule: %+v", loaded)
	}
}

func TestWorkflowScheduleStoreEnableDisableKeepsOneRow(t *testing.T) {
	db := scheduleTestDB(t)
	store := NewWorkflowScheduleStore(db)
	now := time.Date(2026, 8, 13, 12, 0, 0, 0, time.UTC)
	schedule := validWorkflowSchedule(now.Add(time.Hour))
	schedule.Enabled = false
	schedule.State = ScheduleStateDisabled
	schedule.NextRunAt = nil
	if err := store.Upsert(schedule, now); err != nil {
		t.Fatalf("save disabled schedule: %v", err)
	}
	loaded, err := store.GetByWorkflow(schedule.WorkflowID)
	if err != nil || loaded.Enabled || loaded.State != ScheduleStateDisabled {
		t.Fatalf("disabled schedule = %+v, %v", loaded, err)
	}
	next := now.Add(time.Hour)
	loaded.Enabled = true
	loaded.State = ScheduleStateOK
	loaded.NextRunAt = &next
	if err := store.Upsert(loaded, now.Add(time.Minute)); err != nil {
		t.Fatalf("enable schedule: %v", err)
	}
	loaded, err = store.GetByWorkflow(schedule.WorkflowID)
	if err != nil || !loaded.Enabled || loaded.Revision != 2 {
		t.Fatalf("enabled schedule = %+v, %v", loaded, err)
	}
	var count int
	if err := db.ReadDB.QueryRow(`SELECT COUNT(1) FROM workflow_schedules WHERE workflow_id = ?`, schedule.WorkflowID).Scan(&count); err != nil {
		t.Fatalf("count schedules: %v", err)
	}
	if count != 1 {
		t.Fatalf("schedule row count = %d, want 1", count)
	}
}

func TestWorkflowScheduleStoreConfigureUsesOptimisticRevision(t *testing.T) {
	db := scheduleTestDB(t)
	store := NewWorkflowScheduleStore(db)
	now := time.Date(2026, 8, 13, 12, 0, 0, 0, time.UTC)
	schedule := validWorkflowSchedule(now.Add(time.Hour))
	if err := store.Configure(schedule, 0, now); err != nil {
		t.Fatalf("initial Configure failed: %v", err)
	}
	if err := store.Configure(schedule, 0, now.Add(time.Minute)); !errors.Is(err, ErrWorkflowScheduleConflict) {
		t.Fatalf("duplicate create error = %v, want revision conflict", err)
	}
	loaded, err := store.GetByWorkflow(schedule.WorkflowID)
	if err != nil {
		t.Fatalf("GetByWorkflow failed: %v", err)
	}
	loaded.LocalTime = "18:45"
	if err := store.Configure(loaded, loaded.Revision, now.Add(time.Minute)); err != nil {
		t.Fatalf("revision update failed: %v", err)
	}
	if err := store.Configure(loaded, loaded.Revision, now.Add(2*time.Minute)); !errors.Is(err, ErrWorkflowScheduleConflict) {
		t.Fatalf("stale update error = %v, want revision conflict", err)
	}
	updated, err := store.GetByWorkflow(schedule.WorkflowID)
	if err != nil {
		t.Fatalf("GetByWorkflow after update failed: %v", err)
	}
	if updated.Revision != 2 || updated.LocalTime != "18:45" {
		t.Fatalf("updated schedule = %+v", updated)
	}
}

func TestWorkflowScheduleStoreRejectsCrossPackOverwriteAndCorruptState(t *testing.T) {
	db := scheduleTestDB(t)
	store := NewWorkflowScheduleStore(db)
	now := time.Date(2026, 8, 13, 12, 0, 0, 0, time.UTC)
	schedule := validWorkflowSchedule(now.Add(time.Hour))
	if err := store.Upsert(schedule, now); err != nil {
		t.Fatalf("Upsert failed: %v", err)
	}

	other := *schedule
	other.PackID = "other.pack"
	if err := store.Upsert(&other, now.Add(time.Minute)); err == nil || !strings.Contains(err.Error(), "ownership mismatch") {
		t.Fatalf("expected ownership mismatch, got %v", err)
	}

	if _, err := db.WriteDB.Exec(`UPDATE workflow_schedules SET schema_version = 99 WHERE workflow_id = ?`, schedule.WorkflowID); err != nil {
		t.Fatalf("corrupt schedule: %v", err)
	}
	if _, err := store.GetByWorkflow(schedule.WorkflowID); err == nil || !strings.Contains(err.Error(), "unsupported schedule schema_version") {
		t.Fatalf("expected future schema failure, got %v", err)
	}
	if _, err := db.WriteDB.Exec(`UPDATE workflow_schedules SET schema_version = 1, enabled = 2 WHERE workflow_id = ?`, schedule.WorkflowID); err != nil {
		t.Fatalf("corrupt enabled flag: %v", err)
	}
	if _, err := store.GetByWorkflow(schedule.WorkflowID); !errors.Is(err, ErrInvalidWorkflowSchedule) {
		t.Fatalf("expected invalid row error, got %v", err)
	}
}

func TestValidateWorkflowScheduleTable(t *testing.T) {
	now := time.Date(2026, 8, 13, 12, 0, 0, 0, time.UTC)
	tests := []struct {
		name   string
		mutate func(*WorkflowSchedule)
		valid  bool
	}{
		{name: "valid enabled", valid: true},
		{name: "valid disabled", valid: true, mutate: func(s *WorkflowSchedule) { s.Enabled = false; s.State = ScheduleStateDisabled; s.NextRunAt = nil }},
		{name: "future schema", mutate: func(s *WorkflowSchedule) { s.SchemaVersion = 2 }},
		{name: "raw cron", mutate: func(s *WorkflowSchedule) { s.Kind = "cron" }},
		{name: "bad local time", mutate: func(s *WorkflowSchedule) { s.LocalTime = "9:00" }},
		{name: "unknown timezone", mutate: func(s *WorkflowSchedule) { s.Timezone = "Mars/Olympus" }},
		{name: "local timezone", mutate: func(s *WorkflowSchedule) { s.Timezone = "Local" }},
		{name: "catch up policy", mutate: func(s *WorkflowSchedule) { s.MissedRunPolicy = "catch_up" }},
		{name: "enabled disabled state", mutate: func(s *WorkflowSchedule) { s.State = ScheduleStateDisabled }},
		{name: "attention without category", mutate: func(s *WorkflowSchedule) { s.State = ScheduleStateNeedsAttention }},
		{name: "unknown error category", mutate: func(s *WorkflowSchedule) { s.State = ScheduleStateNeedsAttention; s.ErrorCategory = "future_error" }},
		{name: "non advancing next run", mutate: func(s *WorkflowSchedule) { previous := s.NextRunAt.Add(time.Hour); s.LastScheduledFor = &previous }},
		{name: "oversized pack", mutate: func(s *WorkflowSchedule) { s.PackID = strings.Repeat("p", MaxSchedulePackIDLength+1) }},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			schedule := validWorkflowSchedule(now.Add(time.Hour))
			if test.mutate != nil {
				test.mutate(schedule)
			}
			err := ValidateWorkflowSchedule(schedule)
			if test.valid && err != nil {
				t.Fatalf("expected valid schedule: %v", err)
			}
			if !test.valid && err == nil {
				t.Fatal("expected validation failure")
			}
		})
	}
}

func TestWorkflowScheduleStoreNotFound(t *testing.T) {
	db := scheduleTestDB(t)
	_, err := NewWorkflowScheduleStore(db).GetByWorkflow("missing")
	if !errors.Is(err, ErrWorkflowScheduleNotFound) {
		t.Fatalf("expected ErrWorkflowScheduleNotFound, got %v", err)
	}
}

func scheduleTestDB(t *testing.T) *DB {
	t.Helper()
	db, err := NewDB(filepath.Join(t.TempDir(), "schedule.db"))
	if err != nil {
		t.Fatalf("NewDB failed: %v", err)
	}
	t.Cleanup(db.Close)
	wfStore := NewWorkflowStore(db)
	if err := wfStore.Create(&Workflow{ID: "workflow-1", Name: "DailyOps", NodesJSON: `[]`, EdgesJSON: `[]`}); err != nil {
		t.Fatalf("create workflow: %v", err)
	}
	return db
}

func validWorkflowSchedule(next time.Time) *WorkflowSchedule {
	next = next.UTC()
	return &WorkflowSchedule{
		WorkflowID:      "workflow-1",
		PackID:          "official.dailyops-rest-telegram",
		SchemaVersion:   WorkflowScheduleSchemaVersion,
		Enabled:         true,
		Kind:            ScheduleKindDaily,
		LocalTime:       "19:00",
		Timezone:        "Asia/Bangkok",
		MissedRunPolicy: ScheduleMissedRunSkip,
		NextRunAt:       &next,
		State:           ScheduleStateOK,
	}
}
