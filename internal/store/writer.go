package store

type Writer struct{}

func NewWriter() *Writer {
	return &Writer{}
}

func (w *Writer) WriteAt(p []byte, off int64) (n int, err error)
