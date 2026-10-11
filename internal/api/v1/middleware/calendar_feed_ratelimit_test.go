package middleware_test

import (
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/EduThemes/paper-lms/internal/api/v1/middleware"
)

// G096: the public calendar feed is polled by calendar clients from
// shared IPs. It must neither be held to the login budget nor eat into
// it: exhausting sign-in attempts from an IP leaves the feed working,
// and heavy feed polling leaves sign-in working.
func TestCalendarFeedRateLimit_SeparateFromLoginBucket(t *testing.T) {
	app := fiber.New()
	login := middleware.AuthRateLimit()
	app.Post("/login", login, func(c *fiber.Ctx) error { return c.SendString("ok") })
	app.Get("/feed", middleware.CalendarFeedRateLimit(), func(c *fiber.Ctx) error { return c.SendString("ics") })

	do := func(method, path string) int {
		resp, err := app.Test(httptest.NewRequest(method, path, nil))
		require.NoError(t, err)
		return resp.StatusCode
	}

	// More feed polls than the login budget allows: all served.
	for i := 0; i < 30; i++ {
		require.Equal(t, fiber.StatusOK, do("GET", "/feed"), "feed poll %d", i)
	}
	// ...and sign-in still has its full budget.
	for i := 0; i < 10; i++ {
		require.Equal(t, fiber.StatusOK, do("POST", "/login"), "login %d", i)
	}
	assert.Equal(t, fiber.StatusTooManyRequests, do("POST", "/login"))
	// A throttled sign-in IP can still refresh its calendars.
	assert.Equal(t, fiber.StatusOK, do("GET", "/feed"))
}

func TestCalendarFeedRateLimit_EventuallyThrottles(t *testing.T) {
	app := fiber.New()
	app.Get("/feed", middleware.CalendarFeedRateLimit(), func(c *fiber.Ctx) error { return c.SendString("ics") })
	limited := false
	for i := 0; i < 500 && !limited; i++ {
		resp, err := app.Test(httptest.NewRequest("GET", "/feed", nil))
		require.NoError(t, err)
		if resp.StatusCode == fiber.StatusTooManyRequests {
			limited = true
			assert.NotEmpty(t, resp.Header.Get("Retry-After"))
		}
	}
	assert.True(t, limited, "one IP can't build calendars without limit")
}
