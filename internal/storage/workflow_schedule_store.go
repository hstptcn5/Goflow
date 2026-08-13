package storage

import (
	"database/sql"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"
)

const (
	WorkflowScheduleSchemaVersion = 1
	ScheduleKindDaily             = "daily"
	ScheduleMissedRunSkip         = "skip"
	ScheduleStateOK               = "OK"
	ScheduleStateDisabled         = "DISABLED"
	ScheduleStateNeedsAttention   = "NEEDS_ATTENTION"
	ScheduleErrorInvalid          = "schedule_invalid"
	ScheduleErrorMissedSkipped    = "schedule_missed_skipped"
	ScheduleErrorSetup            = "setup_incomplete"
	ScheduleErrorRevalidation     = "revalidation_required"
	ScheduleErrorInactive         = "workflow_inactive"
	ScheduleErrorAlreadyRunning   = "already_running"
	ScheduleErrorInternal         = "internal_error"

	MaxScheduleWorkflowIDLength = 200
	MaxSchedulePackIDLength     = 200
	MaxScheduleTimezoneLength   = 255
	MaxScheduleExecutionID      = 200
	MaxScheduleErrorCategory    = 64
)

var scheduleLocalTimePattern = regexp.MustCompile(`^(?:[01][0-9]|2[0-3]):[0-5][0-9]$`)

var ErrWorkflowScheduleNotFound = errors.New("workflow schedule not found")
var ErrInvalidWorkflowSchedule = errors.New("invalid workflow schedule")

type WorkflowSchedule struct {
	WorkflowID       string     `json:"workflow_id"`
	PackID           string     `json:"pack_id"`
	SchemaVersion    int        `json:"schema_version"`
	Revision         int64      `json:"revision"`
	Enabled          bool       `json:"enabled"`
	Kind             string     `json:"kind"`
	LocalTime        string     `json:"local_time"`
	Timezone         string     `json:"timezone"`
	MissedRunPolicy  string     `json:"missed_run_policy"`
	LastScheduledFor *time.Time `json:"last_scheduled_for,omitempty"`
	NextRunAt        *time.Time `json:"next_run_at,omitempty"`
	LastExecutionID  string     `json:"last_execution_id,omitempty"`
	State            string     `json:"state"`
	ErrorCategory    string     `json:"error_category,omitempty"`
	CreatedAt        time.Time  `json:"created_at"`
	UpdatedAt        time.Time  `json:"updated_at"`
}

type ScheduleAdvance struct {
	ExpectedRevision int64
	ScheduledFor     *time.Time
	NextRunAt        time.Time
	ExecutionID      string
	State            string
	ErrorCategory    string
	UpdatedAt        time.Time
}

type WorkflowScheduleStore struct {
	db *DB
}

func NewWorkflowScheduleStore(db *DB) *WorkflowScheduleStore {
	return &WorkflowScheduleStore{db: db}
}

func ValidateWorkflowSchedule(schedule *WorkflowSchedule) error {
	if schedule == nil {
		return fmt.Errorf("schedule is required")
	}
	if err := validateScheduleText(schedule.WorkflowID, "workflow_id", MaxScheduleWorkflowIDLength, true); err != nil {
		return err
	}
	if err := validateScheduleText(schedule.PackID, "pack_id", MaxSchedulePackIDLength, true); err != nil {
		return err
	}
	if schedule.SchemaVersion != WorkflowScheduleSchemaVersion {
		return fmt.Errorf("unsupported schedule schema_version %d", schedule.SchemaVersion)
	}
	if schedule.Revision < 0 {
		return fmt.Errorf("schedule revision must not be negative")
	}
	if schedule.Kind != ScheduleKindDaily {
		return fmt.Errorf("schedule kind must be %q", ScheduleKindDaily)
	}
	if !scheduleLocalTimePattern.MatchString(schedule.LocalTime) {
		return fmt.Errorf("schedule local_time must use HH:MM")
	}
	if err := validateScheduleTimezone(schedule.Timezone); err != nil {
		return err
	}
	if schedule.MissedRunPolicy != ScheduleMissedRunSkip {
		return fmt.Errorf("schedule missed_run_policy must be %q", ScheduleMissedRunSkip)
	}
	switch schedule.State {
	case ScheduleStateOK, ScheduleStateDisabled, ScheduleStateNeedsAttention:
	default:
		return fmt.Errorf("schedule state is not supported")
	}
	if schedule.Enabled && schedule.State == ScheduleStateDisabled {
		return fmt.Errorf("enabled schedule cannot have DISABLED state")
	}
	if !schedule.Enabled && schedule.State != ScheduleStateDisabled {
		return fmt.Errorf("disabled schedule must have DISABLED state")
	}
	if err := validateScheduleText(schedule.LastExecutionID, "last_execution_id", MaxScheduleExecutionID, false); err != nil {
		return err
	}
	if err := validateScheduleText(schedule.ErrorCategory, "error_category", MaxScheduleErrorCategory, false); err != nil {
		return err
	}
	if schedule.ErrorCategory != "" && !isScheduleErrorCategory(schedule.ErrorCategory) {
		return fmt.Errorf("schedule error_category is not supported")
	}
	if schedule.State == ScheduleStateNeedsAttention && schedule.ErrorCategory == "" {
		return fmt.Errorf("NEEDS_ATTENTION schedule requires error_category")
	}
	if schedule.State != ScheduleStateNeedsAttention && schedule.ErrorCategory != "" {
		return fmt.Errorf("schedule error_category requires NEEDS_ATTENTION state")
	}
	if schedule.LastScheduledFor != nil && schedule.NextRunAt != nil && !schedule.NextRunAt.After(*schedule.LastScheduledFor) {
		return fmt.Errorf("schedule next_run_at must be after last_scheduled_for")
	}
	return nil
}

func (s *WorkflowScheduleStore) Upsert(schedule *WorkflowSchedule, now time.Time) error {
	if err := ValidateWorkflowSchedule(schedule); err != nil {
		return err
	}
	if now.IsZero() {
		now = time.Now()
	}
	now = now.UTC()
	result, err := s.db.WriteDB.Exec(`
		INSERT INTO workflow_schedules (
			workflow_id, pack_id, schema_version, revision, enabled, kind,
			local_time, timezone, missed_run_policy, last_scheduled_for,
			next_run_at, last_execution_id, state, error_category,
			created_at, updated_at
		) VALUES (?, ?, ?, 1, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(workflow_id) DO UPDATE SET
			pack_id = excluded.pack_id,
			schema_version = excluded.schema_version,
			revision = workflow_schedules.revision + 1,
			enabled = excluded.enabled,
			kind = excluded.kind,
			local_time = excluded.local_time,
			timezone = excluded.timezone,
			missed_run_policy = excluded.missed_run_policy,
			last_scheduled_for = excluded.last_scheduled_for,
			next_run_at = excluded.next_run_at,
			last_execution_id = excluded.last_execution_id,
			state = excluded.state,
			error_category = excluded.error_category,
			updated_at = CASE
				WHEN excluded.updated_at > workflow_schedules.updated_at THEN excluded.updated_at
				ELSE workflow_schedules.updated_at
			END
		WHERE workflow_schedules.pack_id = excluded.pack_id
	`, schedule.WorkflowID, schedule.PackID, schedule.SchemaVersion,
		boolInt(schedule.Enabled), schedule.Kind, schedule.LocalTime, schedule.Timezone,
		schedule.MissedRunPolicy, schedule.LastScheduledFor, schedule.NextRunAt,
		nullableString(schedule.LastExecutionID), schedule.State,
		nullableString(schedule.ErrorCategory), now, now)
	if err != nil {
		return fmt.Errorf("save workflow schedule: %w", err)
	}
	if affected, err := result.RowsAffected(); err == nil && affected == 0 {
		return fmt.Errorf("save workflow schedule: pack_id ownership mismatch")
	}
	return nil
}

func (s *WorkflowScheduleStore) GetByWorkflow(workflowID string) (*WorkflowSchedule, error) {
	row := s.db.ReadDB.QueryRow(`
		SELECT workflow_id, pack_id, schema_version, revision, enabled, kind,
			local_time, timezone, missed_run_policy, last_scheduled_for,
			next_run_at, last_execution_id, state, error_category,
			created_at, updated_at
		FROM workflow_schedules
		WHERE workflow_id = ?
	`, workflowID)
	schedule, err := scanWorkflowSchedule(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrWorkflowScheduleNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("%w: stored row could not be decoded", ErrInvalidWorkflowSchedule)
	}
	if err := ValidateWorkflowSchedule(schedule); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalidWorkflowSchedule, err)
	}
	if schedule.Revision <= 0 || schedule.CreatedAt.IsZero() || schedule.UpdatedAt.IsZero() || schedule.UpdatedAt.Before(schedule.CreatedAt) {
		return nil, fmt.Errorf("%w: invalid persisted metadata", ErrInvalidWorkflowSchedule)
	}
	return schedule, nil
}

func (s *WorkflowScheduleStore) Advance(workflowID, packID string, advance ScheduleAdvance) (bool, error) {
	if advance.ExpectedRevision <= 0 {
		return false, fmt.Errorf("expected schedule revision must be positive")
	}
	if advance.NextRunAt.IsZero() {
		return false, fmt.Errorf("next_run_at is required")
	}
	if advance.UpdatedAt.IsZero() {
		advance.UpdatedAt = time.Now()
	}
	if err := validateScheduleText(advance.ExecutionID, "last_execution_id", MaxScheduleExecutionID, false); err != nil {
		return false, err
	}
	if err := validateScheduleText(advance.ErrorCategory, "error_category", MaxScheduleErrorCategory, false); err != nil {
		return false, err
	}
	if advance.ErrorCategory != "" && !isScheduleErrorCategory(advance.ErrorCategory) {
		return false, fmt.Errorf("schedule error_category is not supported")
	}
	if advance.State != ScheduleStateOK && advance.State != ScheduleStateNeedsAttention {
		return false, fmt.Errorf("advance state must be OK or NEEDS_ATTENTION")
	}
	if advance.State == ScheduleStateNeedsAttention && advance.ErrorCategory == "" {
		return false, fmt.Errorf("NEEDS_ATTENTION advance requires error_category")
	}
	if advance.State == ScheduleStateOK && advance.ErrorCategory != "" {
		return false, fmt.Errorf("OK advance must not include error_category")
	}
	result, err := s.db.WriteDB.Exec(`
		UPDATE workflow_schedules
		SET last_scheduled_for = ?, next_run_at = ?, last_execution_id = ?,
			state = ?, error_category = ?, revision = revision + 1,
			updated_at = CASE WHEN ? > updated_at THEN ? ELSE updated_at END
		WHERE workflow_id = ? AND pack_id = ? AND revision = ? AND enabled = 1
	`, advance.ScheduledFor, advance.NextRunAt.UTC(), nullableString(advance.ExecutionID),
		advance.State, nullableString(advance.ErrorCategory), advance.UpdatedAt.UTC(), advance.UpdatedAt.UTC(),
		workflowID, packID, advance.ExpectedRevision)
	if err != nil {
		return false, fmt.Errorf("advance workflow schedule: %w", err)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return false, fmt.Errorf("advance workflow schedule: %w", err)
	}
	return affected == 1, nil
}

type scheduleScanner interface {
	Scan(dest ...interface{}) error
}

func scanWorkflowSchedule(row scheduleScanner) (*WorkflowSchedule, error) {
	var schedule WorkflowSchedule
	var enabled int
	var lastScheduledFor, nextRunAt sql.NullTime
	var lastExecutionID, errorCategory sql.NullString
	if err := row.Scan(
		&schedule.WorkflowID, &schedule.PackID, &schedule.SchemaVersion,
		&schedule.Revision, &enabled, &schedule.Kind, &schedule.LocalTime,
		&schedule.Timezone, &schedule.MissedRunPolicy, &lastScheduledFor,
		&nextRunAt, &lastExecutionID, &schedule.State, &errorCategory,
		&schedule.CreatedAt, &schedule.UpdatedAt,
	); err != nil {
		return nil, err
	}
	if enabled != 0 && enabled != 1 {
		return nil, fmt.Errorf("enabled must be boolean")
	}
	schedule.Enabled = enabled == 1
	if lastScheduledFor.Valid {
		value := lastScheduledFor.Time.UTC()
		schedule.LastScheduledFor = &value
	}
	if nextRunAt.Valid {
		value := nextRunAt.Time.UTC()
		schedule.NextRunAt = &value
	}
	schedule.LastExecutionID = lastExecutionID.String
	schedule.ErrorCategory = errorCategory.String
	return &schedule, nil
}

func validateScheduleTimezone(value string) error {
	if err := validateScheduleText(value, "timezone", MaxScheduleTimezoneLength, true); err != nil {
		return err
	}
	if value == "Local" || strings.Contains(value, "..") || strings.HasPrefix(value, "/") || strings.Contains(value, `\`) {
		return fmt.Errorf("schedule timezone must be an IANA timezone")
	}
	if _, err := time.LoadLocation(value); err != nil {
		return fmt.Errorf("schedule timezone must be an IANA timezone")
	}
	return nil
}

func validateScheduleText(value, field string, max int, required bool) error {
	trimmed := strings.TrimSpace(value)
	if required && trimmed == "" {
		return fmt.Errorf("schedule %s is required", field)
	}
	if len(value) > max {
		return fmt.Errorf("schedule %s exceeds %d character limit", field, max)
	}
	if strings.IndexByte(value, 0) >= 0 || strings.ContainsAny(value, "\r\n") {
		return fmt.Errorf("schedule %s contains unsupported characters", field)
	}
	return nil
}

func boolInt(value bool) int {
	if value {
		return 1
	}
	return 0
}

func isScheduleErrorCategory(value string) bool {
	switch value {
	case ScheduleErrorInvalid, ScheduleErrorMissedSkipped, ScheduleErrorSetup,
		ScheduleErrorRevalidation, ScheduleErrorInactive,
		ScheduleErrorAlreadyRunning, ScheduleErrorInternal:
		return true
	default:
		return false
	}
}
