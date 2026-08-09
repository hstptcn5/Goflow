package pack

import (
	"archive/zip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
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
	if err := VerifyBundleArchive(result.ArchivePath, buildLimits{}); err != nil {
		t.Fatalf("VerifyBundleArchive failed: %v", err)
	}
	assertPackInfoMatchesZipEntries(t, result.ArchivePath)
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

func TestBuildUsesResolvedSourcePathForInternalSymlink(t *testing.T) {
	dir := writeBuildPack(t, func(m *Manifest) {
		m.Assets = []string{"assets/link.txt"}
	})
	target := filepath.Join(dir, "assets", "target.txt")
	writeFile(t, target, "inside")
	if err := os.Symlink(target, filepath.Join(dir, "assets", "link.txt")); err != nil {
		t.Skipf("symlink not supported: %v", err)
	}
	resolved, err := resolveExistingRegularInside(dir, "assets/link.txt")
	if err != nil {
		t.Fatalf("resolve symlink: %v", err)
	}
	targetResolved, err := filepath.EvalSymlinks(target)
	if err != nil {
		t.Fatalf("resolve target: %v", err)
	}
	if resolved != targetResolved {
		t.Fatalf("expected resolved source path %s, got %s", targetResolved, resolved)
	}
}

func TestBuildPreservesExecutableIntentInZipModes(t *testing.T) {
	if runtime.GOOS == "windows" {
		archivePath := filepath.Join(t.TempDir(), "modes.zip")
		file, err := os.Create(archivePath)
		if err != nil {
			t.Fatalf("create mode zip: %v", err)
		}
		err = writeZip(file, []archiveFile{
			{archivePath: runtimeEntry(CurrentPlatform()), generated: []byte("runtime"), mode: 0755},
			{archivePath: "pack/plugins/exec.sh", generated: []byte("exec"), mode: 0755},
			{archivePath: "pack/plugins/plain.sh", generated: []byte("plain"), mode: 0600},
			{archivePath: "pack/assets/sample.txt", generated: []byte("asset"), mode: 0600},
			{archivePath: "pack/workflows/main.json", generated: []byte("{}"), mode: 0600},
			{archivePath: "pack/pack.json", generated: []byte("{}"), mode: 0600},
			{archivePath: "README.txt", generated: []byte("readme"), mode: 0600},
			{archivePath: "PACK_INFO.json", generated: []byte("{}"), mode: 0600},
		})
		if closeErr := file.Close(); closeErr != nil && err == nil {
			err = closeErr
		}
		if err != nil {
			t.Fatalf("write mode zip: %v", err)
		}
		assertZipModePolicy(t, archivePath)
		return
	}
	dir := writeBuildPack(t, func(m *Manifest) {
		m.Plugins = []string{"plugins/exec.sh", "plugins/plain.sh"}
		m.Assets = []string{"assets/sample.txt"}
	})
	execPlugin := filepath.Join(dir, "plugins", "exec.sh")
	plainPlugin := filepath.Join(dir, "plugins", "plain.sh")
	writeFile(t, execPlugin, "exec")
	writeFile(t, plainPlugin, "plain")
	writeFile(t, filepath.Join(dir, "assets", "sample.txt"), "asset")
	if err := os.Chmod(execPlugin, 0755); err != nil {
		t.Fatalf("chmod exec plugin: %v", err)
	}
	if err := os.Chmod(plainPlugin, 0600); err != nil {
		t.Fatalf("chmod plain plugin: %v", err)
	}
	result := mustBuild(t, dir, writeRuntimeFixture(t, "runtime"), t.TempDir())
	assertZipModePolicy(t, result.ArchivePath)
}

func assertZipModePolicy(t *testing.T, archivePath string) {
	t.Helper()
	modes := zipEntryModes(t, archivePath)
	if modes[runtimeEntry(CurrentPlatform())]&0111 == 0 {
		t.Fatalf("runtime is not executable: %v", modes[runtimeEntry(CurrentPlatform())])
	}
	if modes["pack/plugins/exec.sh"]&0111 == 0 {
		t.Fatalf("executable plugin lost execute bit: %v", modes["pack/plugins/exec.sh"])
	}
	if modes["pack/plugins/plain.sh"]&0111 != 0 {
		t.Fatalf("plain plugin became executable: %v", modes["pack/plugins/plain.sh"])
	}
	for _, path := range []string{"pack/assets/sample.txt", "pack/workflows/main.json", "pack/pack.json", "README.txt", "PACK_INFO.json"} {
		if modes[path]&0111 != 0 {
			t.Fatalf("%s unexpectedly executable: %v", path, modes[path])
		}
	}
	for path, mode := range modes {
		if mode&(os.ModeSetuid|os.ModeSetgid|os.ModeSticky) != 0 {
			t.Fatalf("%s has special permission bits: %v", path, mode)
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
	entries, err := os.ReadDir(outputDir)
	if err != nil {
		t.Fatalf("read output dir: %v", err)
	}
	if len(entries) != 1 || entries[0].Name() != second.ArchiveName {
		t.Fatalf("force build left backup/temp files: %v", entries)
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

func TestBuildVerificationFailureDoesNotPublishOrLeaveTemporaryFile(t *testing.T) {
	dir := writeBuildPack(t)
	outputDir := t.TempDir()
	finalPath := filepath.Join(outputDir, "example.hello-webhook-0.1.0-"+CurrentPlatform()+".zip")
	writeFile(t, finalPath, "old archive")
	_, err := Build(BuildOptions{
		PackDir:     dir,
		OutputDir:   outputDir,
		RuntimePath: writeRuntimeFixture(t, "runtime"),
		Force:       true,
		corruptTempBeforeVerify: func(path string) error {
			return os.WriteFile(path, []byte("not a zip"), 0600)
		},
	})
	if err == nil || !strings.Contains(err.Error(), "verify temporary archive") {
		t.Fatalf("expected verification failure, got %v", err)
	}
	if got := string(mustReadFile(t, finalPath)); got != "old archive" {
		t.Fatalf("old archive was changed: %q", got)
	}
	entries, err := os.ReadDir(outputDir)
	if err != nil {
		t.Fatalf("read output dir: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected only old archive, got %v", entries)
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
	if volume := filepath.VolumeName(dir); volume != "" && strings.Contains(raw, volume) {
		t.Fatalf("PACK_INFO contains local volume name: %s", raw)
	}
	if strings.Contains(raw, dir) || strings.Contains(raw, runtimePath) {
		t.Fatalf("PACK_INFO contains local path: %s", raw)
	}
	if strings.Contains(raw, "20") && strings.Contains(raw, "T") {
		t.Fatalf("PACK_INFO appears to contain timestamp: %s", raw)
	}
}

func TestVerifyBundleArchiveRejectsInvalidFixtures(t *testing.T) {
	validItem := zipFixtureItem{Name: "goflow.exe", Body: "runtime"}
	validInfo := PackInfo{
		SchemaVersion: SupportedSchema,
		PackID:        "example.hello-webhook",
		PackVersion:   "0.1.0",
		Target:        "windows-amd64",
		RuntimeEntry:  "goflow.exe",
		EntryWorkflow: "pack/workflows/main.json",
		Files:         []PackInfoFile{fixtureInventory(validItem)},
	}
	tests := []struct {
		name  string
		items []zipFixtureItem
		info  *PackInfo
		want  string
	}{
		{name: "bad hash", items: []zipFixtureItem{validItem}, info: &PackInfo{SchemaVersion: SupportedSchema, Files: []PackInfoFile{{Path: "goflow.exe", SHA256: "bad", Size: int64(len(validItem.Body))}}}, want: "does not match"},
		{name: "missing inventory entry", items: nil, info: &PackInfo{SchemaVersion: SupportedSchema, Files: []PackInfoFile{{Path: "missing.txt", SHA256: "bad", Size: 1}}}, want: "missing from ZIP"},
		{name: "extra entry", items: []zipFixtureItem{validItem, {Name: "extra.txt", Body: "extra"}}, info: &validInfo, want: "not listed"},
		{name: "duplicate entry", items: []zipFixtureItem{validItem, validItem}, info: &validInfo, want: "duplicate ZIP entry"},
		{name: "duplicate PACK_INFO", items: []zipFixtureItem{{Name: "PACK_INFO.json", Body: "{}"}}, info: &validInfo, want: "duplicate ZIP entry"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := writeZipFixture(t, tt.items, tt.info, "")
			err := VerifyBundleArchive(path, buildLimits{})
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("expected %q error, got %v", tt.want, err)
			}
		})
	}
	t.Run("missing PACK_INFO", func(t *testing.T) {
		path := writeZipFixture(t, []zipFixtureItem{validItem}, nil, "")
		err := VerifyBundleArchive(path, buildLimits{})
		if err == nil || !strings.Contains(err.Error(), "PACK_INFO.json is missing") {
			t.Fatalf("expected missing PACK_INFO error, got %v", err)
		}
	})
	t.Run("malformed PACK_INFO", func(t *testing.T) {
		path := writeZipFixture(t, []zipFixtureItem{validItem}, nil, "{")
		err := VerifyBundleArchive(path, buildLimits{})
		if err == nil || !strings.Contains(err.Error(), "malformed") {
			t.Fatalf("expected malformed PACK_INFO error, got %v", err)
		}
	})
}

func TestVerifyBundleArchiveRejectsActualSizeLimitViolation(t *testing.T) {
	tests := []struct {
		name       string
		item       zipFixtureItem
		info       PackInfo
		limits     buildLimits
		want       string
		wantVerify bool
	}{
		{
			name: "asset",
			item: zipFixtureItem{Name: "pack/assets/big.txt", Body: "123456"},
			info: PackInfo{SchemaVersion: SupportedSchema},
			limits: buildLimits{
				MaxResourceBytes: 3,
			},
			want:       "entry exceeds",
			wantVerify: true,
		},
		{
			name:       "manifest",
			item:       zipFixtureItem{Name: "pack/pack.json", Body: strings.Repeat("x", MaxManifestBytes+1)},
			info:       PackInfo{SchemaVersion: SupportedSchema},
			want:       "entry exceeds",
			wantVerify: true,
		},
		{
			name:       "workflow",
			item:       zipFixtureItem{Name: "pack/workflows/main.json", Body: strings.Repeat("x", MaxWorkflowBytes+1)},
			info:       PackInfo{SchemaVersion: SupportedSchema, EntryWorkflow: "pack/workflows/main.json"},
			want:       "entry exceeds",
			wantVerify: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.info.Files = []PackInfoFile{fixtureInventory(tt.item)}
			path := writeZipFixture(t, []zipFixtureItem{tt.item}, &tt.info, "")
			err := VerifyBundleArchive(path, tt.limits)
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("expected %q actual size limit error, got %v", tt.want, err)
			}
			if tt.wantVerify && !strings.Contains(err.Error(), "verify "+tt.item.Name) {
				t.Fatalf("expected streaming verifier error for %s, got %v", tt.item.Name, err)
			}
		})
	}
}

func TestPublishArchiveFallbackRollback(t *testing.T) {
	errPublish := errors.New("publish failed")
	ops := &scriptedPublishOps{
		renameErrors: []error{os.ErrExist, nil, errPublish, nil},
	}
	err := publishArchive("temp.zip", "final.zip", true, ops)
	if err == nil || !strings.Contains(err.Error(), "restored") {
		t.Fatalf("expected restored publish error, got %v", err)
	}
	wantRenames := []string{"temp.zip->final.zip", "final.zip->final.zip.backup-temp.zip", "temp.zip->final.zip", "final.zip.backup-temp.zip->final.zip"}
	if strings.Join(ops.renames, "|") != strings.Join(wantRenames, "|") {
		t.Fatalf("unexpected rename sequence: %#v", ops.renames)
	}
}

func TestPublishArchiveFallbackSuccessRemovesBackup(t *testing.T) {
	ops := &scriptedPublishOps{renameErrors: []error{os.ErrExist, nil, nil}}
	if err := publishArchive("temp.zip", "final.zip", true, ops); err != nil {
		t.Fatalf("publishArchive failed: %v", err)
	}
	if len(ops.removes) != 1 || ops.removes[0] != "final.zip.backup-temp.zip" {
		t.Fatalf("expected backup removal, got %#v", ops.removes)
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

func assertPackInfoMatchesZipEntries(t *testing.T, archivePath string) {
	t.Helper()
	info := readPackInfo(t, archivePath)
	for _, item := range info.Files {
		actual := zipEntryHashAndSize(t, archivePath, item.Path)
		if actual.SHA256 != item.SHA256 || actual.Size != item.Size {
			t.Fatalf("%s inventory mismatch: got %#v want %#v", item.Path, actual, item)
		}
	}
}

func zipEntryHashAndSize(t *testing.T, archivePath, entry string) PackInfoFile {
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
		actual, err := hashZipFileLimited(file, MaxRuntimeBytes)
		if err != nil {
			t.Fatalf("hash zip entry: %v", err)
		}
		return actual
	}
	t.Fatalf("missing entry %s", entry)
	return PackInfoFile{}
}

func zipEntryModes(t *testing.T, archivePath string) map[string]os.FileMode {
	t.Helper()
	reader, err := zip.OpenReader(archivePath)
	if err != nil {
		t.Fatalf("open zip: %v", err)
	}
	defer reader.Close()
	modes := map[string]os.FileMode{}
	for _, file := range reader.File {
		modes[file.Name] = file.FileInfo().Mode()
	}
	return modes
}

func mustReadFile(t *testing.T, path string) []byte {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return data
}

type zipFixtureItem struct {
	Name string
	Body string
}

func fixtureInventory(item zipFixtureItem) PackInfoFile {
	sum := sha256.Sum256([]byte(item.Body))
	return PackInfoFile{Path: item.Name, SHA256: hex.EncodeToString(sum[:]), Size: int64(len(item.Body))}
}

func writeZipFixture(t *testing.T, items []zipFixtureItem, info *PackInfo, rawInfo string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "fixture.zip")
	file, err := os.Create(path)
	if err != nil {
		t.Fatalf("create zip: %v", err)
	}
	zw := zip.NewWriter(file)
	for _, item := range items {
		writer, err := zw.Create(item.Name)
		if err != nil {
			t.Fatalf("create entry: %v", err)
		}
		if _, err := writer.Write([]byte(item.Body)); err != nil {
			t.Fatalf("write entry: %v", err)
		}
	}
	if info != nil || rawInfo != "" {
		writer, err := zw.Create("PACK_INFO.json")
		if err != nil {
			t.Fatalf("create PACK_INFO: %v", err)
		}
		data := []byte(rawInfo)
		if info != nil {
			data, err = json.Marshal(info)
			if err != nil {
				t.Fatalf("marshal PACK_INFO: %v", err)
			}
		}
		if _, err := writer.Write(data); err != nil {
			t.Fatalf("write PACK_INFO: %v", err)
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatalf("close zip: %v", err)
	}
	if err := file.Close(); err != nil {
		t.Fatalf("close file: %v", err)
	}
	return path
}

type scriptedPublishOps struct {
	renameErrors []error
	renames      []string
	removes      []string
}

func (ops *scriptedPublishOps) Rename(oldPath, newPath string) error {
	ops.renames = append(ops.renames, oldPath+"->"+newPath)
	if len(ops.renameErrors) == 0 {
		return nil
	}
	err := ops.renameErrors[0]
	ops.renameErrors = ops.renameErrors[1:]
	return err
}

func (ops *scriptedPublishOps) Remove(path string) error {
	ops.removes = append(ops.removes, path)
	return nil
}
