package handlers_test

// F-013 (continued): AssignmentOverride Get / Update / Delete must
// refuse when the override's assignment.CourseID does not match the
// URL's :course_id. The pre-fix handler only verified
// override.AssignmentID == :assignment_id; a teacher in course 10
// could PUT /courses/10/assignments/<assignment-from-course-99>/
// overrides/<override-of-that-assignment> and the override-→-
// assignment tie would silently pass while the assignment lived in
// a different course.
//
// This test covers DeleteOverride; Update + Get follow the same
// assertOverrideInCourse helper, so a passing test here locks all
// three sites against regression.

import (
	"net/http"
	"testing"

	"github.com/gofiber/fiber/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"github.com/EduThemes/paper-lms/internal/api/v1/handlers"
	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/service"
	"github.com/EduThemes/paper-lms/internal/testutil"
	"github.com/EduThemes/paper-lms/internal/testutil/mocks"
)

func overrideAuthStub(callerAccountID uint, enrollmentType string) fiber.Handler {
	return func(c *fiber.Ctx) error {
		c.Locals("user_id", uint(7))
		c.Locals("account_id", callerAccountID)
		c.Locals("enrollment_type", enrollmentType)
		return c.Next()
	}
}

func TestDeleteOverride_RejectsCrossCourseAssignment(t *testing.T) {
	overrideRepo := new(mocks.MockAssignmentOverrideRepository)
	overrideStudentRepo := new(mocks.MockAssignmentOverrideStudentRepository)
	enrollmentRepo := new(mocks.MockEnrollmentRepository)
	sectionRepo := new(mocks.MockSectionRepository)
	assignmentRepo := new(mocks.MockAssignmentRepository)

	overrideRepo.On("FindByID", mock.Anything, uint(55)).Return(&models.AssignmentOverride{
		ID:           55,
		AssignmentID: 42,
	}, nil)
	// The override→assignment tie matches (override.AssignmentID == :assignment_id).
	// But the assignment lives in course 99, NOT the URL's course 10.
	assignmentRepo.On("FindByID", mock.Anything, uint(42), uint(1)).Return(&models.Assignment{
		ID:       42,
		CourseID: 99,
	}, nil)

	overrideService := service.NewOverrideService(overrideRepo, overrideStudentRepo, enrollmentRepo, sectionRepo)
	assignmentService := service.NewAssignmentService(assignmentRepo)

	h := handlers.NewAssignmentOverrideHandler(overrideService, assignmentService)
	app := testutil.SetupTestApp()
	app.Delete("/courses/:course_id/assignments/:assignment_id/overrides/:override_id",
		overrideAuthStub(1, "TeacherEnrollment"),
		h.DeleteOverride)

	resp := testutil.MakeRequest(app, http.MethodDelete, "/courses/10/assignments/42/overrides/55", nil)
	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
	overrideRepo.AssertNotCalled(t, "Delete", mock.Anything, mock.Anything)
}

func TestDeleteOverride_AcceptsSameCourseAssignment(t *testing.T) {
	overrideRepo := new(mocks.MockAssignmentOverrideRepository)
	overrideStudentRepo := new(mocks.MockAssignmentOverrideStudentRepository)
	enrollmentRepo := new(mocks.MockEnrollmentRepository)
	sectionRepo := new(mocks.MockSectionRepository)
	assignmentRepo := new(mocks.MockAssignmentRepository)

	overrideRepo.On("FindByID", mock.Anything, uint(55)).Return(&models.AssignmentOverride{
		ID:           55,
		AssignmentID: 42,
	}, nil)
	assignmentRepo.On("FindByID", mock.Anything, uint(42), uint(1)).Return(&models.Assignment{
		ID:       42,
		CourseID: 10, // matches URL :course_id
	}, nil)
	overrideRepo.On("Delete", mock.Anything, uint(55)).Return(nil)

	overrideService := service.NewOverrideService(overrideRepo, overrideStudentRepo, enrollmentRepo, sectionRepo)
	assignmentService := service.NewAssignmentService(assignmentRepo)

	h := handlers.NewAssignmentOverrideHandler(overrideService, assignmentService)
	app := testutil.SetupTestApp()
	app.Delete("/courses/:course_id/assignments/:assignment_id/overrides/:override_id",
		overrideAuthStub(1, "TeacherEnrollment"),
		h.DeleteOverride)

	resp := testutil.MakeRequest(app, http.MethodDelete, "/courses/10/assignments/42/overrides/55", nil)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	overrideRepo.AssertCalled(t, "Delete", mock.Anything, uint(55))
}
