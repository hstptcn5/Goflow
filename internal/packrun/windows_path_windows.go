//go:build windows

package packrun

import (
	"os"
	"path/filepath"
	"strings"
)

// Windows normally exposes rundll32.exe from System32 through PATH, but some
// locked-down or customized user environments omit System32. Pack Run uses
// rundll32 only to open the local appliance URL in the default browser, so make
// that standard Windows directory discoverable without requiring users to edit
// their machine-wide PATH.
func init() {
	ensureWindowsSystem32OnPath()
}

func ensureWindowsSystem32OnPath() {
	root := strings.TrimSpace(os.Getenv("SystemRoot"))
	if root == "" {
		root = strings.TrimSpace(os.Getenv("WINDIR"))
	}
	if root == "" {
		return
	}

	system32 := filepath.Join(root, "System32")
	if _, err := os.Stat(filepath.Join(system32, "rundll32.exe")); err != nil {
		return
	}

	current := os.Getenv("PATH")
	for _, entry := range filepath.SplitList(current) {
		if strings.EqualFold(filepath.Clean(entry), filepath.Clean(system32)) {
			return
		}
	}

	if current == "" {
		_ = os.Setenv("PATH", system32)
		return
	}
	_ = os.Setenv("PATH", system32+string(os.PathListSeparator)+current)
}
