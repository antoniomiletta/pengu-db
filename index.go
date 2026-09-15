package pengudb

type indexEntry struct {
	offset  int64
	keySize uint32
	valSize uint32
}
