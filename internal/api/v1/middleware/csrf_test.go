package middleware_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/EduThemes/paper-lms/internal/api/v1/middleware"
	"github.com/EduThemes/paper-lms/internal/testutil"
	"github.com/gofiber/fiber/v2"
	"github.com/stretchr/testify/assert"
)

// setupCSRFApp mounts CSRFProtection on a simple Fiber app with a POST handler
// that echoes 200. The CSRF middleware is the only thing in front of the
// handler so the assertions are purely about the middleware's behavior.
func setupCSRFApp() *fiber.App {
	app := testutil.SetupTestApp()
	app.Use(middleware.CSRFProtection())
	app.Post("/echo", func(c *fiber.Ctx) error {
		return c.Status(fiber.StatusOK).JSON(fiber.Map{"ok": true})
	})
	return app
}

// TestCSRFProtection_CookieAuthMissingHeader_403 — a browser session (cookie
// auth) POSTing without the X-CSRF-Token header is the exact attack shape
// CSRF protection is designed for. Must stay 403.
func TestCSRFProtection_CookieAuthMissingHeader_403(t *testing.T) {
	app := setupCSRFApp()

	req := httptest.NewRequest(http.MethodPost, "/echo", nil)
	req.Header.Set("Content-Type", "application/json")
	// A cookie-auth client typically has the CSRF cookie set (from a prior
	// GET) but a forged cross-site POST cannot include the matching header.
	req.AddCookie(&http.Cookie{Name: "paper_csrf", Value: "deadbeef"})
	req.AddCookie(&http.Cookie{Name: "paper_session", Value: "stub-session-token"})

	resp, err := app.Test(req, -1)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusForbidden, resp.StatusCode)
}

// TestCSRFProtection_CookieAuthValidPair_200 — a legitimate same-origin POST
// from the SPA reads the CSRF cookie via JS and echoes it in the header.
// Must stay 200.
func TestCSRFProtection_CookieAuthValidPair_200(t *testing.T) {
	app := setupCSRFApp()

	const csrfToken = "matching-csrf-token-value"

	req := httptest.NewRequest(http.MethodPost, "/echo", nil)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-CSRF-Token", csrfToken)
	req.AddCookie(&http.Cookie{Name: "paper_csrf", Value: csrfToken})
	req.AddCookie(&http.Cookie{Name: "paper_session", Value: "stub-session-token"})

	resp, err := app.Test(req, -1)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
}

// TestCSRFProtection_BearerAuthNoCSRFHeader_200 — the fix. A Personal Access
// Token or OAuth2 caller carries `Authorization: Bearer ...` and no CSRF
// cookie. Bearer tokens are not ambient credentials (the browser does not
// attach them automatically) so CSRF cannot apply. The middleware must
// short-circuit and let the request through to the auth middleware, which
// validates the token independently.
func TestCSRFProtection_BearerAuthNoCSRFHeader_200(t *testing.T) {
	app := setupCSRFApp()

	req := httptest.NewRequest(http.MethodPost, "/echo", nil)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer some-personal-access-token")
	// No paper_csrf cookie, no X-CSRF-Token header — same shape as curl/CLI/SDK.

	resp, err := app.Test(req, -1)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
}
