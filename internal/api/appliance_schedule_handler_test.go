package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"goflow/internal/pack"
	"goflow/internal/packsetup"
	"goflow/internal/storage"
)

type fixedApplianceClock struct {
	now time.Time
}

func (c fixedApplianceClock) Now() time.Time { return c.now }

func TestApplianceScheduleCreateUpdateAndConflict(t *testing.T) {
	db, err := storage.NewDB(filepath.Join(t.TempDir(), "goflow.db"))
	if err != nil {
		t.Fatalf("NewDB failed: %v", err)
	}
	t.Cleanup(db.Close)
	now := time.Date(2026, 8, 9, 1, 0, 0, 0, time.UTC)
	createScheduleTestWorkflow(t, db, "workflow-1")
	appliance := &ApplianceContext{
		Enabled:       true,
		PackID:        "official.dailyops-rest-telegram",
		WorkflowID:    "workflow-1",
		ScheduleStore: storage.NewWorkflowScheduleStore(db),
		ScheduleClock: fixedApplianceClock{now: now},
	}

	get := httptest.NewRecorder()
	applianceScheduleHandler(appliance, storage.NewExecutionStore(db)).ServeHTTP(get, httptest.NewRequest(http.MethodGet, "/api/appliance/schedule", nil))
	if get.Code != http.StatusOK {
		t.Fatalf("default GET status = %d, body=%s", get.Code, get.Body.String())
	}
	var initial applianceScheduleView
	if err := json.Unmarshal(get.Body.Bytes(), &initial); err != nil {
		t.Fatalf("decode initial view: %v", err)
	}
	if initial.Configured || initial.Enabled || initial.Revision != 0 || initial.LocalTime != "09:00" || initial.Timezone != "UTC" {
		t.Fatalf("unexpected initial view: %+v", initial)
	}

	created := saveApplianceSchedule(t, appliance, storage.NewExecutionStore(db), map[string]interface{}{
		"expected_revision": 0,
		"enabled":           true,
		"local_time":        "08:05",
		"timezone":          "Asia/Bangkok",
	}, http.StatusOK)
	if !created.Enabled || created.Revision != 1 || created.NextRunAt == nil ||
		!created.NextRunAt.Equal(time.Date(2026, 8, 9, 1, 5, 0, 0, time.UTC)) {
		t.Fatalf("unexpected created schedule: %+v", created)
	}

	saveApplianceSchedule(t, appliance, storage.NewExecutionStore(db), map[string]interface{}{
		"expected_revision": 0,
		"enabled":           false,
		"local_time":        "09:00",
		"timezone":          "UTC",
	}, http.StatusConflict)

	disabled := saveApplianceSchedule(t, appliance, storage.NewExecutionStore(db), map[string]interface{}{
		"expected_revision": created.Revision,
		"enabled":           false,
		"local_time":        "18:30",
		"timezone":          "Asia/Bangkok",
	}, http.StatusOK)
	if disabled.Enabled || disabled.State != storage.ScheduleStateDisabled || disabled.NextRunAt != nil || disabled.Revision != 2 {
		t.Fatalf("unexpected disabled schedule: %+v", disabled)
	}
}

func TestApplianceScheduleRejectsInvalidTimezone(t *testing.T) {
	db, err := storage.NewDB(filepath.Join(t.TempDir(), "goflow.db"))
	if err != nil {
		t.Fatalf("NewDB failed: %v", err)
	}
	t.Cleanup(db.Close)
	createScheduleTestWorkflow(t, db, "workflow-1")
	appliance := &ApplianceContext{
		Enabled:       true,
		PackID:        "pack-1",
		WorkflowID:    "workflow-1",
		ScheduleStore: storage.NewWorkflowScheduleStore(db),
		ScheduleClock: fixedApplianceClock{now: time.Date(2026, 8, 9, 1, 0, 0, 0, time.UTC)},
	}
	saveApplianceSchedule(t, appliance, storage.NewExecutionStore(db), map[string]interface{}{
		"expected_revision": 0,
		"enabled":           true,
		"local_time":        "08:05",
		"timezone":          "Local",
	}, http.StatusBadRequest)
	if _, err := appliance.ScheduleStore.GetByWorkflow(appliance.WorkflowID); err != storage.ErrWorkflowScheduleNotFound {
		t.Fatalf("invalid schedule was persisted: %v", err)
	}
}

func TestApplianceConfigChangeReopensSetupWithoutChangingSchedule(t *testing.T) {
	dir := t.TempDir()
	appliance := &ApplianceContext{
		Enabled: true, PackID: "pack-1", PackName: "Pack", PackVersion: "1.0.0",
		WorkflowID: "workflow-1", DataDir: dir,
		ConfigSchema: []pack.ConfigField{{
			Key: "source_url", Type: "url", Required: true,
		}},
	}
	manifest := applianceManifest(appliance)
	if _, err := packsetup.SaveConfig(dir, manifest, map[string]interface{}{"source_url": "https://old.example.test/feed"}); err != nil {
		t.Fatalf("save initial config: %v", err)
	}
	if _, err := packsetup.SaveState(dir, manifest, true, time.Date(2026, 8, 9, 1, 0, 0, 0, time.UTC)); err != nil {
		t.Fatalf("save complete state: %v", err)
	}
	payload := bytes.NewBufferString(`{"values":{"source_url":"https://new.example.test/feed"}}`)
	recorder := httptest.NewRecorder()
	applianceSaveConfigHandler(appliance).ServeHTTP(recorder, httptest.NewRequest(http.MethodPost, "/api/appliance/setup/config", payload))
	if recorder.Code != http.StatusOK {
		t.Fatalf("save config status = %d, body=%s", recorder.Code, recorder.Body.String())
	}
	state, err := packsetup.LoadState(dir, manifest)
	if err != nil {
		t.Fatalf("LoadState failed: %v", err)
	}
	if state.Completed {
		t.Fatal("changed config left setup completed")
	}
}

func TestApplianceSetupExposesBoundedMigrationAttentionCategory(t *testing.T) {
	dir := t.TempDir()
	appliance := &ApplianceContext{
		Enabled: true, PackID: "example.attention", PackName: "Attention Pack",
		PackVersion: "2.0.0", WorkflowID: "workflow-1", DataDir: dir,
	}
	registry, err := packsetup.NewMigrationRegistry()
	if err != nil {
		t.Fatalf("NewMigrationRegistry: %v", err)
	}
	if _, err := packsetup.ApplyMigrations(
		dir, applianceManifest(appliance), "1.0.0", registry, packsetup.MigrationOptions{},
	); err != nil {
		t.Fatalf("ApplyMigrations: %v", err)
	}
	recorder := httptest.NewRecorder()
	applianceSetupHandler(appliance, nil).ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/api/appliance/setup", nil))
	if recorder.Code != http.StatusOK {
		t.Fatalf("setup status = %d, body=%s", recorder.Code, recorder.Body.String())
	}
	var response map[string]interface{}
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode setup response: %v", err)
	}
	if response["attention_category"] != packsetup.MigrationUserReview {
		t.Fatalf("attention category = %#v", response["attention_category"])
	}
	for _, forbidden := range []string{"backup_relative", "applied_steps", "from_version", "to_version"} {
		if _, exposed := response[forbidden]; exposed {
			t.Fatalf("setup response exposed internal migration field %q", forbidden)
		}
	}
}

func createScheduleTestWorkflow(t *testing.T, db *storage.DB, workflowID string) {
	t.Helper()
	err := storage.NewWorkflowStore(db).Create(&storage.Workflow{
		ID: workflowID, Name: "Managed workflow", NodesJSON: "[]", EdgesJSON: "[]",
		MaxConcurrentRuns: 1, ConcurrencyPolicy: "reject",
	})
	if err != nil {
		t.Fatalf("create workflow: %v", err)
	}
}

func saveApplianceSchedule(t *testing.T, appliance *ApplianceContext, execStore *storage.ExecutionStore, body map[string]interface{}, wantStatus int) applianceScheduleView {
	t.Helper()
	payload, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("marshal request: %v", err)
	}
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPut, "/api/appliance/schedule", bytes.NewReader(payload))
	applianceSaveScheduleHandler(appliance, execStore).ServeHTTP(recorder, request)
	if recorder.Code != wantStatus {
		t.Fatalf("save status = %d, want %d, body=%s", recorder.Code, wantStatus, recorder.Body.String())
	}
	var view applianceScheduleView
	if wantStatus == http.StatusOK {
		if err := json.Unmarshal(recorder.Body.Bytes(), &view); err != nil {
			t.Fatalf("decode schedule view: %v", err)
		}
	}
	return view
}
