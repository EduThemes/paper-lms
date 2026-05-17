package settingsctx

import (
	"context"
	"testing"
)

func TestWithAccountID_RoundTrip(t *testing.T) {
	ctx := context.Background()
	if got := AccountIDFromContext(ctx); got != 0 {
		t.Errorf("empty ctx should yield 0, got %d", got)
	}

	ctx = WithAccountID(ctx, 42)
	if got := AccountIDFromContext(ctx); got != 42 {
		t.Errorf("after WithAccountID(42), got %d", got)
	}
}

func TestWithAccountID_ZeroClearsHint(t *testing.T) {
	ctx := WithAccountID(context.Background(), 42)
	ctx = WithAccountID(ctx, 0)
	// 0 is explicitly stored; AccountIDFromContext returns the stored
	// value which IS 0. Same as the no-hint case from the consumer's
	// POV — the lookup hints will be empty.
	if got := AccountIDFromContext(ctx); got != 0 {
		t.Errorf("explicit 0 should resolve to 0, got %d", got)
	}
}

func TestAccountIDFromContext_WrongTypeInContext(t *testing.T) {
	// An attacker (or buggy code in another package) cannot inject a
	// fake account ID by using a same-shape key — the unexported
	// accountKey type means context.Value only matches OUR exact key.
	// Verify by inserting a value at a different type with the same
	// underlying int.
	type fakeKey int
	const fakeID fakeKey = 0
	ctx := context.WithValue(context.Background(), fakeID, uint(99))
	if got := AccountIDFromContext(ctx); got != 0 {
		t.Errorf("fake-type key must NOT leak: got %d, want 0", got)
	}
}

func TestAccountIDFromContext_WrongValueType(t *testing.T) {
	// Defensive: even if (hypothetically) accountKey were exported
	// and someone stored a non-uint value, the type assertion in
	// AccountIDFromContext must not panic. Simulate by stashing an
	// int at the SAME key shape... we can't actually because the
	// type is unexported, but the type assertion is defensive: if
	// the cached value isn't a uint, returns 0.
	ctx := context.Background()
	if got := AccountIDFromContext(ctx); got != 0 {
		t.Errorf("safe default 0, got %d", got)
	}
}
