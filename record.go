package pengudb

import (
	"encoding/binary"
	"fmt"
	"hash/crc32"
	"io"
	"os"
)

type record struct {
	crc     uint32
	typ     uint8
	keySize uint32
	valSize uint32
	key     []byte
	val     []byte
}

const (
	typeNormal    = 0x01
	typeTombstone = 0x02
)

const (
	sizeCRC     = 4
	sizeTyp     = 1
	sizeKeySize = 4
	sizeValSize = 4

	sizeHeader = sizeCRC + sizeTyp + sizeKeySize + sizeValSize

	offsetCRC     = 0
	offsetTyp     = offsetCRC + sizeCRC
	offsetKeySize = offsetTyp + sizeTyp
	offsetValSize = offsetKeySize + sizeKeySize
)

// decode parses a Record from r returning the record and the total bytes read.
// It uses a Reader instead of a file descriptor, avoiding multiple syscalls.
// It can be sequentially called to fully parse a log file.
func decode(r io.Reader) (*record, int, error) {
	var header [sizeHeader]byte
	if _, err := io.ReadFull(r, header[:]); err != nil {
		return nil, 0, err
	}
	crc := binary.BigEndian.Uint32(header[:offsetTyp])
	typ := header[offsetTyp]
	keySize := binary.BigEndian.Uint32(header[offsetKeySize:offsetValSize])
	valSize := binary.BigEndian.Uint32(header[offsetValSize:])

	payload := make([]byte, keySize+valSize)
	if _, err := io.ReadFull(r, payload); err != nil {
		return nil, 0, err
	}

	key := payload[:keySize]
	val := payload[keySize:]

	checksum := crc32.ChecksumIEEE(header[sizeCRC:])
	checksum = crc32.Update(checksum, crc32.IEEETable, payload)
	if checksum != crc {
		return nil, 0, fmt.Errorf("decode: %w (expected %d, got %d)", errCRCMismatch, crc, checksum)
	}

	rec := &record{
		crc:     crc,
		typ:     typ,
		keySize: keySize,
		valSize: valSize,
		key:     key,
		val:     val,
	}
	n := int(sizeHeader + keySize + valSize)

	return rec, n, nil
}

func encode(typ uint8, key, val []byte) []byte {
	keySize := uint32(len(key))
	valSize := uint32(len(val))
	payloadSize := keySize + valSize

	buf := make([]byte, 0, sizeHeader+payloadSize)

	// CRC placeholder
	buf = append(buf, 0, 0, 0, 0)

	buf = append(buf, typ)

	buf = binary.BigEndian.AppendUint32(buf, keySize)
	buf = binary.BigEndian.AppendUint32(buf, valSize)

	buf = append(buf, key...)
	buf = append(buf, val...)

	crc := crc32.ChecksumIEEE(buf[sizeCRC:])
	binary.BigEndian.PutUint32(buf[:sizeCRC], crc)

	return buf
}

// decodeAt parses a Record at the offset and returns it.
// Contrary to Decode, it uses readAt (pread() syscall) to read into the file,
// and should not be called sequentially to parse a full log file.
func decodeAt(f *os.File, offset int64) (*record, error) {
	var header [sizeHeader]byte
	if _, err := f.ReadAt(header[:], offset); err != nil {
		return nil, err
	}

	crc := binary.BigEndian.Uint32(header[:offsetTyp])
	typ := header[offsetTyp]
	keySize := binary.BigEndian.Uint32(header[offsetKeySize:offsetValSize])
	valSize := binary.BigEndian.Uint32(header[offsetValSize:])

	payload := make([]byte, keySize+valSize)
	if _, err := f.ReadAt(payload, offset+sizeHeader); err != nil {
		return nil, err
	}

	key := payload[:keySize]
	val := payload[keySize:]

	checksum := crc32.ChecksumIEEE(header[sizeCRC:])
	checksum = crc32.Update(checksum, crc32.IEEETable, payload)
	if checksum != crc {
		return nil, fmt.Errorf("decode: %w (expected %d, got %d)", errCRCMismatch, crc, checksum)
	}

	rec := &record{
		crc:     crc,
		typ:     typ,
		keySize: keySize,
		valSize: valSize,
		key:     key,
		val:     val,
	}

	return rec, nil
}
