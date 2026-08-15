package communityartifact

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestVerifyChecksumFile(t *testing.T) {
	dir := t.TempDir()
	archive := filepath.Join(dir, "goflow-community-1.0.0-rc.1-linux-amd64.zip")
	data := []byte("archive bytes")
	if err := os.WriteFile(archive, data, 0600); err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(data)
	digest := hex.EncodeToString(sum[:])
	tests := []struct {
		name, content string
		oversized     bool
		wantOK        bool
	}{
		{name: "correct", content: digest + "  " + filepath.Base(archive) + "\n", wantOK: true},
		{name: "wrong digest", content: strings.Repeat("0", 64) + "  " + filepath.Base(archive) + "\n"},
		{name: "wrong filename", content: digest + "  other.zip\n"},
		{name: "absolute filename", content: digest + "  /tmp/archive.zip\n"},
		{name: "traversal filename", content: digest + "  ../archive.zip\n"},
		{name: "uppercase digest", content: strings.ToUpper(digest) + "  " + filepath.Base(archive) + "\n"},
		{name: "malformed digest", content: "not-a-digest  " + filepath.Base(archive) + "\n"},
		{name: "multiple lines", content: digest + "  " + filepath.Base(archive) + "\n" + digest + "  " + filepath.Base(archive) + "\n"},
		{name: "trailing content", content: digest + "  " + filepath.Base(archive) + "\nextra"},
		{name: "oversized", oversized: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			checksum := filepath.Join(t.TempDir(), "archive.zip.sha256")
			content := tt.content
			if tt.oversized {
				content = strings.Repeat("x", MaxChecksumBytes+1)
			}
			if err := os.WriteFile(checksum, []byte(content), 0600); err != nil {
				t.Fatal(err)
			}
			err := VerifyChecksumFile(archive, checksum)
			if tt.wantOK && err != nil {
				t.Fatal(err)
			}
			if !tt.wantOK && err == nil {
				t.Fatal("invalid checksum was accepted")
			}
		})
	}
}

func TestBuildRejectsExistingArchiveOrChecksum(t *testing.T) {
	for _, existing := range []string{"archive", "checksum"} {
		t.Run(existing, func(t *testing.T) {
			runtimePath, licensePath := fixtureFiles(t, "linux-amd64")
			output := t.TempDir()
			archive := filepath.Join(output, "goflow-community-"+ReleaseVersion+"-linux-amd64.zip")
			path := archive
			if existing == "checksum" {
				path += ".sha256"
			}
			if err := os.WriteFile(path, []byte("existing"), 0600); err != nil {
				t.Fatal(err)
			}
			if _, err := Build(BuildOptions{RuntimePath: runtimePath, LicensePath: licensePath, OutputDir: output, Commit: testCommit, Target: "linux-amd64"}); err == nil {
				t.Fatal("existing destination was overwritten")
			}
		})
	}
}
