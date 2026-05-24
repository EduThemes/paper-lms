package auth

import (
	"sync"
	"time"
)

// SAMLReplayCache tracks SAML assertion IDs that have already been
// consumed by the ACS endpoint, so a captured-in-flight assertion
// cannot be replayed within the NotOnOrAfter validity window (or the
// 5-minute clock-skew grace on either side).
//
// SECURITY (F-054 partial): the pre-fix path validated NotBefore /
// NotOnOrAfter but never recorded which assertions had already been
// consumed. An attacker who captured a SAML response (browser
// extension, MITM, log file, debug capture) could replay it any
// number of times within the validity window — typically 5 minutes.
//
// Storage: in-memory, single-process. Multi-pod deployments need a
// Redis-backed implementation (mirrors the TokenBlacklist /
// rate-limit Store pattern); the in-memory variant covers the
// single-process case and is strictly better than the pre-fix
// "no cache at all" behavior. The cache self-evicts on a 10-minute
// sweep so memory stays bounded even when assertions arrive at high
// rate.
type SAMLReplayCache struct {
	mu      sync.Mutex
	seen    map[string]time.Time // assertion ID -> expiry
	cleanup *time.Ticker
}

// NewSAMLReplayCache returns an in-memory replay cache. The caller
// should hold one instance per SAMLHandler (or share globally) so
// every ACS call consults the same map.
func NewSAMLReplayCache() *SAMLReplayCache {
	c := &SAMLReplayCache{
		seen:    make(map[string]time.Time),
		cleanup: time.NewTicker(10 * time.Minute),
	}
	go c.purgeLoop()
	return c
}

// CheckAndStore returns true (along with no error) the FIRST time it
// sees `assertionID`; subsequent calls within `expiresAt` return
// false (replay detected). expiresAt should be the assertion's
// NotOnOrAfter + the clock-skew grace; entries past this time are
// evicted by the purge loop.
//
// A zero-length assertion ID is treated as a replay (refuse) — SAML
// assertions MUST have a non-empty ID per the spec; absence means
// either a parser bug or a hand-crafted malicious response.
func (c *SAMLReplayCache) CheckAndStore(assertionID string, expiresAt time.Time) bool {
	if assertionID == "" {
		return false
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	now := time.Now()
	if exp, seen := c.seen[assertionID]; seen && now.Before(exp) {
		return false // replay
	}
	c.seen[assertionID] = expiresAt
	return true
}

func (c *SAMLReplayCache) purgeLoop() {
	for range c.cleanup.C {
		c.mu.Lock()
		now := time.Now()
		for id, exp := range c.seen {
			if now.After(exp) {
				delete(c.seen, id)
			}
		}
		c.mu.Unlock()
	}
}
