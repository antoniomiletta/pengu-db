package pengu

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

type Store struct {
	Path    string
	mu      sync.RWMutex
	journal *Journal
	index   map[string]indexEntry
}

// Open initializes storage and builds the index
func Open(path string) (*Store, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, fmt.Errorf("failed to create path: %w", err)
	}

	journal, err := openJournal(path)
	if err != nil {
		return nil, err
	}

	idx := make(map[string]indexEntry)
	buildIndex := func(rec *Record, offset int64) error {
		if rec.Typ == TypeTombstone {
			delete(idx, string(rec.Key))
		} else {
			idx[string(rec.Key)] = indexEntry{
				offset:  offset,
				valSize: rec.ValSize,
				keySize: rec.KeySize,
			}
		}
		return nil
	}

	if err := journal.replay(buildIndex); err != nil {
		return nil, fmt.Errorf("failed to replay log file: %w", err)
	}

	return &Store{
		Path:    path,
		journal: journal,
		index:   idx,
	}, nil
}

func (s *Store) Close() error {
	return s.journal.file.Close()
}

// Set appends a new key-value pair to the log file.
func (s *Store) Set(key, value []byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	data := encode(TypeNormal, key, value)

	n, err := s.journal.file.Write(data)
	if err != nil {
		return err
	}
	s.journal.file.Sync()

	s.index[string(key)] = indexEntry{
		offset:  s.journal.endOffset,
		keySize: uint32(len(key)),
		valSize: uint32(len(value)),
	}

	s.journal.endOffset += int64(n)

	return nil
}

// Get retrieves the value for the given key.
func (s *Store) Get(key []byte) ([]byte, error) {
	s.mu.RLock()
	entry, exists := s.index[string(key)]
	s.mu.RUnlock()

	if !exists {
		return nil, errors.New("record not found")
	}

	// readValueAt is thread safe, no need to RLock.
	val, err := readValueAt(s.journal.file, entry)
	if err != nil {
		return nil, err
	}

	return val, nil
}

// Delete appends a tombstone record for the given key.
func (s *Store) Delete(key []byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.index[string(key)]; !exists {
		return nil
	}

	data := encode(TypeTombstone, key, nil)

	n, err := s.journal.file.Write(data)
	if err != nil {
		return err
	}
	s.journal.file.Sync()

	delete(s.index, string(key))

	s.journal.endOffset += int64(n)

	return nil
}

// Iter creates a snapshot of the current index, iterates it and applies fn
// to each entry.
//
// The index is only locked during the creation of the snapshot,
// so a long-running fn() does not block index writes.
func (s *Store) Iter(fn func(key, val []byte)) error {
	type snapshotEntry struct {
		key   string
		entry indexEntry
	}

	s.mu.RLock()
	snapshot := make([]snapshotEntry, 0, len(s.index))
	for k, e := range s.index {
		snapshot = append(snapshot, snapshotEntry{key: k, entry: e})
	}
	s.mu.RUnlock()

	for _, e := range snapshot {
		val, err := readValueAt(s.journal.file, e.entry)
		if err != nil {
			return err
		}

		fn([]byte(e.key), val)
	}

	return nil
}
