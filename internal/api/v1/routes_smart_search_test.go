// Route-level test for the smart-search Reindex gate.
//
// Bug being defended: `routes_p3_features.go` used to unconditionally
// mount `POST /courses/:course_id/smart_search/reindex`, but the
// handler is wired in `cmd/server/main.go` with `nil` Sources because
// the per-content reindex adapters (announcement / assignment / page /
// discussion topic listers) aren't built yet. The result was a 501
// Not Implemented advertised as a real endpoint — clients (and audit
// tools) cannot distinguish "feature disabled" from "feature broken,
// please retry."
//
// Fix: gate the route mount on `r.SmartSearchHandler.Sources != nil`
// inside `mountSmartSearchRoutes`. When sources are nil, the route
// simply does not exist (Fiber returns 404), which is the honest
// shape — same convention as `if r.GamificationHandler == nil
// { return }` in `routes_gamification.go`.
//
// This test pins both halves of the contract:
//   - Sources == nil  → POST reindex returns 404 (not 501).
//   - Sources != nil  → POST reindex returns NOT 404 (route mounted).
package v1

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/recover"

	"github.com/EduThemes/paper-lms/internal/api/v1/handlers"
)

// fakeReindexSources satisfies handlers.ReindexSourceLister so we can
// exercise the positive-control (Sources != nil) branch without
// standing up the per-content repositories.
type fakeReindexSources struct{}

func (fakeReindexSources) ListIndexable(_ context.Context, _ uint) ([]handlers.ReindexableItem, error) {
	return nil, nil
}

func newSmartSearchRouterForTest(sources handlers.ReindexSourceLister) (*Router, *fiber.App) {
	// nil *service.SmartSearchService is acceptable here — the
	// handler's Reindex method short-circuits on Sources == nil
	// (gate-off case) and is never invoked in the gate-on case
	// because the route 404s before reaching the handler. For the
	// positive control, ListIndexable returns an empty slice so
	// Reindex never dereferences the search service.
	h := handlers.NewSmartSearchHandler(nil, sources)
	r := &Router{SmartSearchHandler: h}
	app := fiber.New(fiber.Config{
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
	// The test passes nil to NewSmartSearchHandler's service arg —
	// when a route IS mounted and gets invoked, the handler will
	// panic dereferencing nil. We want the panic to surface as a
	// 5xx (proof the route is mounted) rather than crashing the
	// test runner. fiber/recover converts panics to errors that
	// the ErrorHandler above turns into 500s.
	app.Use(recover.New())
	// Stub middleware that sets the Locals the handlers expect from
	// the real Protected() chain. This test is about presence vs
	// absence of the route — the handlers must not panic on missing
	// Locals when they DO get called.
	stub := func(c *fiber.Ctx) error {
		c.Locals("user_id", uint(1))
		c.Locals("account_id", uint(1))
		return c.Next()
	}
	app.Use(stub)
	noop := func(c *fiber.Ctx) error { return c.Next() }
	r.mountSmartSearchRoutes(app, noop, noop)
	return r, app
}

// TestSmartSearchReindex_RouteGate_NoSources_404 is the audit-locking
// case: when main.go passes `nil` to NewSmartSearchHandler (today's
// production wiring), POST .../reindex must 404, NOT 501. A 501 would
// advertise an unimplemented endpoint and invite retry storms.
func TestSmartSearchReindex_RouteGate_NoSources_404(t *testing.T) {
	_, app := newSmartSearchRouterForTest(nil)

	req := httptest.NewRequest("POST", "/courses/42/smart_search/reindex", nil)
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("Sources == nil: want 404, got %d (501 means the gate regressed and we're advertising an unimplemented endpoint again)", resp.StatusCode)
	}
}

// TestSmartSearchReindex_RouteGate_WithSources_Mounted is the
// positive control: once sources land, the route MUST mount. Any
// status other than 404 demonstrates the gate let it through. (We
// intentionally don't assert 200 — the handler dereferences a nil
// search service in this stripped-down test app, so the actual status
// is a 5xx from the ErrorHandler. What matters is that 404 is no
// longer the answer.)
func TestSmartSearchReindex_RouteGate_WithSources_Mounted(t *testing.T) {
	_, app := newSmartSearchRouterForTest(fakeReindexSources{})

	req := httptest.NewRequest("POST", "/courses/42/smart_search/reindex", nil)
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}
	if resp.StatusCode == http.StatusNotFound {
		t.Fatalf("Sources != nil: route should be mounted, got 404 (the gate is too aggressive)")
	}
}

// TestSmartSearchSearch_RouteGate_AlwaysMounted documents the
// invariant that the Search endpoint is NOT gated on Sources. Search
// reads from content_embeddings directly and works the moment any
// row is indexed by any path (manual reindex, future webhook, future
// background worker). Gating Search on Sources would be a regression.
func TestSmartSearchSearch_RouteGate_AlwaysMounted(t *testing.T) {
	_, app := newSmartSearchRouterForTest(nil)

	req := httptest.NewRequest("GET", "/courses/42/smart_search?q=hello", nil)
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}
	if resp.StatusCode == http.StatusNotFound {
		t.Fatalf("Search must always be mounted; got 404")
	}
}
