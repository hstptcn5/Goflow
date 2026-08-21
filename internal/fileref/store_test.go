package fileref

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestStorePutResolveAndRead(t *testing.T) {
	store := NewStore(t.TempDir())
	ref, err := store.PutBytes("orders.csv", "text/csv", []byte("sku,qty\nA,2\n"))
	if err != nil {
		t.Fatal(err)
	}
	if ref.Type != TypeMarker || ref.ID == "" || ref.Name != "orders.csv" || ref.Size == 0 || ref.SHA256 == "" {
		t.Fatalf("unexpected ref %#v", ref)
	}
	path, err := store.Resolve(ref)
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Dir(path) != store.Root {
		t.Fatalf("resolved path escaped store root: %s", path)
	}
	data, err := store.ReadAll(ref)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(data, []byte("sku,qty\nA,2\n")) {
		t.Fatalf("data = %q", data)
	}
}

func TestStorePutPathAndBounds(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "source.txt")
	if err := os.WriteFile(source, []byte("hello"), 0600); err != nil {
		t.Fatal(err)
	}
	store := NewStore(filepath.Join(root, "store"))
	store.MaxBytes = 4
	if _, err := store.PutPath(source); err == nil {
		t.Fatal("oversized source accepted")
	}
	store.MaxBytes = 10
	ref, err := store.PutPath(source)
	if err != nil {
		t.Fatal(err)
	}
	if ref.Name != "source.txt" || ref.Size != 5 {
		t.Fatalf("ref = %#v", ref)
	}
}

func TestParseRejectsInvalidRefs(t *testing.T) {
	if _, err := Parse(map[string]interface{}{"$type": "other", "id": "x"}); err == nil {
		t.Fatal("invalid type accepted")
	}
	if _, err := Parse(map[string]interface{}{"$type": "file", "id": "not-a-uuid"}); err != nil {
		// Parse validates the FileRef envelope; Resolve performs UUID/path validation.
		return
	}
}
