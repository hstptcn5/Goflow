package pack

import (
	"archive/zip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"time"
)

const (
	MaxPackResourceBytes = 100 << 20
	MaxPackPayloadBytes  = 512 << 20
	MaxRuntimeBytes      = 256 << 20
	MaxPackInfoBytes     = 1 << 20
	MaxBundleEntries     = 4096
)

var zipTimestamp = time.Date(1980, 1, 1, 0, 0, 0, 0, time.UTC)

type BuildOptions struct {
	PackDir       string
	OutputDir     string
	Target        string
	Force         bool
	RuntimePath   string
	RuntimeGOOS   string
	RuntimeGOARCH string

	limits                  buildLimits
	publishOps              publishOps
	corruptTempBeforeVerify func(string) error
}

type BuildResult struct {
	ArchivePath string
	ArchiveName string
	Target      string
}

type PackInfo struct {
	SchemaVersion        int            `json:"schema_version"`
	PackID               string         `json:"pack_id"`
	PackVersion          string         `json:"pack_version"`
	Target               string         `json:"target"`
	RuntimeEntry         string         `json:"runtime_entry"`
	EntryWorkflow        string         `json:"entry_workflow"`
	Files                []PackInfoFile `json:"files"`
	RequiredCapabilities []string       `json:"required_capabilities,omitempty"`
}

type PackInfoFile struct {
	Path   string `json:"path"`
	SHA256 string `json:"sha256"`
	Size   int64  `json:"size"`
}

type archiveFile struct {
	archivePath  string
	sourcePath   string
	generated    []byte
	payloadCount bool
	mode         os.FileMode
}

type buildLimits struct {
	MaxResourceBytes int64
	MaxPayloadBytes  int64
	MaxRuntimeBytes  int64
	MaxPackInfoBytes int64
	MaxEntries       int
}

func defaultBuildLimits() buildLimits {
	return buildLimits{
		MaxResourceBytes: MaxPackResourceBytes,
		MaxPayloadBytes:  MaxPackPayloadBytes,
		MaxRuntimeBytes:  MaxRuntimeBytes,
		MaxPackInfoBytes: MaxPackInfoBytes,
		MaxEntries:       MaxBundleEntries,
	}
}

func (limits buildLimits) withDefaults() buildLimits {
	defaults := defaultBuildLimits()
	if limits.MaxResourceBytes == 0 {
		limits.MaxResourceBytes = defaults.MaxResourceBytes
	}
	if limits.MaxPayloadBytes == 0 {
		limits.MaxPayloadBytes = defaults.MaxPayloadBytes
	}
	if limits.MaxRuntimeBytes == 0 {
		limits.MaxRuntimeBytes = defaults.MaxRuntimeBytes
	}
	if limits.MaxPackInfoBytes == 0 {
		limits.MaxPackInfoBytes = defaults.MaxPackInfoBytes
	}
	if limits.MaxEntries == 0 {
		limits.MaxEntries = defaults.MaxEntries
	}
	return limits
}

func Build(opts BuildOptions) (*BuildResult, error) {
	if strings.TrimSpace(opts.PackDir) == "" {
		return nil, fmt.Errorf("pack directory is required")
	}
	if strings.TrimSpace(opts.OutputDir) == "" {
		return nil, fmt.Errorf("output directory is required")
	}
	if opts.RuntimeGOOS == "" {
		opts.RuntimeGOOS = runtime.GOOS
	}
	if opts.RuntimeGOARCH == "" {
		opts.RuntimeGOARCH = runtime.GOARCH
	}
	hostTarget := Platform(opts.RuntimeGOOS, opts.RuntimeGOARCH)
	if opts.Target == "" {
		opts.Target = hostTarget
	}
	if opts.Target != hostTarget {
		return nil, fmt.Errorf("target %q does not match runtime platform %q; cross-target pack build is not supported", opts.Target, hostTarget)
	}
	loaded, err := Load(opts.PackDir)
	if err != nil {
		return nil, err
	}
	if !containsString(loaded.Manifest.SupportedPlatforms, opts.Target) {
		return nil, fmt.Errorf("target %q is not listed in supported_platforms", opts.Target)
	}
	if opts.RuntimePath == "" {
		opts.RuntimePath, err = os.Executable()
		if err != nil {
			return nil, fmt.Errorf("runtime: resolve current executable: %w", err)
		}
	}
	runtimeInfo, err := os.Stat(opts.RuntimePath)
	if err != nil {
		return nil, fmt.Errorf("runtime: %w", err)
	}
	if !runtimeInfo.Mode().IsRegular() {
		return nil, fmt.Errorf("runtime: source must be a regular file")
	}
	limits := opts.limits.withDefaults()
	if runtimeInfo.Size() > limits.MaxRuntimeBytes {
		return nil, fmt.Errorf("runtime: source exceeds %d byte limit", limits.MaxRuntimeBytes)
	}

	outputDir, err := filepath.Abs(opts.OutputDir)
	if err != nil {
		return nil, fmt.Errorf("output: resolve directory: %w", err)
	}
	if err := os.MkdirAll(outputDir, 0700); err != nil {
		return nil, fmt.Errorf("output: create directory: %w", err)
	}
	archiveName := fmt.Sprintf("%s-%s-%s.zip", loaded.Manifest.ID, loaded.Manifest.Version, opts.Target)
	archivePath := filepath.Join(outputDir, archiveName)
	if _, err := os.Stat(archivePath); err == nil && !opts.Force {
		return nil, fmt.Errorf("output archive already exists: %s", archivePath)
	} else if err != nil && !os.IsNotExist(err) {
		return nil, fmt.Errorf("output: check archive: %w", err)
	}

	files, err := buildArchiveFileList(loaded, opts.RuntimePath, runtimeEntry(opts.Target))
	if err != nil {
		return nil, err
	}
	if err := validateArchiveFiles(files, opts.Target, limits); err != nil {
		return nil, err
	}
	inventory, err := inventoryFiles(files)
	if err != nil {
		return nil, err
	}
	if err := validateInventorySizes(inventory, limits, "pack/"+loaded.Manifest.EntryWorkflow); err != nil {
		return nil, err
	}
	packInfo := PackInfo{
		SchemaVersion:        SupportedSchema,
		PackID:               loaded.Manifest.ID,
		PackVersion:          loaded.Manifest.Version,
		Target:               opts.Target,
		RuntimeEntry:         runtimeEntry(opts.Target),
		EntryWorkflow:        "pack/" + loaded.Manifest.EntryWorkflow,
		Files:                inventory,
		RequiredCapabilities: append([]string(nil), loaded.Manifest.RequiredCapabilities...),
	}
	packInfoData, err := json.MarshalIndent(packInfo, "", "  ")
	if err != nil {
		return nil, err
	}
	packInfoData = append(packInfoData, '\n')
	files = append(files, archiveFile{archivePath: "PACK_INFO.json", generated: packInfoData, mode: 0600})
	sort.Slice(files, func(i, j int) bool {
		return files[i].archivePath < files[j].archivePath
	})

	temp, err := os.CreateTemp(outputDir, "."+archiveName+".tmp-*")
	if err != nil {
		return nil, fmt.Errorf("output: create temporary archive: %w", err)
	}
	tempPath := temp.Name()
	defer func() {
		_ = os.Remove(tempPath)
	}()
	if err := writeZip(temp, files); err != nil {
		_ = temp.Close()
		return nil, err
	}
	if err := temp.Close(); err != nil {
		return nil, fmt.Errorf("output: close temporary archive: %w", err)
	}
	if opts.corruptTempBeforeVerify != nil {
		if err := opts.corruptTempBeforeVerify(tempPath); err != nil {
			return nil, fmt.Errorf("test hook: corrupt temporary archive: %w", err)
		}
	}
	if err := VerifyBundleArchive(tempPath, limits); err != nil {
		return nil, fmt.Errorf("output: verify temporary archive: %w", err)
	}
	ops := opts.publishOps
	if ops == nil {
		ops = osPublishOps{}
	}
	if err := publishArchive(tempPath, archivePath, opts.Force, ops); err != nil {
		return nil, err
	}
	return &BuildResult{ArchivePath: archivePath, ArchiveName: archiveName, Target: opts.Target}, nil
}

func Platform(goos, goarch string) string {
	return goos + "-" + goarch
}

func CurrentPlatform() string {
	return Platform(runtime.GOOS, runtime.GOARCH)
}

func runtimeEntry(target string) string {
	if strings.HasPrefix(target, "windows-") {
		return "goflow.exe"
	}
	return "goflow"
}

func buildArchiveFileList(loaded *Pack, runtimePath, runtimeEntry string) ([]archiveFile, error) {
	runtimeManifest, err := runtimeManifestData(loaded.ManifestPath)
	if err != nil {
		return nil, err
	}
	files := []archiveFile{
		{archivePath: runtimeEntry, sourcePath: runtimePath, mode: 0755},
		{archivePath: "pack/pack.json", generated: runtimeManifest, payloadCount: true, mode: 0600},
		{archivePath: "pack/" + loaded.Manifest.EntryWorkflow, sourcePath: loaded.EntryWorkflowPath, payloadCount: true, mode: 0600},
		{archivePath: "README.txt", generated: []byte(readmeText(runtimeEntry)), mode: 0600},
	}
	for _, logical := range loaded.Manifest.Plugins {
		sourcePath, err := resolveExistingRegularInside(loaded.Root, logical)
		if err != nil {
			return nil, fmt.Errorf("path: plugins entry %q: %w", logical, err)
		}
		files = append(files, archiveFile{archivePath: "pack/" + logical, sourcePath: sourcePath, payloadCount: true, mode: pluginArchiveMode(sourcePath)})
	}
	for _, logical := range loaded.Manifest.Assets {
		sourcePath, err := resolveExistingRegularInside(loaded.Root, logical)
		if err != nil {
			return nil, fmt.Errorf("path: assets entry %q: %w", logical, err)
		}
		files = append(files, archiveFile{archivePath: "pack/" + logical, sourcePath: sourcePath, payloadCount: true, mode: 0600})
	}
	return files, nil
}

func runtimeManifestData(path string) ([]byte, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("manifest: read runtime copy: %w", err)
	}
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("manifest: prepare runtime copy: %w", err)
	}
	delete(raw, "offline_test_fixture")
	runtimeData, err := json.MarshalIndent(raw, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("manifest: encode runtime copy: %w", err)
	}
	return append(runtimeData, '\n'), nil
}

func pluginArchiveMode(path string) os.FileMode {
	info, err := os.Stat(path)
	if err != nil || info.Mode().Perm()&0111 == 0 {
		return 0600
	}
	return 0755
}

func validateArchiveFiles(files []archiveFile, target string, limits buildLimits) error {
	seen := map[string]string{}
	totalPayload := int64(0)
	for _, file := range files {
		if err := validateArchivePath(file.archivePath, target); err != nil {
			return fmt.Errorf("archive path %q: %w", file.archivePath, err)
		}
		key := file.archivePath
		if strings.HasPrefix(target, "windows-") {
			key = strings.ToLower(key)
		}
		if prior, ok := seen[key]; ok {
			return fmt.Errorf("duplicate archive path %q collides with %q", file.archivePath, prior)
		}
		seen[key] = file.archivePath
		if file.sourcePath == "" {
			continue
		}
		info, err := os.Stat(file.sourcePath)
		if err != nil {
			return fmt.Errorf("stat %s: %w", file.archivePath, err)
		}
		if !info.Mode().IsRegular() {
			return fmt.Errorf("%s must be a regular file", file.archivePath)
		}
		if file.payloadCount {
			isResource := strings.HasPrefix(file.archivePath, "pack/plugins/") || strings.HasPrefix(file.archivePath, "pack/assets/")
			if isResource && info.Size() > limits.MaxResourceBytes {
				return fmt.Errorf("%s exceeds %d byte per-file limit", file.archivePath, limits.MaxResourceBytes)
			}
			totalPayload += info.Size()
			if totalPayload > limits.MaxPayloadBytes {
				return fmt.Errorf("pack payload exceeds %d byte total limit", limits.MaxPayloadBytes)
			}
		}
	}
	return nil
}

func validateInventorySizes(inventory []PackInfoFile, limits buildLimits, entryWorkflow string) error {
	totalPayload := int64(0)
	for _, file := range inventory {
		switch file.Path {
		case "pack/pack.json":
			if file.Size > MaxManifestBytes {
				return fmt.Errorf("%s exceeds %d byte manifest limit", file.Path, MaxManifestBytes)
			}
		case entryWorkflow:
			if file.Size > MaxWorkflowBytes {
				return fmt.Errorf("%s exceeds %d byte workflow limit", file.Path, MaxWorkflowBytes)
			}
		}
		isResource := strings.HasPrefix(file.Path, "pack/plugins/") || strings.HasPrefix(file.Path, "pack/assets/")
		if isResource && file.Size > limits.MaxResourceBytes {
			return fmt.Errorf("%s exceeds %d byte per-file limit", file.Path, limits.MaxResourceBytes)
		}
		if strings.HasPrefix(file.Path, "pack/") {
			totalPayload += file.Size
			if totalPayload > limits.MaxPayloadBytes {
				return fmt.Errorf("pack payload exceeds %d byte total limit", limits.MaxPayloadBytes)
			}
		}
	}
	return nil
}

func validateArchivePath(path, target string) error {
	if strings.TrimSpace(path) == "" {
		return fmt.Errorf("path is required")
	}
	if strings.Contains(path, `\`) || strings.HasPrefix(path, "/") {
		return fmt.Errorf("must be a relative slash path")
	}
	for _, segment := range strings.Split(path, "/") {
		if segment == "" || segment == "." || segment == ".." {
			return fmt.Errorf("must not contain empty, dot, or traversal segments")
		}
		if strings.HasPrefix(target, "windows-") {
			if strings.Contains(segment, ":") {
				return fmt.Errorf("Windows path segment must not contain colon")
			}
			if strings.HasSuffix(segment, ".") || strings.HasSuffix(segment, " ") {
				return fmt.Errorf("Windows path segment must not end with dot or space")
			}
			if isWindowsReservedName(segment) {
				return fmt.Errorf("Windows reserved path segment %q", segment)
			}
		}
	}
	return nil
}

func inventoryFiles(files []archiveFile) ([]PackInfoFile, error) {
	inventory := make([]PackInfoFile, 0, len(files))
	for _, file := range files {
		sum, size, err := fileHashAndSize(file)
		if err != nil {
			return nil, err
		}
		inventory = append(inventory, PackInfoFile{Path: file.archivePath, SHA256: sum, Size: size})
	}
	sort.Slice(inventory, func(i, j int) bool {
		return inventory[i].Path < inventory[j].Path
	})
	return inventory, nil
}

func fileHashAndSize(file archiveFile) (string, int64, error) {
	h := sha256.New()
	var size int64
	if file.generated != nil {
		if _, err := h.Write(file.generated); err != nil {
			return "", 0, err
		}
		size = int64(len(file.generated))
	} else {
		read, err := streamFile(file.sourcePath, h)
		if err != nil {
			return "", 0, fmt.Errorf("hash %s: %w", file.archivePath, err)
		}
		size = read
	}
	return hex.EncodeToString(h.Sum(nil)), size, nil
}

func writeZip(w io.Writer, files []archiveFile) error {
	zw := zip.NewWriter(w)
	for _, file := range files {
		header := &zip.FileHeader{
			Name:     file.archivePath,
			Method:   zip.Deflate,
			Modified: zipTimestamp,
		}
		header.SetMode(sanitizedArchiveMode(file.mode))
		writer, err := zw.CreateHeader(header)
		if err != nil {
			_ = zw.Close()
			return fmt.Errorf("zip: create %s: %w", file.archivePath, err)
		}
		if file.generated != nil {
			if _, err := writer.Write(file.generated); err != nil {
				_ = zw.Close()
				return fmt.Errorf("zip: write %s: %w", file.archivePath, err)
			}
			continue
		}
		if _, err := streamFile(file.sourcePath, writer); err != nil {
			_ = zw.Close()
			return fmt.Errorf("zip: write %s: %w", file.archivePath, err)
		}
	}
	if err := zw.Close(); err != nil {
		return fmt.Errorf("zip: close: %w", err)
	}
	return nil
}

func sanitizedArchiveMode(mode os.FileMode) os.FileMode {
	if mode&0111 != 0 {
		return 0755
	}
	return 0600
}

func VerifyBundleArchive(path string, limits buildLimits) error {
	limits = limits.withDefaults()
	reader, err := zip.OpenReader(path)
	if err != nil {
		return fmt.Errorf("open zip: %w", err)
	}
	defer reader.Close()
	if len(reader.File) > limits.MaxEntries {
		return fmt.Errorf("zip has %d entries, exceeds %d entry limit", len(reader.File), limits.MaxEntries)
	}
	seen := map[string]bool{}
	var packInfoFile *zip.File
	for _, file := range reader.File {
		if seen[file.Name] {
			return fmt.Errorf("duplicate ZIP entry %q", file.Name)
		}
		seen[file.Name] = true
		if file.Name == "PACK_INFO.json" {
			if packInfoFile != nil {
				return fmt.Errorf("PACK_INFO.json appears multiple times")
			}
			packInfoFile = file
		}
	}
	if packInfoFile == nil {
		return fmt.Errorf("PACK_INFO.json is missing")
	}
	packInfoData, _, err := readZipFileLimited(packInfoFile, limits.MaxPackInfoBytes)
	if err != nil {
		return fmt.Errorf("read PACK_INFO.json: %w", err)
	}
	var info PackInfo
	if err := json.Unmarshal(packInfoData, &info); err != nil {
		return fmt.Errorf("PACK_INFO.json is malformed: %w", err)
	}
	inventory := map[string]PackInfoFile{}
	for _, item := range info.Files {
		if item.Path == "" {
			return fmt.Errorf("PACK_INFO inventory contains empty path")
		}
		if _, ok := inventory[item.Path]; ok {
			return fmt.Errorf("PACK_INFO inventory contains duplicate path %q", item.Path)
		}
		inventory[item.Path] = item
	}
	for _, file := range reader.File {
		if file.Name == "PACK_INFO.json" {
			continue
		}
		item, ok := inventory[file.Name]
		if !ok {
			return fmt.Errorf("ZIP entry %q is not listed in PACK_INFO inventory", file.Name)
		}
		actual, err := hashZipFileLimited(file, verifyLimitForPath(file.Name, limits, info.EntryWorkflow))
		if err != nil {
			return fmt.Errorf("verify %s: %w", file.Name, err)
		}
		if actual.Size != item.Size || actual.SHA256 != item.SHA256 {
			return fmt.Errorf("ZIP entry %q does not match PACK_INFO inventory", file.Name)
		}
	}
	for path := range inventory {
		if !seen[path] {
			return fmt.Errorf("PACK_INFO inventory entry %q is missing from ZIP", path)
		}
	}
	return validateInventorySizes(sliceInventory(inventory), limits, info.EntryWorkflow)
}

func VerifyBundleArchiveFile(path string) error {
	return VerifyBundleArchive(path, buildLimits{})
}

func ReadBundleArchiveInfo(path string) (*PackInfo, error) {
	limits := defaultBuildLimits()
	reader, err := zip.OpenReader(path)
	if err != nil {
		return nil, fmt.Errorf("open zip: %w", err)
	}
	defer reader.Close()
	for _, file := range reader.File {
		if file.Name != "PACK_INFO.json" {
			continue
		}
		data, _, err := readZipFileLimited(file, limits.MaxPackInfoBytes)
		if err != nil {
			return nil, fmt.Errorf("read PACK_INFO.json: %w", err)
		}
		var info PackInfo
		if err := json.Unmarshal(data, &info); err != nil {
			return nil, fmt.Errorf("PACK_INFO.json is malformed: %w", err)
		}
		return &info, nil
	}
	return nil, fmt.Errorf("PACK_INFO.json is missing")
}

func VerifyExtractedBundle(root string) (*PackInfo, error) {
	limits := defaultBuildLimits()
	rootAbs, err := filepath.Abs(root)
	if err != nil {
		return nil, fmt.Errorf("resolve bundle directory: %w", err)
	}
	rootEval, err := filepath.EvalSymlinks(rootAbs)
	if err != nil {
		return nil, fmt.Errorf("resolve bundle directory symlinks: %w", err)
	}
	packInfoPath := filepath.Join(rootEval, "PACK_INFO.json")
	data, _, err := readFileLimited(packInfoPath, limits.MaxPackInfoBytes)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("PACK_INFO.json is missing: %w", err)
		}
		return nil, fmt.Errorf("read PACK_INFO.json: %w", err)
	}
	var info PackInfo
	if err := json.Unmarshal(data, &info); err != nil {
		return nil, fmt.Errorf("PACK_INFO.json is malformed: %w", err)
	}
	if info.SchemaVersion != SupportedSchema {
		return nil, fmt.Errorf("PACK_INFO schema_version must be %d", SupportedSchema)
	}
	if strings.TrimSpace(info.PackID) == "" || strings.TrimSpace(info.PackVersion) == "" || strings.TrimSpace(info.EntryWorkflow) == "" {
		return nil, fmt.Errorf("PACK_INFO is missing required pack metadata")
	}
	loaded, err := Load(filepath.Join(rootEval, "pack"))
	if err != nil {
		return nil, err
	}
	manifest := loaded.Manifest
	if info.PackID != manifest.ID {
		return nil, fmt.Errorf("PACK_INFO pack_id %q does not match pack.json id %q", info.PackID, manifest.ID)
	}
	if info.PackVersion != manifest.Version {
		return nil, fmt.Errorf("PACK_INFO pack_version %q does not match pack.json version %q", info.PackVersion, manifest.Version)
	}
	if !equalStringSlices(info.RequiredCapabilities, manifest.RequiredCapabilities) {
		return nil, fmt.Errorf("PACK_INFO required_capabilities do not match pack.json")
	}
	expectedEntryWorkflow := "pack/" + manifest.EntryWorkflow
	if info.EntryWorkflow != expectedEntryWorkflow {
		return nil, fmt.Errorf("PACK_INFO entry_workflow %q does not match pack.json entry_workflow %q", info.EntryWorkflow, expectedEntryWorkflow)
	}
	if !containsString(manifest.SupportedPlatforms, info.Target) {
		return nil, fmt.Errorf("PACK_INFO target %q is not listed in supported_platforms", info.Target)
	}
	expectedRuntime, err := expectedRuntimeEntry(info.Target)
	if err != nil {
		return nil, err
	}
	if info.RuntimeEntry != expectedRuntime {
		return nil, fmt.Errorf("PACK_INFO runtime_entry %q does not match target %q expected %q", info.RuntimeEntry, info.Target, expectedRuntime)
	}
	if len(info.Files) > limits.MaxEntries {
		return nil, fmt.Errorf("PACK_INFO inventory has %d entries, exceeds %d entry limit", len(info.Files), limits.MaxEntries)
	}
	seen := map[string]bool{}
	for _, item := range info.Files {
		if item.Path == "" {
			return nil, fmt.Errorf("PACK_INFO inventory contains empty path")
		}
		if seen[item.Path] {
			return nil, fmt.Errorf("PACK_INFO inventory contains duplicate path %q", item.Path)
		}
		if item.Path == "PACK_INFO.json" {
			return nil, fmt.Errorf("PACK_INFO.json must not be listed in PACK_INFO inventory")
		}
		seen[item.Path] = true
		if err := validateArchivePath(item.Path, info.Target); err != nil {
			return nil, fmt.Errorf("PACK_INFO inventory path %q: %w", item.Path, err)
		}
		actual, err := hashExtractedFileLimited(rootEval, item.Path, verifyLimitForPath(item.Path, limits, info.EntryWorkflow))
		if err != nil {
			return nil, fmt.Errorf("verify %s: %w", item.Path, err)
		}
		if actual.Size != item.Size || actual.SHA256 != item.SHA256 {
			return nil, fmt.Errorf("extracted file %q does not match PACK_INFO inventory", item.Path)
		}
	}
	if !seen[info.RuntimeEntry] {
		return nil, fmt.Errorf("PACK_INFO inventory is missing runtime entry %q", info.RuntimeEntry)
	}
	if !seen["pack/pack.json"] {
		return nil, fmt.Errorf("PACK_INFO inventory is missing pack/pack.json")
	}
	if !seen[info.EntryWorkflow] {
		return nil, fmt.Errorf("PACK_INFO inventory is missing entry workflow %q", info.EntryWorkflow)
	}
	for _, logical := range manifest.Plugins {
		path := "pack/" + logical
		if !seen[path] {
			return nil, fmt.Errorf("PACK_INFO inventory is missing manifest plugin %q", path)
		}
	}
	for _, logical := range manifest.Assets {
		path := "pack/" + logical
		if !seen[path] {
			return nil, fmt.Errorf("PACK_INFO inventory is missing manifest asset %q", path)
		}
	}
	if err := validateInventorySizes(info.Files, limits, info.EntryWorkflow); err != nil {
		return nil, err
	}
	if err := verifyNoUnexpectedExtractedFiles(rootEval, seen, limits); err != nil {
		return nil, err
	}
	return &info, nil
}

func equalStringSlices(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func expectedRuntimeEntry(target string) (string, error) {
	parts := strings.Split(target, "-")
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return "", fmt.Errorf("PACK_INFO target %q is invalid", target)
	}
	switch parts[0] {
	case "windows":
		return "goflow.exe", nil
	case "linux", "darwin":
		return "goflow", nil
	default:
		return "", fmt.Errorf("PACK_INFO target %q is unsupported", target)
	}
}

func verifyNoUnexpectedExtractedFiles(root string, inventory map[string]bool, limits buildLimits) error {
	count := 0
	return filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if path == root {
			return nil
		}
		if entry.Type()&os.ModeSymlink != 0 {
			return fmt.Errorf("extracted bundle contains symlink %q", path)
		}
		if entry.IsDir() {
			return nil
		}
		if !entry.Type().IsRegular() {
			return fmt.Errorf("extracted bundle contains non-regular file %q", path)
		}
		count++
		if count > limits.MaxEntries {
			return fmt.Errorf("extracted bundle has more than %d file entries", limits.MaxEntries)
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		slashPath := filepath.ToSlash(rel)
		if slashPath == "PACK_INFO.json" {
			return nil
		}
		if !inventory[slashPath] {
			return fmt.Errorf("extracted file %q is not listed in PACK_INFO inventory", slashPath)
		}
		return nil
	})
}

func hashExtractedFileLimited(root, slashPath string, limit int64) (PackInfoFile, error) {
	osRelPath, err := portablePathToOS(slashPath)
	if err != nil {
		return PackInfoFile{}, err
	}
	path := filepath.Join(root, osRelPath)
	info, err := os.Lstat(path)
	if err != nil {
		return PackInfoFile{}, err
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return PackInfoFile{}, fmt.Errorf("symlinks are not allowed in extracted bundles")
	}
	resolved, err := filepath.EvalSymlinks(path)
	if err != nil {
		return PackInfoFile{}, err
	}
	if !isWithin(root, resolved) {
		return PackInfoFile{}, fmt.Errorf("path resolves outside the bundle directory")
	}
	if !info.Mode().IsRegular() {
		return PackInfoFile{}, fmt.Errorf("path must be a regular file")
	}
	h := sha256.New()
	size, err := streamFileLimited(path, h, limit)
	if err != nil {
		return PackInfoFile{}, err
	}
	return PackInfoFile{Path: slashPath, SHA256: hex.EncodeToString(h.Sum(nil)), Size: size}, nil
}

func streamFileLimited(path string, writer io.Writer, limit int64) (int64, error) {
	file, err := os.Open(path)
	if err != nil {
		return 0, err
	}
	defer file.Close()
	counter := &countingWriter{writer: writer}
	if _, err := io.Copy(counter, io.LimitReader(file, limit+1)); err != nil {
		return counter.n, err
	}
	if counter.n > limit {
		return counter.n, fmt.Errorf("entry exceeds %d byte limit", limit)
	}
	return counter.n, nil
}

func verifyLimitForPath(path string, limits buildLimits, entryWorkflow string) int64 {
	switch {
	case path == "goflow" || path == "goflow.exe":
		return limits.MaxRuntimeBytes
	case path == "pack/pack.json":
		return MaxManifestBytes
	case path == entryWorkflow:
		return MaxWorkflowBytes
	case strings.HasPrefix(path, "pack/plugins/") || strings.HasPrefix(path, "pack/assets/"):
		return limits.MaxResourceBytes
	default:
		return limits.MaxPayloadBytes
	}
}

func readZipFileLimited(file *zip.File, limit int64) ([]byte, int64, error) {
	rc, err := file.Open()
	if err != nil {
		return nil, 0, err
	}
	defer rc.Close()
	data, err := io.ReadAll(io.LimitReader(rc, limit+1))
	if err != nil {
		return nil, int64(len(data)), err
	}
	if int64(len(data)) > limit {
		return nil, int64(len(data)), fmt.Errorf("entry exceeds %d byte limit", limit)
	}
	return data, int64(len(data)), nil
}

func readFileLimited(path string, limit int64) ([]byte, int64, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, 0, err
	}
	defer file.Close()
	data, err := io.ReadAll(io.LimitReader(file, limit+1))
	if err != nil {
		return nil, int64(len(data)), err
	}
	if int64(len(data)) > limit {
		return nil, int64(len(data)), fmt.Errorf("entry exceeds %d byte limit", limit)
	}
	return data, int64(len(data)), nil
}

func hashZipFileLimited(file *zip.File, limit int64) (PackInfoFile, error) {
	rc, err := file.Open()
	if err != nil {
		return PackInfoFile{}, err
	}
	defer rc.Close()
	h := sha256.New()
	counter := &countingWriter{writer: h}
	if _, err := io.Copy(counter, io.LimitReader(rc, limit+1)); err != nil {
		return PackInfoFile{}, err
	}
	if counter.n > limit {
		return PackInfoFile{}, fmt.Errorf("entry exceeds %d byte limit", limit)
	}
	return PackInfoFile{Path: file.Name, SHA256: hex.EncodeToString(h.Sum(nil)), Size: counter.n}, nil
}

func sliceInventory(inventory map[string]PackInfoFile) []PackInfoFile {
	out := make([]PackInfoFile, 0, len(inventory))
	for _, item := range inventory {
		out = append(out, item)
	}
	return out
}

func streamFile(path string, writer io.Writer) (int64, error) {
	file, err := os.Open(path)
	if err != nil {
		return 0, err
	}
	defer file.Close()
	counter := &countingWriter{writer: writer}
	if _, err := io.Copy(counter, file); err != nil {
		return counter.n, err
	}
	return counter.n, nil
}

type countingWriter struct {
	writer io.Writer
	n      int64
}

func (w *countingWriter) Write(p []byte) (int, error) {
	n, err := w.writer.Write(p)
	w.n += int64(n)
	return n, err
}

func readmeText(runtimeEntry string) string {
	command := runtimeEntry
	if runtimeEntry == "goflow" {
		command = "./goflow"
	}
	return fmt.Sprintf(`Goflow Pack portable bundle

Start this pack:
  %s

CLI equivalent:
  %s pack run pack

Run without opening a browser:
  %s pack run pack --no-open

Validate this pack:
  %s pack validate pack

Pack Run binds only to 127.0.0.1 and prints the local URL.
Runtime state is stored outside this extracted bundle by default:
  Windows: %%LOCALAPPDATA%%/Goflow/packs/<pack-id>/
  macOS: ~/Library/Application Support/Goflow/packs/<pack-id>/
  Linux: $XDG_DATA_HOME/Goflow/packs/<pack-id>/ or ~/.local/share/Goflow/packs/<pack-id>/

Back up goflow.db together with goflow.master.key from that data directory.

Do not place credentials in pack.json or workflow files.
Packaged plugin execution is not supported in Pack Run MVP.
Stop the server with Ctrl+C in the terminal running this executable.
`, command, command, command, command)
}

func containsString(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

type publishOps interface {
	Rename(oldPath, newPath string) error
	Remove(path string) error
}

type osPublishOps struct{}

func (osPublishOps) Rename(oldPath, newPath string) error { return os.Rename(oldPath, newPath) }
func (osPublishOps) Remove(path string) error             { return os.Remove(path) }

func publishArchive(tempPath, finalPath string, force bool, ops publishOps) error {
	if !force {
		if err := ops.Rename(tempPath, finalPath); err != nil {
			return fmt.Errorf("output: move archive into place: %w", err)
		}
		return nil
	}
	if err := ops.Rename(tempPath, finalPath); err == nil {
		return nil
	} else if !replaceNeedsFallback(err) {
		return fmt.Errorf("output: replace archive: %w", err)
	}
	backupPath := finalPath + ".backup-" + filepath.Base(tempPath)
	if err := ops.Rename(finalPath, backupPath); err != nil {
		return fmt.Errorf("output: move existing archive to backup %s: %w", backupPath, err)
	}
	if err := ops.Rename(tempPath, finalPath); err != nil {
		restoreErr := ops.Rename(backupPath, finalPath)
		if restoreErr != nil {
			return fmt.Errorf("output: publish failed: %v; rollback failed from backup %s: %w", err, backupPath, restoreErr)
		}
		return fmt.Errorf("output: publish failed and existing archive was restored: %w", err)
	}
	if err := ops.Remove(backupPath); err != nil {
		return fmt.Errorf("output: remove backup %s after successful publish: %w", backupPath, err)
	}
	return nil
}

func replaceNeedsFallback(err error) bool {
	lower := strings.ToLower(err.Error())
	return errors.Is(err, os.ErrExist) ||
		errors.Is(err, os.ErrPermission) ||
		strings.Contains(lower, "exist") ||
		strings.Contains(lower, "access is denied")
}
