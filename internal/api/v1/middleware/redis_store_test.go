package middleware

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
)

// newTestStore spins up an in-process miniredis and returns a RedisStore
// wired to it plus a cleanup. miniredis supports the Lua subset we use
// (ZADD, ZCARD, ZRANGE, ZREMRANGEBYSCORE, PEXPIRE).
func newTestStore(t *testing.T) (*RedisStore, *miniredis.Miniredis) {
	t.Helper()
	mr := miniredis.RunT(t)
	store, err := NewRedisStore("redis://" + mr.Addr())
	if err != nil {
		t.Fatalf("NewRedisStore: %v", err)
	}
	t.Cleanup(func() {
		_ = store.Close()
	})
	return store, mr
}

func TestRedisStore_FirstNRequestsAllowed(t *testing.T) {
	store, _ := newTestStore(t)
	const max = 5
	window := time.Second

	for i := 0; i < max; i++ {
		allowed, remaining, retry := store.Allow("ip:1.2.3.4", max, window)
		if !allowed {
			t.Fatalf("request %d: expected allowed, got denied (retry=%v)", i+1, retry)
		}
		wantRemaining := max - 1 - i
		if remaining != wantRemaining {
			t.Errorf("request %d: remaining = %d, want %d", i+1, remaining, wantRemaining)
		}
	}
}

func TestRedisStore_NPlusOneDenied(t *testing.T) {
	store, _ := newTestStore(t)
	const max = 3
	window := time.Minute

	for i := 0; i < max; i++ {
		allowed, _, _ := store.Allow("user:42", max, window)
		if !allowed {
			t.Fatalf("setup: request %d should have been allowed", i+1)
		}
	}

	allowed, remaining, retry := store.Allow("user:42", max, window)
	if allowed {
		t.Fatal("N+1th request: expected denied, got allowed")
	}
	if remaining != 0 {
		t.Errorf("denied request: remaining = %d, want 0", remaining)
	}
	if retry <= 0 {
		t.Errorf("denied request: retryAfter = %v, want > 0", retry)
	}
	if retry > window {
		t.Errorf("denied request: retryAfter = %v, want <= %v", retry, window)
	}
}

func TestRedisStore_AfterWindowExpiresAllowedAgain(t *testing.T) {
	store, mr := newTestStore(t)
	const max = 2
	window := 500 * time.Millisecond

	// Exhaust the budget.
	for i := 0; i < max; i++ {
		allowed, _, _ := store.Allow("ip:9.9.9.9", max, window)
		if !allowed {
			t.Fatalf("setup: request %d should have been allowed", i+1)
		}
	}
	if allowed, _, _ := store.Allow("ip:9.9.9.9", max, window); allowed {
		t.Fatal("setup: expected N+1th to be denied")
	}

	// FastForward miniredis past the window. miniredis only advances the
	// keyspace clock; the script uses time.Now() in the Go side via the
	// `now` ARGV. So we need to sleep a hair past the window in real time
	// AND fast-forward miniredis so PEXPIRE'd ZSETs get garbage-collected.
	mr.FastForward(window + 100*time.Millisecond)
	time.Sleep(window + 50*time.Millisecond)

	allowed, _, _ := store.Allow("ip:9.9.9.9", max, window)
	if !allowed {
		t.Fatal("after window expires: expected allowed, got denied")
	}
}

func TestRedisStore_IndependentBudgetsPerKey(t *testing.T) {
	store, _ := newTestStore(t)
	const max = 2
	window := time.Minute

	// Exhaust user:1.
	for i := 0; i < max; i++ {
		if allowed, _, _ := store.Allow("user:1", max, window); !allowed {
			t.Fatalf("setup user:1 request %d should have been allowed", i+1)
		}
	}
	if allowed, _, _ := store.Allow("user:1", max, window); allowed {
		t.Fatal("setup: expected user:1 N+1th to be denied")
	}

	// user:2 should be unaffected.
	for i := 0; i < max; i++ {
		allowed, remaining, _ := store.Allow("user:2", max, window)
		if !allowed {
			t.Fatalf("user:2 request %d: expected allowed, got denied", i+1)
		}
		wantRemaining := max - 1 - i
		if remaining != wantRemaining {
			t.Errorf("user:2 request %d: remaining = %d, want %d", i+1, remaining, wantRemaining)
		}
	}
}

func TestRedisStore_FailOpenOnConnectionError(t *testing.T) {
	// Build a store pointed at a dead Redis: spin up miniredis, then
	// close it. Allow() should LOG and return allowed=true rather than
	// propagate the connection error.
	mr := miniredis.RunT(t)
	store, err := NewRedisStore("redis://" + mr.Addr())
	if err != nil {
		t.Fatalf("NewRedisStore: %v", err)
	}
	mr.Close() // kill the backend mid-test

	// Sanity check: a direct PING should now fail.
	ctx, cancel := context.WithTimeout(context.Background(), 250*time.Millisecond)
	defer cancel()
	if err := store.client.Ping(ctx).Err(); err == nil {
		t.Fatal("expected PING to fail after miniredis.Close()")
	}

	allowed, remaining, retry := store.Allow("ip:fail", 5, time.Minute)
	if !allowed {
		t.Errorf("fail-open: expected allowed=true, got false")
	}
	if remaining != 5 {
		t.Errorf("fail-open: expected remaining=max (5), got %d", remaining)
	}
	if retry != 0 {
		t.Errorf("fail-open: expected retryAfter=0, got %v", retry)
	}
}

// TestRedisStore_BadURLAtConstruction verifies NewRedisStore surfaces
// errors at boot rather than deferring them to first request.
func TestRedisStore_BadURLAtConstruction(t *testing.T) {
	_, err := NewRedisStore("not-a-valid-url")
	if err == nil {
		t.Fatal("expected NewRedisStore to reject a malformed URL")
	}
}

// Sanity: ensure RedisStore satisfies the Store interface.
var _ Store = (*RedisStore)(nil)
