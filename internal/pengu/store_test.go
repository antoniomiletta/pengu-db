package pengu

import (
	"bytes"
	"fmt"
	"path/filepath"
	"testing"
)

func openStore(t *testing.T) *Store {
	t.Helper()

	store, err := Open(fmt.Sprintf("%s/test.log", t.TempDir()))
	if err != nil {
		t.Fatalf("failed to open store: %v", err)
	}
	return store
}

func seed(t *testing.T, store *Store) {
	t.Helper()

	store.Set([]byte("key-1"), []byte("value-1"))
	store.Set([]byte("key-2"), []byte("value-2"))
	store.Set([]byte("key-3"), []byte("value-3"))
	store.Set([]byte("key-3"), []byte("other-value-3"))
	store.Delete([]byte("key-2"))
}

func seedGarbage(t *testing.T, store *Store) {
	t.Helper()

	for i := 0; i < 10; i++ {
		key := []byte(fmt.Sprintf("key-%d", i))
		val := []byte(fmt.Sprintf("value-%d", i))
		store.Set(key, val)
	}

	for i := 0; i < 5; i++ {
		key := []byte(fmt.Sprintf("key-%d", i))
		val := []byte(fmt.Sprintf("new-value-padding-%d", i))
		store.Set(key, val)
	}
	store.Delete([]byte("key-8"))
	store.Delete([]byte("key-9"))
}

func TestStore_CoreAPI(t *testing.T) {
	store := openStore(t)
	defer store.Close()

	key := []byte("test_key")
	val := []byte("test_value")

	if err := store.Set(key, val); err != nil {
		t.Fatalf("set: %v", err)
	}
	got, err := store.Get(key)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if !bytes.Equal(val, got) {
		t.Fatalf("expected: %s, got: %s", val, got)
	}

	if err := store.Delete(key); err != nil {
		t.Fatalf("delete: %v", err)
	}
	deleted, _ := store.Get(key)
	if deleted != nil {
		t.Fatalf("expected nil, got %s", deleted)
	}

	validateInternalState(t, store)
}

func TestStore_LifecycleConsistency(t *testing.T) {
	store := openStore(t)
	defer store.Close()

	key := []byte("key")
	val := []byte("value")
	smaller := []byte("smaller_value")
	larger := []byte("laaaarger_value")

	store.Set(key, val)
	validateInternalState(t, store)

	store.Set(key, val)
	validateInternalState(t, store)

	store.Set(key, smaller)
	validateInternalState(t, store)

	store.Set(key, larger)
	validateInternalState(t, store)

	store.Delete(key)
	validateInternalState(t, store)
}

func TestStore_CrashRecovery(t *testing.T) {
	dir := t.TempDir()
	testLog := filepath.Join(dir, "test.log")

	store, err := Open(testLog)
	if err != nil {
		t.Fatalf("failed to open: %v", err)
	}

	seed(t, store)
	validateInternalState(t, store)
	store.Close()

	store2, err := Open(testLog)
	if err != nil {
		t.Fatalf("failed to reopen: %v", err)
	}
	defer store2.Close()

	validateInternalState(t, store2)
}

func TestStore_Compaction(t *testing.T) {
	store := openStore(t)
	defer store.Close()

	seedGarbage(t, store)

	store.mu.RLock()
	totalBefore := store.Journal.endOffset
	activeBefore := store.Journal.activeBytes
	store.mu.RUnlock()

	if totalBefore <= activeBefore {
		t.Fatalf("setup failed: expected garbage bytes before compaction, got: total: %d, active; %d", totalBefore, activeBefore)
	}

	if err := store.Compact(); err != nil {
		t.Fatalf("failed to compact file: %v", err)
	}

	store.mu.RLock()
	totalAfter := store.Journal.endOffset
	activeAfter := store.Journal.activeBytes
	store.mu.RUnlock()

	if totalAfter != activeAfter {
		t.Fatalf("found garbage after compaction: expected all active bytes, got: %d/%d", activeAfter, totalAfter)
	}

	validateInternalState(t, store)
}

// TODO: test concurrency

// validateInternalState inspects and validates the store's index and metadata.
func validateInternalState(t *testing.T, store *Store) {
	t.Helper()
	store.mu.RLock()
	defer store.mu.RUnlock()

	var totalActiveBytes int64

	for k, e := range store.index {
		bytes := int64(SizeHeader + e.keySize + e.valSize)

		val, err := readValueAt(store.Journal.file, e)
		if err != nil {
			t.Fatalf("invariant broken: index: %s points to unreadable offset: %d: %v", k, e.offset, err)
		}

		if val == nil {
			t.Fatalf("invariant broken: index: %s points to nil value (offset: %d)", k, e.offset)
		}

		if _, err := decodeAt(store.Journal.file, e.offset); err != nil {
			t.Fatalf("invariant broken: index: %s points to invalid record (offset: %d): %v", k, e.offset, err)
		}

		totalActiveBytes += bytes
	}

	if totalActiveBytes != store.Journal.activeBytes {
		t.Fatalf("invariant broken: active bytes: expected: %d, got: %d", totalActiveBytes, store.Journal.activeBytes)
	}
}
