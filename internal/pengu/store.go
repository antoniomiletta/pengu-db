package pengu

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

type Store struct {
	mu      sync.RWMutex
	Journal *Journal
	index   map[string]indexEntry
}

// Open initializes storage and builds the index
func Open(path string) (*Store, error) {
	f, err := os.OpenFile(path, LogFileFlags, LogFilePerm)
	if err != nil {
		return nil, fmt.Errorf("failed to create/open log file: %w", err)
	}

	journal := newJournal(path, f, 0, 0)

	idx := make(map[string]indexEntry)

	buildIndex := func(rec *Record, offset int64) (int64, error) {
		existing, exists := idx[string(rec.Key)]

		if rec.Typ == TypeTombstone {
			var existingSize int64

			if exists {
				delete(idx, string(rec.Key))
				existingSize = int64(SizeHeader + existing.keySize + existing.valSize)
			}

			return -existingSize, nil
		} else {
			var recSize = int64(SizeHeader + rec.KeySize + rec.ValSize)
			var existingSize int64

			if exists {
				existingSize = int64(SizeHeader + existing.keySize + existing.valSize)
			}

			idx[string(rec.Key)] = indexEntry{
				offset:  offset,
				valSize: rec.ValSize,
				keySize: rec.KeySize,
			}

			deltaBytes := recSize - existingSize
			return deltaBytes, nil
		}
	}

	if err := journal.replay(buildIndex); err != nil {
		return nil, fmt.Errorf("failed to replay log file: %w", err)
	}

	return &Store{
		Journal: journal,
		index:   idx,
	}, nil
}

func (s *Store) Close() {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.Journal.obsolete.Store(false)
	s.Journal.Release() // Release Master ref
}

// Set appends a new key-value pair to the log file.
func (s *Store) Set(key, value []byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	data := encode(TypeNormal, key, value)

	n, err := s.Journal.file.Write(data)
	if err != nil {
		return err
	}
	s.Journal.file.Sync()

	var recSize = int64(n)
	var existingSize int64

	existing, exists := s.index[string(key)]
	if exists {
		existingSize = int64(SizeHeader + existing.keySize + existing.valSize)
	}

	s.index[string(key)] = indexEntry{
		offset:  s.Journal.endOffset,
		keySize: uint32(len(key)),
		valSize: uint32(len(value)),
	}

	deltaBytes := recSize - existingSize

	s.Journal.endOffset += int64(n)
	s.Journal.activeBytes += deltaBytes

	return nil
}

// Get retrieves the value for the given key.
func (s *Store) Get(key []byte) ([]byte, error) {
	s.mu.RLock()

	entry, exists := s.index[string(key)]
	if !exists {
		s.mu.RUnlock()
		return nil, errors.New("record not found")
	}

	j := s.Journal
	j.rc.Add(1)
	defer j.Release()

	s.mu.RUnlock()

	// read thread safely, j wont be removed unless it's obsolete.
	val, err := readValueAt(j.file, entry)
	if err != nil {
		return nil, err
	}

	return val, nil
}

// Delete appends a tombstone record for the given key.
func (s *Store) Delete(key []byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	var existingSize int64

	existing, exists := s.index[string(key)]
	if exists {
		existingSize = int64(SizeHeader + existing.keySize + existing.valSize)
	} else {
		// Ignore tombstone to inexistent keys
		return nil
	}

	data := encode(TypeTombstone, key, nil)

	n, err := s.Journal.file.Write(data)
	if err != nil {
		return err
	}
	s.Journal.file.Sync()

	delete(s.index, string(key))

	s.Journal.endOffset += int64(n)
	s.Journal.activeBytes -= existingSize

	return nil
}

// Iter creates a snapshot of the current index, iterates it and applies fn
// to the record corresponding to each entry.
//
// The index is only locked during the creation of the snapshot,
// so a long-running fn() does not block index writes.
func (s *Store) Iter(fn func(key, val []byte) error) error {
	type snapshotEntry struct {
		key   string
		entry indexEntry
	}

	s.mu.RLock()

	snapshot := make([]snapshotEntry, 0, len(s.index))
	for k, e := range s.index {
		snapshot = append(snapshot, snapshotEntry{key: k, entry: e})
	}

	j := s.Journal
	j.rc.Add(1)
	defer j.Release()

	s.mu.RUnlock()

	for _, e := range snapshot {
		val, err := readValueAt(j.file, e.entry)
		if err != nil {
			return err
		}

		fn([]byte(e.key), val)
	}

	return nil
}

func (s *Store) Compact() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	var success bool

	baseName := StampedLogFile()
	tmpPath := filepath.Join(filepath.Dir(s.Journal.Path), baseName+".tmp")
	finalPath := filepath.Join(filepath.Dir(s.Journal.Path), baseName+".log")

	tmp, err := os.Create(tmpPath)
	if err != nil {
		return fmt.Errorf("failed to create temp file: %w", err)
	}
	defer func() {
		if !success {
			os.Remove(tmpPath)
		}
	}()

	newOffset := int64(0)
	newIdx := make(map[string]indexEntry, len(s.index))
	writer := bufio.NewWriterSize(tmp, 64*1024)

	for key, entry := range s.index {
		val, err := readValueAt(s.Journal.file, entry)
		if err != nil {
			return err
		}

		data := encode(TypeNormal, []byte(key), val)

		n, err := writer.Write(data)
		if err != nil {
			return err
		}

		newIdx[key] = indexEntry{
			offset:  newOffset,
			keySize: uint32(len(key)),
			valSize: uint32(len(val)),
		}

		newOffset += int64(n)
	}

	if err := writer.Flush(); err != nil {
		return err
	}

	if err := tmp.Sync(); err != nil {
		return err
	}

	if err := os.Rename(tmpPath, finalPath); err != nil {
		return err
	}

	newJournal := newJournal(finalPath, tmp, newOffset, newOffset)

	old := s.Journal
	s.Journal = newJournal
	s.index = newIdx

	old.obsolete.Store(true)
	old.Release()

	success = true
	return nil
}

func StampedLogFile() string {
	return fmt.Sprintf("data-%d", time.Now().UnixNano())
}
