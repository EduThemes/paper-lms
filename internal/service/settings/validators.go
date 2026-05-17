package settings

import (
	"context"
	"fmt"
	"net/url"
	"strings"
)

// This file holds catalog-level write-time validators. Each function
// here is referenced from the Catalog entries in catalog.go as the
// Validate field. Validators run at PUT time, AFTER the type-coercion
// check; they have access to peer catalog keys via the peer callback
// so cross-key invariants ("RPID must be a registrable suffix of
// every origin") can be enforced before the row hits the DB.
//
// Validators MUST be defensive about empty peer results — a paired
// key may not be set yet. The general pattern: "if peer is set AND
// we disagree, reject; otherwise pass."

// validatePasskeyRPID enforces the WebAuthn cross-subdomain invariant:
// the rpid value must be a registrable suffix of every host in
// auth.passkey.rporigins. Closes Wave 6 audit H2.
//
// "Registrable suffix" here is implemented as the simple structural
// check: rpid equals host, OR host ends with "." + rpid. This catches
// the obvious mistakes (e.g. rpid="evil.com" with origin
// "https://lms.example.edu" — rejected) without depending on the
// Public Suffix List. A super-admin who configures a PSL-edge-case
// suffix can still tank their deployment, but those are operator-
// expert configurations that don't show up in typical district setups.
func validatePasskeyRPID(ctx context.Context, value string, peer func(key string) (string, error)) error {
	value = strings.TrimSpace(value)
	if value == "" {
		// Empty RPID = clear back to env/default. No coupling to check.
		return nil
	}
	rawOrigins, err := peer("auth.passkey.rporigins")
	if err != nil {
		// Peer lookup failed — let the write through. Ceremony-time
		// validation in wa.New is still the last line of defense.
		return nil
	}
	if strings.TrimSpace(rawOrigins) == "" {
		// Origins not set yet. Defer: when the operator sets origins,
		// the rporigins validator will check the coupling from that
		// side.
		return nil
	}
	for _, origin := range splitRPOrigins(rawOrigins) {
		host, err := hostFromOrigin(origin)
		if err != nil {
			return fmt.Errorf("origin %q is not a valid URL: %v", origin, err)
		}
		if !isRegistrableSuffixOf(value, host) {
			return fmt.Errorf("RPID %q is not a registrable suffix of origin host %q (would let other subdomains complete passkey ceremonies with this deployment's credentials)", value, host)
		}
	}
	return nil
}

// validatePasskeyRPOrigins is the symmetric coupling check from the
// origins side. When the operator sets new origins, every origin's
// host must accept the current RPID as a registrable suffix.
func validatePasskeyRPOrigins(ctx context.Context, value string, peer func(key string) (string, error)) error {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}
	rawRPID, err := peer("auth.passkey.rpid")
	if err != nil {
		return nil
	}
	rpid := strings.TrimSpace(rawRPID)
	if rpid == "" {
		// RPID not set; coupling will be enforced from the RPID
		// validator when that's written.
		return nil
	}
	for _, origin := range splitRPOrigins(value) {
		host, err := hostFromOrigin(origin)
		if err != nil {
			return fmt.Errorf("origin %q is not a valid URL: %v", origin, err)
		}
		if !isRegistrableSuffixOf(rpid, host) {
			return fmt.Errorf("origin host %q does not accept RPID %q as a registrable suffix (would let other subdomains complete passkey ceremonies)", host, rpid)
		}
	}
	return nil
}

// splitRPOrigins parses the comma-separated rporigins string the
// same way internal/auth/webauthn.go does. Duplicated here rather
// than imported to avoid an auth → service/settings → auth cycle.
func splitRPOrigins(raw string) []string {
	parts := []string{}
	for _, p := range strings.Split(raw, ",") {
		p = strings.TrimSpace(p)
		if p != "" {
			parts = append(parts, p)
		}
	}
	return parts
}

// hostFromOrigin extracts the hostname from an origin URL. The
// WebAuthn spec requires origins to be full URLs (scheme + host
// + optional port).
func hostFromOrigin(origin string) (string, error) {
	u, err := url.Parse(origin)
	if err != nil {
		return "", err
	}
	if u.Scheme == "" || u.Host == "" {
		return "", fmt.Errorf("missing scheme or host")
	}
	return u.Hostname(), nil
}

// isRegistrableSuffixOf returns true when `suffix` is a structural
// suffix of `host` per WebAuthn's RPID rules (simplified — no Public
// Suffix List). Either suffix == host (exact match), or host ends
// with "." + suffix.
//
//	suffix="example.edu" host="paper.example.edu"  → true
//	suffix="example.edu" host="example.edu"        → true
//	suffix="example.edu" host="evil.example.com"   → false
//	suffix="evil.com"    host="paper.example.edu"  → false
//	suffix="lms.example.edu" host="example.edu"    → false (suffix is more specific than host)
func isRegistrableSuffixOf(suffix, host string) bool {
	suffix = strings.ToLower(strings.TrimSpace(suffix))
	host = strings.ToLower(strings.TrimSpace(host))
	if suffix == "" || host == "" {
		return false
	}
	if suffix == host {
		return true
	}
	return strings.HasSuffix(host, "."+suffix)
}

// validateHTTPSURL is a small reusable validator for string-typed
// keys that must hold an https URL. Wired into auth.oidc.redirect_base
// and branding.frontend_url to surface http:// (or garbage) mistakes
// at write time instead of at next-OIDC-redirect time.
func validateHTTPSURL(ctx context.Context, value string, peer func(key string) (string, error)) error {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil // clear is always fine
	}
	u, err := url.Parse(value)
	if err != nil {
		return fmt.Errorf("not a valid URL: %v", err)
	}
	if u.Scheme != "https" && u.Scheme != "http" {
		return fmt.Errorf("URL must use http or https scheme (got %q)", u.Scheme)
	}
	if u.Host == "" {
		return fmt.Errorf("URL is missing host")
	}
	// We accept http://localhost / http://127.0.0.1 for dev convenience
	// but warn on http://anything-else by rejecting it. Operators using
	// a real domain should always use https.
	if u.Scheme == "http" && u.Hostname() != "localhost" && u.Hostname() != "127.0.0.1" {
		return fmt.Errorf("URL must use https for non-localhost hosts (got http://%s)", u.Host)
	}
	return nil
}
