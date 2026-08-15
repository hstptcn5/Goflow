package buildinfo

import (
	"fmt"
	"runtime"
	"strings"
)

const (
	DevelopmentChannel = "development"
	CommunityRCChannel = "community-rc"
	CommunityRCVersion = "1.0.0-rc.1"
)

// These values are populated for official builds with -ldflags -X. Empty values
// deliberately resolve to development identity and never imply authenticity.
var (
	BuildVersion string
	BuildChannel string
	BuildCommit  string
	BuildTarget  string
)

type Info struct {
	Version   string `json:"version"`
	Channel   string `json:"channel"`
	Commit    string `json:"commit"`
	Target    string `json:"target"`
	GoVersion string `json:"go_version"`
}

func Current(embeddedVersion string) Info {
	version := bounded("", strings.TrimSpace(embeddedVersion), "development")
	if validVersion(BuildVersion) {
		version = BuildVersion
	}
	channel := DevelopmentChannel
	if BuildChannel == CommunityRCChannel && BuildVersion == CommunityRCVersion && validCommit(BuildCommit) && validTarget(BuildTarget) {
		channel = CommunityRCChannel
	}
	commit := "unknown"
	if validCommit(BuildCommit) {
		commit = strings.ToLower(BuildCommit)
	}
	target := runtime.GOOS + "-" + runtime.GOARCH
	if validTarget(BuildTarget) {
		target = BuildTarget
	}
	return Info{Version: version, Channel: channel, Commit: commit, Target: target, GoVersion: runtime.Version()}
}

func validVersion(value string) bool {
	return value == CommunityRCVersion
}

func (i Info) ValidateOfficial(version, commit, target string) error {
	if i.Version != version || i.Channel != CommunityRCChannel || i.Commit != strings.ToLower(commit) || i.Target != target {
		return fmt.Errorf("runtime identity mismatch: got version=%q channel=%q commit=%q target=%q", i.Version, i.Channel, i.Commit, i.Target)
	}
	return nil
}

func bounded(primary, secondary, fallback string) string {
	for _, value := range []string{primary, secondary} {
		value = strings.TrimSpace(value)
		if value != "" && len(value) <= 64 && !strings.ContainsAny(value, "\r\n\t") {
			return value
		}
	}
	return fallback
}

func validCommit(value string) bool {
	if len(value) != 40 {
		return false
	}
	for _, c := range value {
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F')) {
			return false
		}
	}
	return true
}

func validTarget(value string) bool {
	switch value {
	case "linux-amd64", "linux-arm64", "windows-amd64", "darwin-amd64", "darwin-arm64":
		return true
	default:
		return false
	}
}
