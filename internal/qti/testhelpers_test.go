package qti

import (
	"archive/zip"
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// makeIMSCCFromDir walks a fixture directory and produces an in-memory
// .imscc zip suitable for openIMSCCFromBytes(). Used by tests so we
// can keep the fixture as readable XML files rather than a binary blob.
func makeIMSCCFromDir(t *testing.T, dir string) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	if err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(dir, path)
		if err != nil {
			return err
		}
		rel = strings.ReplaceAll(rel, "\\", "/")
		w, err := zw.Create(rel)
		if err != nil {
			return err
		}
		f, err := os.Open(path)
		if err != nil {
			return err
		}
		defer f.Close()
		_, err = io.Copy(w, f)
		return err
	}); err != nil {
		t.Fatalf("walk fixture dir: %v", err)
	}
	if err := zw.Close(); err != nil {
		t.Fatalf("close zip: %v", err)
	}
	return buf.Bytes()
}

// writeTempZip writes the zip bytes to a temp file and returns the path.
// Used when the importer entry point requires a path (openIMSCC).
func writeTempZip(t *testing.T, data []byte) string {
	t.Helper()
	f, err := os.CreateTemp("", "qti-test-*.imscc")
	if err != nil {
		t.Fatalf("create temp: %v", err)
	}
	if _, err := f.Write(data); err != nil {
		t.Fatalf("write temp: %v", err)
	}
	f.Close()
	t.Cleanup(func() { _ = os.Remove(f.Name()) })
	return f.Name()
}
