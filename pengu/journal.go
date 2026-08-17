package pengu

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
)

type Journal struct {
	file      *os.File
	endOffset int64
}

const LogFileFlags = os.O_CREATE | os.O_RDWR | os.O_APPEND
const LogFilePerm = 0o644

func openJournal(path string) (*Journal, error) {
	f, err := os.OpenFile(path, LogFileFlags, LogFilePerm)
	if err != nil {
		return nil, fmt.Errorf("failed to open log file: %w", err)
	}

	return &Journal{
		file: f,
	}, nil
}

// replay scans the log from start to finish
func (j *Journal) replay(fn func(rec *Record, offset int64) error) error {
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
			// TODO: handle unexpected EOF
			return err
		}

		if err := fn(rec, pos); err != nil {
			return err
		}

		pos += int64(n)
		j.endOffset = pos
	}

	return nil
}

func (j *Journal) close() error {
	return j.file.Close()
}
