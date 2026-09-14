package pengu

import "errors"

var (
	ErrRecordNotFound = errors.New("record not found")
	ErrCRCMismatch    = errors.New("crc mismatch")
)
