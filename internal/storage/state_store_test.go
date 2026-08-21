package storage

import (
	"path/filepath"
	"sync"
	"testing"
)

func newStateTestDB(t *testing.T) *DB {
	t.Helper()
	db, err := NewDB(filepath.Join(t.TempDir(), "state.db"))
	if err != nil {
		t.Fatal(err)
	}
	return db
}

func TestStateStoreWorkflowAndGlobalScopes(t *testing.T) {
	db := newStateTestDB(t)
	defer db.Close()
	store := NewStateStore(db)

	if err := store.Set("workflow", "wf-a", "cursor", map[string]interface{}{"page": 2}); err != nil {
		t.Fatal(err)
	}
	if err := store.Set("workflow", "wf-b", "cursor", "other"); err != nil {
		t.Fatal(err)
	}
	if err := store.Set("global", "ignored", "release", "v1"); err != nil {
		t.Fatal(err)
	}

	value, found, err := store.Get("workflow", "wf-a", "cursor")
	if err != nil || !found {
		t.Fatalf("workflow state get found=%v err=%v", found, err)
	}
	if value.(map[string]interface{})["page"] != float64(2) {
		t.Fatalf("workflow state = %#v", value)
	}
	value, found, err = store.Get("global", "any", "release")
	if err != nil || !found || value != "v1" {
		t.Fatalf("global state = %#v found=%v err=%v", value, found, err)
	}
}

func TestStateStoreIncrementIsPersistentAndTyped(t *testing.T) {
	db := newStateTestDB(t)
	defer db.Close()
	store := NewStateStore(db)

	value, err := store.Increment("workflow", "wf", "count", 2)
	if err != nil || value != 2 {
		t.Fatalf("first increment = %v err=%v", value, err)
	}
	value, err = store.Increment("workflow", "wf", "count", 0.5)
	if err != nil || value != 2.5 {
		t.Fatalf("second increment = %v err=%v", value, err)
	}
	if err := store.Set("workflow", "wf", "bad", "not-a-number"); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Increment("workflow", "wf", "bad", 1); err == nil {
		t.Fatal("increment accepted non-numeric state")
	}
}

func TestStateStoreIncrementAcrossWriterConnections(t *testing.T) {
	path := filepath.Join(t.TempDir(), "shared-state.db")
	dbA, err := NewDB(path)
	if err != nil {
		t.Fatal(err)
	}
	defer dbA.Close()
	dbB, err := NewDB(path)
	if err != nil {
		t.Fatal(err)
	}
	defer dbB.Close()

	storeA := NewStateStore(dbA)
	storeB := NewStateStore(dbB)
	stores := []*StateStore{storeA, storeB}

	const increments = 20
	var wg sync.WaitGroup
	errCh := make(chan error, increments)
	for i := 0; i < increments; i++ {
		store := stores[i%len(stores)]
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := store.Increment("global", "ignored", "counter", 1)
			errCh <- err
		}()
	}
	wg.Wait()
	close(errCh)
	for err := range errCh {
		if err != nil {
			t.Fatalf("concurrent increment failed: %v", err)
		}
	}

	value, found, err := storeA.Get("global", "ignored", "counter")
	if err != nil || !found {
		t.Fatalf("counter found=%v err=%v", found, err)
	}
	if value != float64(increments) {
		t.Fatalf("counter = %#v, want %d", value, increments)
	}
}

func TestStateStoreDeleteAndValidation(t *testing.T) {
	db := newStateTestDB(t)
	defer db.Close()
	store := NewStateStore(db)
	if err := store.Set("workflow", "wf", "key", true); err != nil {
		t.Fatal(err)
	}
	deleted, err := store.Delete("workflow", "wf", "key")
	if err != nil || !deleted {
		t.Fatalf("delete deleted=%v err=%v", deleted, err)
	}
	if _, found, err := store.Get("workflow", "wf", "key"); err != nil || found {
		t.Fatalf("deleted key found=%v err=%v", found, err)
	}
	if err := store.Set("unknown", "wf", "key", 1); err == nil {
		t.Fatal("unknown scope accepted")
	}
	if err := store.Set("workflow", "", "key", 1); err == nil {
		t.Fatal("workflow scope accepted empty workflow id")
	}
}
