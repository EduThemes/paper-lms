package security

// Tests for the shared SSRF guard. The handler-package's
// existing ssrf_guard_test.go covers the validateExternalURL
// wrapper end-to-end via the OIDC test endpoint; this file
// pins the package-level contract.

import (
	"context"
	"errors"
	"net"
	"strings"
	"testing"
)

func TestValidateExternalURL_AcceptsPublicHTTPS(t *testing.T) {
	// example.com resolves to a public IP via DNS in CI; if the test
	// environment has no internet, the LookupIPAddr returns an
	// error which is also a valid "blocked" path. So we accept
	// either nil OR an SSRF block that mentions DNS — neither
	// indicates a wrong validation outcome for the security
	// contract.
	err := ValidateExternalURL(context.Background(), "https://example.com/")
	if err == nil {
		return // CI with internet
	}
	if !errors.Is(err, ErrSSRFBlocked) {
		t.Fatalf("got unexpected error type: %v", err)
	}
	// Acceptable: DNS failed inside the sandbox. Verify it's not a
	// false positive on a public IP being misclassified.
	if !strings.Contains(err.Error(), "DNS lookup failed") {
		t.Fatalf("expected DNS-failure path for sandboxed env, got: %v", err)
	}
}

func TestValidateExternalURL_RejectsHTTP(t *testing.T) {
	err := ValidateExternalURL(context.Background(), "http://example.com/")
	if !errors.Is(err, ErrSSRFBlocked) {
		t.Fatalf("expected ErrSSRFBlocked, got: %v", err)
	}
	if !strings.Contains(err.Error(), "https") {
		t.Fatalf("expected message to mention https requirement; got: %v", err)
	}
}

func TestValidateExternalURL_RejectsCustomPort(t *testing.T) {
	err := ValidateExternalURL(context.Background(), "https://example.com:8443/")
	if !errors.Is(err, ErrSSRFBlocked) {
		t.Fatalf("expected ErrSSRFBlocked, got: %v", err)
	}
}

func TestValidateExternalURL_RejectsBlockedSuffix(t *testing.T) {
	cases := []string{
		"https://something.local/",
		"https://service.internal/",
		"https://x.localhost/",
		"https://anything.lan/",
		"https://corp-internal.intranet/",
		"https://localhost/",
	}
	for _, u := range cases {
		err := ValidateExternalURL(context.Background(), u)
		if !errors.Is(err, ErrSSRFBlocked) {
			t.Fatalf("expected block for %s, got: %v", u, err)
		}
	}
}

func TestClassifyIP_BlocksKnownRanges(t *testing.T) {
	cases := []struct {
		ip       string
		wantWord string // substring expected in the classify return
	}{
		{"127.0.0.1", "loopback"},
		{"10.0.0.1", "private"},
		{"172.16.5.10", "private"},
		{"192.168.1.1", "private"},
		{"169.254.169.254", "link-local"}, // AWS / GCP / Azure metadata IP
		{"100.64.1.1", "CGNAT"},
		{"0.0.0.0", "unspecified"},
		{"192.0.2.5", "TEST-NET-1"},
		{"198.51.100.5", "TEST-NET-2"},
		{"203.0.113.5", "TEST-NET-3"},
	}
	for _, tc := range cases {
		if got := classifyIPString(t, tc.ip); !strings.Contains(strings.ToLower(got), strings.ToLower(tc.wantWord)) {
			t.Fatalf("classifyIP(%s) = %q, want substring %q", tc.ip, got, tc.wantWord)
		}
	}
}

func classifyIPString(t *testing.T, s string) string {
	t.Helper()
	parsed := net.ParseIP(s)
	if parsed == nil {
		t.Fatalf("could not parse %s", s)
	}
	return classifyIP(parsed)
}

// SafeDialContext is the connect-time DNS-rebinding defense. It must
// reject a non-443 port and any host whose resolved IP is internal,
// before any TCP connection is attempted. These cases resolve locally
// (no external network needed).
func TestSafeDialContext_Blocks(t *testing.T) {
	ctx := context.Background()
	cases := []struct {
		name string
		addr string
	}{
		{"non-443 port", "example.com:80"},
		{"loopback hostname", "localhost:443"},
		{"private ip literal", "10.0.0.1:443"},
		{"link-local metadata ip", "169.254.169.254:443"},
		{"ipv6 loopback literal", "[::1]:443"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			conn, err := SafeDialContext(ctx, "tcp", c.addr)
			if conn != nil {
				_ = conn.Close()
				t.Fatalf("expected no connection for %s", c.addr)
			}
			if !errors.Is(err, ErrSSRFBlocked) {
				t.Errorf("expected ErrSSRFBlocked for %s, got %v", c.addr, err)
			}
		})
	}
}

func TestSafeTransport_UsesSafeDialer(t *testing.T) {
	tr := SafeTransport()
	if tr.DialContext == nil {
		t.Fatal("SafeTransport must set DialContext")
	}
	// The dialer must enforce the same block (sanity that it's wired).
	if _, err := tr.DialContext(context.Background(), "tcp", "169.254.169.254:443"); !errors.Is(err, ErrSSRFBlocked) {
		t.Errorf("SafeTransport.DialContext should block metadata IP, got %v", err)
	}
}
