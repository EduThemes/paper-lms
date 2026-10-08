package middleware_test

// Regression tests for PermissionMiddleware (core authz lane):
//   B064 / B078  the course-guard admin bypass is tenant-scoped.
//   B149         students / observers can't reach unpublished courses.
//   B069         account-scoped admin routes refuse another tenant's
//                :account_id in the URL.
//   B063         a masquerade can never exceed the real user's role.
// All data is made up.

import (
	"context"
	"errors"
	"net/http"
	"testing"

	"github.com/gofiber/fiber/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"github.com/EduThemes/paper-lms/internal/api/v1/middleware"
	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/service"
	"github.com/EduThemes/paper-lms/internal/testutil"
	"github.com/EduThemes/paper-lms/internal/testutil/mocks"
)

var errMissing = errors.New("record not found")

type permFixture struct {
	users       *mocks.MockUserRepository
	enrollments *mocks.MockEnrollmentRepository
	courses     *mocks.MockCourseRepository
	pm          *middleware.PermissionMiddleware
}

func newPermFixture() *permFixture {
	f := &permFixture{
		users:       new(mocks.MockUserRepository),
		enrollments: new(mocks.MockEnrollmentRepository),
		courses:     new(mocks.MockCourseRepository),
	}
	f.pm = middleware.NewPermissionMiddleware(f.enrollments, f.users, f.courses)
	return f
}

// course 101 lives in account 1 with the given state; lookups scoped to any
// other account miss, like the real tenant-scoped repo.
func (f *permFixture) course(state models.CourseWorkflow) {
	c := &models.Course{ID: 101, AccountID: 1, WorkflowState: state}
	f.courses.On("FindByID", mock.Anything, uint(101), uint(1)).Return(c, nil)
	f.courses.On("FindByID", mock.Anything, uint(101), uint(0)).Return(c, nil)
	f.courses.On("FindByID", mock.Anything, uint(101), uint(2)).Return(nil, errMissing)
}

func (f *permFixture) user(id, account uint, role string) {
	f.users.On("FindByID", mock.Anything, id, uint(0)).Return(&models.User{ID: id, AccountID: account, Role: role}, nil)
}

func stub(userID, accountID uint, extra map[string]interface{}) fiber.Handler {
	return func(c *fiber.Ctx) error {
		c.Locals("user_id", userID)
		c.Locals("account_id", accountID)
		for k, v := range extra {
			c.Locals(k, v)
		}
		return c.Next()
	}
}

func ok(c *fiber.Ctx) error { return c.JSON(fiber.Map{"ok": true}) }

// ---------------------------------------------------------------------------
// B064 / B078 — course guards
// ---------------------------------------------------------------------------

func TestCourseGuards_AdminBypassIsTenantScoped(t *testing.T) {
	cases := []struct {
		name       string
		role       string
		callerAcct uint
		want       int
	}{
		{"admin of the course's tenant", "admin", 1, http.StatusOK},
		{"admin of another tenant", "admin", 2, http.StatusNotFound},
		{"super_admin of another tenant", "super_admin", 2, http.StatusOK},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := newPermFixture()
			f.course(models.CourseAvailable)
			f.user(3, tc.callerAcct, tc.role)

			app := testutil.SetupTestApp()
			app.Use(stub(3, tc.callerAcct, nil))
			// LTI AGS line items + course delete are the B078 / B064 surfaces.
			app.Get("/api/v1/lti/courses/:course_id/line_items", f.pm.RequireEnrolled(), ok)
			app.Delete("/api/v1/courses/:id", f.pm.RequireInstructor(), ok)

			assert.Equal(t, tc.want, testutil.MakeRequest(app, http.MethodGet, "/api/v1/lti/courses/101/line_items", nil).StatusCode)
			assert.Equal(t, tc.want, testutil.MakeRequest(app, http.MethodDelete, "/api/v1/courses/101", nil).StatusCode)
		})
	}
}

// A JWT that still says role=admin (stale claim, is_admin Local set by the
// auth middleware) must not bypass the course guard once the DB says user.
func TestCourseGuards_StaleAdminClaimIgnored(t *testing.T) {
	f := newPermFixture()
	f.course(models.CourseAvailable)
	f.user(3, 1, "user")
	f.enrollments.On("FindByUserAndCourse", mock.Anything, uint(3), uint(101), uint(1)).Return(nil, errMissing)

	app := testutil.SetupTestApp()
	app.Use(stub(3, 1, map[string]interface{}{"is_admin": true, "user_role": "admin"}))
	app.Delete("/api/v1/courses/:id", f.pm.RequireInstructor(), ok)

	assert.Equal(t, http.StatusForbidden, testutil.MakeRequest(app, http.MethodDelete, "/api/v1/courses/101", nil).StatusCode)
}

// ---------------------------------------------------------------------------
// B149 — unpublished courses
// ---------------------------------------------------------------------------

func TestRequireEnrolled_UnpublishedCourse(t *testing.T) {
	cases := []struct {
		enrollment string
		state      models.CourseWorkflow
		want       int
	}{
		{"StudentEnrollment", models.CourseUnpublished, http.StatusForbidden},
		{"StudentEnrollment", models.CourseClaimed, http.StatusForbidden},
		{"ObserverEnrollment", models.CourseCreated, http.StatusForbidden},
		{"TeacherEnrollment", models.CourseUnpublished, http.StatusOK},
		{"TaEnrollment", models.CourseClaimed, http.StatusOK},
		{"DesignerEnrollment", models.CourseUnpublished, http.StatusOK},
		{"StudentEnrollment", models.CourseAvailable, http.StatusOK},
		{"StudentEnrollment", models.CourseCompleted, http.StatusOK},
	}
	for _, tc := range cases {
		t.Run(tc.enrollment+"/"+string(tc.state), func(t *testing.T) {
			f := newPermFixture()
			f.course(tc.state)
			f.user(7, 1, "user")
			f.enrollments.On("FindByUserAndCourse", mock.Anything, uint(7), uint(101), uint(1)).
				Return(&models.Enrollment{ID: 1, UserID: 7, CourseID: 101, Type: tc.enrollment, WorkflowState: "active"}, nil)

			app := testutil.SetupTestApp()
			app.Use(stub(7, 1, nil))
			app.Get("/api/v1/courses/:course_id/modules", f.pm.RequireEnrolled(), ok)

			assert.Equal(t, tc.want, testutil.MakeRequest(app, http.MethodGet, "/api/v1/courses/101/modules", nil).StatusCode)
		})
	}
}

// ---------------------------------------------------------------------------
// B069 — URL :account_id
// ---------------------------------------------------------------------------

func TestRequireAdmin_URLAccountMustBeCallersTenant(t *testing.T) {
	cases := []struct {
		name string
		role string
		path string
		want int
	}{
		{"own account, :account_id", "admin", "/api/v1/accounts/2/terms", http.StatusOK},
		{"other account, :account_id", "admin", "/api/v1/accounts/1/terms", http.StatusNotFound},
		{"other account audit log", "admin", "/api/v1/accounts/1/audit_log", http.StatusNotFound},
		{"other account roles", "admin", "/api/v1/accounts/1/roles", http.StatusNotFound},
		{"other account, :id feature flag", "admin", "/api/v1/accounts/1/features/new_gradebook", http.StatusNotFound},
		{"own account, :id feature flag", "admin", "/api/v1/accounts/2/features/new_gradebook", http.StatusOK},
		{"non-account admin route", "admin", "/api/v1/admin/audit_log/summary", http.StatusOK},
		{"super_admin, other account", "super_admin", "/api/v1/accounts/1/terms", http.StatusOK},
		{"plain user", "user", "/api/v1/accounts/2/terms", http.StatusForbidden},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := newPermFixture()
			f.user(3, 2, tc.role)

			app := testutil.SetupTestApp()
			app.Use(stub(3, 2, nil))
			admin := f.pm.RequireAdmin()
			app.Get("/api/v1/accounts/:account_id/terms", admin, ok)
			app.Get("/api/v1/accounts/:account_id/audit_log", admin, ok)
			app.Get("/api/v1/accounts/:account_id/roles", admin, ok)
			app.Put("/api/v1/accounts/:id/features/:feature", admin, ok)
			app.Get("/api/v1/accounts/:id/features/:feature", admin, ok)
			app.Get("/api/v1/admin/audit_log/summary", admin, ok)

			assert.Equal(t, tc.want, testutil.MakeRequest(app, http.MethodGet, tc.path, nil).StatusCode)
		})
	}
}

// ---------------------------------------------------------------------------
// B063 — masquerade privilege cap
// ---------------------------------------------------------------------------

func TestRequireSuperAdmin_MasqueradeCannotEscalate(t *testing.T) {
	f := newPermFixture()
	f.user(9, 1, "super_admin") // impersonated user
	f.user(3, 1, "admin")       // real user behind the masquerade

	app := testutil.SetupTestApp()
	app.Use(stub(9, 1, map[string]interface{}{"masquerade_by": uint(3)}))
	app.Get("/api/v1/superadmin/settings", f.pm.RequireSuperAdmin(), ok)
	app.Get("/api/v1/accounts/:account_id/terms", f.pm.RequireAdmin(), ok)

	assert.Equal(t, http.StatusForbidden,
		testutil.MakeRequest(app, http.MethodGet, "/api/v1/superadmin/settings", nil).StatusCode,
		"an admin masquerading as a super_admin must not get super-admin access")
	// Capped at admin: still an admin of their own tenant.
	assert.Equal(t, http.StatusOK,
		testutil.MakeRequest(app, http.MethodGet, "/api/v1/accounts/1/terms", nil).StatusCode)
}

func TestRequireAdmin_MasqueradeAsAdminByTeacherImpossible(t *testing.T) {
	// Defense in depth: if a masquerade token ever names a non-admin real
	// user, the session gets at most that user's role.
	f := newPermFixture()
	f.user(5, 1, "admin") // impersonated
	f.user(6, 1, "user")  // real user

	app := testutil.SetupTestApp()
	app.Use(stub(5, 1, map[string]interface{}{"masquerade_by": uint(6)}))
	app.Get("/api/v1/accounts/:account_id/terms", f.pm.RequireAdmin(), ok)

	assert.Equal(t, http.StatusForbidden, testutil.MakeRequest(app, http.MethodGet, "/api/v1/accounts/1/terms", nil).StatusCode)
}

func TestRequireSuperAdmin_RealSuperAdminPasses(t *testing.T) {
	f := newPermFixture()
	f.user(9, 1, "super_admin")

	app := testutil.SetupTestApp()
	app.Use(stub(9, 1, nil))
	app.Get("/api/v1/superadmin/settings", f.pm.RequireSuperAdmin(), ok)

	assert.Equal(t, http.StatusOK, testutil.MakeRequest(app, http.MethodGet, "/api/v1/superadmin/settings", nil).StatusCode)
}

// RequireSelfOrAdmin: an admin may only act on users of their own tenant.
func TestRequireSelfOrAdmin_TenantScoped(t *testing.T) {
	f := newPermFixture()
	f.user(3, 2, "admin")
	f.users.On("FindByID", mock.Anything, uint(40), uint(2)).Return(&models.User{ID: 40, AccountID: 2}, nil)
	f.users.On("FindByID", mock.Anything, uint(41), uint(2)).Return(nil, errMissing)

	app := testutil.SetupTestApp()
	app.Use(stub(3, 2, nil))
	app.Get("/api/v1/users/:user_id/profile", f.pm.RequireSelfOrAdmin(), ok)

	assert.Equal(t, http.StatusOK, testutil.MakeRequest(app, http.MethodGet, "/api/v1/users/40/profile", nil).StatusCode)
	assert.Equal(t, http.StatusForbidden, testutil.MakeRequest(app, http.MethodGet, "/api/v1/users/41/profile", nil).StatusCode)
}

// ---------------------------------------------------------------------------
// B172 — AuditWrites masquerade attribution
// ---------------------------------------------------------------------------

type ctxEmitter struct {
	ctx     context.Context
	userID  uint
	payload string
	calls   int
}

func (e *ctxEmitter) LogEvent(ctx context.Context, _ string, userID uint, _ *uint, _ *uint, _ string, _ uint, _, payload, _, _ string) error {
	e.ctx, e.userID, e.payload = ctx, userID, payload
	e.calls++
	return nil
}

func TestAuditWrites_RecordsMasquerader(t *testing.T) {
	emitter := &ctxEmitter{}
	app := testutil.SetupTestApp()
	app.Use(stub(8, 1, map[string]interface{}{"masquerade_by": uint(3)}))
	app.Use(middleware.AuditWrites(emitter, "http.write"))
	app.Post("/api/v1/things", func(c *fiber.Ctx) error { return c.SendStatus(http.StatusCreated) })

	resp := testutil.MakeRequest(app, http.MethodPost, "/api/v1/things", nil)
	assert.Equal(t, http.StatusCreated, resp.StatusCode)
	if assert.Equal(t, 1, emitter.calls) {
		assert.Equal(t, uint(8), emitter.userID)
		assert.Equal(t, uint(3), service.MasqueraderFromContext(emitter.ctx), "the real (masquerading) user must reach the audit row")
		assert.JSONEq(t, `{"method":"POST","path":"/api/v1/things","masquerade_by":3}`, emitter.payload)
	}
}

func TestAuditWrites_PayloadIsJSON(t *testing.T) {
	emitter := &ctxEmitter{}
	app := testutil.SetupTestApp()
	app.Use(stub(8, 1, nil))
	app.Use(middleware.AuditWrites(emitter, "http.write"))
	app.Delete("/api/v1/things/:id", func(c *fiber.Ctx) error { return c.JSON(fiber.Map{"delete": true}) })

	testutil.MakeRequest(app, http.MethodDelete, "/api/v1/things/4", nil)
	if assert.Equal(t, 1, emitter.calls) {
		assert.JSONEq(t, `{"method":"DELETE","path":"/api/v1/things/4"}`, emitter.payload)
		assert.Equal(t, uint(0), service.MasqueraderFromContext(emitter.ctx))
	}
}
