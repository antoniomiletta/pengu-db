package pengu

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

type Store struct {
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
		if rec.typ == TypeTombstone {
			delete(idx, string(rec.key))
		} else {
			idx[string(rec.key)] = indexEntry{
				offset:  offset,
				valSize: uint32(rec.valSize),
			}
		}
		return nil
	}

	if err := journal.replay(buildIndex); err != nil {
		journal.close()
		return nil, fmt.Errorf("failed to replay log file: %w", err)
	}

	return &Store{
		journal: journal,
		index:   idx,
	}, nil
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

	s.index[string(key)] = indexEntry{
		offset:  s.journal.endOffset,
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
		return nil, os.ErrNotExist
	}

	data, err := readAt(s.journal.file, entry, uint32(len(key)))
	if err != nil {
		return nil, err
	}

	return data, nil
}

// Delete appends a tombstone record for the given key.
func (s *Store) Delete(key []byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.index[string(key)]; !exists {
		println("tem nao pai")
		return nil
	}

	data := encode(TypeTombstone, key, nil)

	n, err := s.journal.file.Write(data)
	if err != nil {
		return err
	}

	delete(s.index, string(key))

	s.journal.endOffset += int64(n)

	return nil
}
