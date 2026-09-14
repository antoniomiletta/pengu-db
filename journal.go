package pengu

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"sync/atomic"
	"time"
)

// TODO: Stat() helper for locking and reading endOffset and activeBytes.
type Journal struct {
	Path        string
	file        *os.File
	rc          atomic.Int32
	obsolete    atomic.Bool
	endOffset   int64
	activeBytes int64
}

const LogFileFlags = os.O_CREATE | os.O_RDWR | os.O_APPEND
const LogFilePerm = 0o644

// newJournal returns a journey instance with a master ref.
// You don't have to manually increment rc on every new instance.
func newJournal(path string, f *os.File, endOffset int64, activeBytes int64) *Journal {
	j := &Journal{
		Path:        path,
		file:        f,
		endOffset:   endOffset,
		activeBytes: activeBytes,
	}
	j.rc.Store(1) // Master ref
	return j
}

func (j *Journal) Release() {
	if j.rc.Add(-1) == 0 {
		j.file.Close()

		if j.obsolete.Load() {
			os.Remove(j.Path)
		}
	}
}

// replay scans the log from start to finish, applies fn to each record
// and updates journal metadata.
func (j *Journal) replay(fn func(rec *Record, offset int64) (int64, error)) error {
	if _, err := j.file.Seek(0, io.SeekStart); err != nil {
		return fmt.Errorf("failed to seek to start of journal: %w", err)
	}

	var pos int64
	reader := bufio.NewReaderSize(j.file, 64*1024)
	for {
		rec, n, err := Decode(reader)
		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			if errors.Is(err, io.ErrUnexpectedEOF) || errors.Is(err, ErrCRCMismatch) {
				log.Printf("[WARNING] torn/corrupted record at offset %d. Truncating file.\n", j.endOffset)
				if err := j.file.Truncate(j.endOffset); err != nil {
					return fmt.Errorf("failed to truncate file: %w", err)
				}
				// Seek to truncation point. Truncate does not move the file cursor,
				// next Write would zero-pad the gap between truncation point -> current cursor.
				if _, err := j.file.Seek(j.endOffset, io.SeekStart); err != nil {
					return fmt.Errorf("failed to seek after truncation: %w", err)
				}
				break
			}
			return err
		}

		delta, err := fn(rec, pos)
		if err != nil {
			return err
		}

		pos += int64(n)
		j.endOffset = pos
		j.activeBytes += delta
	}

	return nil
}

// readValueAt reads directly into an index offset and returns the record value,
// ignoring the header and key.
//
// It is thread-safe for concurrent readers and writers, but when used concurrently, it should follow a
// journal.rc increment, so compaction can't delete an obsolete log file while it's still being referenced.
func readValueAt(f *os.File, entry indexEntry) ([]byte, error) {
	valOffset := entry.offset + SizeHeader + int64(entry.keySize)
	val := make([]byte, entry.valSize)

	if _, err := f.ReadAt(val, valOffset); err != nil {
		return nil, err
	}

	return val, nil
}

func StampedLogFile() string {
	return fmt.Sprintf("data-%d", time.Now().UnixNano())
}
