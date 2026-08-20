package pengu

import (
	"encoding/binary"
	"fmt"
	"hash/crc32"
	"io"
)

type Record struct {
	CRC     uint32
	Typ     uint8
	KeySize uint32
	ValSize uint32
	Key     []byte
	Val     []byte
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

// Decode parses a Record from r returning the record and the total bytes read.
// It uses a Reader instead of a file descriptor, avoiding multiple syscalls.
// It can be sequentially called to fully parse a log file.
func Decode(r io.Reader) (*Record, int, error) {
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
		return nil, 0, fmt.Errorf("decode: %w (expected %d, got %d)", ErrCRCMismatch, crc, checksum)
	}

	rec := &Record{
		CRC:     crc,
		Typ:     typ,
		KeySize: keySize,
		ValSize: valSize,
		Key:     key,
		Val:     val,
	}
	n := int(SizeHeader + keySize + valSize)

	return rec, n, nil
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

// DecodeAt parses a Record at the offset and returns it.
// Contrary to Decode, it uses readAt (pread() syscall) to read into the file,
// and should not be called sequentially to parse a full log file.
// func DecodeAt(f *os.File, offset int64) (*Record, error)
