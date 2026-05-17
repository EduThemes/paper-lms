// Package settingsctx is a tiny leaf package holding the context-key
// helpers that thread per-request scope hints (today: AccountID) into
// the shared settingsLookup closure declared in cmd/server/main.go.
//
// Why a separate package?
// ───────────────────────
// The natural home for these helpers is internal/service/settings,
// but service consumers (notification_delivery_service,
// ai_assist_service) can't import that package — Go's cycle detector
// rejects the chain:
//
//	internal/service → internal/service/settings → internal/auth (secretbox)
//	                                                  ↑               ↓
//	                                                  └── auth_audit ──┘
//
// auth_audit.go imports service, so service can't import settings
// either directly or transitively. Same cycle blocks internal/auth.
//
// settingsctx is a leaf — depends only on the stdlib `context`
// package — so EVERY consumer (service, auth, storage, even settings
// itself) can import it without a cycle. The lookup closure in
// main.go reads AccountIDFromContext(ctx) and materializes the
// settings.ScopeHints from there.
//
// SECURITY NOTE
// ──────────────
// The context-key approach uses Go's type-identity rules: the
// `accountKey` type below is unexported, so an attacker (or
// adversarial code in a separate package) cannot construct an
// instance of this type and inject a fake value. The only way a
// context can carry an AccountID for this package is via
// WithAccountID. Other packages can use the SAME shape (int-based
// key starting at 0) without colliding — Go's context.Value
// compares by type identity, not by underlying type.
package settingsctx

import "context"

// accountKey is the unexported type used as the context-value key.
// Unexported = no outside package can construct it = no collision.
type accountKey int

const accountIDKey accountKey = 0

// WithAccountID returns a derived context carrying the per-request
// account scope hint. Pass the result to any consumer that resolves
// settings — the lookup closure transparently switches from
// instance-scope to account-scope resolution.
//
// Pass accountID=0 to explicitly clear any inherited scope hint —
// useful for background workers that want instance-scope resolution
// regardless of the inbound request context.
func WithAccountID(ctx context.Context, accountID uint) context.Context {
	return context.WithValue(ctx, accountIDKey, accountID)
}

// AccountIDFromContext extracts the per-request account ID set by
// WithAccountID, or 0 if nothing was set. Zero is treated as
// "instance scope" by the resolution chain — the same as no hint.
func AccountIDFromContext(ctx context.Context) uint {
	if v, ok := ctx.Value(accountIDKey).(uint); ok {
		return v
	}
	return 0
}
