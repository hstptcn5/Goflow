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
)

const AppInfoFileName = "GOFLOW_APP.json"

type AppBuildOptions struct {
	PackDir     string
	OutputDir   string
	RuntimePath string
	Target      string
	Force       bool
}

type AppBuildResult struct {
	AppPath string `json:"app_path"`
	AppName string `json:"app_name"`
	Target  string `json:"target"`
}

type AppInfo struct {
	SchemaVersion int            `json:"schema_version"`
	PackID        string         `json:"pack_id"`
	PackVersion   string         `json:"pack_version"`
	Target        string         `json:"target"`
	EntryWorkflow string         `json:"entry_workflow"`
	Files         []PackInfoFile `json:"files"`
}

// BuildApp creates a same-platform, self-contained executable. The executable
// prefix is the current Goflow runtime and the Pack is an appended, verified ZIP.
func BuildApp(opts AppBuildOptions) (*AppBuildResult, error) {
	if strings.TrimSpace(opts.PackDir) == "" || strings.TrimSpace(opts.OutputDir) == "" {
		return nil, fmt.Errorf("pack directory and output directory are required")
	}
	if opts.Target == "" {
		opts.Target = CurrentPlatform()
	}
	if opts.Target != CurrentPlatform() {
		return nil, fmt.Errorf("target %q does not match runtime platform %q", opts.Target, CurrentPlatform())
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
	if err != nil || !runtimeInfo.Mode().IsRegular() {
		return nil, fmt.Errorf("runtime: source must be a regular file")
	}
	if runtimeInfo.Size() > MaxRuntimeBytes {
		return nil, fmt.Errorf("runtime: source exceeds %d byte limit", MaxRuntimeBytes)
	}

	files, err := appPayloadFiles(loaded)
	if err != nil {
		return nil, err
	}
	if err := validateArchiveFiles(files, opts.Target, defaultBuildLimits()); err != nil {
		return nil, err
	}
	inventory, err := inventoryFiles(files)
	if err != nil {
		return nil, err
	}
	if err := validateInventorySizes(inventory, defaultBuildLimits(), "pack/"+loaded.Manifest.EntryWorkflow); err != nil {
		return nil, err
	}
	info := AppInfo{SchemaVersion: SupportedSchema, PackID: loaded.Manifest.ID, PackVersion: loaded.Manifest.Version, Target: opts.Target, EntryWorkflow: "pack/" + loaded.Manifest.EntryWorkflow, Files: inventory}
	infoData, err := json.MarshalIndent(info, "", "  ")
	if err != nil {
		return nil, err
	}
	files = append(files, archiveFile{archivePath: AppInfoFileName, generated: append(infoData, '\n'), mode: 0600})
	sort.Slice(files, func(i, j int) bool { return files[i].archivePath < files[j].archivePath })

	if err := os.MkdirAll(opts.OutputDir, 0700); err != nil {
		return nil, fmt.Errorf("output: create directory: %w", err)
	}
	name := loaded.Manifest.ID
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	finalPath := filepath.Join(opts.OutputDir, name)
	if _, err := os.Stat(finalPath); err == nil && !opts.Force {
		return nil, fmt.Errorf("output app already exists: %s", finalPath)
	} else if err != nil && !os.IsNotExist(err) {
		return nil, err
	}
	temp, err := os.CreateTemp(opts.OutputDir, "."+name+".tmp-*")
	if err != nil {
		return nil, err
	}
	tempPath := temp.Name()
	defer os.Remove(tempPath)
	runtimeFile, err := os.Open(opts.RuntimePath)
	if err != nil {
		_ = temp.Close()
		return nil, err
	}
	offset, err := io.Copy(temp, io.LimitReader(runtimeFile, MaxRuntimeBytes+1))
	_ = runtimeFile.Close()
	if err != nil || offset != runtimeInfo.Size() {
		_ = temp.Close()
		return nil, fmt.Errorf("runtime: copy failed")
	}
	zw := zip.NewWriter(temp)
	zw.SetOffset(offset)
	if err := writeAppZip(zw, files); err != nil {
		_ = temp.Close()
		return nil, err
	}
	if err := temp.Chmod(0755); err != nil {
		_ = temp.Close()
		return nil, err
	}
	if err := temp.Close(); err != nil {
		return nil, err
	}
	if _, err := VerifyEmbeddedApp(tempPath); err != nil {
		return nil, fmt.Errorf("output: verify embedded app: %w", err)
	}
	if opts.Force {
		_ = os.Remove(finalPath)
	}
	if err := os.Rename(tempPath, finalPath); err != nil {
		return nil, fmt.Errorf("output: publish app: %w", err)
	}
	return &AppBuildResult{AppPath: finalPath, AppName: name, Target: opts.Target}, nil
}

func appPayloadFiles(loaded *Pack) ([]archiveFile, error) {
	manifest, err := runtimeManifestData(loaded.ManifestPath)
	if err != nil {
		return nil, err
	}
	files := []archiveFile{
		{archivePath: "pack/pack.json", generated: manifest, payloadCount: true, mode: 0600},
		{archivePath: "pack/" + loaded.Manifest.EntryWorkflow, sourcePath: loaded.EntryWorkflowPath, payloadCount: true, mode: 0600},
	}
	for _, logical := range append(append([]string{}, loaded.Manifest.Plugins...), loaded.Manifest.Assets...) {
		path, err := resolveExistingRegularInside(loaded.Root, logical)
		if err != nil {
			return nil, err
		}
		files = append(files, archiveFile{archivePath: "pack/" + logical, sourcePath: path, payloadCount: true, mode: pluginArchiveMode(path)})
	}
	return files, nil
}

func writeAppZip(zw *zip.Writer, files []archiveFile) error {
	for _, file := range files {
		header := &zip.FileHeader{Name: file.archivePath, Method: zip.Deflate, Modified: zipTimestamp}
		header.SetMode(sanitizedArchiveMode(file.mode))
		writer, err := zw.CreateHeader(header)
		if err != nil {
			_ = zw.Close()
			return err
		}
		if file.generated != nil {
			_, err = writer.Write(file.generated)
		} else {
			_, err = streamFile(file.sourcePath, writer)
		}
		if err != nil {
			_ = zw.Close()
			return err
		}
	}
	return zw.Close()
}

// VerifyEmbeddedApp validates the inventory without extracting any file.
func VerifyEmbeddedApp(path string) (*AppInfo, error) {
	reader, err := zip.OpenReader(path)
	if err != nil {
		return nil, err
	}
	defer reader.Close()
	if len(reader.File) > MaxBundleEntries {
		return nil, fmt.Errorf("app has too many entries")
	}
	entries := map[string]*zip.File{}
	for _, file := range reader.File {
		if entries[file.Name] != nil {
			return nil, fmt.Errorf("duplicate app entry %q", file.Name)
		}
		entries[file.Name] = file
	}
	meta := entries[AppInfoFileName]
	if meta == nil {
		return nil, fmt.Errorf("%s is missing", AppInfoFileName)
	}
	data, _, err := readZipFileLimited(meta, MaxPackInfoBytes)
	if err != nil {
		return nil, err
	}
	var info AppInfo
	if err := json.Unmarshal(data, &info); err != nil {
		return nil, err
	}
	if info.SchemaVersion != SupportedSchema || info.Target != CurrentPlatform() {
		return nil, fmt.Errorf("embedded app metadata is incompatible with this runtime")
	}
	inventoryPaths := map[string]bool{}
	for _, item := range info.Files {
		if item.Size < 0 {
			return nil, fmt.Errorf("embedded app file %q has an invalid size", item.Path)
		}
		if inventoryPaths[item.Path] {
			return nil, fmt.Errorf("embedded app inventory duplicates %q", item.Path)
		}
		inventoryPaths[item.Path] = true
	}
	if !inventoryPaths["pack/pack.json"] || !inventoryPaths[info.EntryWorkflow] {
		return nil, fmt.Errorf("embedded app is missing its manifest or entry workflow")
	}
	if err := validateInventorySizes(info.Files, defaultBuildLimits(), info.EntryWorkflow); err != nil {
		return nil, err
	}
	if len(info.Files)+1 != len(entries) {
		return nil, fmt.Errorf("embedded app contains an unlisted file")
	}
	for path := range entries {
		if path != AppInfoFileName && !inventoryPaths[path] {
			return nil, fmt.Errorf("embedded app contains unlisted file %q", path)
		}
	}
	for _, expected := range info.Files {
		if !strings.HasPrefix(expected.Path, "pack/") || validateArchivePath(expected.Path, info.Target) != nil {
			return nil, fmt.Errorf("invalid embedded app path %q", expected.Path)
		}
		file := entries[expected.Path]
		if file == nil {
			return nil, fmt.Errorf("embedded app file %q is missing", expected.Path)
		}
		rc, err := file.Open()
		if err != nil {
			return nil, err
		}
		h := sha256.New()
		size, copyErr := io.Copy(h, io.LimitReader(rc, MaxPackPayloadBytes+1))
		closeErr := rc.Close()
		if copyErr != nil || closeErr != nil || size != expected.Size || hex.EncodeToString(h.Sum(nil)) != expected.SHA256 {
			return nil, fmt.Errorf("embedded app file %q failed integrity verification", expected.Path)
		}
	}
	return &info, nil
}

func IsEmbeddedApp(path string) bool {
	reader, err := zip.OpenReader(path)
	if err != nil {
		return false
	}
	defer reader.Close()
	for _, file := range reader.File {
		if file.Name == AppInfoFileName {
			return true
		}
	}
	return false
}

// ExtractEmbeddedApp verifies first, then extracts only bounded Pack files.
func ExtractEmbeddedApp(path, destination string) (string, *AppInfo, error) {
	info, err := VerifyEmbeddedApp(path)
	if err != nil {
		return "", nil, err
	}
	reader, err := zip.OpenReader(path)
	if err != nil {
		return "", nil, err
	}
	defer reader.Close()
	for _, file := range reader.File {
		if !strings.HasPrefix(file.Name, "pack/") {
			continue
		}
		target := filepath.Join(destination, filepath.FromSlash(file.Name))
		if err := os.MkdirAll(filepath.Dir(target), 0700); err != nil {
			return "", nil, err
		}
		rc, err := file.Open()
		if err != nil {
			return "", nil, err
		}
		out, err := os.OpenFile(target, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
		if err == nil {
			_, err = io.Copy(out, io.LimitReader(rc, MaxPackPayloadBytes+1))
			if closeErr := out.Close(); err == nil {
				err = closeErr
			}
		}
		_ = rc.Close()
		if err != nil {
			return "", nil, err
		}
	}
	return filepath.Join(destination, "pack"), info, nil
}
