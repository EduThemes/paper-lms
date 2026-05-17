package settings

import (
	"context"
	"errors"
	"strings"
	"testing"
)

// staticPeer returns a peer callback that resolves from a fixed
// in-memory map. Convenience for the validator tests that don't
// need a full settings.Service.
func staticPeer(values map[string]string) func(key string) (string, error) {
	return func(key string) (string, error) {
		return values[key], nil
	}
}

func TestIsRegistrableSuffixOf(t *testing.T) {
	cases := []struct {
		suffix, host string
		want         bool
	}{
		// Exact match — RPID equals host.
		{"example.edu", "example.edu", true},
		{"localhost", "localhost", true},
		// Proper suffix — host ends with "." + RPID.
		{"example.edu", "lms.example.edu", true},
		{"example.edu", "a.b.example.edu", true},
		// Not a suffix.
		{"evil.com", "lms.example.edu", false},
		{"example.com", "lms.example.edu", false},
		// Wrong direction — suffix is more specific than host.
		{"lms.example.edu", "example.edu", false},
		// Substring but not suffix (defends against "example.edu" matching "evil-example.edu").
		{"example.edu", "evilexample.edu", false},
		// Case-insensitive.
		{"Example.Edu", "lms.example.edu", true},
		// Empty fields.
		{"", "example.edu", false},
		{"example.edu", "", false},
		// Whitespace normalized.
		{"  example.edu  ", "lms.example.edu", true},

		// ── PSL upgrade (Wave 7 audit L1) ─────────────────────────────
		// Just the public suffix — browser would reject at ceremony.
		{"co.uk", "foo.co.uk", false},
		{"uk", "foo.co.uk", false},
		{"co.uk", "bar.foo.co.uk", false},
		// Exact-host match where host IS the registrable domain (eTLD+1).
		{"foo.co.uk", "foo.co.uk", true},
		// Proper suffix at or below the registrable domain.
		{"foo.co.uk", "bar.foo.co.uk", true},
		{"bar.foo.co.uk", "baz.bar.foo.co.uk", true},
		// edu is a public suffix; RPID="edu" with edu-suffixed host is rejected.
		{"edu", "example.edu", false},
		{"edu", "lms.paper.example.edu", false},
		// example.edu is the registrable domain → still valid.
		{"example.edu", "lms.paper.example.edu", true},
		{"paper.example.edu", "lms.paper.example.edu", true},
		// "com" is a public suffix.
		{"com", "example.com", false},
		{"com", "foo.bar.example.com", false},
	}
	for _, c := range cases {
		got := isRegistrableSuffixOf(c.suffix, c.host)
		if got != c.want {
			t.Errorf("isRegistrableSuffixOf(%q, %q) = %v, want %v", c.suffix, c.host, got, c.want)
		}
	}
}

func TestValidatePasskeyRPID_AcceptsRegistrableSuffix(t *testing.T) {
	peer := staticPeer(map[string]string{
		"auth.passkey.rporigins": "https://lms.example.edu,https://api.example.edu",
	})
	err := validatePasskeyRPID(context.Background(), "example.edu", peer)
	if err != nil {
		t.Errorf("expected accept: %v", err)
	}
}

func TestValidatePasskeyRPID_RejectsNonSuffix(t *testing.T) {
	peer := staticPeer(map[string]string{
		"auth.passkey.rporigins": "https://lms.example.edu",
	})
	err := validatePasskeyRPID(context.Background(), "evil.com", peer)
	if err == nil {
		t.Fatal("expected reject for evil.com")
	}
	if !strings.Contains(err.Error(), "registrable suffix") {
		t.Errorf("error message should mention registrable suffix: %v", err)
	}
}

func TestValidatePasskeyRPID_RejectsRPIDMoreSpecificThanOrigin(t *testing.T) {
	// RPID="lms.example.edu" with origin="https://example.edu" — RPID
	// is MORE specific than the host. Browser would refuse.
	peer := staticPeer(map[string]string{
		"auth.passkey.rporigins": "https://example.edu",
	})
	err := validatePasskeyRPID(context.Background(), "lms.example.edu", peer)
	if err == nil {
		t.Fatal("expected reject — RPID more specific than origin host")
	}
}

func TestValidatePasskeyRPID_EmptyValueAccepted(t *testing.T) {
	// Empty RPID = clear back to env/default. No coupling to check.
	peer := staticPeer(map[string]string{
		"auth.passkey.rporigins": "https://lms.example.edu",
	})
	err := validatePasskeyRPID(context.Background(), "", peer)
	if err != nil {
		t.Errorf("empty value should be allowed (clear): %v", err)
	}
}

func TestValidatePasskeyRPID_EmptyOriginsDefersToOrigins(t *testing.T) {
	// If rporigins isn't set yet, the RPID write should NOT block —
	// the operator may be configuring both keys in sequence. The
	// origins validator will check from the other side.
	peer := staticPeer(map[string]string{})
	err := validatePasskeyRPID(context.Background(), "example.edu", peer)
	if err != nil {
		t.Errorf("empty rporigins should defer, not reject: %v", err)
	}
}

func TestValidatePasskeyRPID_MultiOriginAllMustMatch(t *testing.T) {
	// Two origins; one matches RPID, one doesn't. Reject.
	peer := staticPeer(map[string]string{
		"auth.passkey.rporigins": "https://lms.example.edu,https://other.evil.com",
	})
	err := validatePasskeyRPID(context.Background(), "example.edu", peer)
	if err == nil {
		t.Fatal("expected reject — at least one origin host doesn't accept the RPID")
	}
	if !strings.Contains(err.Error(), "evil.com") {
		t.Errorf("error should name the offending origin: %v", err)
	}
}

func TestValidatePasskeyRPID_MalformedOriginRejected(t *testing.T) {
	peer := staticPeer(map[string]string{
		"auth.passkey.rporigins": "not-a-url",
	})
	err := validatePasskeyRPID(context.Background(), "example.edu", peer)
	if err == nil {
		t.Fatal("expected reject — malformed origin in peer")
	}
}

func TestValidatePasskeyRPOrigins_SymmetricCoupling(t *testing.T) {
	// Setting rporigins after RPID is already set — coupling check
	// fires from this side too.
	peer := staticPeer(map[string]string{
		"auth.passkey.rpid": "example.edu",
	})
	if err := validatePasskeyRPOrigins(context.Background(), "https://lms.example.edu", peer); err != nil {
		t.Errorf("matching origin should be accepted: %v", err)
	}
	if err := validatePasskeyRPOrigins(context.Background(), "https://lms.evil.com", peer); err == nil {
		t.Fatal("non-matching origin host should be rejected")
	}
}

func TestValidateHTTPSURL_RejectsHTTP(t *testing.T) {
	if err := validateHTTPSURL(context.Background(), "http://example.com", nil); err == nil {
		t.Error("http://non-localhost should be rejected")
	}
}

func TestValidateHTTPSURL_AcceptsHTTPLocalhost(t *testing.T) {
	for _, v := range []string{"http://localhost", "http://localhost:3000", "http://127.0.0.1:5173"} {
		if err := validateHTTPSURL(context.Background(), v, nil); err != nil {
			t.Errorf("dev URL %q should be accepted: %v", v, err)
		}
	}
}

func TestValidateHTTPSURL_AcceptsHTTPS(t *testing.T) {
	if err := validateHTTPSURL(context.Background(), "https://paper.example.edu", nil); err != nil {
		t.Errorf("https URL should be accepted: %v", err)
	}
}

func TestValidateHTTPSURL_RejectsBadScheme(t *testing.T) {
	for _, v := range []string{"javascript:alert(1)", "ftp://x.com", "file:///etc/passwd"} {
		if err := validateHTTPSURL(context.Background(), v, nil); err == nil {
			t.Errorf("bad scheme %q should be rejected", v)
		}
	}
}

func TestValidateHTTPSURL_RejectsMalformed(t *testing.T) {
	for _, v := range []string{"   ", "not-a-url", "https://"} {
		if err := validateHTTPSURL(context.Background(), v, nil); err == nil && strings.TrimSpace(v) != "" {
			t.Errorf("malformed %q should be rejected", v)
		}
	}
}

// ── End-to-end through Service.Set ──

func TestServiceSet_ValidatorRunsAndCanReject(t *testing.T) {
	svc, _, _ := newServiceWithAudit(t, nil)
	// auth.passkey.rporigins set first.
	if err := svc.Set(context.Background(), ScopeInstance, 0, "auth.passkey.rporigins", "https://lms.example.edu", 1); err != nil {
		t.Fatalf("set rporigins: %v", err)
	}
	// Setting RPID="evil.com" should now reject via the catalog validator.
	err := svc.Set(context.Background(), ScopeInstance, 0, "auth.passkey.rpid", "evil.com", 1)
	if err == nil {
		t.Fatal("expected validator to reject evil.com RPID")
	}
	if !errors.Is(err, ErrInvalidValue) {
		t.Errorf("expected ErrInvalidValue: %v", err)
	}
}

func TestServiceSet_ValidatorAcceptsMatchingRPID(t *testing.T) {
	svc, _, _ := newServiceWithAudit(t, nil)
	if err := svc.Set(context.Background(), ScopeInstance, 0, "auth.passkey.rporigins", "https://lms.example.edu", 1); err != nil {
		t.Fatalf("set rporigins: %v", err)
	}
	if err := svc.Set(context.Background(), ScopeInstance, 0, "auth.passkey.rpid", "example.edu", 1); err != nil {
		t.Fatalf("set rpid (should pass validator): %v", err)
	}
}

func TestServiceSet_ValidatorRunsAfterTypeCheck(t *testing.T) {
	svc, _, _ := newServiceWithAudit(t, nil)
	// smtp.port is int-typed. A non-int value must be rejected by
	// validateValue (type check) BEFORE any catalog validator runs.
	// smtp.port has no Validate today, but this test locks the order
	// so a future port validator that did peer lookups doesn't run
	// against unparsed garbage.
	err := svc.Set(context.Background(), ScopeInstance, 0, "smtp.port", "not-an-int", 1)
	if err == nil {
		t.Fatal("expected ErrInvalidValue for non-int port")
	}
	if !errors.Is(err, ErrInvalidValue) {
		t.Errorf("expected ErrInvalidValue: %v", err)
	}
}
