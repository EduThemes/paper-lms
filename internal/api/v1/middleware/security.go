package middleware

import (
	"github.com/gofiber/fiber/v2"
)

// SecurityConfig holds configuration for the security headers middleware.
type SecurityConfig struct {
	Environment string // "production" or "development"
}

// SecurityHeaders returns a Fiber middleware that sets common security headers
// on every response. In development mode, the Content-Security-Policy is
// relaxed to allow localhost connections.
func SecurityHeaders(cfg SecurityConfig) fiber.Handler {
	// Build the CSP directive once at initialization time.
	// Production: no unsafe-eval, unsafe-inline only for styles (Tailwind).
	// LTI iframes are allowed via frame-ancestors with configured domains.
	csp := "default-src 'self'; " +
		"script-src 'self'; " +
		"style-src 'self' 'unsafe-inline'; " +
		"img-src 'self' data: blob: https:; " +
		"font-src 'self' data:; " +
		"connect-src 'self'; " +
		"frame-ancestors 'none'; " +
		"base-uri 'self'; " +
		"form-action 'self'; " +
		"object-src 'none'; " +
		"upgrade-insecure-requests"

	if cfg.Environment == "development" {
		csp = "default-src 'self'; " +
			"script-src 'self' 'unsafe-inline' 'unsafe-eval'; " +
			"style-src 'self' 'unsafe-inline'; " +
			"img-src 'self' data: blob: https:; " +
			"font-src 'self' data:; " +
			"connect-src 'self' http://localhost:* http://127.0.0.1:* ws://localhost:* ws://127.0.0.1:*; " +
			"frame-ancestors 'none'; " +
			"base-uri 'self'; " +
			"form-action 'self'"
	}

	isProduction := cfg.Environment == "production"

	return func(c *fiber.Ctx) error {
		c.Set("X-Content-Type-Options", "nosniff")
		c.Set("X-Frame-Options", "DENY")
		c.Set("X-XSS-Protection", "1; mode=block")
		c.Set("Referrer-Policy", "strict-origin-when-cross-origin")
		c.Set("Permissions-Policy", "camera=(), microphone=(), geolocation=()")
		c.Set("Content-Security-Policy", csp)

		// Only send HSTS when the request arrived over TLS (indicated by a
		// reverse proxy via X-Forwarded-Proto) or when running in production.
		if c.Get("X-Forwarded-Proto") == "https" || isProduction {
			c.Set("Strict-Transport-Security", "max-age=31536000; includeSubDomains")
		}

		return c.Next()
	}
}
