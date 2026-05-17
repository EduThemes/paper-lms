package handlers

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/url"
	"strings"
)

// ErrSSRFBlocked is returned by validateExternalURL when the URL
// targets a host or network range that's off-limits to test actions
// originating from the server.
var ErrSSRFBlocked = errors.New("URL targets a restricted destination")

// validateExternalURL enforces SSRF defense-in-depth on URLs that
// originate from request bodies and end up as outbound HTTP targets
// (today: the OIDC discovery test in Wave 3). The contract:
//
//  1. Scheme MUST be https. Plain http would let a misconfigured
//     issuer return a cleartext discovery doc that an in-network
//     attacker could MITM into pointing at attacker infrastructure.
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
//     bypass (169.254.169.254 listens on 80; an https proxy on
//     443 may not).
//
// The DNS lookup happens here so the IP-range check is done on the
// *resolved* address, not just the hostname text. A DNS-rebinding
// attack would defeat a hostname-text-only check; resolve+block does
// not because the resolved IP at TIME-OF-CHECK is what we block on.
// (TOCTOU between this check and the actual HTTP request is a real
// concern in pathological cases — Go's net/http uses Resolver which
// can re-resolve between the check and the connect. We accept that
// risk for v1; a defense-in-depth follow-up is using a custom
// http.Transport that re-checks the connection's RemoteAddr.)
//
// All super_admin endpoints already require authentication and the
// `super_admin` role, so a successful SSRF requires compromising a
// platform operator first — but a Canvas-CVE-style escalation through
// a non-super endpoint that happened to expose this URL would
// otherwise leak the server's network position. Defense-in-depth.
func validateExternalURL(ctx context.Context, rawURL string) error {
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

	// Resolve the host to one or more IPs and reject if any resolution
	// lands in a private/reserved range. We reject on ANY blocked
	// answer rather than ALL, because a multi-record response that
	// includes a private IP is the canonical DNS-rebinding pivot.
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

// blockedHostSuffixes intercepts hostnames that resolve to internal
// infrastructure on common LAN configurations. The IP-range block
// below catches most of these on resolution, but some networks
// proxy these suffixes to private destinations via /etc/hosts or
// split-horizon DNS — listing the suffixes is belt-and-suspenders.
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
//
// Blocked ranges (per IANA + cloud-provider conventions):
//   - Loopback (127.0.0.0/8, ::1)
//   - Private IPv4 (10/8, 172.16/12, 192.168/16) — RFC 1918
//   - Link-local (169.254/16, fe80::/10) — incl. AWS/GCP/Azure
//     instance metadata at 169.254.169.254 and IPv6 equivalent.
//   - Shared address space (100.64/10) — RFC 6598 carrier-grade NAT
//   - Multicast (224/4, ff00::/8)
//   - Unspecified (0.0.0.0/8, ::/128)
//   - IPv6 unique local (fc00::/7) — RFC 4193
//   - IETF reserved test ranges
//   - Broadcast (255.255.255.255)
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

	// net.IP.IsPrivate covers 10/8, 172.16/12, 192.168/16, fc00::/7,
	// and (per Go 1.17+) fd00::/8. It does NOT cover CGNAT 100.64/10
	// or IETF documentation ranges — add explicitly.
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
