// Package auth's MFAAttemptTracker rate-limits second-factor
// submissions against a single pending-MFA token.
//
// Threat model: an attacker who phishes a user's pending_token (from
// a fake login page or an XSS) has 5 minutes to brute-force the
// 6-digit TOTP code OR a recovery code. Without a per-token attempt
// limit they can try ~1,000,000 codes in that window — TOTP is
// trivially crackable. With a 5-attempt cap, the attacker has a
// 5/1,000,000 ≈ 0.0005% chance per phish.
//
// Why in-memory (sync.Map) rather than DB:
//   * The state is bounded by the 5-minute pending-token TTL.
//   * The hot path is a counter increment — DB writes per attempt
//     would add 5-10ms of latency on the most security-sensitive
//     endpoint in the codebase.
//   * Server restart resets counters — acceptable because the same
//     restart invalidates pending tokens (signed against a JWT secret
//     that persists, but the in-flight attempts associated with the
//     active session are torn down).
//   * Horizontal scaling: stickiness via the reverse proxy is fine
//     for v1. Multi-pod deployments would need Redis here, which is
//     a Phase 11 concern.
//
// Keying: SHA-256 of the pending_token. Never store the raw token
// (it's a JWT — large; we just want a stable hash for the map key).
//
// Sweep: a background goroutine evicts entries older than 5 minutes
// every 60 seconds. Bounded memory.
package auth

import (
	"crypto/sha256"
	"encoding/hex"
	"sync"
	"time"
)

// Per-endpoint attempt caps. Recovery codes are higher-value targets
// (an exhausted recovery code permanently locks the user out of that
// fallback), so the cap is tighter.
const (
	MaxVerifyAttempts   = 5
	MaxRecoveryAttempts = 3
)

// Per-ACCOUNT caps over a rolling window. The per-token cap above is
// defeated by re-minting: an attacker who knows the password can restart
// the login flow to get a fresh pending token (and a fresh 5-attempt
// budget) indefinitely. The per-account counter is keyed on the user id
// inside the pending token, so re-minting does not reset it. The caps are
// a small multiple of the per-token caps (≈3 token re-mints' worth) and
// the window comfortably exceeds the 5-minute pending-token TTL.
const (
	MaxVerifyAttemptsPerAccount   = 15
	MaxRecoveryAttemptsPerAccount = 6
	accountAttemptWindow          = 15 * time.Minute
)

// MFAAttemptTracker is the per-process in-memory counter store.
type MFAAttemptTracker struct {
	store     sync.Map // tokenHash → *attemptRecord
	userStore sync.Map // userID    → *accountRecord
	now       func() time.Time
}

type attemptRecord struct {
	mu        sync.Mutex
	verify    int
	recovery  int
	createdAt time.Time
}

// accountRecord tracks attempts per user id across a rolling window,
// surviving pending-token re-mints.
type accountRecord struct {
	mu          sync.Mutex
	verify      int
	recovery    int
	windowStart time.Time
}

// NewMFAAttemptTracker constructs a tracker and starts a background
// sweep. Pass nowFn=nil to use time.Now; tests can inject a clock.
func NewMFAAttemptTracker(nowFn func() time.Time) *MFAAttemptTracker {
	t := &MFAAttemptTracker{
		now: nowFn,
	}
	if t.now == nil {
		t.now = time.Now
	}
	go t.sweepLoop()
	return t
}

// CheckAndIncrementVerify returns nil if the verify attempt is
// allowed (counter still <= cap) and increments the counter. Returns
// ErrTooManyMFAAttempts when the cap is exceeded.
func (t *MFAAttemptTracker) CheckAndIncrementVerify(pendingToken string) error {
	return t.checkAndIncrement(pendingToken, true)
}

// CheckAndIncrementRecovery is the recovery-code variant. Tighter cap.
func (t *MFAAttemptTracker) CheckAndIncrementRecovery(pendingToken string) error {
	return t.checkAndIncrement(pendingToken, false)
}

func (t *MFAAttemptTracker) checkAndIncrement(pendingToken string, isVerify bool) error {
	key := hashToken(pendingToken)
	v, _ := t.store.LoadOrStore(key, &attemptRecord{createdAt: t.now()})
	rec := v.(*attemptRecord)
	rec.mu.Lock()
	defer rec.mu.Unlock()
	if isVerify {
		if rec.verify >= MaxVerifyAttempts {
			return errTooManyMFA
		}
		rec.verify++
	} else {
		if rec.recovery >= MaxRecoveryAttempts {
			return errTooManyMFA
		}
		rec.recovery++
	}
	return nil
}

// CheckAndIncrementVerifyForUser applies the per-account window cap for a
// TOTP verify. Call it in addition to CheckAndIncrementVerify so a fresh
// pending token cannot reset the budget.
func (t *MFAAttemptTracker) CheckAndIncrementVerifyForUser(userID uint) error {
	return t.checkAndIncrementUser(userID, true)
}

// CheckAndIncrementRecoveryForUser is the recovery-code variant.
func (t *MFAAttemptTracker) CheckAndIncrementRecoveryForUser(userID uint) error {
	return t.checkAndIncrementUser(userID, false)
}

func (t *MFAAttemptTracker) checkAndIncrementUser(userID uint, isVerify bool) error {
	v, _ := t.userStore.LoadOrStore(userID, &accountRecord{windowStart: t.now()})
	rec := v.(*accountRecord)
	rec.mu.Lock()
	defer rec.mu.Unlock()
	// Roll the window forward once it has fully elapsed.
	if t.now().Sub(rec.windowStart) >= accountAttemptWindow {
		rec.verify = 0
		rec.recovery = 0
		rec.windowStart = t.now()
	}
	if isVerify {
		if rec.verify >= MaxVerifyAttemptsPerAccount {
			return errTooManyMFA
		}
		rec.verify++
	} else {
		if rec.recovery >= MaxRecoveryAttemptsPerAccount {
			return errTooManyMFA
		}
		rec.recovery++
	}
	return nil
}

// Reset clears the counter for a token after a successful verify.
// Not strictly necessary (the token is single-use anyway), but tidy.
// The per-account window record is intentionally NOT cleared here: a
// successful login should not hand an attacker a fresh budget.
func (t *MFAAttemptTracker) Reset(pendingToken string) {
	t.store.Delete(hashToken(pendingToken))
}

// sweepLoop evicts attempt records older than 6 minutes (a small
// buffer over the 5-minute pending-token TTL). Runs every 60 seconds.
func (t *MFAAttemptTracker) sweepLoop() {
	ticker := time.NewTicker(60 * time.Second)
	defer ticker.Stop()
	for range ticker.C {
		cutoff := t.now().Add(-6 * time.Minute)
		t.store.Range(func(k, v any) bool {
			rec := v.(*attemptRecord)
			rec.mu.Lock()
			expired := rec.createdAt.Before(cutoff)
			rec.mu.Unlock()
			if expired {
				t.store.Delete(k)
			}
			return true
		})
		// Evict per-account records whose window has fully elapsed.
		userCutoff := t.now().Add(-accountAttemptWindow)
		t.userStore.Range(func(k, v any) bool {
			rec := v.(*accountRecord)
			rec.mu.Lock()
			expired := rec.windowStart.Before(userCutoff)
			rec.mu.Unlock()
			if expired {
				t.userStore.Delete(k)
			}
			return true
		})
	}
}

func hashToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

type errTooMany struct{}

func (errTooMany) Error() string {
	return "too many MFA attempts on this pending token; log in again"
}

// errTooManyMFA is the sentinel returned by CheckAndIncrement*. Use
// IsTooManyMFAAttempts(err) in handlers to translate to HTTP 429.
var errTooManyMFA = errTooMany{}

// IsTooManyMFAAttempts returns true iff the error indicates the
// per-token attempt cap was exceeded.
func IsTooManyMFAAttempts(err error) bool {
	if err == nil {
		return false
	}
	_, ok := err.(errTooMany)
	return ok
}
