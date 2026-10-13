package storage

import (
	"context"
	"io"
	"strings"
	"testing"
)

func TestLocalBackendRoundTrip(t *testing.T) {
	l := NewLocalBackend(t.TempDir())
	ctx := context.Background()
	if err := l.Put(ctx, "files/1/notes.txt", strings.NewReader("hello"), "text/plain"); err != nil {
		t.Fatalf("Put: %v", err)
	}
	r, err := l.Get(ctx, "files/1/notes.txt")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	b, _ := io.ReadAll(r)
	_ = r.Close()
	if string(b) != "hello" {
		t.Fatalf("got %q", b)
	}
	if err := l.Delete(ctx, "files/1/notes.txt"); err != nil {
		t.Fatalf("Delete: %v", err)
	}
}

func TestLocalBackendRejectsKeysOutsideBase(t *testing.T) {
	l := NewLocalBackend(t.TempDir())
	ctx := context.Background()
	for _, key := range []string{"../escape.txt", "files/../../escape.txt", "/etc/hosts", ""} {
		if err := l.Put(ctx, key, strings.NewReader("x"), ""); err == nil {
			t.Errorf("Put(%q) succeeded", key)
		}
		if _, err := l.Get(ctx, key); err == nil {
			t.Errorf("Get(%q) succeeded", key)
		}
		if err := l.Delete(ctx, key); err == nil {
			t.Errorf("Delete(%q) succeeded", key)
		}
		if _, err := l.URL(ctx, key); err == nil {
			t.Errorf("URL(%q) succeeded", key)
		}
	}
}
