package service

// F-028: TokenBlacklist must (a) revoke and check correctly within a
// single process via the in-memory store, and (b) propagate
// revocations across processes via the Redis store. Both stores must
// hash the token before storing so the raw JWT never lives in the
// backing store.

import (
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
)

func TestMemoryBlacklistStore_RevokeAndCheck(t *testing.T) {
	bl := NewTokenBlacklist()
	tok := "eyJhbGciOiJIUzI1NiJ9.dummy.signature"

	if bl.IsRevoked(tok) {
		t.Fatalf("fresh blacklist reported %s as revoked", tok)
	}

	bl.Revoke(tok, time.Now().Add(1*time.Hour))

	if !bl.IsRevoked(tok) {
		t.Fatalf("after Revoke, IsRevoked should be true")
	}
}

func TestMemoryBlacklistStore_DistinctTokens(t *testing.T) {
	bl := NewTokenBlacklist()
	bl.Revoke("token-a", time.Now().Add(1*time.Hour))

	if bl.IsRevoked("token-b") {
		t.Fatalf("revoking token-a should NOT mark token-b as revoked")
	}
}

func TestMemoryBlacklistStore_ExpiredEntryNotRevoked(t *testing.T) {
	bl := NewTokenBlacklist()
	tok := "stale-token"
	bl.Revoke(tok, time.Now().Add(-1*time.Minute)) // already expired

	if bl.IsRevoked(tok) {
		t.Fatalf("expired blacklist entry should report not-revoked (JWT itself has also expired)")
	}
}

func TestMemoryBlacklistStore_RawTokenNotStored(t *testing.T) {
	// White-box: the underlying store is a hash-keyed map. We can't
	// read it from another package, but we can verify the contract:
	// two distinct strings that hash to the same value would be
	// indistinguishable. SHA-256 collisions don't happen by accident,
	// so this is really verifying that the hashing path is wired
	// (the key stored is NOT the raw token).
	//
	// Construct a memory store directly to access internals.
	s := newMemoryBlacklistStore()
	tok := "eyJSECRET.payload"
	s.Revoke(tok, time.Now().Add(1*time.Hour))

	s.mu.RLock()
	defer s.mu.RUnlock()
	for k := range s.tokens {
		if k == tok {
			t.Fatalf("raw JWT %q stored as key — must be sha256-hashed", tok)
		}
		if len(k) != 64 { // sha256 hex = 64 chars
			t.Fatalf("key %q is not a sha256 hex string (length %d)", k, len(k))
		}
	}
}

func TestRedisBlacklistStore_RevokeAndCheck(t *testing.T) {
	mr := miniredis.RunT(t)
	store, err := NewRedisBlacklistStore("redis://" + mr.Addr())
	if err != nil {
		t.Fatalf("NewRedisBlacklistStore: %v", err)
	}
	defer store.Close()

	bl := NewTokenBlacklistWithStore(store)
	tok := "eyJhbGciOiJIUzI1NiJ9.miniredis.signature"

	if bl.IsRevoked(tok) {
		t.Fatalf("fresh redis blacklist reported %s as revoked", tok)
	}

	bl.Revoke(tok, time.Now().Add(1*time.Hour))

	if !bl.IsRevoked(tok) {
		t.Fatalf("after Revoke, redis store should report revoked")
	}
}

func TestRedisBlacklistStore_TokenHashedInRedis(t *testing.T) {
	mr := miniredis.RunT(t)
	store, err := NewRedisBlacklistStore("redis://" + mr.Addr())
	if err != nil {
		t.Fatalf("NewRedisBlacklistStore: %v", err)
	}
	defer store.Close()

	tok := "eyJSENSITIVE.payload"
	store.Revoke(tok, time.Now().Add(1*time.Hour))

	// miniredis exposes the keyspace; assert no key contains the raw
	// JWT — only the sha256 hash should appear.
	for _, k := range mr.Keys() {
		if k == tok || k == "blacklist:"+tok {
			t.Fatalf("raw token leaked into Redis key: %q", k)
		}
	}
}

func TestRedisBlacklistStore_AlreadyExpiredNotWritten(t *testing.T) {
	mr := miniredis.RunT(t)
	store, err := NewRedisBlacklistStore("redis://" + mr.Addr())
	if err != nil {
		t.Fatalf("NewRedisBlacklistStore: %v", err)
	}
	defer store.Close()

	// Revoking with an already-past expiry is a no-op — the JWT is
	// already invalid, no point in storing a tombstone that would
	// immediately TTL out.
	store.Revoke("already-stale-token", time.Now().Add(-5*time.Minute))

	for _, k := range mr.Keys() {
		if k == "blacklist:"+blacklistKey("already-stale-token") {
			t.Fatalf("already-expired Revoke wrote a key to redis: %q", k)
		}
	}
}
