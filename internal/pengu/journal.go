package pengu

import (
	"bufio"
	"encoding/binary"
	"errors"
	"fmt"
	"hash/crc32"
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

// replay scans the log from start to finish, applies fn to each record.
// and updates the journal end offset
func (j *Journal) replay(fn func(rec *Record, offset int64) error) error {
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

// readValueAt reads directly into an index offset and returns its value,
// ignoring the header and key.
func readValueAt(f *os.File, entry indexEntry) ([]byte, error) {
	valOffset := entry.offset + SizeHeader + int64(entry.keySize)
	val := make([]byte, entry.valSize)

	if _, err := f.ReadAt(val, valOffset); err != nil {
		return nil, err
	}

	return val, nil
}

func encode(typ uint8, key, val []byte) []byte {
	keySize := uint32(len(key))
	valSize := uint32(len(val))
	payloadSize := keySize + valSize

	buf := make([]byte, 0, SizeHeader+payloadSize)

	// CRC placeholder
	buf = append(buf, 0, 0, 0, 0)

	buf = append(buf, typ)

	buf = binary.BigEndian.AppendUint32(buf, keySize)
	buf = binary.BigEndian.AppendUint32(buf, valSize)

	buf = append(buf, key...)
	buf = append(buf, val...)

	crc := crc32.ChecksumIEEE(buf[SizeCRC:])
	binary.BigEndian.PutUint32(buf[:SizeCRC], crc)

	return buf
}
