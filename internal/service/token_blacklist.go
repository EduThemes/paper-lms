package service

import (
	"sync"
	"time"
)

// TokenBlacklist provides in-memory JWT revocation. When a user logs out,
// their token is added to the blacklist until it naturally expires. This
// prevents a logged-out JWT from being reused during its remaining lifetime.
type TokenBlacklist struct {
	mu      sync.RWMutex
	tokens  map[string]time.Time // token -> expiry
	cleanup *time.Ticker
}

func NewTokenBlacklist() *TokenBlacklist {
	bl := &TokenBlacklist{
		tokens:  make(map[string]time.Time),
		cleanup: time.NewTicker(10 * time.Minute),
	}
	go bl.purgeLoop()
	return bl
}

// Revoke adds a token to the blacklist. It will be automatically removed
// after expiresAt, since the JWT is no longer valid anyway.
func (bl *TokenBlacklist) Revoke(token string, expiresAt time.Time) {
	bl.mu.Lock()
	defer bl.mu.Unlock()
	bl.tokens[token] = expiresAt
}

// IsRevoked returns true if the token has been explicitly revoked via logout.
func (bl *TokenBlacklist) IsRevoked(token string) bool {
	bl.mu.RLock()
	defer bl.mu.RUnlock()
	_, revoked := bl.tokens[token]
	return revoked
}

// purgeLoop removes expired tokens from the blacklist periodically so that
// memory usage stays bounded. Tokens are only kept until their natural JWT
// expiry time.
func (bl *TokenBlacklist) purgeLoop() {
	for range bl.cleanup.C {
		bl.mu.Lock()
		now := time.Now()
		for token, exp := range bl.tokens {
			if now.After(exp) {
				delete(bl.tokens, token)
			}
		}
		bl.mu.Unlock()
	}
}
