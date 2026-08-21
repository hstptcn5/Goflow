package nodes

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"goflow/internal/fileref"
)

func TestFileTriggerDetectsCreatedAndModifiedFiles(t *testing.T) {
	root := t.TempDir()
	t.Setenv("GOFLOW_FILE_ALLOWED_ROOTS", root)
	store := fileref.NewStore(filepath.Join(root, ".managed"))
	executor := NewFileTriggerExecutorWithStore(store)
	state := map[string]interface{}{}
	ctx := NewExecutionContext("wf", "exec")
	ctx.StateGet = func(scope, key string) (interface{}, bool, error) {
		value, ok := state[scope+":"+key]
		return value, ok, nil
	}
	ctx.StateSet = func(scope, key string, value interface{}) error {
		state[scope+":"+key] = value
		return nil
	}
	node := &Node{Params: map[string]interface{}{"path": root, "pattern": "*.txt"}}

	out, err := executor.Execute(ctx, node)
	if err != nil {
		t.Fatal(err)
	}
	if out.(map[string]interface{})["count"] != 0 {
		t.Fatalf("first run should establish baseline: %#v", out)
	}

	filePath := filepath.Join(root, "a.txt")
	if err := os.WriteFile(filePath, []byte("one"), 0600); err != nil {
		t.Fatal(err)
	}
	out, err = executor.Execute(ctx, node)
	if err != nil {
		t.Fatal(err)
	}
	events := out.(map[string]interface{})["events"].([]map[string]interface{})
	if len(events) != 1 || events[0]["event"] != "CREATED" {
		t.Fatalf("created events = %#v", events)
	}
	if _, ok := events[0]["file"].(fileref.Ref); !ok {
		t.Fatalf("event did not contain FileRef: %#v", events[0])
	}

	if err := os.WriteFile(filePath, []byte("two-two"), 0600); err != nil {
		t.Fatal(err)
	}
	future := time.Now().Add(2 * time.Second)
	if err := os.Chtimes(filePath, future, future); err != nil {
		t.Fatal(err)
	}
	out, err = executor.Execute(ctx, node)
	if err != nil {
		t.Fatal(err)
	}
	events = out.(map[string]interface{})["events"].([]map[string]interface{})
	if len(events) != 1 || events[0]["event"] != "MODIFIED" {
		t.Fatalf("modified events = %#v", events)
	}
}

func TestFileTriggerSnapshotJSONRoundTripPreservesNanoseconds(t *testing.T) {
	original := map[string]fileTriggerStamp{
		"a.txt": {Size: 123, ModUnix: 1770000000123456789},
	}
	encoded := encodeFileTriggerSnapshot(original)
	raw, err := json.Marshal(encoded)
	if err != nil {
		t.Fatal(err)
	}
	var persisted interface{}
	if err := json.Unmarshal(raw, &persisted); err != nil {
		t.Fatal(err)
	}
	decoded := decodeFileTriggerSnapshot(persisted)
	if decoded["a.txt"] != original["a.txt"] {
		t.Fatalf("snapshot changed after JSON persistence: got=%#v want=%#v raw=%s", decoded["a.txt"], original["a.txt"], raw)
	}
}

func TestFileTriggerCanEmitExistingOnFirstRun(t *testing.T) {
	root := t.TempDir()
	t.Setenv("GOFLOW_FILE_ALLOWED_ROOTS", root)
	if err := os.WriteFile(filepath.Join(root, "existing.csv"), []byte("x"), 0600); err != nil {
		t.Fatal(err)
	}
	state := map[string]interface{}{}
	ctx := NewExecutionContext("wf", "exec")
	ctx.StateGet = func(scope, key string) (interface{}, bool, error) { return nil, false, nil }
	ctx.StateSet = func(scope, key string, value interface{}) error { state[key] = value; return nil }
	out, err := NewFileTriggerExecutorWithStore(fileref.NewStore(filepath.Join(root, ".managed"))).Execute(ctx, &Node{Params: map[string]interface{}{
		"path": root, "pattern": "*.csv", "emit_existing": true,
	}})
	if err != nil {
		t.Fatal(err)
	}
	if out.(map[string]interface{})["count"] != 1 {
		t.Fatalf("output = %#v", out)
	}
}
