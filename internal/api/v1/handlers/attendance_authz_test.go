package handlers_test

// F-009: per-student attendance reads must refuse cross-student
// access from regular students. The pre-fix path was gated by
// `enrolled` (any role), so a student in the course could read any
// other student's attendance just by changing the :user_id path
// param.

import (
	"net/http"
	"testing"

	"github.com/gofiber/fiber/v2"
	"github.com/stretchr/testify/assert"

	"github.com/EduThemes/paper-lms/internal/api/v1/handlers"
	"github.com/EduThemes/paper-lms/internal/testutil"
)

// attendanceAuthStub mirrors the production auth Locals shape. role is
// either "student", "teacher", or "admin"; the handler reads
// enrollment_type / is_admin to decide.
func attendanceAuthStub(callerUserID uint, enrollmentType string, isAdmin bool) fiber.Handler {
	return func(c *fiber.Ctx) error {
		c.Locals("user_id", callerUserID)
		c.Locals("account_id", uint(1))
		c.Locals("enrollment_type", enrollmentType)
		c.Locals("is_admin", isAdmin)
		return c.Next()
	}
}

// TestAttendance_StudentCannotReadOtherStudent — the F-009 repro.
// Caller = student 7, URL = /courses/1/attendance/users/42 (a
// different student). The handler MUST 404 before the service call.
func TestAttendance_StudentCannotReadOtherStudent(t *testing.T) {
	// Service can be nil — the authz check runs before any service
	// invocation, so the test never reaches the service layer.
	h := handlers.NewAttendanceHandler(nil, nil)
	app := testutil.SetupTestApp()
	app.Get("/courses/:course_id/attendance/users/:user_id",
		attendanceAuthStub(7, "StudentEnrollment", false),
		h.GetStudentAttendance)

	resp := testutil.MakeRequest(app, http.MethodGet, "/courses/1/attendance/users/42", nil)
	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
}

// TestAttendance_StudentCannotReadOtherStudent_Summary — same gate on
// the /summary endpoint.
func TestAttendance_StudentCannotReadOtherStudent_Summary(t *testing.T) {
	h := handlers.NewAttendanceHandler(nil, nil)
	app := testutil.SetupTestApp()
	app.Get("/courses/:course_id/attendance/users/:user_id/summary",
		attendanceAuthStub(7, "StudentEnrollment", false),
		h.GetStudentAttendanceSummary)

	resp := testutil.MakeRequest(app, http.MethodGet, "/courses/1/attendance/users/42/summary", nil)
	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
}

// Note: the happy-path tests (self / teacher / admin can read) require
// a real or mocked AttendanceService, which depends on the
// AttendanceRepository. Those happy-path assertions are covered by
// the existing attendance integration tests when a postgres test DB
// is wired. The defense-in-depth gate above is what F-009 is about —
// the deny case is the critical contract.
