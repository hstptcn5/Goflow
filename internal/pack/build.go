package pack

import (
	"archive/zip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
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
}

type BuildResult struct {
	ArchivePath string
	ArchiveName string
	Target      string
}

type PackInfo struct {
	SchemaVersion int            `json:"schema_version"`
	PackID        string         `json:"pack_id"`
	PackVersion   string         `json:"pack_version"`
	Target        string         `json:"target"`
	RuntimeEntry  string         `json:"runtime_entry"`
	EntryWorkflow string         `json:"entry_workflow"`
	Files         []PackInfoFile `json:"files"`
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
	if err := validateArchiveFiles(files, opts.Target); err != nil {
		return nil, err
	}
	inventory, err := inventoryFiles(files)
	if err != nil {
		return nil, err
	}
	packInfo := PackInfo{
		SchemaVersion: SupportedSchema,
		PackID:        loaded.Manifest.ID,
		PackVersion:   loaded.Manifest.Version,
		Target:        opts.Target,
		RuntimeEntry:  runtimeEntry(opts.Target),
		EntryWorkflow: "pack/" + loaded.Manifest.EntryWorkflow,
		Files:         inventory,
	}
	packInfoData, err := json.MarshalIndent(packInfo, "", "  ")
	if err != nil {
		return nil, err
	}
	packInfoData = append(packInfoData, '\n')
	files = append(files, archiveFile{archivePath: "PACK_INFO.json", generated: packInfoData})
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
	if opts.Force {
		if err := os.Rename(tempPath, archivePath); err != nil {
			if !os.IsExist(err) {
				return nil, fmt.Errorf("output: replace archive: %w", err)
			}
			if err := os.Remove(archivePath); err != nil {
				return nil, fmt.Errorf("output: remove existing archive after successful build: %w", err)
			}
			if err := os.Rename(tempPath, archivePath); err != nil {
				return nil, fmt.Errorf("output: move archive into place after removing existing archive: %w", err)
			}
		}
	} else if err := os.Rename(tempPath, archivePath); err != nil {
		return nil, fmt.Errorf("output: move archive into place: %w", err)
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
	files := []archiveFile{
		{archivePath: runtimeEntry, sourcePath: runtimePath},
		{archivePath: "pack/pack.json", sourcePath: loaded.ManifestPath, payloadCount: true},
		{archivePath: "pack/" + loaded.Manifest.EntryWorkflow, sourcePath: loaded.EntryWorkflowPath, payloadCount: true},
		{archivePath: "README.txt", generated: []byte(readmeText(runtimeEntry))},
	}
	for _, logical := range loaded.Manifest.Plugins {
		sourcePath, err := resolveExistingRegularInside(loaded.Root, logical)
		if err != nil {
			return nil, fmt.Errorf("path: plugins entry %q: %w", logical, err)
		}
		files = append(files, archiveFile{archivePath: "pack/" + logical, sourcePath: sourcePath, payloadCount: true})
	}
	for _, logical := range loaded.Manifest.Assets {
		sourcePath, err := resolveExistingRegularInside(loaded.Root, logical)
		if err != nil {
			return nil, fmt.Errorf("path: assets entry %q: %w", logical, err)
		}
		files = append(files, archiveFile{archivePath: "pack/" + logical, sourcePath: sourcePath, payloadCount: true})
	}
	return files, nil
}

func validateArchiveFiles(files []archiveFile, target string) error {
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
			if isResource && info.Size() > MaxPackResourceBytes {
				return fmt.Errorf("%s exceeds %d byte per-file limit", file.archivePath, MaxPackResourceBytes)
			}
			totalPayload += info.Size()
			if totalPayload > MaxPackPayloadBytes {
				return fmt.Errorf("pack payload exceeds %d byte total limit", MaxPackPayloadBytes)
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
		if file.archivePath == "goflow" || file.archivePath == "goflow.exe" {
			header.SetMode(0755)
		} else {
			header.SetMode(0600)
		}
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

Validate this pack:
  %s pack validate pack

This bundle does not automatically install or run the workflow.
Import or execution support will be added in a later Goflow Pack phase.

Do not place credentials in pack.json or workflow files.
`, command)
}

func containsString(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}
