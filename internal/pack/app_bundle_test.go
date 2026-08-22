package pack

import (
	"os"
	"path/filepath"
	"testing"
)

func TestBuildAndExtractEmbeddedApp(t *testing.T) {
	dir := writeBuildPack(t, func(m *Manifest) {
		m.RequiredCapabilities = []string{CapabilityPackV1, CapabilityAppUIV1}
		m.RunUI = &RunUI{
			InputMode: "webhook_body", OutputMode: "cards", OutputNodeID: "trigger", SubmitLabel: "Run",
			InputFields: []RunField{{Key: "amount", Label: "Amount", Type: "number", Required: true}},
		}
		m.Branding = &Branding{Icon: "G", AccentColor: "#2563EB"}
	})
	runtimePath := writeRuntimeFixture(t, "runtime-prefix")
	result, err := BuildApp(AppBuildOptions{PackDir: dir, OutputDir: t.TempDir(), RuntimePath: runtimePath})
	if err != nil {
		t.Fatalf("BuildApp failed: %v", err)
	}
	if !IsEmbeddedApp(result.AppPath) {
		t.Fatal("generated file was not detected as an embedded app")
	}
	info, err := VerifyEmbeddedApp(result.AppPath)
	if err != nil || info.PackID == "" {
		t.Fatalf("VerifyEmbeddedApp failed: %#v %v", info, err)
	}
	destination := t.TempDir()
	packDir, _, err := ExtractEmbeddedApp(result.AppPath, destination)
	if err != nil {
		t.Fatalf("ExtractEmbeddedApp failed: %v", err)
	}
	if _, err := os.Stat(filepath.Join(packDir, ManifestFile)); err != nil {
		t.Fatalf("extracted manifest missing: %v", err)
	}
}

func TestLoadRejectsRunUIWithoutCapability(t *testing.T) {
	dir := writeValidPack(t, func(m *Manifest) {
		m.RunUI = &RunUI{InputMode: "direct", OutputMode: "auto"}
	})
	assertLoadError(t, dir, CapabilityAppUIV1)
}

func TestLoadRejectsMissingRunUIOutputNode(t *testing.T) {
	dir := writeValidPack(t, func(m *Manifest) {
		m.RequiredCapabilities = []string{CapabilityPackV1, CapabilityAppUIV1}
		m.RunUI = &RunUI{InputMode: "direct", OutputMode: "json", OutputNodeID: "missing"}
	})
	assertLoadError(t, dir, "output_node_id")
}
