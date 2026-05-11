package storage

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// LocalBackend stores files on the local filesystem.
type LocalBackend struct {
	basePath string
}

// NewLocalBackend creates a LocalBackend that stores files under basePath.
func NewLocalBackend(basePath string) *LocalBackend {
	return &LocalBackend{basePath: basePath}
}

func (l *LocalBackend) Put(_ context.Context, key string, r io.Reader, _ string) error {
	fullPath := filepath.Join(l.basePath, key)

	dir := filepath.Dir(fullPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("storage: could not create directory %s: %w", dir, err)
	}

	f, err := os.Create(fullPath)
	if err != nil {
		return fmt.Errorf("storage: could not create file %s: %w", fullPath, err)
	}
	defer f.Close()

	if _, err := io.Copy(f, r); err != nil {
		return fmt.Errorf("storage: could not write file %s: %w", fullPath, err)
	}

	return nil
}

func (l *LocalBackend) Get(_ context.Context, key string) (io.ReadCloser, error) {
	fullPath := filepath.Join(l.basePath, key)
	f, err := os.Open(fullPath)
	if err != nil {
		return nil, fmt.Errorf("storage: could not open file %s: %w", fullPath, err)
	}
	return f, nil
}

func (l *LocalBackend) Delete(_ context.Context, key string) error {
	fullPath := filepath.Join(l.basePath, key)
	if err := os.Remove(fullPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("storage: could not delete file %s: %w", fullPath, err)
	}
	return nil
}

func (l *LocalBackend) URL(_ context.Context, key string) (string, error) {
	return filepath.Join(l.basePath, key), nil
}
