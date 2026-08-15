package buildinfo

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const testCommit = "0123456789abcdef0123456789abcdef01234567"

func TestCurrentAcceptsCompleteOfficialIdentity(t *testing.T) {
	reset := setBuildValues(CommunityStableVersion, CommunityStableChannel, testCommit, CurrentTarget())
	defer reset()
	got := Current("ignored")
	if err := got.ValidateOfficial(CommunityStableVersion, testCommit, CurrentTarget()); err != nil {
		t.Fatal(err)
	}
}

func TestCurrentRejectsIncompleteOrInconsistentOfficialIdentity(t *testing.T) {
	mismatchedTarget := "linux-amd64"
	if mismatchedTarget == CurrentTarget() {
		mismatchedTarget = "darwin-arm64"
	}
	tests := []struct {
		name, version, channel, commit, target string
	}{
		{name: "wrong channel", version: CommunityStableVersion, channel: "community-rc", commit: testCommit, target: CurrentTarget()},
		{name: "wrong version", version: "1.0.0-rc.1", channel: CommunityStableChannel, commit: testCommit, target: CurrentTarget()},
		{name: "malformed commit", version: CommunityStableVersion, channel: CommunityStableChannel, commit: strings.Repeat("z", 40), target: CurrentTarget()},
		{name: "missing commit", version: CommunityStableVersion, channel: CommunityStableChannel, target: CurrentTarget()},
		{name: "unsupported target", version: CommunityStableVersion, channel: CommunityStableChannel, commit: testCommit, target: "plan9-amd64"},
		{name: "runtime mismatched target", version: CommunityStableVersion, channel: CommunityStableChannel, commit: testCommit, target: mismatchedTarget},
		{name: "version control injection", version: CommunityStableVersion + "\nforged", channel: CommunityStableChannel, commit: testCommit, target: CurrentTarget()},
		{name: "channel control injection", version: CommunityStableVersion, channel: CommunityStableChannel + "\t", commit: testCommit, target: CurrentTarget()},
		{name: "commit control injection", version: CommunityStableVersion, channel: CommunityStableChannel, commit: testCommit[:39] + "\n", target: CurrentTarget()},
		{name: "target control injection", version: CommunityStableVersion, channel: CommunityStableChannel, commit: testCommit, target: CurrentTarget() + "\r"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reset := setBuildValues(tt.version, tt.channel, tt.commit, tt.target)
			defer reset()
			assertDevelopmentIdentity(t, Current(CommunityStableVersion), CommunityStableVersion)
		})
	}
}

func TestCurrentDevelopmentFallback(t *testing.T) {
	reset := setBuildValues("", "", "", "")
	defer reset()
	assertDevelopmentIdentity(t, Current("dev-snapshot"), "dev-snapshot")
}

func TestCurrentBoundsInvalidEmbeddedVersion(t *testing.T) {
	reset := setBuildValues("", "", "", "")
	defer reset()
	assertDevelopmentIdentity(t, Current("dev\nforged"), "development")
}

func TestValidateOfficialRejectsRuntimeMismatchedTarget(t *testing.T) {
	other := "linux-amd64"
	if other == CurrentTarget() {
		other = "darwin-arm64"
	}
	info := Info{Version: CommunityStableVersion, Channel: CommunityStableChannel, Commit: testCommit, Target: other, GoVersion: "go-test"}
	if err := info.ValidateOfficial(CommunityStableVersion, testCommit, other); err == nil {
		t.Fatal("runtime-mismatched official identity was accepted")
	}
}

func TestRepositoryVersionMatchesCommunityStableVersion(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("..", "..", "VERSION"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(string(data)) != CommunityStableVersion {
		t.Fatalf("VERSION does not match canonical Community Stable version")
	}
}

func assertDevelopmentIdentity(t *testing.T, got Info, version string) {
	t.Helper()
	if got.Version != version || got.Channel != DevelopmentChannel || got.Commit != "unknown" || got.Target != CurrentTarget() {
		t.Fatalf("unexpected development identity: %#v", got)
	}
}

func setBuildValues(version, channel, commit, target string) func() {
	oldVersion, oldChannel, oldCommit, oldTarget := BuildVersion, BuildChannel, BuildCommit, BuildTarget
	BuildVersion, BuildChannel, BuildCommit, BuildTarget = version, channel, commit, target
	return func() {
		BuildVersion, BuildChannel, BuildCommit, BuildTarget = oldVersion, oldChannel, oldCommit, oldTarget
	}
}
