package handlers

// B178 / B179 regression tests.

import (
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/recover"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"github.com/EduThemes/paper-lms/internal/api/v1/middleware"
	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/testutil/mocks"
)

// B179: course staff see the activity trail but never other users'
// IP addresses or user agents; admins do.
func TestAuditRowRedactsNetworkInfoForNonAdmins(t *testing.T) {
	row := func(isAdmin bool) fiber.Map {
		var out fiber.Map
		app := fiber.New()
		app.Get("/", func(c *fiber.Ctx) error {
			if isAdmin {
				c.Locals("is_admin", true)
			}
			c.Locals("enrollment_type", "TeacherEnrollment")
			out = redactAuditNetworkInfo(auditLogToJSON(&models.AuditLog{ID: 1, IPAddress: "198.51.100.7", UserAgent: "ExampleBrowser/1.0"}), c)
			return c.SendStatus(200)
		})
		_, _ = app.Test(httptest.NewRequest("GET", "/", nil), -1)
		return out
	}

	teacher := row(false)
	_, hasIP := teacher["ip_address"]
	_, hasUA := teacher["user_agent"]
	assert.False(t, hasIP, "teacher must not see ip_address")
	assert.False(t, hasUA, "teacher must not see user_agent")

	admin := row(true)
	assert.Equal(t, "198.51.100.7", admin["ip_address"])
	assert.Equal(t, "ExampleBrowser/1.0", admin["user_agent"])
}

// B178 / B179: the teacher-only gate the router now uses for
// DELETE /courses/:id and the course audit log refuses a TA and admits
// a teacher.
func TestTeacherOnlyGateRefusesTA(t *testing.T) {
	enrollments := new(mocks.MockEnrollmentRepository)
	users := new(mocks.MockUserRepository)
	users.On("FindByID", mock.Anything, mock.Anything, mock.Anything).Return(&models.User{ID: 5, AccountID: 1, Role: "user"}, nil)
	enrollments.On("FindByUserAndCourse", mock.Anything, uint(5), uint(7), uint(1)).
		Return(&models.Enrollment{ID: 1, UserID: 5, CourseID: 7, Type: "TaEnrollment", WorkflowState: models.EnrollmentActive}, nil).Once()
	enrollments.On("FindByUserAndCourse", mock.Anything, uint(5), uint(7), uint(1)).
		Return(&models.Enrollment{ID: 2, UserID: 5, CourseID: 7, Type: "TeacherEnrollment", WorkflowState: models.EnrollmentActive}, nil).Once()
	pm := middleware.NewPermissionMiddleware(enrollments, users)

	app := fiber.New()
	app.Use(func(c *fiber.Ctx) error {
		c.Locals("user_id", uint(5))
		c.Locals("account_id", uint(1))
		return c.Next()
	})
	app.Delete("/api/v1/courses/:id", pm.RequireCourseRole("TeacherEnrollment"), func(c *fiber.Ctx) error {
		return c.SendStatus(200)
	})

	resp, _ := app.Test(httptest.NewRequest("DELETE", "/api/v1/courses/7", nil), -1)
	assert.Equal(t, fiber.StatusForbidden, resp.StatusCode, "TA must not delete the course")
	resp, _ = app.Test(httptest.NewRequest("DELETE", "/api/v1/courses/7", nil), -1)
	assert.Equal(t, fiber.StatusOK, resp.StatusCode, "teacher may delete the course")
}

// The account audit log trusts no URL :account_id: an admin of account 1
// asking for account 2's trail (users' IPs and user agents included)
// gets a 404 before any lookup runs.
func TestAccountAuditLogRefusesOtherTenant(t *testing.T) {
	h := NewAuditHandler(nil) // never reached on the refused path
	app := fiber.New()
	app.Use(recover.New())
	app.Use(func(c *fiber.Ctx) error {
		c.Locals("user_id", uint(5))
		c.Locals("account_id", uint(1))
		c.Locals("is_admin", true)
		return c.Next()
	})
	app.Get("/accounts/:account_id/audit_log", h.GetAccountAuditLog)

	resp, err := app.Test(httptest.NewRequest("GET", "/accounts/2/audit_log", nil), -1)
	assert.NoError(t, err)
	assert.Equal(t, fiber.StatusNotFound, resp.StatusCode)
}
