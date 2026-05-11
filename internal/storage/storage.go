// Package storage provides a pluggable file storage abstraction.
// Implementations include local disk and S3-compatible object storage.
package storage

import (
	"context"
	"io"
)

// Backend is the interface for file storage operations.
// All implementations must be safe for concurrent use.
type Backend interface {
	// Put stores a file at the given key, reading from r.
	// The key is a relative path (e.g., "Course/123/uuid/file.pdf").
	Put(ctx context.Context, key string, r io.Reader, contentType string) error

	// Get returns a ReadCloser for the file at the given key.
	// The caller must close the reader when done.
	Get(ctx context.Context, key string) (io.ReadCloser, error)

	// Delete removes the file at the given key.
	Delete(ctx context.Context, key string) error

	// URL returns a URL for downloading the file at the given key.
	// For local storage, this returns the local file path.
	// For S3, this returns a presigned URL.
	URL(ctx context.Context, key string) (string, error)
}
