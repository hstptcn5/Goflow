package pack

import (
	"archive/zip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"testing"
)

func TestBuildBundleValidZipLayoutAndContents(t *testing.T) {
	dir := writeBuildPack(t, func(m *Manifest) {
		m.Plugins = []string{"plugins/plugin.txt"}
		m.Assets = []string{"assets/sample.txt"}
	})
	writeFile(t, filepath.Join(dir, "plugins", "plugin.txt"), "plugin payload")
	writeFile(t, filepath.Join(dir, "assets", "sample.txt"), "asset payload")
	writeFile(t, filepath.Join(dir, ".env"), "SECRET=value")
	writeFile(t, filepath.Join(dir, "goflow.db"), "db")
	writeFile(t, filepath.Join(dir, "goflow.master.key"), "key")
	writeFile(t, filepath.Join(dir, "notes.txt"), "notes")
	writeFile(t, filepath.Join(dir, "plugins", "unlisted.txt"), "unlisted plugin")
	writeFile(t, filepath.Join(dir, "assets", "unlisted.txt"), "unlisted asset")
	runtimePath := writeRuntimeFixture(t, "runtime payload")
	outputDir := t.TempDir()

	result, err := Build(BuildOptions{
		PackDir:     dir,
		OutputDir:   outputDir,
		RuntimePath: runtimePath,
	})
	if err != nil {
		t.Fatalf("Build failed: %v", err)
	}

	members := zipMembers(t, result.ArchivePath)
	want := []string{
		runtimeEntry(CurrentPlatform()),
		"PACK_INFO.json",
		"README.txt",
		"pack/assets/sample.txt",
		"pack/pack.json",
		"pack/plugins/plugin.txt",
		"pack/workflows/main.json",
	}
	sort.Strings(want)
	if strings.Join(members, "\n") != strings.Join(want, "\n") {
		t.Fatalf("unexpected members:\ngot:\n%s\nwant:\n%s", strings.Join(members, "\n"), strings.Join(want, "\n"))
	}
	assertZipEntry(t, result.ArchivePath, runtimeEntry(CurrentPlatform()), "runtime payload")
	assertZipEntry(t, result.ArchivePath, "pack/workflows/main.json", validWorkflowJSON())
	assertZipEntry(t, result.ArchivePath, "pack/plugins/plugin.txt", "plugin payload")
	assertZipEntry(t, result.ArchivePath, "pack/assets/sample.txt", "asset payload")
	for _, unexpected := range []string{"pack/.env", "pack/goflow.db", "pack/goflow.master.key", "pack/notes.txt", "pack/plugins/unlisted.txt", "pack/assets/unlisted.txt"} {
		if hasZipMember(members, unexpected) {
			t.Fatalf("unexpected archive member %s", unexpected)
		}
	}
	info := readPackInfo(t, result.ArchivePath)
	if info.RuntimeEntry != runtimeEntry(CurrentPlatform()) || info.EntryWorkflow != "pack/workflows/main.json" {
		t.Fatalf("unexpected PACK_INFO: %#v", info)
	}
	if !inventoryHasPath(info.Files, runtimeEntry(CurrentPlatform())) {
		t.Fatalf("runtime missing from inventory: %#v", info.Files)
	}
	if inventoryHasPath(info.Files, "PACK_INFO.json") {
		t.Fatalf("PACK_INFO must not be self-inventoried")
	}
}

func TestBuildDereferencesInternalSymlinkAsRegularZipFile(t *testing.T) {
	dir := writeBuildPack(t, func(m *Manifest) {
		m.Assets = []string{"assets/link.txt"}
	})
	target := filepath.Join(dir, "assets", "target.txt")
	writeFile(t, target, "inside target")
	if err := os.Symlink(target, filepath.Join(dir, "assets", "link.txt")); err != nil {
		t.Skipf("symlink not supported: %v", err)
	}
	result := mustBuild(t, dir, writeRuntimeFixture(t, "runtime"), t.TempDir())
	assertZipEntry(t, result.ArchivePath, "pack/assets/link.txt", "inside target")
	reader, err := zip.OpenReader(result.ArchivePath)
	if err != nil {
		t.Fatalf("open zip: %v", err)
	}
	defer reader.Close()
	for _, file := range reader.File {
		if file.Name == "pack/assets/link.txt" && file.FileInfo().Mode()&os.ModeSymlink != 0 {
			t.Fatalf("asset symlink was stored as symlink object")
		}
	}
}

func TestBuildRejectsUnsupportedManifestTarget(t *testing.T) {
	unsupported := "linux-amd64"
	if CurrentPlatform() == unsupported {
		unsupported = "windows-amd64"
	}
	dir := writeValidPack(t, func(m *Manifest) {
		m.SupportedPlatforms = []string{unsupported}
	})
	_, err := Build(BuildOptions{PackDir: dir, OutputDir: t.TempDir(), RuntimePath: writeRuntimeFixture(t, "runtime")})
	if err == nil || !strings.Contains(err.Error(), "supported_platforms") {
		t.Fatalf("expected supported_platforms error, got %v", err)
	}
}

func TestBuildRejectsTargetDifferentFromRuntimePlatform(t *testing.T) {
	dir := writeBuildPack(t)
	_, err := Build(BuildOptions{
		PackDir:       dir,
		OutputDir:     t.TempDir(),
		Target:        "linux-amd64",
		RuntimeGOOS:   "windows",
		RuntimeGOARCH: "amd64",
		RuntimePath:   writeRuntimeFixture(t, "runtime"),
	})
	if err == nil || !strings.Contains(err.Error(), "cross-target") {
		t.Fatalf("expected cross-target error, got %v", err)
	}
}

func TestBuildRejectsMissingOrDirectoryRuntime(t *testing.T) {
	dir := writeBuildPack(t)
	_, err := Build(BuildOptions{PackDir: dir, OutputDir: t.TempDir(), RuntimePath: filepath.Join(t.TempDir(), "missing")})
	if err == nil || !strings.Contains(err.Error(), "runtime") {
		t.Fatalf("expected missing runtime error, got %v", err)
	}
	_, err = Build(BuildOptions{PackDir: dir, OutputDir: t.TempDir(), RuntimePath: t.TempDir()})
	if err == nil || !strings.Contains(err.Error(), "regular file") {
		t.Fatalf("expected runtime directory error, got %v", err)
	}
}

func TestBuildExistingOutputRequiresForceAndForceReplaces(t *testing.T) {
	dir := writeBuildPack(t)
	runtimePath := writeRuntimeFixture(t, "runtime one")
	outputDir := t.TempDir()
	first := mustBuild(t, dir, runtimePath, outputDir)
	originalHash := fileSHA256(t, first.ArchivePath)
	_, err := Build(BuildOptions{PackDir: dir, OutputDir: outputDir, RuntimePath: runtimePath})
	if err == nil || !strings.Contains(err.Error(), "already exists") {
		t.Fatalf("expected existing archive error, got %v", err)
	}
	writeFile(t, runtimePath, "runtime two")
	second, err := Build(BuildOptions{PackDir: dir, OutputDir: outputDir, RuntimePath: runtimePath, Force: true})
	if err != nil {
		t.Fatalf("force build failed: %v", err)
	}
	if second.ArchivePath != first.ArchivePath {
		t.Fatalf("force replaced wrong archive: got %s want %s", second.ArchivePath, first.ArchivePath)
	}
	if got := fileSHA256(t, second.ArchivePath); got == originalHash {
		t.Fatalf("--force did not replace archive content")
	}
}

func TestBuildFailureDoesNotLeaveTemporaryFile(t *testing.T) {
	dir := writeBuildPack(t, func(m *Manifest) {
		m.Assets = []string{"assets/big.bin"}
	})
	big := filepath.Join(dir, "assets", "big.bin")
	writeFile(t, big, "")
	if err := os.Truncate(big, MaxPackResourceBytes+1); err != nil {
		t.Fatalf("truncate big asset: %v", err)
	}
	outputDir := t.TempDir()
	_, err := Build(BuildOptions{PackDir: dir, OutputDir: outputDir, RuntimePath: writeRuntimeFixture(t, "runtime")})
	if err == nil {
		t.Fatalf("expected build failure")
	}
	entries, err := os.ReadDir(outputDir)
	if err != nil {
		t.Fatalf("read output dir: %v", err)
	}
	if len(entries) != 0 {
		t.Fatalf("expected no temp output files, got %v", entries)
	}
}

func TestBuildRejectsDuplicateArchivePath(t *testing.T) {
	dir := writeBuildPack(t, func(m *Manifest) {
		m.Assets = []string{"workflows/main.json"}
	})
	_, err := Build(BuildOptions{PackDir: dir, OutputDir: t.TempDir(), RuntimePath: writeRuntimeFixture(t, "runtime")})
	if err == nil || !strings.Contains(err.Error(), "duplicate archive path") {
		t.Fatalf("expected duplicate archive path error, got %v", err)
	}
}

func TestBuildRejectsWindowsCaseInsensitiveCollision(t *testing.T) {
	dir := writeValidPack(t, func(m *Manifest) {
		m.SupportedPlatforms = []string{"windows-amd64"}
		m.Plugins = []string{"plugins/tool.txt"}
		m.Assets = []string{"plugins/TOOL.txt"}
	})
	writeFile(t, filepath.Join(dir, "plugins", "tool.txt"), "lower")
	if runtime.GOOS != "windows" {
		writeFile(t, filepath.Join(dir, "plugins", "TOOL.txt"), "upper")
	}
	_, err := Build(BuildOptions{
		PackDir:       dir,
		OutputDir:     t.TempDir(),
		Target:        "windows-amd64",
		RuntimeGOOS:   "windows",
		RuntimeGOARCH: "amd64",
		RuntimePath:   writeRuntimeFixture(t, "runtime"),
	})
	if err == nil || !strings.Contains(err.Error(), "collides") {
		t.Fatalf("expected case-insensitive collision, got %v", err)
	}
}

func TestBuildRejectsWindowsUnsafePathSegmentsDuringValidation(t *testing.T) {
	tests := []string{"assets/bad:name.txt", "assets/trailingdot.", "assets/trailingspace ", "assets/CON.txt", "assets/LPT1.log"}
	for _, path := range tests {
		t.Run(path, func(t *testing.T) {
			dir := writeValidPack(t, func(m *Manifest) {
				m.Assets = []string{path}
			})
			_, err := Build(BuildOptions{PackDir: dir, OutputDir: t.TempDir(), RuntimePath: writeRuntimeFixture(t, "runtime")})
			if err == nil {
				t.Fatalf("expected unsafe path error")
			}
		})
	}
}

func TestBuildRejectsPerFileAndTotalPayloadLimits(t *testing.T) {
	t.Run("per file", func(t *testing.T) {
		dir := writeBuildPack(t, func(m *Manifest) {
			m.Assets = []string{"assets/big.bin"}
		})
		big := filepath.Join(dir, "assets", "big.bin")
		writeFile(t, big, "")
		if err := os.Truncate(big, MaxPackResourceBytes+1); err != nil {
			t.Fatalf("truncate big asset: %v", err)
		}
		_, err := Build(BuildOptions{PackDir: dir, OutputDir: t.TempDir(), RuntimePath: writeRuntimeFixture(t, "runtime")})
		if err == nil || !strings.Contains(err.Error(), "per-file limit") {
			t.Fatalf("expected per-file limit error, got %v", err)
		}
	})
	t.Run("total", func(t *testing.T) {
		assetPaths := make([]string, 0, 6)
		dir := writeBuildPack(t, func(m *Manifest) {
			for i := 0; i < 6; i++ {
				assetPaths = append(assetPaths, "assets/part"+string(rune('a'+i))+".bin")
			}
			m.Assets = assetPaths
		})
		for _, asset := range assetPaths {
			path := filepath.Join(dir, filepath.FromSlash(asset))
			writeFile(t, path, "")
			if err := os.Truncate(path, 90<<20); err != nil {
				t.Fatalf("truncate asset: %v", err)
			}
		}
		_, err := Build(BuildOptions{PackDir: dir, OutputDir: t.TempDir(), RuntimePath: writeRuntimeFixture(t, "runtime")})
		if err == nil || !strings.Contains(err.Error(), "total limit") {
			t.Fatalf("expected total limit error, got %v", err)
		}
	})
}

func TestBuildDeterministicOutputAndPackInfoHasNoLocalState(t *testing.T) {
	dir := writeBuildPack(t)
	runtimePath := writeRuntimeFixture(t, "runtime")
	outputA := t.TempDir()
	outputB := t.TempDir()
	a := mustBuild(t, dir, runtimePath, outputA)
	b := mustBuild(t, dir, runtimePath, outputB)
	hashA := fileSHA256(t, a.ArchivePath)
	hashB := fileSHA256(t, b.ArchivePath)
	if hashA != hashB {
		t.Fatalf("expected deterministic zip hash, got %s and %s", hashA, hashB)
	}
	raw := readZipEntry(t, a.ArchivePath, "PACK_INFO.json")
	if strings.Contains(raw, filepath.VolumeName(dir)) || strings.Contains(raw, dir) || strings.Contains(raw, runtimePath) {
		t.Fatalf("PACK_INFO contains local path: %s", raw)
	}
	if strings.Contains(raw, "20") && strings.Contains(raw, "T") {
		t.Fatalf("PACK_INFO appears to contain timestamp: %s", raw)
	}
}

func TestBuildRegressionSemVerHyphenAndDoubleHyphenID(t *testing.T) {
	for _, version := range []string{"1.0.0-alpha-beta", "1.0.0-alpha-beta.1", "1.0.0-x-y-z+build.1"} {
		t.Run("semver "+version, func(t *testing.T) {
			dir := writeBuildPack(t, func(m *Manifest) {
				m.Version = version
			})
			if _, err := Build(BuildOptions{PackDir: dir, OutputDir: t.TempDir(), RuntimePath: writeRuntimeFixture(t, "runtime")}); err != nil {
				t.Fatalf("expected SemVer %q to build, got %v", version, err)
			}
		})
	}
	for _, version := range []string{"1.0.0-", "1.0.0-alpha..1", "1.0.0-01", "1.0.0-alpha.01"} {
		t.Run("invalid semver "+version, func(t *testing.T) {
			dir := writeBuildPack(t, func(m *Manifest) {
				m.Version = version
			})
			assertLoadError(t, dir, "SemVer")
		})
	}
	dir := writeBuildPack(t, func(m *Manifest) {
		m.ID = "example--pack"
	})
	assertLoadError(t, dir, "id")
}

func writeBuildPack(t *testing.T, edits ...func(*Manifest)) string {
	t.Helper()
	edits = append([]func(*Manifest){func(m *Manifest) {
		m.SupportedPlatforms = []string{CurrentPlatform()}
	}}, edits...)
	return writeValidPack(t, edits...)
}

func writeRuntimeFixture(t *testing.T, data string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), runtimeEntry(CurrentPlatform()))
	writeFile(t, path, data)
	return path
}

func mustBuild(t *testing.T, dir, runtimePath, outputDir string) *BuildResult {
	t.Helper()
	result, err := Build(BuildOptions{PackDir: dir, OutputDir: outputDir, RuntimePath: runtimePath})
	if err != nil {
		t.Fatalf("Build failed: %v", err)
	}
	return result
}

func zipMembers(t *testing.T, path string) []string {
	t.Helper()
	reader, err := zip.OpenReader(path)
	if err != nil {
		t.Fatalf("open zip: %v", err)
	}
	defer reader.Close()
	names := make([]string, 0, len(reader.File))
	for _, file := range reader.File {
		names = append(names, file.Name)
	}
	sort.Strings(names)
	return names
}

func hasZipMember(members []string, name string) bool {
	for _, member := range members {
		if member == name {
			return true
		}
	}
	return false
}

func assertZipEntry(t *testing.T, archivePath, entry, want string) {
	t.Helper()
	if got := readZipEntry(t, archivePath, entry); got != want {
		t.Fatalf("unexpected %s contents: got %q want %q", entry, got, want)
	}
}

func readZipEntry(t *testing.T, archivePath, entry string) string {
	t.Helper()
	reader, err := zip.OpenReader(archivePath)
	if err != nil {
		t.Fatalf("open zip: %v", err)
	}
	defer reader.Close()
	for _, file := range reader.File {
		if file.Name != entry {
			continue
		}
		rc, err := file.Open()
		if err != nil {
			t.Fatalf("open zip entry %s: %v", entry, err)
		}
		defer rc.Close()
		data, err := io.ReadAll(rc)
		if err != nil {
			t.Fatalf("read zip entry %s: %v", entry, err)
		}
		return string(data)
	}
	t.Fatalf("missing zip entry %s", entry)
	return ""
}

func readPackInfo(t *testing.T, archivePath string) PackInfo {
	t.Helper()
	var info PackInfo
	if err := json.Unmarshal([]byte(readZipEntry(t, archivePath, "PACK_INFO.json")), &info); err != nil {
		t.Fatalf("parse PACK_INFO: %v", err)
	}
	return info
}

func inventoryHasPath(files []PackInfoFile, path string) bool {
	for _, file := range files {
		if file.Path == path {
			return true
		}
	}
	return false
}

func fileSHA256(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}
