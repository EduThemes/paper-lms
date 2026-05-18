package handlers_test

// Tests for the super_admin role gate on /superadmin/* routes.
//
// Threat model — Canvas-LMS site_admin precedent
// ───────────────────────────────────────────────
// Canvas's site_admin CVEs (e.g. CVE-2021-32585) traced to a
// cross-tenant superuser role where individual gates forgot to check
// correctly. The matrix below pins the contract we promise:
//
//   role         /superadmin/settings           cross-tenant settings read
//   ────────     ─────────────────────────────  ──────────────────────────
//   none/anon    401 (Protected, not tested     n/a (can't reach)
//                here — protected by Protected)
//   user         403                            n/a
//   admin        403                            n/a
//   admin@root   403 (account 1 is NOT special  n/a
//                for the super_admin gate; only
//                role=="super_admin" passes)
//   super_admin  200                            200 (intended)
//
// The admin@root case is critical: RequireAdmin historically lets
// admins of account_id=1 (the root) act across child tenants. The
// super_admin gate MUST NOT honor that — only the literal
// role=="super_admin" should pass. A regression here is exactly the
// Canvas-style escalation we're defending against.

import (
	"context"
	"encoding/base64"
	"net/http"
	"testing"

	"github.com/gofiber/fiber/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"github.com/EduThemes/paper-lms/internal/api/v1/handlers"
	"github.com/EduThemes/paper-lms/internal/api/v1/middleware"
	"github.com/EduThemes/paper-lms/internal/auth"
	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/service/settings"
	"github.com/EduThemes/paper-lms/internal/testutil"
	"github.com/EduThemes/paper-lms/internal/testutil/mocks"
)

// mountSuperAdminRoutes wires the production RequireSuperAdmin gate
// in front of the production handler. The auth stub stands in for
// Protected (sets user_id + account_id Locals). The user repo mock
// returns a user with the requested role — this is the exact path
// RequireSuperAdmin walks.
func mountSuperAdminRoutes(t *testing.T, callerUserID, callerAccountID uint, role string) *fiber.App {
	t.Helper()

	// secretbox key — Set calls in the seed path need it.
	key := make([]byte, 32)
	t.Setenv("MFA_ENCRYPTION_KEY", base64.StdEncoding.EncodeToString(key))
	_ = auth.EnsureKeysLoaded()

	userRepo := new(mocks.MockUserRepository)
	user := &models.User{
		ID:        callerUserID,
		AccountID: callerAccountID,
		Role:      role,
	}
	userRepo.On("FindByID", mock.Anything, callerUserID, uint(0)).Return(user, nil)

	enrollmentRepo := new(mocks.MockEnrollmentRepository)
	pm := middleware.NewPermissionMiddleware(enrollmentRepo, userRepo)

	repo := newMemSettingRepo()
	svc := settings.NewService(repo, &fakeAccountAncestry{parents: map[uint]uint{}}, nil)
	svc.SetEnvReader(func(string) string { return "" })
	handler := handlers.NewSuperAdminSettingsHandler(svc, nil, nil)

	app := testutil.SetupTestApp()

	// Stand in for middleware.Protected — sets the Locals
	// RequireSuperAdmin reads. NOTE: we deliberately do NOT set
	// is_super_admin Locals here — RequireSuperAdmin must determine
	// that itself from the DB-fetched user.Role.
	app.Use(func(c *fiber.Ctx) error {
		if callerUserID != 0 {
			c.Locals("user_id", callerUserID)
			c.Locals("account_id", callerAccountID)
		}
		return c.Next()
	})

	superAdmin := app.Group("/superadmin", pm.RequireSuperAdmin())
	superAdmin.Get("/settings/groups", handler.Groups)
	superAdmin.Get("/settings", handler.List)
	superAdmin.Get("/settings/:key", handler.Get)

	return app
}

func TestSuperAdminGate_UserRoleIs403(t *testing.T) {
	app := mountSuperAdminRoutes(t, 10, 1, "user")
	resp := testutil.MakeRequest(app, http.MethodGet, "/superadmin/settings", nil)
	assert.Equal(t, http.StatusForbidden, resp.StatusCode)
}

func TestSuperAdminGate_AccountAdminIs403(t *testing.T) {
	// Same tenant, role=admin. The admin can manage their own
	// tenant via RequireAdmin elsewhere, but MUST NOT reach
	// /superadmin/*.
	app := mountSuperAdminRoutes(t, 10, 1, "admin")
	resp := testutil.MakeRequest(app, http.MethodGet, "/superadmin/settings", nil)
	assert.Equal(t, http.StatusForbidden, resp.StatusCode)
}

func TestSuperAdminGate_RootAccountAdminIs403(t *testing.T) {
	// CANVAS-PRECEDENT CASE: account 1 is the root account; an
	// admin of account 1 can act across child tenants via
	// RequireAdmin's legacy site-admin fallback. RequireSuperAdmin
	// MUST NOT honor that — only the literal role super_admin passes.
	app := mountSuperAdminRoutes(t, 10, 1, "admin")
	resp := testutil.MakeRequest(app, http.MethodGet, "/superadmin/settings/groups", nil)
	assert.Equal(t, http.StatusForbidden, resp.StatusCode,
		"admin of account 1 must NOT inherit super-admin via legacy site-admin behavior")
}

func TestSuperAdminGate_SuperAdminIs200(t *testing.T) {
	app := mountSuperAdminRoutes(t, 10, 1, "super_admin")
	resp := testutil.MakeRequest(app, http.MethodGet, "/superadmin/settings", nil)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
}

func TestSuperAdminGate_SuperAdminAccessesGroups(t *testing.T) {
	app := mountSuperAdminRoutes(t, 10, 1, "super_admin")
	resp := testutil.MakeRequest(app, http.MethodGet, "/superadmin/settings/groups", nil)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
}

func TestSuperAdminGate_MissingUserLocalsIs403(t *testing.T) {
	// callerUserID=0 means we don't set user_id Locals — simulates
	// a route mounted behind RequireSuperAdmin without Protected.
	// The gate must return 403 rather than panic or bypass.
	app := mountSuperAdminRoutes(t, 0, 0, "")
	resp := testutil.MakeRequest(app, http.MethodGet, "/superadmin/settings", nil)
	assert.Equal(t, http.StatusForbidden, resp.StatusCode)
}

func TestSuperAdminGate_CrossTenantReadAllowed(t *testing.T) {
	// A super_admin from tenant A asking for settings resolved at
	// tenant B's account_id must succeed (cross-tenant by design,
	// not the Canvas-CVE escalation).
	app := mountSuperAdminRoutes(t, 10, 1, "super_admin")

	resp := testutil.MakeRequest(app, http.MethodGet, "/superadmin/settings?account_id=2", nil)
	assert.Equal(t, http.StatusOK, resp.StatusCode,
		"super_admin crossing tenant boundaries via ?account_id is the intended behavior")
}

func TestSuperAdminGate_CompromisedRoleLiteralRejected(t *testing.T) {
	// Defensive: only the exact literal "super_admin" passes. A
	// near-miss like "Super_Admin" or "super-admin" or " super_admin "
	// must NOT pass. This locks the case-sensitive, no-trim contract
	// so a future code change can't relax it silently.
	for _, badRole := range []string{
		"Super_Admin",
		"super-admin",
		"SUPER_ADMIN",
		" super_admin",
		"super_admin ",
	} {
		t.Run(badRole, func(t *testing.T) {
			app := mountSuperAdminRoutes(t, 10, 1, badRole)
			resp := testutil.MakeRequest(app, http.MethodGet, "/superadmin/settings", nil)
			assert.Equal(t, http.StatusForbidden, resp.StatusCode,
				"role string %q must not satisfy super_admin", badRole)
		})
	}
}

// Compile-time sanity: confirm the production wiring at
// internal/api/v1/routes_super_admin.go uses the same RequireSuperAdmin
// gate this test exercises. If someone changes the production gate to
// a different middleware (or removes it), the route group below would
// no longer use the variable, this test would still pass, and the
// production wiring would be broken — so we also assert structurally
// that PermissionMiddleware.RequireSuperAdmin exists and returns a
// non-nil fiber.Handler.
func TestSuperAdminGate_HandlerExistsAndCompiles(t *testing.T) {
	pm := middleware.NewPermissionMiddleware(
		new(mocks.MockEnrollmentRepository),
		new(mocks.MockUserRepository),
	)
	gate := pm.RequireSuperAdmin()
	if gate == nil {
		t.Fatal("PermissionMiddleware.RequireSuperAdmin must return a non-nil handler")
	}
	// Belt-and-suspenders — context-free invocation should fail
	// closed (no Locals → 403). We construct a minimal Fiber app
	// just to feed the handler a *fiber.Ctx.
	app := testutil.SetupTestApp()
	app.Get("/g", gate)
	resp := testutil.MakeRequest(app, http.MethodGet, "/g", nil)
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("RequireSuperAdmin without Locals: expected 403, got %d", resp.StatusCode)
	}
	_ = context.Background()
}
