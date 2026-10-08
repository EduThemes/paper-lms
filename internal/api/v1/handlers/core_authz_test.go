package handlers_test

// Regression tests for the core authz lane (cross-cutting authorization):
//   B047 ResourceAuthorizer failures must stop the request.
//   B064 the admin bypass is tenant-scoped.
//   B149 students can't reach unpublished courses.
//   B063 an admin can't masquerade as a super_admin.
//   B069 account-scoped term/role endpoints refuse other tenants' rows.
//   sweep: requireParticipant / getUserID no longer fall through.
// All data is made up.

import (
	"errors"
	"net/http"
	"testing"

	"github.com/gofiber/fiber/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"github.com/EduThemes/paper-lms/internal/api/v1/handlers"
	"github.com/EduThemes/paper-lms/internal/api/v1/middleware"
	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/service"
	"github.com/EduThemes/paper-lms/internal/testutil"
	"github.com/EduThemes/paper-lms/internal/testutil/mocks"
)

var errNotFound = errors.New("record not found")

func callerStub(userID, accountID uint) fiber.Handler {
	return func(c *fiber.Ctx) error {
		c.Locals("user_id", userID)
		c.Locals("account_id", accountID)
		return c.Next()
	}
}

// authzProbe mounts a route that runs an authz check and then a
// "destructive" step, so the test can prove the step never runs on denial.
func authzProbe(authz *handlers.ResourceAuthorizer, check func(*handlers.ResourceAuthorizer, *fiber.Ctx) error, ran *bool) fiber.Handler {
	return func(c *fiber.Ctx) error {
		if err := check(authz, c); err != nil {
			return err
		}
		*ran = true
		return c.JSON(fiber.Map{"delete": true})
	}
}

// ---------------------------------------------------------------------------
// B047
// ---------------------------------------------------------------------------

// A student who calls an instructor-only action gets a 403 AND the action
// does not run (previously the 403 body was written, nil returned, and the
// handler carried on to the delete).
func TestResourceAuthorizer_DenialStopsRequest(t *testing.T) {
	userRepo := new(mocks.MockUserRepository)
	enrollRepo := new(mocks.MockEnrollmentRepository)
	courseRepo := new(mocks.MockCourseRepository)
	userRepo.On("FindByID", mock.Anything, uint(7), uint(0)).Return(&models.User{ID: 7, AccountID: 1, Role: "user"}, nil)
	courseRepo.On("FindByID", mock.Anything, uint(101), uint(1)).Return(&models.Course{ID: 101, AccountID: 1, WorkflowState: models.CourseAvailable}, nil)
	enrollRepo.On("FindByUserAndCourse", mock.Anything, uint(7), uint(101), uint(1)).
		Return(&models.Enrollment{ID: 1, UserID: 7, CourseID: 101, Type: "StudentEnrollment", WorkflowState: "active"}, nil)
	authz := handlers.NewResourceAuthorizer(enrollRepo, userRepo, courseRepo)

	ran := false
	app := testutil.SetupTestApp()
	app.Use(callerStub(7, 1))
	app.Delete("/announcements/:id", authzProbe(authz, func(a *handlers.ResourceAuthorizer, c *fiber.Ctx) error {
		return a.RequireCourseInstructor(c, 101)
	}, &ran))

	resp := testutil.MakeRequest(app, http.MethodDelete, "/announcements/5", nil)
	assert.Equal(t, http.StatusForbidden, resp.StatusCode)
	body, _ := testutil.ParseJSONMap(resp)
	assert.NotContains(t, body, "delete", "the destructive step's body must not be written")
	assert.False(t, ran, "the destructive step must not run after an authz denial")
}

// Same contract through a real handler: a non-owner can't delete someone
// else's calendar event.
func TestDeleteCalendarEvent_NonOwnerRefused(t *testing.T) {
	calRepo := new(mocks.MockCalendarEventRepository)
	userRepo := new(mocks.MockUserRepository)
	calRepo.On("FindByID", mock.Anything, uint(55), uint(1)).
		Return(&models.CalendarEvent{ID: 55, ContextType: "User", ContextID: 99, CreatedByUserID: 99, Title: "Office hours"}, nil)
	userRepo.On("FindByID", mock.Anything, uint(7), uint(0)).Return(&models.User{ID: 7, AccountID: 1, Role: "user"}, nil)

	authz := handlers.NewResourceAuthorizer(new(mocks.MockEnrollmentRepository), userRepo, new(mocks.MockCourseRepository))
	h := handlers.NewCalendarEventHandler(service.NewCalendarService(calRepo), authz)
	app := testutil.SetupTestApp()
	app.Use(callerStub(7, 1))
	app.Delete("/calendar_events/:id", h.DeleteEvent)

	resp := testutil.MakeRequest(app, http.MethodDelete, "/calendar_events/55", nil)
	assert.Equal(t, http.StatusForbidden, resp.StatusCode)
	calRepo.AssertNotCalled(t, "Delete", mock.Anything, mock.Anything)
}

// ---------------------------------------------------------------------------
// B064 / B078
// ---------------------------------------------------------------------------

func TestResourceAuthorizer_AdminBypassIsTenantScoped(t *testing.T) {
	cases := []struct {
		name       string
		role       string
		callerAcct uint
		wantStatus int
	}{
		{"admin of the course's tenant passes", "admin", 1, http.StatusOK},
		{"admin of another tenant gets 404", "admin", 2, http.StatusNotFound},
		{"super_admin crosses tenants", "super_admin", 2, http.StatusOK},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			userRepo := new(mocks.MockUserRepository)
			courseRepo := new(mocks.MockCourseRepository)
			userRepo.On("FindByID", mock.Anything, uint(3), uint(0)).Return(&models.User{ID: 3, AccountID: tc.callerAcct, Role: tc.role}, nil)
			course := &models.Course{ID: 101, AccountID: 1, WorkflowState: models.CourseAvailable}
			courseRepo.On("FindByID", mock.Anything, uint(101), uint(1)).Return(course, nil)
			courseRepo.On("FindByID", mock.Anything, uint(101), uint(0)).Return(course, nil)
			courseRepo.On("FindByID", mock.Anything, uint(101), uint(2)).Return(nil, errNotFound)
			authz := handlers.NewResourceAuthorizer(new(mocks.MockEnrollmentRepository), userRepo, courseRepo)

			ran := false
			app := testutil.SetupTestApp()
			app.Use(callerStub(3, tc.callerAcct))
			app.Delete("/sections/:id", authzProbe(authz, func(a *handlers.ResourceAuthorizer, c *fiber.Ctx) error {
				return a.RequireCourseInstructor(c, 101)
			}, &ran))

			resp := testutil.MakeRequest(app, http.MethodDelete, "/sections/1", nil)
			assert.Equal(t, tc.wantStatus, resp.StatusCode)
			assert.Equal(t, tc.wantStatus == http.StatusOK, ran)
		})
	}
}

func TestResourceAuthorizer_OwnerOrAdmin_CrossTenantAdminRefused(t *testing.T) {
	userRepo := new(mocks.MockUserRepository)
	userRepo.On("FindByID", mock.Anything, uint(3), uint(0)).Return(&models.User{ID: 3, AccountID: 2, Role: "admin"}, nil)
	// Owner 42 lives in account 1, so a scoped lookup from account 2 misses.
	userRepo.On("FindByID", mock.Anything, uint(42), uint(2)).Return(nil, errNotFound)
	authz := handlers.NewResourceAuthorizer(new(mocks.MockEnrollmentRepository), userRepo, new(mocks.MockCourseRepository))

	ran := false
	app := testutil.SetupTestApp()
	app.Use(callerStub(3, 2))
	app.Delete("/portfolios/:id", authzProbe(authz, func(a *handlers.ResourceAuthorizer, c *fiber.Ctx) error {
		return a.RequireOwnerOrAdmin(c, 42)
	}, &ran))

	resp := testutil.MakeRequest(app, http.MethodDelete, "/portfolios/1", nil)
	assert.Equal(t, http.StatusForbidden, resp.StatusCode)
	assert.False(t, ran)
}

// ---------------------------------------------------------------------------
// B149
// ---------------------------------------------------------------------------

func TestResourceAuthorizer_StudentRefusedInUnpublishedCourse(t *testing.T) {
	for _, tc := range []struct {
		enrollment string
		state      models.CourseWorkflow
		want       int
	}{
		{"StudentEnrollment", models.CourseUnpublished, http.StatusForbidden},
		{"ObserverEnrollment", models.CourseClaimed, http.StatusForbidden},
		{"TeacherEnrollment", models.CourseUnpublished, http.StatusOK},
		{"StudentEnrollment", models.CourseAvailable, http.StatusOK},
		{"StudentEnrollment", models.CourseCompleted, http.StatusOK},
	} {
		t.Run(tc.enrollment+"/"+string(tc.state), func(t *testing.T) {
			userRepo := new(mocks.MockUserRepository)
			enrollRepo := new(mocks.MockEnrollmentRepository)
			courseRepo := new(mocks.MockCourseRepository)
			userRepo.On("FindByID", mock.Anything, uint(7), uint(0)).Return(&models.User{ID: 7, AccountID: 1, Role: "user"}, nil)
			courseRepo.On("FindByID", mock.Anything, uint(101), uint(1)).Return(&models.Course{ID: 101, AccountID: 1, WorkflowState: tc.state}, nil)
			enrollRepo.On("FindByUserAndCourse", mock.Anything, uint(7), uint(101), uint(1)).
				Return(&models.Enrollment{ID: 1, Type: tc.enrollment, WorkflowState: "active"}, nil)
			authz := handlers.NewResourceAuthorizer(enrollRepo, userRepo, courseRepo)

			ran := false
			app := testutil.SetupTestApp()
			app.Use(callerStub(7, 1))
			app.Get("/conferences/:id", authzProbe(authz, func(a *handlers.ResourceAuthorizer, c *fiber.Ctx) error {
				return a.RequireCourseEnrolled(c, 101)
			}, &ran))

			resp := testutil.MakeRequest(app, http.MethodGet, "/conferences/1", nil)
			assert.Equal(t, tc.want, resp.StatusCode)
		})
	}
}

// ---------------------------------------------------------------------------
// B063
// ---------------------------------------------------------------------------

func setupMasquerade(t *testing.T, target *models.User) (*fiber.App, *mocks.MockUserRepository) {
	t.Helper()
	userRepo := new(mocks.MockUserRepository)
	// Caller 3 is an account admin of tenant 1.
	userRepo.On("FindByID", mock.Anything, uint(3), uint(0)).Return(&models.User{ID: 3, AccountID: 1, Role: "admin"}, nil)
	userRepo.On("FindByID", mock.Anything, target.ID, uint(1)).Return(target, nil)

	pm := middleware.NewPermissionMiddleware(new(mocks.MockEnrollmentRepository), userRepo, new(mocks.MockCourseRepository))
	h := handlers.NewUserHandler(service.NewUserService(userRepo), "test-jwt-secret", "test", nil, nil, nil)
	app := testutil.SetupTestApp()
	app.Use(callerStub(3, 1))
	app.Post("/api/v1/users/:id/masquerade", pm.RequireAdmin(), h.StartMasquerade)
	return app, userRepo
}

func TestStartMasquerade_AdminCannotBecomeSuperAdmin(t *testing.T) {
	app, _ := setupMasquerade(t, &models.User{ID: 9, AccountID: 1, Role: "super_admin", Name: "Platform Operator"})
	resp := testutil.MakeRequest(app, http.MethodPost, "/api/v1/users/9/masquerade", nil)
	assert.Equal(t, http.StatusForbidden, resp.StatusCode)
	for _, ck := range resp.Cookies() {
		assert.NotEqual(t, "paper_session", ck.Name, "no masquerade session may be issued")
	}
}

func TestStartMasquerade_AdminCanBecomeUser(t *testing.T) {
	app, _ := setupMasquerade(t, &models.User{ID: 8, AccountID: 1, Role: "user", Name: "Jane Doe"})
	resp := testutil.MakeRequest(app, http.MethodPost, "/api/v1/users/8/masquerade", nil)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
}

// ---------------------------------------------------------------------------
// B069
// ---------------------------------------------------------------------------

// GET /accounts/:account_id/roles/:id/overrides read overrides by bare role
// id. A role from another tenant is now a 404 and its overrides are never
// read.
func TestListRoleOverrides_OtherTenantsRoleNotFound(t *testing.T) {
	roleRepo := new(mocks.MockCustomRoleRepository)
	overrideRepo := new(mocks.MockRoleOverrideRepository)
	roleRepo.On("FindByID", mock.Anything, uint(30), uint(2)).Return(nil, errNotFound)
	h := handlers.NewCustomRoleHandler(service.NewCustomRoleService(roleRepo, overrideRepo, new(mocks.MockEnrollmentRepository)))

	app := testutil.SetupTestApp()
	app.Use(callerStub(3, 2))
	app.Get("/accounts/:account_id/roles/:id/overrides", h.ListOverrides)

	resp := testutil.MakeRequest(app, http.MethodGet, "/accounts/2/roles/30/overrides", nil)
	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
	overrideRepo.AssertNotCalled(t, "ListByRoleID", mock.Anything, mock.Anything)
}

func TestListRoleOverrides_OwnRoleResolves(t *testing.T) {
	roleRepo := new(mocks.MockCustomRoleRepository)
	overrideRepo := new(mocks.MockRoleOverrideRepository)
	roleRepo.On("FindByID", mock.Anything, uint(30), uint(2)).Return(&models.CustomRole{ID: 30, AccountID: 2, Name: "Grader"}, nil)
	overrideRepo.On("ListByRoleID", mock.Anything, uint(30)).Return([]models.RoleOverride{{ID: 1, RoleID: 30, Permission: "manage_grades"}}, nil)
	h := handlers.NewCustomRoleHandler(service.NewCustomRoleService(roleRepo, overrideRepo, new(mocks.MockEnrollmentRepository)))

	app := testutil.SetupTestApp()
	app.Use(callerStub(3, 2))
	app.Get("/accounts/:account_id/roles/:id/overrides", h.ListOverrides)

	resp := testutil.MakeRequest(app, http.MethodGet, "/accounts/2/roles/30/overrides", nil)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
}

// ---------------------------------------------------------------------------
// (result, wrote, err) sweep
// ---------------------------------------------------------------------------

// A non-participant can't update a conversation: requireParticipant used to
// write a 404 and return nil, so UpdateConversation carried on.
func TestUpdateConversation_NonParticipantStops(t *testing.T) {
	convRepo := new(mocks.MockConversationRepository)
	partRepo := new(mocks.MockConversationParticipantRepository)
	msgRepo := new(mocks.MockConversationMessageRepository)
	partRepo.On("ListByConversationID", mock.Anything, uint(12)).
		Return([]models.ConversationParticipant{{ConversationID: 12, UserID: 20}, {ConversationID: 12, UserID: 30}}, nil)

	h := handlers.NewConversationHandler(service.NewConversationService(convRepo, partRepo, msgRepo),
		service.NewUserService(new(mocks.MockUserRepository)), new(mocks.MockAccountRepository), new(mocks.MockEnrollmentRepository), nil)
	app := testutil.SetupTestApp()
	app.Use(callerStub(7, 1))
	app.Put("/conversations/:id", h.UpdateConversation)

	body := testutil.JSONBody(map[string]interface{}{"conversation": map[string]interface{}{"subject": "changed"}})
	resp := testutil.MakeRequest(app, http.MethodPut, "/conversations/12", body)
	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
	convRepo.AssertNotCalled(t, "FindByID", mock.Anything, mock.Anything, mock.Anything)
	convRepo.AssertNotCalled(t, "Update", mock.Anything, mock.Anything)
}

// getUserID used to write 401 and return nil; StartMasquerade then kept
// going with user id 0.
func TestGetUserID_MissingUserStops(t *testing.T) {
	userRepo := new(mocks.MockUserRepository)
	h := handlers.NewUserHandler(service.NewUserService(userRepo), "test-jwt-secret", "test", nil, nil, nil)
	app := testutil.SetupTestApp()
	app.Post("/users/:id/masquerade", func(c *fiber.Ctx) error {
		c.Locals("account_id", uint(1)) // no user_id
		return c.Next()
	}, h.StartMasquerade)

	resp := testutil.MakeRequest(app, http.MethodPost, "/users/8/masquerade", nil)
	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
	userRepo.AssertNotCalled(t, "FindByID", mock.Anything, mock.Anything, mock.Anything)
}
