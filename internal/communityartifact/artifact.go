package communityartifact

import (
	"archive/zip"
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"goflow/internal/buildinfo"
)

const (
	ArtifactMarker   = "UNSIGNED-COMMUNITY-RC"
	SchemaVersion    = 1
	ReleaseVersion   = buildinfo.CommunityRCVersion
	ReleaseChannel   = "community-rc"
	MaxArchiveBytes  = 300 << 20
	MaxRuntimeBytes  = 256 << 20
	MaxMetadataBytes = 64 << 10
	MaxTextBytes     = 1 << 20
	MaxChecksumBytes = 512
)

var zipTimestamp = time.Date(1980, 1, 1, 0, 0, 0, 0, time.UTC)

type MemberInfo struct {
	Path   string `json:"path"`
	SHA256 string `json:"sha256"`
	Size   int64  `json:"size"`
}

type Metadata struct {
	SchemaVersion int          `json:"schema_version"`
	Marker        string       `json:"marker"`
	Version       string       `json:"version"`
	Channel       string       `json:"channel"`
	Commit        string       `json:"commit"`
	Target        string       `json:"target"`
	Files         []MemberInfo `json:"files"`
}

func (m Metadata) RuntimePath() string {
	return runtimeName(m.Target)
}

type BuildOptions struct {
	RuntimePath string
	LicensePath string
	OutputDir   string
	Version     string
	Channel     string
	Commit      string
	Target      string
}

type BuildResult struct {
	ArchivePath  string
	ChecksumPath string
	SHA256       string
	Metadata     Metadata
}

type VerifyOptions struct {
	Version string
	Channel string
	Commit  string
	Target  string
}

type archiveEntry struct {
	name string
	mode os.FileMode
	data []byte
}

func Build(opts BuildOptions) (*BuildResult, error) {
	if opts.Version == "" {
		opts.Version = ReleaseVersion
	}
	if opts.Channel == "" {
		opts.Channel = ReleaseChannel
	}
	if err := validateIdentity(opts.Version, opts.Channel, opts.Commit, opts.Target); err != nil {
		return nil, err
	}
	runtimeData, err := readRegularLimited(opts.RuntimePath, MaxRuntimeBytes)
	if err != nil {
		return nil, fmt.Errorf("runtime: %w", err)
	}
	if err := verifyExecutableTarget(runtimeData, opts.Target); err != nil {
		return nil, fmt.Errorf("runtime: %w", err)
	}
	licenseData, err := readRegularLimited(opts.LicensePath, MaxTextBytes)
	if err != nil {
		return nil, fmt.Errorf("license: %w", err)
	}
	runtimeName := runtimeName(opts.Target)
	readmeData := []byte(readme(runtimeName))
	entries := []archiveEntry{
		{name: runtimeName, mode: 0755, data: runtimeData},
		{name: "README.txt", mode: 0644, data: readmeData},
		{name: "LICENSE", mode: 0644, data: licenseData},
	}
	inventory := inventoryForEntries(entries)
	metadata := Metadata{
		SchemaVersion: SchemaVersion, Marker: ArtifactMarker, Version: opts.Version, Channel: opts.Channel,
		Commit: strings.ToLower(opts.Commit), Target: opts.Target,
		Files: inventory,
	}
	metadataData, err := json.MarshalIndent(metadata, "", "  ")
	if err != nil {
		return nil, err
	}
	metadataData = append(metadataData, '\n')
	entries = append(entries, archiveEntry{name: "COMMUNITY_ARTIFACT.json", mode: 0644, data: metadataData})
	sort.Slice(entries, func(i, j int) bool { return entries[i].name < entries[j].name })
	if err := rejectLeakage(entries, opts.RuntimePath); err != nil {
		return nil, err
	}
	if strings.TrimSpace(opts.OutputDir) == "" {
		return nil, fmt.Errorf("output directory is required")
	}
	if err := os.MkdirAll(opts.OutputDir, 0755); err != nil {
		return nil, fmt.Errorf("create output directory: %w", err)
	}
	archiveName := fmt.Sprintf("goflow-community-%s-%s.zip", opts.Version, opts.Target)
	archivePath := filepath.Join(opts.OutputDir, archiveName)
	checksumPath := archivePath + ".sha256"
	for _, destination := range []string{archivePath, checksumPath} {
		if _, err := os.Stat(destination); err == nil {
			return nil, fmt.Errorf("output destination already exists: %s", filepath.Base(destination))
		} else if !os.IsNotExist(err) {
			return nil, fmt.Errorf("check output destination: %w", err)
		}
	}
	tempArchive, err := os.CreateTemp(opts.OutputDir, ".community-artifact-*.zip")
	if err != nil {
		return nil, err
	}
	tempArchivePath := tempArchive.Name()
	defer os.Remove(tempArchivePath)
	if err := writeArchive(tempArchive, entries); err != nil {
		tempArchive.Close()
		return nil, err
	}
	if err := tempArchive.Close(); err != nil {
		return nil, err
	}
	if _, err := Verify(tempArchivePath, VerifyOptions{Version: opts.Version, Channel: opts.Channel, Commit: opts.Commit, Target: opts.Target}); err != nil {
		return nil, fmt.Errorf("verify temporary archive: %w", err)
	}
	digest, err := hashRegularFile(tempArchivePath, MaxArchiveBytes)
	if err != nil {
		return nil, err
	}
	checksumData := []byte(digest + "  " + archiveName + "\n")
	if err := verifyChecksumData(checksumData, archiveName, digest); err != nil {
		return nil, fmt.Errorf("verify generated checksum: %w", err)
	}
	tempChecksum, err := os.CreateTemp(opts.OutputDir, ".community-checksum-*.sha256")
	if err != nil {
		return nil, err
	}
	tempChecksumPath := tempChecksum.Name()
	defer os.Remove(tempChecksumPath)
	if _, err := tempChecksum.Write(checksumData); err != nil {
		tempChecksum.Close()
		return nil, fmt.Errorf("write temporary checksum: %w", err)
	}
	if err := tempChecksum.Close(); err != nil {
		return nil, fmt.Errorf("close temporary checksum: %w", err)
	}
	if err := publishNoReplace(tempChecksumPath, checksumPath); err != nil {
		return nil, fmt.Errorf("publish checksum: %w", err)
	}
	if err := publishNoReplace(tempArchivePath, archivePath); err != nil {
		_ = os.Remove(checksumPath)
		return nil, fmt.Errorf("publish archive: %w", err)
	}
	if err := VerifyChecksumFile(archivePath, checksumPath); err != nil {
		_ = os.Remove(archivePath)
		_ = os.Remove(checksumPath)
		return nil, fmt.Errorf("verify published checksum: %w", err)
	}
	return &BuildResult{ArchivePath: archivePath, ChecksumPath: checksumPath, SHA256: digest, Metadata: metadata}, nil
}

func Verify(path string, expected VerifyOptions) (*Metadata, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() || info.Size() > MaxArchiveBytes {
		return nil, fmt.Errorf("archive must be a regular file no larger than %d bytes", MaxArchiveBytes)
	}
	r, err := zip.OpenReader(path)
	if err != nil {
		return nil, fmt.Errorf("open ZIP: %w", err)
	}
	defer r.Close()
	if len(r.File) != 4 {
		return nil, fmt.Errorf("archive contains %d entries; expected exactly 4", len(r.File))
	}
	files := make(map[string]*zip.File, 4)
	for _, f := range r.File {
		if _, ok := files[f.Name]; ok {
			return nil, fmt.Errorf("duplicate ZIP entry %q", f.Name)
		}
		if !safeExactName(f.Name) {
			return nil, fmt.Errorf("unexpected or unsafe ZIP entry %q", f.Name)
		}
		if f.Mode()&os.ModeSymlink != 0 || !f.Mode().IsRegular() {
			return nil, fmt.Errorf("ZIP entry %q must be a regular file", f.Name)
		}
		files[f.Name] = f
	}
	metadataData, err := readZipLimited(files["COMMUNITY_ARTIFACT.json"], MaxMetadataBytes)
	if err != nil {
		return nil, fmt.Errorf("read metadata: %w", err)
	}
	var metadata Metadata
	decoder := json.NewDecoder(bytes.NewReader(metadataData))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&metadata); err != nil {
		return nil, fmt.Errorf("decode metadata: %w", err)
	}
	if decoder.Decode(&struct{}{}) != io.EOF {
		return nil, fmt.Errorf("metadata contains trailing JSON")
	}
	if err := validateMetadata(metadata, expected); err != nil {
		return nil, err
	}
	verifiedEntries := []archiveEntry{{name: "COMMUNITY_ARTIFACT.json", data: metadataData}}
	var runtimeData []byte
	for _, member := range metadata.Files {
		limit := int64(MaxTextBytes)
		if member.Path == runtimeName(metadata.Target) {
			limit = MaxRuntimeBytes
		}
		data, err := readZipLimited(files[member.Path], limit)
		if err != nil {
			return nil, fmt.Errorf("read %s: %w", member.Path, err)
		}
		sum := sha256.Sum256(data)
		if member.Size != int64(len(data)) || member.SHA256 != hex.EncodeToString(sum[:]) {
			return nil, fmt.Errorf("member %q hash or size does not match metadata", member.Path)
		}
		verifiedEntries = append(verifiedEntries, archiveEntry{name: member.Path, data: data})
		if member.Path == runtimeName(metadata.Target) {
			runtimeData = data
		}
	}
	if err := verifyExecutableTarget(runtimeData, metadata.Target); err != nil {
		return nil, err
	}
	if err := rejectLeakage(verifiedEntries, ""); err != nil {
		return nil, err
	}
	return &metadata, nil
}

func validateMetadata(m Metadata, expected VerifyOptions) error {
	if m.SchemaVersion != SchemaVersion || m.Marker != ArtifactMarker {
		return fmt.Errorf("unsupported Community artifact marker or schema")
	}
	if err := validateIdentity(m.Version, m.Channel, m.Commit, m.Target); err != nil {
		return err
	}
	if err := validateInventory(m.Files, m.Target); err != nil {
		return err
	}
	if expected.Version != "" && m.Version != expected.Version || expected.Channel != "" && m.Channel != expected.Channel || expected.Commit != "" && m.Commit != strings.ToLower(expected.Commit) || expected.Target != "" && m.Target != expected.Target {
		return fmt.Errorf("artifact identity does not match expected exact build")
	}
	return nil
}

func validateIdentity(version, channel, commit, target string) error {
	if version != ReleaseVersion {
		return fmt.Errorf("unsupported version %q", version)
	}
	if channel != ReleaseChannel {
		return fmt.Errorf("unsupported channel %q", channel)
	}
	if len(commit) != 40 {
		return fmt.Errorf("commit must be a full 40-character SHA")
	}
	if _, err := hex.DecodeString(commit); err != nil {
		return fmt.Errorf("commit must be hexadecimal")
	}
	switch target {
	case "linux-amd64", "linux-arm64", "windows-amd64", "darwin-amd64", "darwin-arm64":
	default:
		return fmt.Errorf("unsupported target %q", target)
	}
	return nil
}

func inventoryForEntries(entries []archiveEntry) []MemberInfo {
	inventory := make([]MemberInfo, 0, len(entries))
	for _, entry := range entries {
		sum := sha256.Sum256(entry.data)
		inventory = append(inventory, MemberInfo{Path: entry.name, SHA256: hex.EncodeToString(sum[:]), Size: int64(len(entry.data))})
	}
	sort.Slice(inventory, func(i, j int) bool { return inventory[i].Path < inventory[j].Path })
	return inventory
}

func validateInventory(inventory []MemberInfo, target string) error {
	expected := map[string]bool{"LICENSE": true, "README.txt": true, runtimeName(target): true}
	seen := make(map[string]bool, len(inventory))
	prior := ""
	for _, member := range inventory {
		if member.Path == "COMMUNITY_ARTIFACT.json" {
			return fmt.Errorf("metadata must not inventory itself")
		}
		if seen[member.Path] {
			return fmt.Errorf("inventory contains duplicate path %q", member.Path)
		}
		seen[member.Path] = true
		if !expected[member.Path] {
			return fmt.Errorf("inventory contains unexpected path %q", member.Path)
		}
		if prior != "" && member.Path <= prior {
			return fmt.Errorf("inventory paths must be in canonical sorted order")
		}
		prior = member.Path
		if member.Size < 0 || len(member.SHA256) != 64 {
			return fmt.Errorf("inventory metadata for %q is invalid", member.Path)
		}
		if decoded, err := hex.DecodeString(member.SHA256); err != nil || hex.EncodeToString(decoded) != member.SHA256 {
			return fmt.Errorf("inventory digest for %q must be lowercase SHA-256", member.Path)
		}
	}
	if len(seen) != len(expected) {
		return fmt.Errorf("inventory must bind exactly runtime, README.txt, and LICENSE")
	}
	for path := range expected {
		if !seen[path] {
			return fmt.Errorf("inventory is missing required path %q", path)
		}
	}
	return nil
}

func VerifyChecksumFile(archivePath, checksumPath string) error {
	checksumData, err := readRegularLimited(checksumPath, MaxChecksumBytes)
	if err != nil {
		return fmt.Errorf("checksum file: %w", err)
	}
	digest, err := hashRegularFile(archivePath, MaxArchiveBytes)
	if err != nil {
		return fmt.Errorf("archive checksum: %w", err)
	}
	return verifyChecksumData(checksumData, filepath.Base(archivePath), digest)
}

func verifyChecksumData(data []byte, expectedArchiveName, digest string) error {
	if len(data) == 0 || len(data) > MaxChecksumBytes {
		return fmt.Errorf("checksum file must contain exactly one bounded line")
	}
	if expectedArchiveName == "" || expectedArchiveName != filepath.Base(expectedArchiveName) || strings.ContainsAny(expectedArchiveName, `/\\`) {
		return fmt.Errorf("expected archive name is invalid")
	}
	expected := digest + "  " + expectedArchiveName + "\n"
	if string(data) != expected {
		return fmt.Errorf("checksum line must contain the lowercase archive SHA-256 and exact basename")
	}
	return nil
}

func hashRegularFile(path string, limit int64) (string, error) {
	info, err := os.Lstat(path)
	if err != nil {
		return "", err
	}
	if !info.Mode().IsRegular() || info.Size() > limit {
		return "", fmt.Errorf("must be a regular file no larger than %d bytes", limit)
	}
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()
	h := sha256.New()
	written, err := io.Copy(h, io.LimitReader(file, limit+1))
	if err != nil {
		return "", err
	}
	if written > limit {
		return "", fmt.Errorf("exceeds %d byte limit", limit)
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

func publishNoReplace(tempPath, destination string) error {
	if err := os.Link(tempPath, destination); err != nil {
		return err
	}
	return os.Remove(tempPath)
}

func runtimeName(target string) string {
	if strings.HasPrefix(target, "windows-") {
		return "goflow.exe"
	}
	return "goflow"
}

func safeExactName(name string) bool {
	switch name {
	case "goflow", "goflow.exe", "COMMUNITY_ARTIFACT.json", "README.txt", "LICENSE":
		return !strings.Contains(name, "/") && !strings.Contains(name, `\`)
	default:
		return false
	}
}

func writeArchive(w io.Writer, entries []archiveEntry) error {
	zw := zip.NewWriter(w)
	for _, entry := range entries {
		h := &zip.FileHeader{Name: entry.name, Method: zip.Deflate, Modified: zipTimestamp}
		h.SetMode(entry.mode)
		out, err := zw.CreateHeader(h)
		if err != nil {
			zw.Close()
			return err
		}
		if _, err := out.Write(entry.data); err != nil {
			zw.Close()
			return err
		}
	}
	return zw.Close()
}

func readRegularLimited(path string, limit int64) ([]byte, error) {
	info, err := os.Lstat(path)
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() {
		return nil, fmt.Errorf("must be a regular file")
	}
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	data, err := io.ReadAll(io.LimitReader(f, limit+1))
	if err != nil {
		return nil, err
	}
	if int64(len(data)) > limit {
		return nil, fmt.Errorf("exceeds %d byte limit", limit)
	}
	return data, nil
}

func readZipLimited(f *zip.File, limit int64) ([]byte, error) {
	if f == nil {
		return nil, fmt.Errorf("entry is missing")
	}
	if f.UncompressedSize64 > uint64(limit) {
		return nil, fmt.Errorf("entry exceeds %d byte limit", limit)
	}
	r, err := f.Open()
	if err != nil {
		return nil, err
	}
	defer r.Close()
	data, err := io.ReadAll(io.LimitReader(r, limit+1))
	if err != nil {
		return nil, err
	}
	if int64(len(data)) > limit {
		return nil, fmt.Errorf("entry exceeds %d byte limit", limit)
	}
	return data, nil
}

func rejectLeakage(entries []archiveEntry, runtimePath string) error {
	markers := [][]byte{[]byte(`C:\Users\`), []byte(`/home/runner/`), []byte(`/Users/runner/`)}
	if cwd, err := os.Getwd(); err == nil && len(cwd) >= 4 {
		markers = append(markers, []byte(cwd), []byte(filepath.ToSlash(cwd)))
	}
	if runtimePath != "" {
		if absolute, err := filepath.Abs(runtimePath); err == nil {
			markers = append(markers, []byte(filepath.Dir(absolute)), []byte(filepath.ToSlash(filepath.Dir(absolute))))
		}
	}
	for _, entry := range entries {
		for _, marker := range markers {
			if len(marker) >= 4 && bytes.Contains(bytes.ToLower(entry.data), bytes.ToLower(marker)) {
				return fmt.Errorf("entry %q leaks a local build path", entry.name)
			}
		}
	}
	return nil
}

func verifyExecutableTarget(data []byte, target string) error {
	parts := strings.Split(target, "-")
	if len(parts) != 2 {
		return fmt.Errorf("invalid target %q", target)
	}
	goos, arch := parts[0], parts[1]
	switch goos {
	case "linux":
		if len(data) < 20 || !bytes.Equal(data[:4], []byte{0x7f, 'E', 'L', 'F'}) {
			return fmt.Errorf("runtime is not an ELF executable for %s", target)
		}
		var order binary.ByteOrder = binary.LittleEndian
		if data[5] == 2 {
			order = binary.BigEndian
		}
		machine := order.Uint16(data[18:20])
		if arch == "amd64" && machine != 62 || arch == "arm64" && machine != 183 {
			return fmt.Errorf("ELF architecture does not match %s", target)
		}
	case "windows":
		if len(data) < 64 || string(data[:2]) != "MZ" {
			return fmt.Errorf("runtime is not a PE executable for %s", target)
		}
		off := int(binary.LittleEndian.Uint32(data[0x3c:0x40]))
		if off < 0 || off+6 > len(data) || string(data[off:off+4]) != "PE\x00\x00" {
			return fmt.Errorf("runtime has invalid PE header")
		}
		machine := binary.LittleEndian.Uint16(data[off+4 : off+6])
		if arch != "amd64" || machine != 0x8664 {
			return fmt.Errorf("PE architecture does not match %s", target)
		}
	case "darwin":
		if len(data) < 8 {
			return fmt.Errorf("runtime is not a Mach-O executable for %s", target)
		}
		magic := binary.BigEndian.Uint32(data[:4])
		var order binary.ByteOrder
		switch magic {
		case 0xfeedfacf:
			order = binary.BigEndian
		case 0xcffaedfe:
			order = binary.LittleEndian
		default:
			return fmt.Errorf("runtime is not a 64-bit Mach-O executable for %s", target)
		}
		cpu := order.Uint32(data[4:8])
		if arch == "amd64" && cpu != 0x01000007 || arch == "arm64" && cpu != 0x0100000c {
			return fmt.Errorf("Mach-O architecture does not match %s", target)
		}
	default:
		return fmt.Errorf("unsupported target %q", target)
	}
	return nil
}

func readme(runtime string) string {
	return "Goflow Community 1.0 Release Candidate\n\nRun .\\goflow.exe on Windows or ./" + runtime + " on Linux/macOS.\nData is stored outside this directory when GOFLOW_DB_PATH and GOFLOW_MASTER_KEY_FILE are configured.\nThis archive is unsigned and does not claim publisher authenticity. Verify the adjacent SHA-256 checksum before use.\n"
}
