package pengudb

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

type journal struct {
	path        string
	file        *os.File
	rc          atomic.Int32
	obsolete    atomic.Bool
	endOffset   int64
	activeBytes int64
}

const logFileFlags = os.O_CREATE | os.O_RDWR | os.O_APPEND
const logFilePerm = 0o644

// newJournal returns a journey instance with a master ref.
// You don't have to manually increment rc on every new instance.
func newJournal(path string, f *os.File, endOffset int64, activeBytes int64) *journal {
	j := &journal{
		path:        path,
		file:        f,
		endOffset:   endOffset,
		activeBytes: activeBytes,
	}
	j.rc.Store(1) // Master ref
	return j
}

func (j *journal) release() {
	if j.rc.Add(-1) == 0 {
		j.file.Close()

		if j.obsolete.Load() {
			os.Remove(j.path)
		}
	}
}

// replay scans the log from start to finish, applying fn to each valid record.
// It automatically truncates the log on invalid records and updates journal metadata.
//
// fn is expected to return the net change in live bytes after each call.
func (j *journal) replay(fn func(rec *record, offset int64) (int64, error)) error {
	if _, err := j.file.Seek(0, io.SeekStart); err != nil {
		return fmt.Errorf("failed to seek to start of journal: %w", err)
	}

	var pos int64
	reader := bufio.NewReaderSize(j.file, 64*1024)

	for {
		rec, n, err := decode(reader)
		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			if errors.Is(err, io.ErrUnexpectedEOF) || errors.Is(err, errCRCMismatch) {
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
	valOffset := entry.offset + sizeHeader + int64(entry.keySize)
	val := make([]byte, entry.valSize)

	if _, err := f.ReadAt(val, valOffset); err != nil {
		return nil, err
	}

	return val, nil
}

func stampedLogFile() string {
	return fmt.Sprintf("data-%d.log", time.Now().UnixNano())
}
