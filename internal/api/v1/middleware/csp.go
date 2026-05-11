package middleware

import (
	"crypto/rand"
	"encoding/base64"
	"os"

	"github.com/gofiber/fiber/v2"
)

// CSPNonceLocalsKey is the c.Locals() key under which the per-request CSP
// nonce is stored. Templates / SPA shell injectors can fetch it via
// c.Locals(middleware.CSPNonceLocalsKey).(string).
const CSPNonceLocalsKey = "csp_nonce"

// cspNonceHeader is exposed on the response so an SPA-aware reverse proxy
// (e.g. nginx with ngx_http_sub_module) can swap a `__CSP_NONCE__`
// placeholder in served HTML for the real nonce.
const cspNonceHeader = "X-CSP-Nonce"

// CSPNonce returns Fiber middleware that issues a fresh per-request nonce,
// stores it on c.Locals, and emits a Content-Security-Policy header that
// whitelists `'self'` plus that nonce for scripts and styles.
//
// Env var semantics (default is ENFORCE — the safe production default):
//
//   - (unset)              → enforce mode. Header: `Content-Security-Policy`.
//   - CSP_REPORT_ONLY=1    → flips to `Content-Security-Policy-Report-Only`.
//     Use this in dev (Vite HMR injects unnonced inline
//     scripts) and during staging rollouts where you want
//     to collect violation reports without blocking.
//   - CSP_DISABLE=1        → emits no CSP header at all. Reserved for raw
//     debugging during a production incident; never use
//     this as a normal operating mode.
//
// CSP_DISABLE takes precedence over CSP_REPORT_ONLY.
func CSPNonce() fiber.Handler {
	disabled := os.Getenv("CSP_DISABLE") == "1"
	headerName := "Content-Security-Policy"
	if os.Getenv("CSP_REPORT_ONLY") == "1" {
		headerName = "Content-Security-Policy-Report-Only"
	}

	return func(c *fiber.Ctx) error {
		var buf [16]byte
		if _, err := rand.Read(buf[:]); err != nil {
			return err
		}
		nonce := base64.StdEncoding.EncodeToString(buf[:])

		// Always populate Locals + X-CSP-Nonce so the nginx sub_filter
		// substitution still works even when CSP is disabled — disabling
		// CSP shouldn't break inline-script rendering downstream.
		c.Locals(CSPNonceLocalsKey, nonce)
		c.Set(cspNonceHeader, nonce)

		if !disabled {
			c.Set(headerName,
				"default-src 'self'; "+
					"script-src 'self' 'nonce-"+nonce+"' 'strict-dynamic'; "+
					"style-src 'self' 'nonce-"+nonce+"'; "+
					"img-src 'self' data: blob: https:; "+
					"font-src 'self' data:; "+
					"connect-src 'self'; "+
					"frame-ancestors 'none'; "+
					"base-uri 'self'; "+
					"form-action 'self'; "+
					"object-src 'none'; "+
					"upgrade-insecure-requests")
		}

		return c.Next()
	}
}
