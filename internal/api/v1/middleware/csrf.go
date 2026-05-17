package middleware

import (
	"crypto/rand"
	"encoding/hex"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
)

const (
	csrfCookieName = "paper_csrf"
	csrfHeaderName = "X-CSRF-Token"
	csrfTokenLen   = 32
)

// CSRFProtection generates and validates CSRF tokens.
// Safe methods (GET, HEAD, OPTIONS) get a token set in a readable cookie.
// Unsafe methods (POST, PUT, DELETE, PATCH) must include the token in the X-CSRF-Token header.
//
// Bearer-token requests (Personal Access Tokens, OAuth2 clients) short-circuit
// the protection: CSRF defends against ambient credentials — the browser
// attaching a session cookie automatically to a cross-site forged request.
// A bearer token is NOT ambient: an attacker page cannot read it out of the
// legitimate client's storage and cannot make the browser attach it on its
// behalf. Forcing a CSRF cookie+header pair on bearer callers breaks every
// programmatic write (PAT, OAuth2, CLI, server-to-server) for zero security
// gain. The bearer token itself is validated independently by the auth
// middleware (see internal/api/v1/middleware/auth.go lines 33-39).
func CSRFProtection() fiber.Handler {
	return func(c *fiber.Ctx) error {
		// Bearer-auth callers are not CSRF-attackable; skip the cookie+header check.
		if strings.HasPrefix(c.Get("Authorization"), "Bearer ") {
			return c.Next()
		}

		method := strings.ToUpper(c.Method())

		// Safe methods: set/refresh the CSRF cookie and continue
		if method == "GET" || method == "HEAD" || method == "OPTIONS" {
			ensureCSRFCookie(c)
			return c.Next()
		}

		// Unsafe methods: validate the token
		cookieToken := c.Cookies(csrfCookieName)
		headerToken := c.Get(csrfHeaderName)

		if cookieToken == "" || headerToken == "" || cookieToken != headerToken {
			return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
				"errors": []fiber.Map{{"message": "CSRF token missing or invalid"}},
			})
		}

		return c.Next()
	}
}

func ensureCSRFCookie(c *fiber.Ctx) {
	if c.Cookies(csrfCookieName) != "" {
		return
	}

	token := generateCSRFToken()
	c.Cookie(&fiber.Cookie{
		Name:     csrfCookieName,
		Value:    token,
		Path:     "/",
		HTTPOnly: false, // Must be readable by JavaScript
		Secure:   c.Protocol() == "https",
		SameSite: "Strict",
		MaxAge:   int(24 * time.Hour / time.Second),
	})
}

func generateCSRFToken() string {
	b := make([]byte, csrfTokenLen)
	if _, err := rand.Read(b); err != nil {
		// Fallback should never happen but don't panic
		return hex.EncodeToString(b)
	}
	return hex.EncodeToString(b)
}
