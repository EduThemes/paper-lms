package settings

import (
	"context"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"net/url"
	"strings"

	"golang.org/x/net/publicsuffix"
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
// auth.passkey.rporigins. Closes Wave 6 audit H2; Wave 7 audit L1
// upgraded the suffix check to be PSL-aware (see
// isRegistrableSuffixOf) so pathological inputs like rpid="co.uk"
// with host="foo.co.uk" — which pass the structural check but get
// rejected by the browser at ceremony time — are now caught at
// write time too.
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

// isRegistrableSuffixOf returns true when `suffix` is a WebAuthn-valid
// "registrable domain suffix" of `host`. This matches the browser's
// RPID acceptance rules, which are defined by the Public Suffix List
// (PSL): a host's registrable domain is the host minus its public
// suffix, plus one label. A valid RPID is either the host itself OR
// any structural suffix of the host that is at least as specific as
// the registrable domain. The PSL itself (e.g. "co.uk", "edu") is
// NOT a valid RPID because nobody can "own" a public suffix.
//
// Backed by golang.org/x/net/publicsuffix (the canonical Go PSL
// library, with the list embedded at compile time — no runtime
// download). Upgraded from the previous structural-only check that
// would erroneously accept rpid="co.uk" for host="foo.co.uk".
//
//	suffix="example.edu" host="paper.example.edu"   → true
//	suffix="example.edu" host="example.edu"         → true
//	suffix="example.edu" host="evil.example.com"    → false
//	suffix="evil.com"    host="paper.example.edu"   → false
//	suffix="lms.example.edu" host="example.edu"     → false (suffix more specific than host)
//	suffix="co.uk"       host="foo.co.uk"           → false (just the public suffix — PSL block)
//	suffix="uk"          host="foo.co.uk"           → false (just a public suffix)
//	suffix="bar.co.uk"   host="foo.bar.co.uk"       → true  (registrable domain of host)
func isRegistrableSuffixOf(suffix, host string) bool {
	suffix = strings.ToLower(strings.TrimSpace(suffix))
	host = strings.ToLower(strings.TrimSpace(host))
	if suffix == "" || host == "" {
		return false
	}
	if suffix == host {
		return true
	}
	if !strings.HasSuffix(host, "."+suffix) {
		return false
	}
	// Structural suffix matches. Now apply the PSL-based rules: suffix
	// must NOT be just the public suffix, and must be at least as
	// specific as the host's registrable domain (eTLD+1).
	publicSuffixOfHost, _ := publicsuffix.PublicSuffix(host)
	if suffix == publicSuffixOfHost {
		return false
	}
	etldPlusOne, err := publicsuffix.EffectiveTLDPlusOne(host)
	if err != nil {
		// Pathological input — host has no registrable domain (e.g. it
		// IS a public suffix). Reject defensively.
		return false
	}
	if labelCount(suffix) < labelCount(etldPlusOne) {
		return false
	}
	return true
}

// labelCount returns the number of DNS labels in a host string. Used
// by isRegistrableSuffixOf to compare suffix specificity against the
// registrable-domain (eTLD+1) specificity. An empty string returns 0.
func labelCount(host string) int {
	if host == "" {
		return 0
	}
	return strings.Count(host, ".") + 1
}

// validateAbsolutePath enforces that filesystem-path settings (the
// path-based SAML cert/key keys) are absolute and don't contain ".."
// segments. Wave 9: defense-in-depth on top of Wave 7's
// extractCertBase64 fix.
//
// Why care about path traversal when the operator who can set this
// is already a super-admin? The Wave 7 audit M1 finding noted that
// SAML cert paths fed straight into os.ReadFile + the metadata
// endpoint, turning the super-admin role into a "read any file the
// server can read" escalation. Wave 7 closed the *exfiltration*
// (metadata refuses to embed non-CERTIFICATE bytes); this
// validator closes the obvious-mistake vector at write time.
//
// We accept absolute paths only — relative paths like "../keys/x.pem"
// or "etc/shadow" would be resolved against the server's CWD and
// could hit unintended files. Empty value is allowed (clear back to
// env/default).
func validateAbsolutePath(ctx context.Context, value string, peer func(key string) (string, error)) error {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}
	if !strings.HasPrefix(value, "/") {
		return fmt.Errorf("path must be absolute (start with /)")
	}
	for _, segment := range strings.Split(value, "/") {
		if segment == ".." {
			return fmt.Errorf("path must not contain '..' segments")
		}
	}
	return nil
}

// validateSAMLCertPEM enforces that the inline-PEM SAML cert setting
// holds a valid X.509 CERTIFICATE block. Same shape check as
// internal/auth/saml.go:extractCertBase64 — duplicated here because
// we can't import auth from settings (cycle), and because we want
// write-time rejection rather than waiting until the next metadata
// fetch surfaces the bad value.
func validateSAMLCertPEM(ctx context.Context, value string, peer func(key string) (string, error)) error {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}
	block, _ := pem.Decode([]byte(value))
	if block == nil {
		return fmt.Errorf("not a valid PEM block — expected -----BEGIN CERTIFICATE-----…")
	}
	if block.Type != "CERTIFICATE" {
		return fmt.Errorf("PEM block type is %q, expected CERTIFICATE (don't paste a private key here)", block.Type)
	}
	if _, err := x509.ParseCertificate(block.Bytes); err != nil {
		return fmt.Errorf("PEM CERTIFICATE block does not parse as X.509: %v", err)
	}
	return nil
}

// validateSAMLKeyPEM enforces that the inline-PEM SAML key setting
// holds a PEM block of a recognized private-key type. We don't
// further validate the key parses (the SAML library will do that
// when it actually signs) — but a non-PEM, or PEM with the wrong
// block type, would fail later in a confusing way.
func validateSAMLKeyPEM(ctx context.Context, value string, peer func(key string) (string, error)) error {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}
	block, _ := pem.Decode([]byte(value))
	if block == nil {
		return fmt.Errorf("not a valid PEM block — expected -----BEGIN PRIVATE KEY-----, BEGIN RSA PRIVATE KEY, or BEGIN EC PRIVATE KEY")
	}
	switch block.Type {
	case "PRIVATE KEY", "RSA PRIVATE KEY", "EC PRIVATE KEY":
		return nil
	case "CERTIFICATE":
		return fmt.Errorf("you pasted a CERTIFICATE here; put it in auth.saml.cert_pem")
	default:
		return fmt.Errorf("PEM block type is %q, expected a PRIVATE KEY variant", block.Type)
	}
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
