// Package security holds defense-in-depth helpers shared across
// handler / service / auth layers. The SSRF guard lives here (not in
// internal/api/v1/handlers) so the OIDC discovery path
// (internal/auth/oidc.go), CAS validation (internal/auth/cas.go),
// OneRoster sync (internal/service/oneroster_service.go), and the
// notification-webhook delivery (internal/service/notification_delivery_service.go)
// can all wear the same guard without dragging in the HTTP-handler
// package as a dependency.
package security

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// ErrSSRFBlocked is returned by ValidateExternalURL when the URL
// targets a host or network range that's off-limits to test actions
// originating from the server.
var ErrSSRFBlocked = errors.New("URL targets a restricted destination")

// ValidateExternalURL enforces SSRF defense-in-depth on URLs that
// originate from request bodies / DB rows and end up as outbound HTTP
// targets. The contract (PENTEST_FINDINGS F-021 through F-026):
//
//  1. Scheme MUST be https. Plain http would let a misconfigured
//     target return a cleartext payload that an in-network attacker
//     could MITM into pointing at attacker infrastructure.
//  2. Host MUST resolve to public unicast IP space. Loopback,
//     RFC1918, link-local, ULA, CGNAT, and IETF-protocol-reserved
//     ranges are all blocked — they're the canonical SSRF targets
//     (cloud metadata endpoints, internal admin panels, K8s API
//     server, etc.).
//  3. Hostname MUST NOT end with .local, .internal, .localhost, or
//     .lan. These suffixes resolve to internal infra on many
//     networks and aren't catchable by IP-range blocks alone.
//  4. Port MUST be either unset (defaults to 443 for https) or
//     explicitly 443. Custom ports are a common cloud-metadata
//     bypass.
//
// The DNS lookup happens here so the IP-range check is done on the
// *resolved* address, not just the hostname text. A DNS-rebinding
// attack would defeat a hostname-text-only check; resolve+block does
// not because the resolved IP at TIME-OF-CHECK is what we block on.
// (TOCTOU between this check and the actual HTTP request is a real
// concern in pathological cases — Go's net/http uses Resolver which
// can re-resolve between the check and the connect. Document risk
// at the call site; SafeDialer below is the defense-in-depth
// follow-up.)
func ValidateExternalURL(ctx context.Context, rawURL string) error {
	u, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil {
		return fmt.Errorf("%w: invalid URL", ErrSSRFBlocked)
	}
	if !strings.EqualFold(u.Scheme, "https") {
		return fmt.Errorf("%w: only https scheme is permitted", ErrSSRFBlocked)
	}
	if u.Host == "" {
		return fmt.Errorf("%w: missing host", ErrSSRFBlocked)
	}

	host := u.Hostname()
	port := u.Port()
	if port != "" && port != "443" {
		return fmt.Errorf("%w: only port 443 is permitted (got %q)", ErrSSRFBlocked, port)
	}

	lowerHost := strings.ToLower(host)
	for _, suffix := range blockedHostSuffixes {
		if strings.HasSuffix(lowerHost, suffix) {
			return fmt.Errorf("%w: hostname suffix %q is restricted", ErrSSRFBlocked, suffix)
		}
	}

	resolver := net.DefaultResolver
	ips, err := resolver.LookupIPAddr(ctx, host)
	if err != nil {
		return fmt.Errorf("%w: DNS lookup failed: %v", ErrSSRFBlocked, err)
	}
	if len(ips) == 0 {
		return fmt.Errorf("%w: host did not resolve", ErrSSRFBlocked)
	}
	for _, ip := range ips {
		if reason := classifyIP(ip.IP); reason != "" {
			return fmt.Errorf("%w: resolved IP %s is %s", ErrSSRFBlocked, ip.IP, reason)
		}
	}
	return nil
}

// SafeDialContext is the connect-time half of the SSRF defense — the
// "SafeDialer follow-up" the ValidateExternalURL comment promised. It
// re-resolves the host, re-classifies every candidate IP, and dials the
// validated IP DIRECTLY. This closes the TOCTOU / DNS-rebinding window:
// without it, net/http re-resolves the hostname between
// ValidateExternalURL's check and the actual connect, so a malicious
// server could pass validation and then flip its A record to
// 169.254.169.254. Because the connection is made to the IP (not the
// hostname), net/http cannot re-resolve to an unvalidated address; TLS
// still verifies against the original hostname (the Transport derives
// SNI / ServerName from the request URL, not from this dial address).
//
// It also re-runs on every redirect hop (each makes a fresh DialContext),
// so following a redirect to an internal host is blocked here too. Only
// port 443 is dialable, matching ValidateExternalURL's https-only stance.
func SafeDialContext(ctx context.Context, network, addr string) (net.Conn, error) {
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		return nil, err
	}
	if port != "443" {
		return nil, fmt.Errorf("%w: only port 443 is permitted (got %q)", ErrSSRFBlocked, port)
	}
	ips, err := net.DefaultResolver.LookupIPAddr(ctx, host)
	if err != nil {
		return nil, fmt.Errorf("%w: DNS lookup failed: %v", ErrSSRFBlocked, err)
	}
	if len(ips) == 0 {
		return nil, fmt.Errorf("%w: host did not resolve", ErrSSRFBlocked)
	}
	for _, ip := range ips {
		if reason := classifyIP(ip.IP); reason != "" {
			return nil, fmt.Errorf("%w: resolved IP %s is %s", ErrSSRFBlocked, ip.IP, reason)
		}
	}
	d := &net.Dialer{Timeout: 10 * time.Second}
	var dialErr error
	for _, ip := range ips {
		conn, derr := d.DialContext(ctx, network, net.JoinHostPort(ip.IP.String(), port))
		if derr == nil {
			return conn, nil
		}
		dialErr = derr
	}
	return nil, dialErr
}

// SafeTransport returns an *http.Transport whose DialContext is
// SafeDialContext. Set it on any *http.Client that fetches an external
// URL (after ValidateExternalURL) to gain the connect-time IP
// re-validation. Cloning the default transport preserves sane proxy /
// keep-alive / timeout settings.
func SafeTransport() *http.Transport {
	t := http.DefaultTransport.(*http.Transport).Clone()
	t.DialContext = SafeDialContext
	return t
}

// blockedHostSuffixes intercepts hostnames that resolve to internal
// infrastructure on common LAN configurations.
var blockedHostSuffixes = []string{
	".local",
	".internal",
	".localhost",
	".lan",
	".intranet",
	".corp",
	".home",
	".arpa",
	"localhost",
}

// classifyIP returns a non-empty reason string when the IP falls in a
// blocked range. Empty string means the address is in routable public
// unicast space.
func classifyIP(ip net.IP) string {
	if ip == nil {
		return "unparseable"
	}
	if ip.IsLoopback() {
		return "loopback"
	}
	if ip.IsUnspecified() {
		return "unspecified"
	}
	if ip.IsLinkLocalUnicast() {
		return "link-local (cloud metadata range)"
	}
	if ip.IsPrivate() {
		return "private/RFC1918"
	}
	if ip.IsMulticast() {
		return "multicast"
	}
	if ip.IsInterfaceLocalMulticast() || ip.IsLinkLocalMulticast() {
		return "local multicast"
	}

	if v4 := ip.To4(); v4 != nil {
		// CGNAT 100.64.0.0/10
		if v4[0] == 100 && (v4[1]&0xC0) == 64 {
			return "CGNAT (100.64/10)"
		}
		// 0.0.0.0/8 "this network"
		if v4[0] == 0 {
			return "0.0.0.0/8 (this network)"
		}
		// 255.255.255.255 broadcast
		if v4[0] == 255 && v4[1] == 255 && v4[2] == 255 && v4[3] == 255 {
			return "broadcast"
		}
		// IETF documentation ranges
		if v4[0] == 192 && v4[1] == 0 && v4[2] == 2 {
			return "TEST-NET-1 (192.0.2/24)"
		}
		if v4[0] == 198 && v4[1] == 51 && v4[2] == 100 {
			return "TEST-NET-2 (198.51.100/24)"
		}
		if v4[0] == 203 && v4[1] == 0 && v4[2] == 113 {
			return "TEST-NET-3 (203.0.113/24)"
		}
	}
	return ""
}
