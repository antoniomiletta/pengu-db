package pengudb

import (
	"bufio"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sync"
)

type Store struct {
	Cfg     StoreConfig
	mu      sync.RWMutex
	journal *journal
	index   map[string]indexEntry
}

type StoreConfig struct {
	// Minumum file size in bytes to allow compaction.
	CompactionMinSize int64
	// Minimum dead byte ratio to allow compaction (0.5 = 50% dead bytes or more).
	CompactionDeadRatio float64
}

// Open initializes storage and builds the index
func Open(dir string, cfg ...StoreConfig) (*Store, error) {
	s := &Store{}

	switch len(cfg) {
	case 0:
		s.Cfg = StoreConfig{
			CompactionMinSize:   1024 * 1024 * 5,
			CompactionDeadRatio: 0.5,
		}
	case 1:
		s.Cfg = cfg[0]
	default:
		return nil, errors.New("Open accepts at most one StoreConfig")
	}

	if err := os.MkdirAll(dir, 0o755); err != nil {
		log.Fatalf("failed to create path: %v", err)
	}

	logs, err := s.discoverLogs(dir)
	if err != nil {
		return nil, err
	}

	var latestLog string
	if len(logs) == 0 {
		latestLog = stampedLogFile()
	} else {
		latestLog = s.latestLog(logs)
	}

	target := fmt.Sprintf("%s/%s", dir, latestLog)

	f, err := os.OpenFile(target, logFileFlags, logFilePerm)
	if err != nil {
		return nil, fmt.Errorf("failed to create/open log file: %w", err)
	}

	s.journal = newJournal(dir, f, 0, 0)
	s.index = make(map[string]indexEntry)

	buildIndex := func(rec *record, offset int64) (int64, error) {
		existing, exists := s.index[string(rec.key)]

		if rec.typ == typeTombstone {
			var existingSize int64

			if exists {
				delete(s.index, string(rec.key))
				existingSize = int64(sizeHeader + existing.keySize + existing.valSize)
			}

			return -existingSize, nil
		} else {
			var recSize = int64(sizeHeader + rec.keySize + rec.valSize)
			var existingSize int64

			if exists {
				existingSize = int64(sizeHeader + existing.keySize + existing.valSize)
			}

			s.index[string(rec.key)] = indexEntry{
				offset:  offset,
				valSize: rec.valSize,
				keySize: rec.keySize,
			}

			deltaBytes := recSize - existingSize
			return deltaBytes, nil
		}
	}

	if err := s.journal.replay(buildIndex); err != nil {
		return nil, fmt.Errorf("failed to replay log file: %w", err)
	}

	return s, nil
}

func (s *Store) Close() {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.journal.obsolete.Store(false)
	s.journal.release() // Release Master ref
}

// Set appends a new key-value pair to the log file.
func (s *Store) Set(key, value []byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	data := encode(typeNormal, key, value)

	n, err := s.journal.file.Write(data)
	if err != nil {
		return err
	}
	s.journal.file.Sync()

	var recSize = int64(n)
	var existingSize int64

	existing, exists := s.index[string(key)]
	if exists {
		existingSize = int64(sizeHeader + existing.keySize + existing.valSize)
	}

	s.index[string(key)] = indexEntry{
		offset:  s.journal.endOffset,
		keySize: uint32(len(key)),
		valSize: uint32(len(value)),
	}

	deltaBytes := recSize - existingSize

	s.journal.endOffset += int64(n)
	s.journal.activeBytes += deltaBytes

	return nil
}

// Get retrieves the value for the given key.
func (s *Store) Get(key []byte) ([]byte, error) {
	s.mu.RLock()

	entry, exists := s.index[string(key)]
	if !exists {
		s.mu.RUnlock()
		return nil, errRecordNotFound
	}

	j := s.journal
	j.rc.Add(1)
	defer j.release()

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
		existingSize = int64(sizeHeader + existing.keySize + existing.valSize)
	} else {
		// Ignore tombstone to inexistent keys
		return nil
	}

	data := encode(typeTombstone, key, nil)

	n, err := s.journal.file.Write(data)
	if err != nil {
		return err
	}
	s.journal.file.Sync()

	delete(s.index, string(key))

	s.journal.endOffset += int64(n)
	s.journal.activeBytes -= existingSize

	return nil
}

func (s *Store) Keys() [][]byte {
	s.mu.RLock()
	defer s.mu.RUnlock()

	keys := make([][]byte, 0, len(s.index))
	for k := range s.index {
		keys = append(keys, []byte(k))
	}
	return keys
}

// Iter creates a snapshot of the current index, iterates it and applies fn
// to each entry.
//
// The lock is only held during the creation of the snapshot,
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

	j := s.journal
	j.rc.Add(1)
	defer j.release()

	s.mu.RUnlock()

	for _, e := range snapshot {
		val, err := readValueAt(j.file, e.entry)
		if err != nil {
			return err
		}

		if err := fn([]byte(e.key), val); err != nil {
			return err
		}
	}

	return nil
}

func (s *Store) Compact() error {
	if s.journal.endOffset < s.Cfg.CompactionMinSize {
		return errLogTooSmall
	}

	deadBytes := s.journal.endOffset - s.journal.activeBytes
	deadRatio := float64(deadBytes / s.journal.endOffset)
	if deadRatio < s.Cfg.CompactionDeadRatio {
		return errInsufficientDeadBytes
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	var success bool

	baseName := stampedLogFile()
	tmpPath := filepath.Join(s.journal.path, baseName+".tmp")
	finalPath := filepath.Join(s.journal.path, baseName)

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
		val, err := readValueAt(s.journal.file, entry)
		if err != nil {
			return err
		}

		data := encode(typeNormal, []byte(key), val)

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

	old := s.journal
	s.journal = newJournal
	s.index = newIdx

	old.obsolete.Store(true)
	old.release()

	success = true
	return nil
}

func (s *Store) discoverLogs(dir string) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}

	var logFiles []string
	for _, entry := range entries {
		if !entry.IsDir() && filepath.Ext(entry.Name()) == ".log" {
			logFiles = append(logFiles, entry.Name())
		}
	}

	return logFiles, nil
}

func (s *Store) latestLog(logs []string) string {
	var latest string
	for _, log := range logs {
		if log > latest {
			latest = log
		}
	}

	return latest
}
