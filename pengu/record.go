package pengu

import (
	"encoding/binary"
	"fmt"
	"hash/crc32"
	"io"
	"os"
)

type Record struct {
	crc     uint32
	typ     uint8
	keySize uint32
	valSize uint32
	key     []byte
	val     []byte
}

const (
	TypeNormal    = 0x01
	TypeTombstone = 0x02
)

const (
	SizeCRC     = 4
	SizeTyp     = 1
	SizeKeySize = 4
	SizeValSize = 4

	SizeHeader = SizeCRC + SizeTyp + SizeKeySize + SizeValSize

	OffsetCRC     = 0
	OffsetTyp     = OffsetCRC + SizeCRC
	OffsetKeySize = OffsetTyp + SizeTyp
	OffsetValSize = OffsetKeySize + SizeKeySize
)

// decode parses a Record from r returning the record and the total bytes read.
func decode(r io.Reader) (*Record, int, error) {
	var header [SizeHeader]byte
	if _, err := io.ReadFull(r, header[:]); err != nil {
		return nil, 0, err
	}
	crc := binary.BigEndian.Uint32(header[:OffsetTyp])
	typ := header[OffsetTyp]
	keySize := binary.BigEndian.Uint32(header[OffsetKeySize:OffsetValSize])
	valSize := binary.BigEndian.Uint32(header[OffsetValSize:])

	payload := make([]byte, keySize+valSize)
	if _, err := io.ReadFull(r, payload); err != nil {
		return nil, 0, err
	}

	key := payload[:keySize]
	val := payload[keySize:]

	checksum := crc32.ChecksumIEEE(header[SizeCRC:])
	checksum = crc32.Update(checksum, crc32.IEEETable, payload)
	if checksum != crc {
		return nil, 0, fmt.Errorf("decode: crc mismatch (expected %d, got %d)", crc, checksum)
	}

	rec := &Record{
		crc:     crc,
		typ:     typ,
		keySize: keySize,
		valSize: valSize,
		key:     key,
		val:     val,
	}
	n := int(SizeHeader + keySize + valSize)

	return rec, n, nil
}

func readAt(f *os.File, entry indexEntry, keySize uint32) ([]byte, error) {
	valOffset := entry.offset + SizeHeader + int64(keySize)
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

	buf := make([]byte, SizeHeader+payloadSize)

	buf[SizeCRC] = typ
	binary.BigEndian.PutUint32(buf[OffsetKeySize:OffsetValSize], keySize)
	binary.BigEndian.PutUint32(buf[OffsetValSize:SizeHeader], valSize)

	copy(buf[SizeHeader:], key)
	copy(buf[SizeHeader+keySize:], val)

	crc := crc32.ChecksumIEEE(buf[SizeCRC:])
	binary.BigEndian.PutUint32(buf[:SizeCRC], crc)

	return buf
}
