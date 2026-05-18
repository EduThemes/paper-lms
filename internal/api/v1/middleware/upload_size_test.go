package middleware_test

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strconv"
	"sync/atomic"
	"testing"

	"github.com/gofiber/fiber/v2"
	"github.com/stretchr/testify/assert"

	"github.com/EduThemes/paper-lms/internal/api/v1/middleware"
	"github.com/EduThemes/paper-lms/internal/settingsctx"
	"github.com/EduThemes/paper-lms/internal/testutil"
)

// stubLookup is a minimal SettingsLookupFunc that records the
// per-call account scope (so tests can assert "the middleware
// stamped the right account") and returns a canned string/error.
type stubLookup struct {
	mu      atomic.Value // last context's AccountID
	value   string
	err     error
	calls   atomic.Int32
}

func (s *stubLookup) fn(ctx context.Context, _ string) (string, error) {
	s.calls.Add(1)
	s.mu.Store(settingsctx.AccountIDFromContext(ctx))
	return s.value, s.err
}

func (s *stubLookup) lastAccountID() uint {
	v := s.mu.Load()
	if v == nil {
		return 0
	}
	return v.(uint)
}

// makeUploadApp builds a Fiber app with EnforceUploadSize mounted on
// POST /upload. A pre-middleware stamps account_id Locals so the
// middleware can resolve the per-tenant scope. user_id is also
// stamped so this test mirrors the real post-auth call site.
//
// BodyLimit is set above the largest test fixture so fasthttp's own
// pre-handler size check doesn't reject Content-Length test cases
// before the EnforceUploadSize middleware sees them. The
// middleware-level cap is the unit under test.
func makeUploadApp(lookup middleware.UploadSizeLookupFunc, accountID uint) *fiber.App {
	app := fiber.New(fiber.Config{
		BodyLimit: 1024 * 1024 * 1024, // 1 GB — covers every test fixture
		ErrorHandler: func(c *fiber.Ctx, err error) error {
			code := fiber.StatusInternalServerError
			if e, ok := err.(*fiber.Error); ok {
				code = e.Code
			}
			return c.Status(code).JSON(fiber.Map{
				"errors": []fiber.Map{{"message": err.Error()}},
			})
		},
	})
	app.Use(func(c *fiber.Ctx) error {
		c.Locals("user_id", uint(42))
		if accountID != 0 {
			c.Locals("account_id", accountID)
		}
		return c.Next()
	})
	app.Post("/upload", middleware.EnforceUploadSize(lookup), func(c *fiber.Ctx) error {
		return c.SendStatus(fiber.StatusCreated)
	})
	return app
}

// makeRequestWithContentLength constructs a POST whose body is `n`
// bytes of zeros. Content-Length is set automatically by httptest from
// the body's length. The middleware reads the Content-Length header to
// decide whether to reject — this is the load-bearing fast path on the
// real upload routes (browsers / curl both emit Content-Length up
// front).
func makeRequestWithContentLength(t *testing.T, app *fiber.App, n int) *http.Response {
	t.Helper()
	body := make([]byte, n)
	req := httptest.NewRequest(http.MethodPost, "/upload", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/octet-stream")
	req.Header.Set(fiber.HeaderContentLength, strconv.Itoa(n))
	req.ContentLength = int64(n)
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}
	return resp
}

// mb is a small helper for readable test fixtures.
const mb = 1024 * 1024

// TestEnforceUploadSize_PerTenantOverride_Caps locks the load-bearing
// contract: when the settings lookup returns a per-tenant override
// (e.g. "1" = 1 MB), the middleware MUST cap at that value, not at the
// catalog default. Without this, the per-tenant knob in the
// super-admin settings UI is a documented no-op.
func TestEnforceUploadSize_PerTenantOverride_Caps(t *testing.T) {
	stub := &stubLookup{value: "1"} // 1 MB per-tenant cap
	app := makeUploadApp(stub.fn, uint(7))

	// 2 MB request — well above the 1 MB per-tenant cap.
	resp := makeRequestWithContentLength(t, app, 2*mb)
	assert.Equal(t, http.StatusRequestEntityTooLarge, resp.StatusCode)
	assert.Equal(t, uint(7), stub.lastAccountID(),
		"middleware must stamp account_id from Locals onto the lookup ctx")

	// Under the cap — pass.
	stub.calls.Store(0)
	resp = makeRequestWithContentLength(t, app, 100*1024) // 100 KB
	assert.Equal(t, http.StatusCreated, resp.StatusCode)
}

// TestEnforceUploadSize_NoOverride_UsesDefault locks the fallback: when
// the lookup returns "" (nothing in the resolution chain), the
// middleware MUST fall through to the 5120 MB catalog default. A 100 MB
// body is well below the default, so should pass.
func TestEnforceUploadSize_NoOverride_UsesDefault(t *testing.T) {
	stub := &stubLookup{value: ""} // empty = no override
	app := makeUploadApp(stub.fn, uint(7))

	// 100 MB — easily under the 5 GB default, should pass.
	resp := makeRequestWithContentLength(t, app, 100*mb)
	assert.Equal(t, http.StatusCreated, resp.StatusCode,
		"100 MB request under 5 GB default cap must pass")
}

// TestEnforceUploadSize_InvalidValue_FallsBackToDefault locks the
// defensive fallback: a non-integer / negative / unparseable lookup
// result must NOT 500 the upload — the cap quietly falls back to the
// catalog default. Settings store hiccups are NEVER allowed to take
// down uploads.
func TestEnforceUploadSize_InvalidValue_FallsBackToDefault(t *testing.T) {
	stub := &stubLookup{value: "not-an-int"}
	app := makeUploadApp(stub.fn, uint(7))

	// 100 MB — should pass at the 5120 MB default fallback.
	resp := makeRequestWithContentLength(t, app, 100*mb)
	assert.Equal(t, http.StatusCreated, resp.StatusCode,
		"unparseable lookup value must NOT 500; fall back to default")
}

// TestEnforceUploadSize_LookupError_FallsBackToDefault locks the same
// invariant as above for a non-nil error path. Same rationale: the
// settings store going sideways must not cascade into a 5xx for every
// teacher trying to upload a file.
func TestEnforceUploadSize_LookupError_FallsBackToDefault(t *testing.T) {
	stub := &stubLookup{err: errors.New("settings store offline")}
	app := makeUploadApp(stub.fn, uint(7))

	resp := makeRequestWithContentLength(t, app, 100*mb)
	assert.Equal(t, http.StatusCreated, resp.StatusCode,
		"lookup error must NOT 500; fall back to default")
}

// TestEnforceUploadSize_NegativeValue_FallsBackToDefault locks the
// defensive fallback for negative / zero lookup values. A misconfigured
// "-1" or "0" in the settings store must NOT result in a stuck-at-zero
// cap that rejects every upload.
func TestEnforceUploadSize_NegativeValue_FallsBackToDefault(t *testing.T) {
	stub := &stubLookup{value: "-1"}
	app := makeUploadApp(stub.fn, uint(7))

	resp := makeRequestWithContentLength(t, app, 100*mb)
	assert.Equal(t, http.StatusCreated, resp.StatusCode,
		"negative cap value must NOT take down uploads; fall back to default")
}

// TestEnforceUploadSize_NilLookup_FallsBackToDefault locks the contract
// that a nil lookup function (defensive: future caller forgets to wire
// it) still produces sensible behavior at the catalog default rather
// than a panic.
func TestEnforceUploadSize_NilLookup_FallsBackToDefault(t *testing.T) {
	app := makeUploadApp(nil, uint(7))

	resp := makeRequestWithContentLength(t, app, 100*mb)
	assert.Equal(t, http.StatusCreated, resp.StatusCode)
}

// TestEnforceUploadSize_NoAccountIDLocal_StillResolves locks the
// instance-only resolution path: a callsite that doesn't carry
// account_id Locals (e.g. an unauthenticated upload route, or a
// background-job-driven upload) still resolves the cap via the
// instance scope. The lookup ctx carries no AccountID hint, so the
// settings chain skips account scope and reads instance/env/default.
func TestEnforceUploadSize_NoAccountIDLocal_StillResolves(t *testing.T) {
	stub := &stubLookup{value: "1"} // 1 MB
	app := makeUploadApp(stub.fn, 0) // no account_id in Locals

	resp := makeRequestWithContentLength(t, app, 2*mb)
	assert.Equal(t, http.StatusRequestEntityTooLarge, resp.StatusCode)
	assert.Equal(t, uint(0), stub.lastAccountID(),
		"middleware must NOT inject a bogus account_id when Locals is empty")
}

// TestEnforceUploadSize_NonBodyMethod_Skips locks that GET / DELETE
// requests bypass the cap check entirely. These methods don't carry an
// upload payload, and a settings-store hiccup on a GET path should
// never block a read.
func TestEnforceUploadSize_NonBodyMethod_Skips(t *testing.T) {
	stub := &stubLookup{value: "1"} // 1 MB — would otherwise reject everything
	app := testutil.SetupTestApp()
	app.Use(func(c *fiber.Ctx) error {
		c.Locals("account_id", uint(7))
		return c.Next()
	})
	app.Get("/read", middleware.EnforceUploadSize(stub.fn), func(c *fiber.Ctx) error {
		return c.SendStatus(fiber.StatusOK)
	})

	resp := testutil.MakeRequest(app, http.MethodGet, "/read", nil)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, int32(0), stub.calls.Load(),
		"GET must not trigger the lookup at all")
}
