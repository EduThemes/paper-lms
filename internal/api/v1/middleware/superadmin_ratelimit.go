package middleware

import (
	"fmt"
	"strconv"
	"time"

	"github.com/gofiber/fiber/v2"
)

// SuperAdminTestRateLimit limits how often a single super-admin can
// fire a single settings test action. 1 request per 30 seconds per
// (user_id, action) tuple — the operator can chain test/email →
// test/oidc → test/anthropic in rapid succession (each is a separate
// bucket), but cannot fat-finger or loop-test the same action.
//
// Per-action bucketing is critical: a global per-user limit would let
// the email test starve out the OIDC test, encouraging operators to
// disable rate limiting in production "just to get setup done."
//
// Keyed by (action, user_id) so the Redis-backed store (when wired)
// also partitions cleanly. Falls back to IP when user_id Locals is
// somehow unset — but RequireSuperAdmin ahead of this middleware
// guarantees a real user_id, so the IP fallback is defense-in-depth
// for misconfigured route chains.
//
// Per the 2026-05-17 Settings-Engine plan §Wave 3.
func SuperAdminTestRateLimit(action string) fiber.Handler {
	const (
		maxRequests = 1
		window      = 30 * time.Second
	)
	rl := newRateLimiter(maxRequests, window)

	return func(c *fiber.Ctx) error {
		key := "settings-test:" + action + ":ip:" + c.IP()
		if uid, ok := c.Locals("user_id").(uint); ok && uid != 0 {
			key = fmt.Sprintf("settings-test:%s:user:%d", action, uid)
		}

		var allowed bool
		var retryAfter time.Duration
		if s := getStore(); s != nil {
			allowed, _, retryAfter = s.Allow(key, maxRequests, window)
		} else {
			allowed, _, retryAfter = rl.allow(key)
		}
		if !allowed {
			seconds := int(retryAfter.Seconds()) + 1
			c.Set("Retry-After", strconv.Itoa(seconds))
			return c.Status(fiber.StatusTooManyRequests).JSON(fiber.Map{
				"errors": []fiber.Map{
					{"message": "Rate limit exceeded for " + action + " test. Try again in " + strconv.Itoa(seconds) + "s."},
				},
			})
		}
		return c.Next()
	}
}
