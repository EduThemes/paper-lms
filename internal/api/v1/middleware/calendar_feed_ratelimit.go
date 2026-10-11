package middleware

import (
	"strconv"
	"time"

	"github.com/gofiber/fiber/v2"
)

// Calendar feed budget (G096): per client IP, in its own bucket.
const (
	calendarFeedMaxRequests = 120
	calendarFeedWindow      = time.Minute
)

// CalendarFeedRateLimit throttles the public ICS feed
// (/feeds/calendars/:token). It keeps its own bucket ("calfeed:ip:<ip>")
// rather than sharing the login limiter's: calendar clients (Google,
// Outlook, Apple, a school NAT full of desktop clients) poll feeds from
// shared IPs, and sharing the 10/min login bucket both broke those
// subscriptions and locked people behind the same IP out of signing in.
// The 256-bit token makes guessing hopeless, so the limit only caps how
// often one IP can make the server build calendars.
func CalendarFeedRateLimit() fiber.Handler {
	rl := newRateLimiter(calendarFeedMaxRequests, calendarFeedWindow)
	return func(c *fiber.Ctx) error {
		key := "calfeed:ip:" + c.IP()
		var allowed bool
		var retryAfter time.Duration
		if s := getStore(); s != nil {
			allowed, _, retryAfter = s.Allow(key, calendarFeedMaxRequests, calendarFeedWindow)
		} else {
			allowed, _, retryAfter = rl.allow(key)
		}
		if !allowed {
			c.Set("Retry-After", strconv.Itoa(int(retryAfter.Seconds())+1))
			return c.Status(fiber.StatusTooManyRequests).JSON(fiber.Map{
				"errors": []fiber.Map{{"message": "Rate limit exceeded. Try again later."}},
			})
		}
		return c.Next()
	}
}
