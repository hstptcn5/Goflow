package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDetectExtractedBundleDir(t *testing.T) {
	dir := t.TempDir()
	if _, ok := detectExtractedBundleDir(dir); ok {
		t.Fatalf("empty directory was detected as bundle")
	}
	if err := os.WriteFile(filepath.Join(dir, "PACK_INFO.json"), []byte("{}"), 0600); err != nil {
		t.Fatalf("write PACK_INFO: %v", err)
	}
	if _, ok := detectExtractedBundleDir(dir); ok {
		t.Fatalf("missing pack/pack.json was detected as bundle")
	}
	if err := os.MkdirAll(filepath.Join(dir, "pack"), 0700); err != nil {
		t.Fatalf("mkdir pack: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "pack", "pack.json"), []byte("{}"), 0600); err != nil {
		t.Fatalf("write pack.json: %v", err)
	}
	got, ok := detectExtractedBundleDir(dir)
	if !ok || got != dir {
		t.Fatalf("detectExtractedBundleDir got %q %t", got, ok)
	}
}
