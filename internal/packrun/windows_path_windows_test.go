//go:build windows

package packrun

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestEnsureWindowsSystem32OnPathAddsStandardBrowserLauncher(t *testing.T) {
	root := t.TempDir()
	system32 := filepath.Join(root, "System32")
	if err := os.MkdirAll(system32, 0o755); err != nil {
		t.Fatalf("create System32: %v", err)
	}
	if err := os.WriteFile(filepath.Join(system32, "rundll32.exe"), []byte("test"), 0o644); err != nil {
		t.Fatalf("create rundll32.exe: %v", err)
	}

	oldRoot := os.Getenv("SystemRoot")
	oldWindir := os.Getenv("WINDIR")
	oldPath := os.Getenv("PATH")
	t.Cleanup(func() {
		_ = os.Setenv("SystemRoot", oldRoot)
		_ = os.Setenv("WINDIR", oldWindir)
		_ = os.Setenv("PATH", oldPath)
	})

	other := filepath.Join(root, "Other")
	_ = os.Setenv("SystemRoot", root)
	_ = os.Setenv("WINDIR", "")
	_ = os.Setenv("PATH", other)

	ensureWindowsSystem32OnPath()
	entries := filepath.SplitList(os.Getenv("PATH"))
	if len(entries) < 2 || !strings.EqualFold(filepath.Clean(entries[0]), filepath.Clean(system32)) {
		t.Fatalf("System32 was not prepended to PATH: %q", os.Getenv("PATH"))
	}

	ensureWindowsSystem32OnPath()
	count := 0
	for _, entry := range filepath.SplitList(os.Getenv("PATH")) {
		if strings.EqualFold(filepath.Clean(entry), filepath.Clean(system32)) {
			count++
		}
	}
	if count != 1 {
		t.Fatalf("System32 was duplicated in PATH: %q", os.Getenv("PATH"))
	}
}
