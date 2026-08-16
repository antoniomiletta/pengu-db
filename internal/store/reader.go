package store

type Reader struct{}

func NewReader() *Reader {
	return &Reader{}
}

func (r *Reader) ReadAt(p []byte, off int64) (n int, err error)
