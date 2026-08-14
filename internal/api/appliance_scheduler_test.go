package api

import (
	"testing"
	"time"

	"goflow/internal/packsetup"
)

func TestApplianceScheduleReadinessCategories(t *testing.T) {
	dir := t.TempDir()
	appliance := &ApplianceContext{
		Enabled: true, PackID: "official.dailyops-rest-telegram",
		PackName: "DailyOps", PackVersion: "0.2.0", WorkflowID: "workflow-1",
		DataDir: dir,
	}
	if ready, category := ApplianceScheduleReadiness(appliance, nil); ready || category != "setup_incomplete" {
		t.Fatalf("missing state readiness = %t %q", ready, category)
	}
	if _, err := packsetup.SaveState(dir, applianceManifest(appliance), true, time.Date(2026, 8, 13, 12, 0, 0, 0, time.UTC)); err != nil {
		t.Fatalf("SaveState failed: %v", err)
	}
	if ready, category := ApplianceScheduleReadiness(appliance, nil); !ready || category != "" {
		t.Fatalf("complete readiness = %t %q", ready, category)
	}
	appliance.PackVersion = "0.3.0"
	if ready, category := ApplianceScheduleReadiness(appliance, nil); ready || category != "revalidation_required" {
		t.Fatalf("version mismatch readiness = %t %q", ready, category)
	}
}
