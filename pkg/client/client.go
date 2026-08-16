package client

import (
	"bytes"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"

	"github.com/antoniomiletta/pengu/internal/store"
)

type DB struct {
	log    string
	reader io.ReaderAt
	writer io.WriterAt
}

func NewDB(path string) *DB {
	logContainer, err := open(path)
	if err != nil {
		log.Fatalf("failed to open container log file: %v", err)
	}
	return &DB{
		log:    logContainer,
		reader: store.NewReader(),
	}
}

const DataDirPerm os.FileMode = 0o755 // rwxr-xr-x

func open(path string) (string, error) {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, DataDirPerm); err != nil {
		return "", fmt.Errorf("failed to create data directory %w", err)
	}

	_, err := os.Create(path)
	if err != nil {
		return "", fmt.Errorf("failed to create data file: %w", err)
	}

	return path, nil
}

func (d *DB) Set(key, value string) error {
	val := fmt.Appendf(nil, "%s:%s", key, value)

	if err := os.WriteFile(d.log, val, DataDirPerm); err != nil {
		return fmt.Errorf("failed to write to file: %w", err)
	}

	return nil
}

func (d *DB) Get(key string) (string, error) {
	b, err := os.ReadFile(d.log)
	if err != nil {
		return "", fmt.Errorf("pengu: failed to read from file: %w", err)
	}

	bslice := bytes.Split(b, []byte(":"))

	return string(bslice[1]), nil

}

// func (d *DB) Delete(key string) error
