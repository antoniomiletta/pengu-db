package pengu

import (
	"bytes"
	"fmt"
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

	store.Set([]byte("key"), []byte("value"))
	store.Set([]byte("key1"), []byte("same_value"))
	store.Set([]byte("key1"), []byte("same_value"))
	store.Set([]byte("key1"), []byte("smaller"))
	store.Set([]byte("key1"), []byte("laaaaaaarger"))
	store.Delete([]byte("key2"))
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
		t.Fatalf("expected nil, got %s", val)
	}

	assertInvariants(t, store)
}

func TestStore_LifecycleConsistency(t *testing.T) {
	store := openStore(t)

	key := []byte("key")
	val := []byte("value")
	smaller := []byte("laaaarger_value")
	larger := []byte("smaller_value")

	store.Set(key, val)
	assertInvariants(t, store)

	store.Set(key, val)
	assertInvariants(t, store)

	store.Set(key, smaller)
	assertInvariants(t, store)

	store.Set(key, larger)
	assertInvariants(t, store)

	store.Delete(key)
	assertInvariants(t, store)
}

func TestStore_CrashRecovery(t *testing.T) {
	store := openStore(t)

	seed(t, store)

	assertInvariants(t, store)
	store.Close()

	store2 := openStore(t)
	defer store2.Close()

	assertInvariants(t, store2)
}

// TODO: test concurrency

// assertInvariants inspects and validates the internal state of store.
func assertInvariants(t *testing.T, store *Store) {
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
