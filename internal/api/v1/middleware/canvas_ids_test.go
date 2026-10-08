package middleware

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeCanvasIDs resolves a fixed table of (kind, value, account) → id.
type fakeCanvasIDs struct {
	ids   map[[3]string]uint
	calls []string
}

func (f *fakeCanvasIDs) ResolveCanvasID(_ context.Context, kind, value string, accountID uint) (uint, error) {
	f.calls = append(f.calls, kind+"="+value)
	acct := "0"
	if accountID != 0 {
		acct = "scoped"
	}
	if id, ok := f.ids[[3]string{kind, value, acct}]; ok {
		return id, nil
	}
	return 0, ErrCanvasIDNotFound
}

func canvasIDsApp(lookup CanvasIDLookup) *fiber.App {
	app := fiber.New()
	api := app.Group("/api/v1")
	protected := api.Group("", func(c *fiber.Ctx) error {
		c.Locals("user_id", uint(42))
		c.Locals("account_id", uint(7))
		return c.Next()
	})
	protected.Use(CanvasIDs(lookup, nil))
	echo := func(name string) fiber.Handler {
		return func(c *fiber.Ctx) error {
			return c.SendString(name + ":" + c.Params("id") + c.Params("user_id") + c.Params("course_id") + c.Params("account_id") + c.Params("section_id"))
		}
	}
	protected.Get("/users/self", echo("self-route"))
	protected.Get("/users/self/groups", echo("self-groups"))
	protected.Get("/users/:id", echo("user"))
	protected.Get("/users/:user_id/tokens", echo("tokens"))
	protected.Get("/courses/:course_id/modules", echo("modules"))
	protected.Get("/courses/:course_id/assignments/:assignment_id/submissions/:user_id", echo("submission"))
	protected.Get("/accounts/:account_id/terms", echo("terms"))
	protected.Get("/sections/:id", echo("section"))
	return app
}

func get(t *testing.T, app *fiber.App, path string) (int, string) {
	t.Helper()
	resp, err := app.Test(httptest.NewRequest(http.MethodGet, path, nil), -1)
	require.NoError(t, err)
	body, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, string(body)
}

func TestCanvasIDs_SelfAlias(t *testing.T) {
	app := canvasIDsApp(&fakeCanvasIDs{})

	// A route that names "self" literally keeps it.
	code, body := get(t, app, "/api/v1/users/self")
	assert.Equal(t, http.StatusOK, code)
	assert.Equal(t, "self-route:", body)
	_, body = get(t, app, "/api/v1/users/self/groups")
	assert.Equal(t, "self-groups:", body)

	// Everywhere else "self" becomes the caller.
	_, body = get(t, app, "/api/v1/users/self/tokens")
	assert.Equal(t, "tokens:42", body)
	_, body = get(t, app, "/api/v1/courses/3/assignments/9/submissions/self")
	assert.Equal(t, "submission:423", body)
	_, body = get(t, app, "/api/v1/accounts/self/terms")
	assert.Equal(t, "terms:7", body)

	// Numeric ids pass through untouched.
	_, body = get(t, app, "/api/v1/users/5")
	assert.Equal(t, "user:5", body)
}

func TestCanvasIDs_SISPrefixes(t *testing.T) {
	lookup := &fakeCanvasIDs{ids: map[[3]string]uint{
		{"sis_course_id", "BIO-101", "scoped"}: 11,
		{"sis_user_id", "S-1", "scoped"}:       21,
		{"sis_login_id", "jdoe", "scoped"}:     22,
		{"sis_section_id", "SEC 1", "scoped"}:  31,
		{"sis_account_id", "ROOT", "scoped"}:   7,
	}}
	app := canvasIDsApp(lookup)

	_, body := get(t, app, "/api/v1/courses/sis_course_id:BIO-101/modules")
	assert.Equal(t, "modules:11", body)
	// URL-encoded colon and a value with a space.
	_, body = get(t, app, "/api/v1/sections/sis_section_id%3ASEC%201")
	assert.Equal(t, "section:31", body)
	_, body = get(t, app, "/api/v1/users/sis_login_id:jdoe/tokens")
	assert.Equal(t, "tokens:22", body)
	// Canvas hex form: "S-1" is 532d31.
	_, body = get(t, app, "/api/v1/users/hex:sis_user_id:532d31")
	assert.Equal(t, "user:21", body)
	_, body = get(t, app, "/api/v1/accounts/sis_account_id:ROOT/terms")
	assert.Equal(t, "terms:7", body)

	// A miss (including another tenant's id) is a 404, never a fallthrough.
	code, _ := get(t, app, "/api/v1/courses/sis_course_id:NOPE/modules")
	assert.Equal(t, http.StatusNotFound, code)

	// A kind that does not fit the collection is left for the handler.
	_, body = get(t, app, "/api/v1/courses/sis_user_id:S-1/modules")
	assert.Equal(t, "modules:sis_user_id:S-1", body)
}

func TestRoutePatternMatches(t *testing.T) {
	split := func(s string) []string { return strings.Split(s, "/") }
	assert.True(t, routePatternMatches(split("/api/v1/users/self"), split("/api/v1/users/self")))
	assert.False(t, routePatternMatches(split("/api/v1/users/self"), split("/api/v1/users/self/tokens")))
	assert.True(t, routePatternMatches(split("/api/v1/courses/:course_id/submissions/self/files"), split("/api/v1/courses/4/submissions/self/files")))
	assert.True(t, routePatternMatches(split("/api/v1/files/*"), split("/api/v1/files/a/b")))
}
