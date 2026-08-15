package buildinfo

import "testing"

func TestCurrentDevelopmentFallbackCannotMasqueradeAsOfficial(t *testing.T) {
	reset := setBuildValues("", CommunityRCChannel, "not-a-commit", "linux-amd64")
	defer reset()
	got := Current("1.0.0-rc.1")
	if got.Channel != DevelopmentChannel || got.Commit != "unknown" || got.Version != "1.0.0-rc.1" {
		t.Fatalf("unexpected development identity: %#v", got)
	}
}

func TestCurrentOfficialIdentity(t *testing.T) {
	commit := "0123456789abcdef0123456789abcdef01234567"
	reset := setBuildValues("1.0.0-rc.1", CommunityRCChannel, commit, "linux-arm64")
	defer reset()
	got := Current("ignored")
	if err := got.ValidateOfficial("1.0.0-rc.1", commit, "linux-arm64"); err != nil {
		t.Fatal(err)
	}
}

func TestCurrentRejectsInvalidInjectedVersion(t *testing.T) {
	commit := "0123456789abcdef0123456789abcdef01234567"
	reset := setBuildValues("1.0.0-rc.1\nforged", CommunityRCChannel, commit, "linux-amd64")
	defer reset()
	got := Current(CommunityRCVersion)
	if got.Version != CommunityRCVersion || got.Channel != DevelopmentChannel {
		t.Fatalf("invalid injection was trusted: %#v", got)
	}
}

func setBuildValues(version, channel, commit, target string) func() {
	oldVersion, oldChannel, oldCommit, oldTarget := BuildVersion, BuildChannel, BuildCommit, BuildTarget
	BuildVersion, BuildChannel, BuildCommit, BuildTarget = version, channel, commit, target
	return func() {
		BuildVersion, BuildChannel, BuildCommit, BuildTarget = oldVersion, oldChannel, oldCommit, oldTarget
	}
}
