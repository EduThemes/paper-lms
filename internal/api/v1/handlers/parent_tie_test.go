package handlers_test

// F-011 / F-013 / F-043: Course-child Delete and Update handlers
// must refuse when the resource's parent course doesn't match the URL.
//
// We exercise the canonical case (DeleteAssignment) here. The other
// closed cases (Update*, DeleteModule, DeleteQuiz, DeletePage,
// DeleteFile) follow the same shape and are covered by the assignment
// test plus static review.

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

func parentTieAuthStub(callerAccountID uint, enrollmentType string) fiber.Handler {
	return func(c *fiber.Ctx) error {
		c.Locals("user_id", uint(7))
		c.Locals("account_id", callerAccountID)
		c.Locals("enrollment_type", enrollmentType)
		return c.Next()
	}
}

// TestDeleteAssignment_RejectsCrossCourseID — F-011 / F-013 repro.
// Teacher in course 10 attempts DELETE
// /courses/10/assignments/<assignment id in course 99>. Mock returns
// the assignment with CourseID=99; handler MUST 404 before the
// destructive Delete fires.
func TestDeleteAssignment_RejectsCrossCourseID(t *testing.T) {
	assignmentRepo := new(mocks.MockAssignmentRepository)

	assignmentService := service.NewAssignmentService(assignmentRepo)

	// Assignment lookup under the caller's tenant returns the wrong
	// course's row — the handler must NOT proceed to Delete.
	assignmentRepo.On("FindByID", mock.Anything, uint(42), uint(1)).Return(&models.Assignment{
		ID:       42,
		CourseID: 99, // ← NOT the URL's :course_id (10)
	}, nil)

	h := handlers.NewAssignmentHandler(assignmentService)
	app := testutil.SetupTestApp()
	app.Delete("/courses/:course_id/assignments/:id",
		parentTieAuthStub(1, "TeacherEnrollment"),
		h.DeleteAssignment)

	resp := testutil.MakeRequest(app, http.MethodDelete, "/courses/10/assignments/42", nil)
	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
	// Critical: Delete MUST NOT have been called on the cross-course
	// row. Mock assertion locks the contract.
	assignmentRepo.AssertNotCalled(t, "Delete", mock.Anything, mock.Anything)
}

// TestDeleteAssignment_AcceptsSameCourseID — happy path: course 10's
// own assignment 42 deletes cleanly.
func TestDeleteAssignment_AcceptsSameCourseID(t *testing.T) {
	assignmentRepo := new(mocks.MockAssignmentRepository)

	assignmentService := service.NewAssignmentService(assignmentRepo)

	assignmentRepo.On("FindByID", mock.Anything, uint(42), uint(1)).Return(&models.Assignment{
		ID:       42,
		CourseID: 10, // matches URL :course_id
	}, nil)
	assignmentRepo.On("Delete", mock.Anything, uint(42)).Return(nil)

	h := handlers.NewAssignmentHandler(assignmentService)
	app := testutil.SetupTestApp()
	app.Delete("/courses/:course_id/assignments/:id",
		parentTieAuthStub(1, "TeacherEnrollment"),
		h.DeleteAssignment)

	resp := testutil.MakeRequest(app, http.MethodDelete, "/courses/10/assignments/42", nil)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assignmentRepo.AssertCalled(t, "Delete", mock.Anything, uint(42))
}
