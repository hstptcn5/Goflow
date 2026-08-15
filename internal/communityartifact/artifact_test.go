package communityartifact

import (
	"archive/zip"
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const testCommit = "0123456789abcdef0123456789abcdef01234567"

func TestBuildIsDeterministicAndVerifiable(t *testing.T) {
	runtimePath, licensePath := fixtureFiles(t, "linux-amd64")
	build := func(dir string) *BuildResult {
		result, err := Build(BuildOptions{RuntimePath: runtimePath, LicensePath: licensePath, OutputDir: dir, Commit: testCommit, Target: "linux-amd64"})
		if err != nil {
			t.Fatal(err)
		}
		return result
	}
	first, second := build(filepath.Join(t.TempDir(), "a")), build(filepath.Join(t.TempDir(), "b"))
	if first.SHA256 != second.SHA256 {
		t.Fatalf("non-deterministic archives: %s != %s", first.SHA256, second.SHA256)
	}
	if _, err := Verify(first.ArchivePath, VerifyOptions{Version: ReleaseVersion, Channel: ReleaseChannel, Commit: testCommit, Target: "linux-amd64"}); err != nil {
		t.Fatal(err)
	}
	checksum, err := os.ReadFile(first.ChecksumPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(checksum) != first.SHA256+"  "+filepath.Base(first.ArchivePath)+"\n" {
		t.Fatalf("unexpected checksum file: %q", checksum)
	}
}

func TestVerifyRejectsAdversarialArchives(t *testing.T) {
	runtimePath, licensePath := fixtureFiles(t, "linux-amd64")
	result, err := Build(BuildOptions{RuntimePath: runtimePath, LicensePath: licensePath, OutputDir: t.TempDir(), Commit: testCommit, Target: "linux-amd64"})
	if err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name   string
		mutate func([]zipItem) []zipItem
		want   string
	}{
		{name: "duplicate", mutate: func(items []zipItem) []zipItem { items[1] = items[0]; return items }, want: "duplicate ZIP entry"},
		{name: "traversal", mutate: func(items []zipItem) []zipItem { items[0].name = "../goflow"; return items }, want: "unsafe ZIP entry"},
		{name: "extra", mutate: func(items []zipItem) []zipItem {
			return append(items, zipItem{name: "SIGNATURE", data: []byte("false")})
		}, want: "entries; expected exactly 4"},
		{name: "symlink", mutate: func(items []zipItem) []zipItem { items[0].mode = os.ModeSymlink | 0777; return items }, want: "regular file"},
		{name: "wrong architecture content", mutate: func(items []zipItem) []zipItem {
			for i := range items {
				if items[i].name == "goflow" {
					binary.LittleEndian.PutUint16(items[i].data[18:20], 183)
				}
			}
			return items
		}, want: "member"},
		{name: "false signing metadata", mutate: func(items []zipItem) []zipItem {
			for i := range items {
				if items[i].name == "COMMUNITY_ARTIFACT.json" {
					items[i].data = bytes.Replace(items[i].data, []byte("\n}"), []byte(",\n  \"signed\": true\n}"), 1)
				}
			}
			return items
		}, want: "unknown field"},
		{name: "modified README", mutate: func(items []zipItem) []zipItem {
			for i := range items {
				if items[i].name == "README.txt" {
					items[i].data = append(items[i].data, []byte("modified")...)
				}
			}
			return items
		}, want: "member"},
		{name: "modified LICENSE", mutate: func(items []zipItem) []zipItem {
			for i := range items {
				if items[i].name == "LICENSE" {
					items[i].data = append(items[i].data, []byte("modified")...)
				}
			}
			return items
		}, want: "member"},
		{name: "local path leakage with updated inventory", mutate: memberMutation("README.txt", []byte(`C:\Users\builder\repo`)), want: "local build path"},
		{name: "duplicate inventory", mutate: metadataMutation(func(m *Metadata) { m.Files = append(m.Files, m.Files[0]) }), want: "duplicate path"},
		{name: "missing inventory", mutate: metadataMutation(func(m *Metadata) { m.Files = m.Files[:len(m.Files)-1] }), want: "exactly runtime"},
		{name: "extra inventory", mutate: metadataMutation(func(m *Metadata) {
			m.Files = append(m.Files, MemberInfo{Path: "extra.txt", SHA256: strings.Repeat("0", 64)})
		}), want: "unexpected path"},
		{name: "wrong inventory size", mutate: metadataMutation(func(m *Metadata) { m.Files[0].Size++ }), want: "hash or size"},
		{name: "wrong inventory hash", mutate: metadataMutation(func(m *Metadata) { m.Files[0].SHA256 = strings.Repeat("0", 64) }), want: "hash or size"},
		{name: "metadata self inventory", mutate: metadataMutation(func(m *Metadata) {
			m.Files = append(m.Files, MemberInfo{Path: "COMMUNITY_ARTIFACT.json", SHA256: strings.Repeat("0", 64)})
		}), want: "must not inventory itself"},
		{name: "noncanonical inventory order", mutate: metadataMutation(func(m *Metadata) { m.Files[0], m.Files[1] = m.Files[1], m.Files[0] }), want: "canonical sorted order"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := rewriteZip(t, result.ArchivePath, tt.mutate)
			_, err := Verify(path, VerifyOptions{})
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("error=%v want containing %q", err, tt.want)
			}
		})
	}
}

func memberMutation(name string, suffix []byte) func([]zipItem) []zipItem {
	return func(items []zipItem) []zipItem {
		for i := range items {
			if items[i].name == name {
				items[i].data = append(items[i].data, suffix...)
			}
		}
		return metadataMutation(func(m *Metadata) {
			for i := range m.Files {
				if m.Files[i].Path != name {
					continue
				}
				for _, item := range items {
					if item.name == name {
						sum := sha256.Sum256(item.data)
						m.Files[i].Size = int64(len(item.data))
						m.Files[i].SHA256 = hex.EncodeToString(sum[:])
					}
				}
			}
		})(items)
	}
}

func metadataMutation(mutate func(*Metadata)) func([]zipItem) []zipItem {
	return func(items []zipItem) []zipItem {
		for i := range items {
			if items[i].name != "COMMUNITY_ARTIFACT.json" {
				continue
			}
			var metadata Metadata
			if err := json.Unmarshal(items[i].data, &metadata); err != nil {
				panic(err)
			}
			mutate(&metadata)
			data, err := json.MarshalIndent(metadata, "", "  ")
			if err != nil {
				panic(err)
			}
			items[i].data = append(data, '\n')
		}
		return items
	}
}

func TestExecutableInspectionRejectsRenamedOrWrongArchitecture(t *testing.T) {
	if err := verifyExecutableTarget([]byte("#!/bin/sh\n"), "linux-amd64"); err == nil {
		t.Fatal("renamed script was accepted as an executable")
	}
	path, _ := fixtureFiles(t, "linux-amd64")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := verifyExecutableTarget(data, "linux-arm64"); err == nil || !strings.Contains(err.Error(), "architecture") {
		t.Fatalf("wrong architecture error=%v", err)
	}
}

func TestBuildRejectsLocalPathLeakage(t *testing.T) {
	runtimePath, licensePath := fixtureFiles(t, "linux-amd64")
	f, err := os.OpenFile(runtimePath, os.O_APPEND|os.O_WRONLY, 0600)
	if err != nil {
		t.Fatal(err)
	}
	_, _ = f.WriteString(`C:\Users\builder\repo`)
	_ = f.Close()
	_, err = Build(BuildOptions{RuntimePath: runtimePath, LicensePath: licensePath, OutputDir: t.TempDir(), Commit: testCommit, Target: "linux-amd64"})
	if err == nil || !strings.Contains(err.Error(), "local build path") {
		t.Fatalf("error=%v", err)
	}
}

type zipItem struct {
	name string
	mode os.FileMode
	data []byte
}

func rewriteZip(t *testing.T, source string, mutate func([]zipItem) []zipItem) string {
	t.Helper()
	r, err := zip.OpenReader(source)
	if err != nil {
		t.Fatal(err)
	}
	items := make([]zipItem, 0, len(r.File))
	for _, f := range r.File {
		rc, e := f.Open()
		if e != nil {
			t.Fatal(e)
		}
		data, e := io.ReadAll(rc)
		rc.Close()
		if e != nil {
			t.Fatal(e)
		}
		items = append(items, zipItem{name: f.Name, mode: f.Mode(), data: data})
	}
	r.Close()
	items = mutate(items)
	path := filepath.Join(t.TempDir(), "mutated.zip")
	out, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	zw := zip.NewWriter(out)
	for _, item := range items {
		h := &zip.FileHeader{Name: item.name, Method: zip.Deflate, Modified: zipTimestamp}
		h.SetMode(item.mode)
		w, e := zw.CreateHeader(h)
		if e != nil {
			t.Fatal(e)
		}
		if _, e = w.Write(item.data); e != nil {
			t.Fatal(e)
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := out.Close(); err != nil {
		t.Fatal(err)
	}
	return path
}

func fixtureFiles(t *testing.T, target string) (string, string) {
	t.Helper()
	dir := t.TempDir()
	runtimePath := filepath.Join(dir, runtimeName(target))
	data := make([]byte, 128)
	switch {
	case strings.HasPrefix(target, "linux-"):
		copy(data, []byte{0x7f, 'E', 'L', 'F', 2, 1})
		machine := uint16(62)
		if strings.HasSuffix(target, "arm64") {
			machine = 183
		}
		binary.LittleEndian.PutUint16(data[18:20], machine)
	case strings.HasPrefix(target, "windows-"):
		copy(data, []byte("MZ"))
		binary.LittleEndian.PutUint32(data[0x3c:0x40], 64)
		copy(data[64:], []byte("PE\x00\x00"))
		binary.LittleEndian.PutUint16(data[68:70], 0x8664)
	}
	if err := os.WriteFile(runtimePath, data, 0755); err != nil {
		t.Fatal(err)
	}
	licensePath := filepath.Join(dir, "LICENSE")
	if err := os.WriteFile(licensePath, []byte("test license\n"), 0644); err != nil {
		t.Fatal(err)
	}
	return runtimePath, licensePath
}

func TestFixtureHashSanity(t *testing.T) {
	runtimePath, _ := fixtureFiles(t, "linux-amd64")
	data, _ := os.ReadFile(runtimePath)
	sum := sha256.Sum256(data)
	if len(hex.EncodeToString(sum[:])) != 64 {
		t.Fatal("bad digest")
	}
}
