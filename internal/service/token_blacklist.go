package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
)

// blacklistKey returns the storage key for a given JWT. Tokens are
// long (~300+ bytes); SHA-256 hashing keeps the in-memory map and the
// Redis key namespace bounded and stops the raw JWT from sitting in
// logs / dump files / a debugger's local-variable view if someone
// captures the process state.
func blacklistKey(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

// BlacklistStore is the storage backend for revoked JWTs. In-memory
// is the default; multi-pod deployments wire a RedisBlacklistStore
// via NewRedisBlacklistStore so a logout on pod A propagates
// immediately to pods B…N.
//
// SECURITY (F-028): the pre-fix in-memory-only implementation meant a
// logged-out token kept working on every pod that hadn't yet seen the
// Revoke call — for up to the token's full 24-hour TTL.
type BlacklistStore interface {
	Revoke(token string, expiresAt time.Time)
	IsRevoked(token string) bool
}

// TokenBlacklist wraps a BlacklistStore. Callers continue to use the
// same Revoke / IsRevoked API; the underlying implementation is
// chosen at construction (see NewTokenBlacklist / NewTokenBlacklistWithStore).
type TokenBlacklist struct {
	store BlacklistStore
}

// NewTokenBlacklist returns a TokenBlacklist backed by the legacy
// in-memory store. Single-process / dev deployments use this.
func NewTokenBlacklist() *TokenBlacklist {
	return &TokenBlacklist{store: newMemoryBlacklistStore()}
}

// NewTokenBlacklistWithStore lets cmd/server/main.go inject a custom
// store (typically RedisBlacklistStore) so multi-pod deployments
// share the revocation set.
func NewTokenBlacklistWithStore(store BlacklistStore) *TokenBlacklist {
	if store == nil {
		store = newMemoryBlacklistStore()
	}
	return &TokenBlacklist{store: store}
}

func (bl *TokenBlacklist) Revoke(token string, expiresAt time.Time) {
	bl.store.Revoke(token, expiresAt)
}

func (bl *TokenBlacklist) IsRevoked(token string) bool {
	return bl.store.IsRevoked(token)
}

// ---------- in-memory store (default) ----------

type memoryBlacklistStore struct {
	mu      sync.RWMutex
	tokens  map[string]time.Time // hashed_token -> expiry
	cleanup *time.Ticker
}

func newMemoryBlacklistStore() *memoryBlacklistStore {
	s := &memoryBlacklistStore{
		tokens:  make(map[string]time.Time),
		cleanup: time.NewTicker(10 * time.Minute),
	}
	go s.purgeLoop()
	return s
}

func (s *memoryBlacklistStore) Revoke(token string, expiresAt time.Time) {
	key := blacklistKey(token)
	s.mu.Lock()
	defer s.mu.Unlock()
	s.tokens[key] = expiresAt
}

func (s *memoryBlacklistStore) IsRevoked(token string) bool {
	key := blacklistKey(token)
	s.mu.RLock()
	defer s.mu.RUnlock()
	exp, revoked := s.tokens[key]
	if !revoked {
		return false
	}
	// If the entry is stale, treat as not-revoked (the JWT will have
	// expired anyway). Purge loop will reclaim the memory.
	return time.Now().Before(exp)
}

func (s *memoryBlacklistStore) purgeLoop() {
	for range s.cleanup.C {
		s.mu.Lock()
		now := time.Now()
		for k, exp := range s.tokens {
			if now.After(exp) {
				delete(s.tokens, k)
			}
		}
		s.mu.Unlock()
	}
}

// ---------- Redis store (cluster-shared) ----------

// RedisBlacklistStore is a cluster-shared BlacklistStore implementation
// backed by a Redis instance. Each revoked token is written under
// `blacklist:<hash>` with a TTL equal to the JWT's remaining lifetime —
// the entry self-deletes when the JWT would have expired anyway.
//
// Key format: "blacklist:" + sha256(token). The hash means Redis
// doesn't store the raw JWT; an attacker reading the Redis dump
// can't replay a revoked token (because the JWT is already revoked)
// but can't read the original token either.
type RedisBlacklistStore struct {
	client *redis.Client
	prefix string
}

// NewRedisBlacklistStore parses a Redis URL and returns a Redis-backed
// store. PING is issued at construction so a misconfigured REDIS_URL
// surfaces at boot, not on the first revocation. The prefix lets
// multiple deployments share a Redis without colliding on the
// blacklist namespace.
func NewRedisBlacklistStore(url string) (*RedisBlacklistStore, error) {
	opts, err := redis.ParseURL(url)
	if err != nil {
		return nil, fmt.Errorf("parse REDIS_URL: %w", err)
	}
	client := redis.NewClient(opts)
	pingCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := client.Ping(pingCtx).Err(); err != nil {
		_ = client.Close()
		return nil, fmt.Errorf("redis ping: %w", err)
	}
	return &RedisBlacklistStore{client: client, prefix: "blacklist:"}, nil
}

// Close releases the underlying Redis connection. Safe to call multiple times.
func (s *RedisBlacklistStore) Close() error {
	if s == nil || s.client == nil {
		return nil
	}
	return s.client.Close()
}

func (s *RedisBlacklistStore) Revoke(token string, expiresAt time.Time) {
	ttl := time.Until(expiresAt)
	if ttl <= 0 {
		// Already expired; nothing to revoke.
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	_ = s.client.Set(ctx, s.prefix+blacklistKey(token), "1", ttl).Err()
}

func (s *RedisBlacklistStore) IsRevoked(token string) bool {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	n, err := s.client.Exists(ctx, s.prefix+blacklistKey(token)).Result()
	if err != nil {
		// Fail-open on a transient Redis error so a Redis outage
		// doesn't lock every user out. Logging is the caller's
		// responsibility (the auth middleware already logs auth
		// failures).
		return false
	}
	return n > 0
}
