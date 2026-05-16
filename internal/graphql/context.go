package graphql

import "context"

// accountIDKey is the context key for the calling tenant's account_id.
// Sprint 2.4 plumbing — every Resolve call now carries the caller's
// account_id alongside the user_id so downstream service.GetByID calls
// can filter at the SQL boundary instead of passing literal 0.
type ctxKey int

const accountIDKey ctxKey = iota

// WithAccountID returns a derived context carrying the tenant ID. Called
// once at the top of Resolve so every nested resolver can read it
// without threading the value through every function signature.
func WithAccountID(ctx context.Context, accountID uint) context.Context {
	return context.WithValue(ctx, accountIDKey, accountID)
}

// AccountIDFromContext returns the tenant ID from the context, or 0 if
// none is set. 0 means "no tenant scope" — the service-layer convention
// is that callers MUST pass a non-zero value for cross-tenant safety;
// 0 is reserved for background jobs and tests.
func AccountIDFromContext(ctx context.Context) uint {
	if v, ok := ctx.Value(accountIDKey).(uint); ok {
		return v
	}
	return 0
}
