package middleware_test

import (
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/EduThemes/paper-lms/internal/api/v1/middleware"
	"github.com/stretchr/testify/assert"
)

// newApp wires the per-user rate limiter behind a fake auth shim that pulls
// the user_id from the X-Test-User header so tests can simulate distinct users
// without standing up the full JWT middleware.
func newPerUserApp(max int, window time.Duration) *fiber.App {
	app := fiber.New()
	app.Use(func(c *fiber.Ctx) error {
		if h := c.Get("X-Test-User"); h != "" {
			var uid uint
			for _, b := range []byte(h) {
				if b < '0' || b > '9' {
					uid = 0
					break
				}
				uid = uid*10 + uint(b-'0')
			}
			c.Locals("user_id", uid)
		}
		return c.Next()
	})
	app.Use(middleware.RateLimitMiddlewareByUser(max, window))
	app.Get("/", func(c *fiber.Ctx) error { return c.SendString("ok") })
	return app
}

func TestRateLimitByUser_SeparateBuckets(t *testing.T) {
	app := newPerUserApp(2, time.Minute)

	hit := func(user string) int {
		req := httptest.NewRequest("GET", "/", nil)
		if user != "" {
			req.Header.Set("X-Test-User", user)
		}
		resp, err := app.Test(req, -1)
		assert.NoError(t, err)
		return resp.StatusCode
	}

	// User 1 burns its quota.
	assert.Equal(t, 200, hit("1"))
	assert.Equal(t, 200, hit("1"))
	assert.Equal(t, 429, hit("1"))

	// User 2 still has full quota — buckets are separate.
	assert.Equal(t, 200, hit("2"))
	assert.Equal(t, 200, hit("2"))
	assert.Equal(t, 429, hit("2"))
}

func TestRateLimitByUser_FallsBackToIP(t *testing.T) {
	app := newPerUserApp(2, time.Minute)

	hit := func() int {
		req := httptest.NewRequest("GET", "/", nil)
		resp, err := app.Test(req, -1)
		assert.NoError(t, err)
		return resp.StatusCode
	}

	// No X-Test-User header — middleware falls back to IP keying. All three
	// requests share the same fake IP, so the third one trips the limit.
	assert.Equal(t, 200, hit())
	assert.Equal(t, 200, hit())
	assert.Equal(t, 429, hit())
}

func TestRateLimitByUser_RetryAfterHeader(t *testing.T) {
	app := newPerUserApp(1, time.Minute)

	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("X-Test-User", "7")
	resp, _ := app.Test(req, -1)
	assert.Equal(t, 200, resp.StatusCode)

	req = httptest.NewRequest("GET", "/", nil)
	req.Header.Set("X-Test-User", "7")
	resp, _ = app.Test(req, -1)
	assert.Equal(t, 429, resp.StatusCode)
	assert.NotEmpty(t, resp.Header.Get("Retry-After"))
}
