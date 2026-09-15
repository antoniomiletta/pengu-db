package pengudb

import "errors"

var (
	errRecordNotFound        = errors.New("record not found")
	errCRCMismatch           = errors.New("crc mismatch")
	errLogTooSmall           = errors.New("log file too small for compaction")
	errInsufficientDeadBytes = errors.New("dead bytes below compaction threshold")
)
