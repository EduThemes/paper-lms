package middleware_test

import (
	"context"
	"net/http"
	"sync/atomic"
	"testing"

	"github.com/gofiber/fiber/v2"
	"github.com/stretchr/testify/assert"

	"github.com/EduThemes/paper-lms/internal/api/v1/middleware"
	"github.com/EduThemes/paper-lms/internal/testutil"
)

// fakeEmitter counts AuditEventEmitter.LogEvent calls. Used to verify the
// AuditWrites middleware fires on 2xx writes and skips otherwise.
type fakeEmitter struct {
	calls atomic.Int32
	last  struct {
		eventType string
		userID    uint
		action    string
	}
}

func (f *fakeEmitter) LogEvent(_ context.Context, eventType string, userID uint, _ *uint, _ *uint, _ string, _ uint, action, _ string, _ string, _ string) error {
	f.calls.Add(1)
	f.last.eventType = eventType
	f.last.userID = userID
	f.last.action = action
	return nil
}

// TestAuditWrites_FiresOnSuccessfulPOST is the Wave C.3 lock — a 2xx POST
// inside the protected group must emit an audit_log row through the
// AuditEventEmitter. If this test regresses, every state-DPA write-side
// audit trail goes dark and the audit's "defined, never called" finding
// re-opens.
func TestAuditWrites_FiresOnSuccessfulPOST(t *testing.T) {
	emitter := &fakeEmitter{}

	app := testutil.SetupTestApp()
	// Auth stub so middleware can read user_id from Locals.
	app.Use(func(c *fiber.Ctx) error {
		c.Locals("user_id", uint(42))
		return c.Next()
	})
	app.Use(middleware.AuditWrites(emitter, "http.write"))
	app.Post("/things", func(c *fiber.Ctx) error {
		return c.Status(fiber.StatusCreated).JSON(fiber.Map{"ok": true})
	})

	resp := testutil.MakeRequest(app, http.MethodPost, "/things", nil)

	assert.Equal(t, http.StatusCreated, resp.StatusCode)
	assert.Equal(t, int32(1), emitter.calls.Load(), "AuditWrites should emit exactly one audit row on a 2xx POST")
	assert.Equal(t, "http.write", emitter.last.eventType)
	assert.Equal(t, uint(42), emitter.last.userID)
}

// TestAuditWrites_SkipsOnGET verifies the middleware does NOT emit on
// read-only methods. The read side is handled per-handler with
// LogPIIAccess to attach the student subject.
func TestAuditWrites_SkipsOnGET(t *testing.T) {
	emitter := &fakeEmitter{}

	app := testutil.SetupTestApp()
	app.Use(func(c *fiber.Ctx) error {
		c.Locals("user_id", uint(42))
		return c.Next()
	})
	app.Use(middleware.AuditWrites(emitter, "http.write"))
	app.Get("/things", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{"ok": true})
	})

	resp := testutil.MakeRequest(app, http.MethodGet, "/things", nil)

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, int32(0), emitter.calls.Load(), "AuditWrites must not emit on GET")
}

// TestAuditWrites_SkipsOnNon2xx verifies a 4xx response does NOT trigger
// audit emission — only successful writes count.
func TestAuditWrites_SkipsOnNon2xx(t *testing.T) {
	emitter := &fakeEmitter{}

	app := testutil.SetupTestApp()
	app.Use(func(c *fiber.Ctx) error {
		c.Locals("user_id", uint(42))
		return c.Next()
	})
	app.Use(middleware.AuditWrites(emitter, "http.write"))
	app.Post("/things", func(c *fiber.Ctx) error {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"err": "bad"})
	})

	resp := testutil.MakeRequest(app, http.MethodPost, "/things", nil)

	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	assert.Equal(t, int32(0), emitter.calls.Load(), "AuditWrites must not emit when status is non-2xx")
}
