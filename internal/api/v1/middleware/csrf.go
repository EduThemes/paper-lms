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
func CSRFProtection() fiber.Handler {
	return func(c *fiber.Ctx) error {
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
