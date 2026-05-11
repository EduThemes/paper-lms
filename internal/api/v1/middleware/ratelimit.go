package middleware

import (
	"strconv"
	"sync"
	"time"

	"github.com/gofiber/fiber/v2"
)

// clientRecord tracks request timestamps for a single client within the sliding window.
type clientRecord struct {
	timestamps []time.Time
	mu         sync.Mutex
}

// rateLimiter holds the shared state for one rate-limiting rule.
type rateLimiter struct {
	clients     sync.Map // map[string]*clientRecord
	maxRequests int
	window      time.Duration
}

// newRateLimiter creates a rateLimiter and starts a background goroutine that
// evicts stale entries every 5 minutes so the map does not grow without bound.
func newRateLimiter(maxRequests int, window time.Duration) *rateLimiter {
	rl := &rateLimiter{
		maxRequests: maxRequests,
		window:      window,
	}

	go rl.cleanup()

	return rl
}

// allow checks whether the given key (IP address) is within the rate limit.
// It returns whether the request is allowed, how many requests remain in the
// current window, and the duration the caller should wait before retrying when
// the limit has been reached.
func (rl *rateLimiter) allow(key string) (allowed bool, remaining int, retryAfter time.Duration) {
	now := time.Now()
	windowStart := now.Add(-rl.window)

	val, _ := rl.clients.LoadOrStore(key, &clientRecord{})
	record := val.(*clientRecord)

	record.mu.Lock()
	defer record.mu.Unlock()

	// Prune timestamps that have fallen outside the sliding window.
	valid := record.timestamps[:0]
	for _, ts := range record.timestamps {
		if ts.After(windowStart) {
			valid = append(valid, ts)
		}
	}
	record.timestamps = valid

	if len(record.timestamps) >= rl.maxRequests {
		// The earliest request in the window dictates when the client can retry.
		earliest := record.timestamps[0]
		retryAfter = rl.window - now.Sub(earliest)
		if retryAfter < 0 {
			retryAfter = 0
		}
		return false, 0, retryAfter
	}

	record.timestamps = append(record.timestamps, now)
	remaining = rl.maxRequests - len(record.timestamps)

	return true, remaining, 0
}

// cleanup runs every 5 minutes and removes entries whose most recent request
// is older than the sliding window, preventing unbounded memory growth.
func (rl *rateLimiter) cleanup() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		now := time.Now()
		windowStart := now.Add(-rl.window)

		rl.clients.Range(func(key, val any) bool {
			record := val.(*clientRecord)

			record.mu.Lock()
			allExpired := true
			for _, ts := range record.timestamps {
				if ts.After(windowStart) {
					allExpired = false
					break
				}
			}
			record.mu.Unlock()

			if allExpired {
				rl.clients.Delete(key)
			}
			return true
		})
	}
}

// RateLimitMiddleware returns a Fiber handler that enforces a sliding-window
// rate limit of maxRequests per window, keyed by client IP address.
func RateLimitMiddleware(maxRequests int, window time.Duration) fiber.Handler {
	rl := newRateLimiter(maxRequests, window)

	return func(c *fiber.Ctx) error {
		ip := c.IP()

		allowed, _, retryAfter := rl.allow(ip)
		if !allowed {
			seconds := int(retryAfter.Seconds()) + 1
			c.Set("Retry-After", strconv.Itoa(seconds))

			return c.Status(fiber.StatusTooManyRequests).JSON(fiber.Map{
				"errors": []fiber.Map{
					{
						"message": "Rate limit exceeded. Try again later.",
					},
				},
			})
		}

		return c.Next()
	}
}

// RateLimitMiddlewareByUser is like RateLimitMiddleware but keys by the
// authenticated user_id from c.Locals (set by RequireAuth). Falls back to IP
// for unauthenticated requests so the limit still applies. Use this for
// expensive per-account operations like AI Assist where shared NAT IPs at
// schools would unfairly aggregate students into one bucket.
func RateLimitMiddlewareByUser(maxRequests int, window time.Duration) fiber.Handler {
	rl := newRateLimiter(maxRequests, window)

	return func(c *fiber.Ctx) error {
		key := "ip:" + c.IP()
		if uid, ok := c.Locals("user_id").(uint); ok && uid != 0 {
			key = "user:" + strconv.FormatUint(uint64(uid), 10)
		}

		allowed, _, retryAfter := rl.allow(key)
		if !allowed {
			seconds := int(retryAfter.Seconds()) + 1
			c.Set("Retry-After", strconv.Itoa(seconds))

			return c.Status(fiber.StatusTooManyRequests).JSON(fiber.Map{
				"errors": []fiber.Map{
					{
						"message": "Rate limit exceeded. Try again later.",
					},
				},
			})
		}

		return c.Next()
	}
}

// AuthRateLimit returns a rate-limiting handler preconfigured for
// authentication endpoints: 10 requests per minute per IP address.
func AuthRateLimit() fiber.Handler {
	return RateLimitMiddleware(10, 1*time.Minute)
}

// UploadRateLimit returns a rate-limiting handler preconfigured for
// file upload and import endpoints: 10 requests per 5 minutes per IP address.
func UploadRateLimit() fiber.Handler {
	return RateLimitMiddleware(10, 5*time.Minute)
}

// ExpensiveOpRateLimit returns a rate-limiting handler preconfigured for
// expensive operations (batch ops, migrations, cloning): 5 requests per minute per IP address.
func ExpensiveOpRateLimit() fiber.Handler {
	return RateLimitMiddleware(5, 1*time.Minute)
}

// AIAssistRateLimit returns a rate-limiting handler preconfigured for AI
// Assist endpoints: 30 requests per 5 minutes per authenticated user. Per-user
// keying so shared school NATs don't penalize the whole class.
func AIAssistRateLimit() fiber.Handler {
	return RateLimitMiddlewareByUser(30, 5*time.Minute)
}
