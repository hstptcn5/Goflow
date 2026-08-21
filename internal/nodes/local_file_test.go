package nodes

import (
	"os"
	"path/filepath"
	"testing"

	"goflow/internal/fileref"
)

func TestLocalFileReadWriteAndList(t *testing.T) {
	root := t.TempDir()
	t.Setenv("GOFLOW_FILE_ALLOWED_ROOTS", root)
	store := fileref.NewStore(filepath.Join(root, ".managed"))
	executor := NewLocalFileExecutorWithStore(store)
	ctx := NewExecutionContext("wf", "exec")

	writeOut, err := executor.Execute(ctx, &Node{Params: map[string]interface{}{
		"operation": "WRITE", "path": filepath.Join(root, "reports", "a.txt"), "content": "hello", "create_directories": true,
	}})
	if err != nil {
		t.Fatal(err)
	}
	if writeOut.(map[string]interface{})["written"] != true {
		t.Fatalf("write output = %#v", writeOut)
	}

	readOut, err := executor.Execute(ctx, &Node{Params: map[string]interface{}{
		"operation": "READ", "path": filepath.Join(root, "reports", "a.txt"),
	}})
	if err != nil {
		t.Fatal(err)
	}
	ref, ok := readOut.(fileref.Ref)
	if !ok || ref.Name != "a.txt" || ref.Size != 5 {
		t.Fatalf("read output = %#v", readOut)
	}

	copyPath := filepath.Join(root, "reports", "copy.txt")
	if _, err := executor.Execute(ctx, &Node{Params: map[string]interface{}{
		"operation": "WRITE", "path": copyPath, "file_ref": ref,
	}}); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(copyPath)
	if err != nil || string(data) != "hello" {
		t.Fatalf("copied data = %q err=%v", data, err)
	}

	listOut, err := executor.Execute(ctx, &Node{Params: map[string]interface{}{
		"operation": "LIST", "path": filepath.Join(root, "reports"), "pattern": "*.txt",
	}})
	if err != nil {
		t.Fatal(err)
	}
	if listOut.(map[string]interface{})["count"] != 2 {
		t.Fatalf("list output = %#v", listOut)
	}
}

func TestLocalFileRejectsOutsideAllowedRoot(t *testing.T) {
	root := t.TempDir()
	outside := t.TempDir()
	t.Setenv("GOFLOW_FILE_ALLOWED_ROOTS", root)
	if _, err := resolveLocalFilePath(filepath.Join(outside, "secret.txt"), true); err == nil {
		t.Fatal("outside path was accepted")
	}
}

func TestLocalFileRejectsSymlinkEscapeWhenSupported(t *testing.T) {
	root := t.TempDir()
	outside := t.TempDir()
	t.Setenv("GOFLOW_FILE_ALLOWED_ROOTS", root)
	link := filepath.Join(root, "escape")
	if err := os.Symlink(outside, link); err != nil {
		t.Skipf("symlink unavailable on this runner: %v", err)
	}
	if _, err := resolveLocalFilePath(filepath.Join(link, "secret.txt"), true); err == nil {
		t.Fatal("symlink escape path was accepted")
	}
}
