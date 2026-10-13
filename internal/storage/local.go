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

// resolve maps a storage key to a path under basePath. Keys are built by
// the services (never raw user input), but an absolute key or one with
// ".." segments is refused so the backend can't be steered outside its
// directory.
func (l *LocalBackend) resolve(key string) (string, error) {
	if !filepath.IsLocal(key) {
		return "", fmt.Errorf("storage: invalid key %q", key)
	}
	return filepath.Join(l.basePath, key), nil
}

func (l *LocalBackend) Put(_ context.Context, key string, r io.Reader, _ string) error {
	fullPath, err := l.resolve(key)
	if err != nil {
		return err
	}

	dir := filepath.Dir(fullPath)
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return fmt.Errorf("storage: could not create directory %s: %w", dir, err)
	}

	f, err := os.Create(fullPath) // #nosec G304 -- resolve() keeps the path under basePath
	if err != nil {
		return fmt.Errorf("storage: could not create file %s: %w", fullPath, err)
	}
	if _, err := io.Copy(f, r); err != nil {
		_ = f.Close()
		return fmt.Errorf("storage: could not write file %s: %w", fullPath, err)
	}

	if err := f.Close(); err != nil {
		return fmt.Errorf("storage: could not close file %s: %w", fullPath, err)
	}

	return nil
}

func (l *LocalBackend) Get(_ context.Context, key string) (io.ReadCloser, error) {
	fullPath, err := l.resolve(key)
	if err != nil {
		return nil, err
	}
	f, err := os.Open(fullPath) // #nosec G304 -- resolve() keeps the path under basePath
	if err != nil {
		return nil, fmt.Errorf("storage: could not open file %s: %w", fullPath, err)
	}
	return f, nil
}

func (l *LocalBackend) Delete(_ context.Context, key string) error {
	fullPath, err := l.resolve(key)
	if err != nil {
		return err
	}
	if err := os.Remove(fullPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("storage: could not delete file %s: %w", fullPath, err)
	}
	return nil
}

func (l *LocalBackend) URL(_ context.Context, key string) (string, error) {
	return l.resolve(key)
}
