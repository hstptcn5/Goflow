package cli

import (
	"bytes"
	"encoding/json"
	"testing"

	"goflow/internal/buildinfo"
)

func TestVersionJSONIsStableAndOffline(t *testing.T) {
	var stdout, stderr bytes.Buffer
	info := buildinfo.Info{Version: "1.0.0", Channel: "community-stable", Commit: "0123456789abcdef0123456789abcdef01234567", Target: "linux-amd64", GoVersion: "go1.test"}
	code := (Runner{Stdout: &stdout, Stderr: &stderr, BuildInfo: info}).Run([]string{"version", "--output", "json"})
	if code != ExitOK || stderr.Len() != 0 {
		t.Fatalf("code=%d stderr=%q", code, stderr.String())
	}
	var got buildinfo.Info
	if err := json.Unmarshal(stdout.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if got != info {
		t.Fatalf("got %#v want %#v", got, info)
	}
}

func TestVersionRejectsUnknownOutput(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := (Runner{Stdout: &stdout, Stderr: &stderr}).Run([]string{"version", "--output", "yaml"})
	if code != ExitInvalidInput {
		t.Fatalf("code=%d stderr=%q", code, stderr.String())
	}
}
