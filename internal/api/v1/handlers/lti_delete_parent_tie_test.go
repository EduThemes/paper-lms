package handlers_test

// F-012 / F-011 LTI variant: DeleteLineItem must refuse when the line
// item's course_id doesn't match the URL :course_id, AND must thread
// the caller's tenant scope into the repo Delete so a cross-tenant id
// soft-fails as a no-op (404, no destructive write).
//
// We exercise the canonical case (DeleteLineItem) here. The other LTI
// repo Delete signatures (LTIResourceLink, LTIToolConfiguration) are
// covered by the interface/mock contract — no handler exposes a
// destructive endpoint for them today (they're admin-internal /
// service-internal), and the widening was applied uniformly so a
// future caller will receive the right shape.

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

func ltiParentTieAuthStub(callerAccountID uint, enrollmentType string) fiber.Handler {
	return func(c *fiber.Ctx) error {
		c.Locals("user_id", uint(7))
		c.Locals("account_id", callerAccountID)
		c.Locals("enrollment_type", enrollmentType)
		return c.Next()
	}
}

// newAGSServiceForTest builds an LTIAGSService with only the
// line-item path wired. Submission + assignment repos are passed as
// fresh mocks; they don't fire on the Delete path because
// DeleteLineItem never calls them.
func newAGSServiceForTest(lineItemRepo *mocks.MockLTILineItemRepository) *service.LTIAGSService {
	resultRepo := new(mocks.MockLTIResultRepository)
	submissionRepo := new(mocks.MockSubmissionRepository)
	assignmentRepo := new(mocks.MockAssignmentRepository)
	return service.NewLTIAGSService(lineItemRepo, resultRepo, submissionRepo, assignmentRepo)
}

// newLTIHandlerForTest builds a minimal LTIHandler. Only the
// agsService slot is exercised by the Delete path; the rest stay nil.
// COPPA-gate repos are nil so the gate is bypassed (the gate also
// only runs on Launch / OIDCLogin).
func newLTIHandlerForTest(agsService *service.LTIAGSService) *handlers.LTIHandler {
	return handlers.NewLTIHandler(nil, agsService, nil, nil, nil, nil, nil, nil)
}

// TestDeleteLineItem_RejectsCrossCourseID — F-012 / F-011 repro.
// Teacher in course 10 attempts DELETE
// /lti/courses/10/line_items/<id from course 99>. Mock returns the
// line item with CourseID=99; handler MUST 404 before the destructive
// Delete fires on the repo.
func TestDeleteLineItem_RejectsCrossCourseID(t *testing.T) {
	lineItemRepo := new(mocks.MockLTILineItemRepository)
	lineItemRepo.On("FindByID", mock.Anything, uint(42)).Return(&models.LTILineItem{
		ID:       42,
		CourseID: 99, // ← NOT the URL's :course_id (10)
		Label:    "cross-tenant target",
	}, nil)

	agsService := newAGSServiceForTest(lineItemRepo)
	h := newLTIHandlerForTest(agsService)

	app := testutil.SetupTestApp()
	app.Delete("/lti/courses/:course_id/line_items/:id",
		ltiParentTieAuthStub(1, "TeacherEnrollment"),
		h.DeleteLineItem)

	resp := testutil.MakeRequest(app, http.MethodDelete, "/lti/courses/10/line_items/42", nil)
	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
	// Critical: Delete MUST NOT have been called. Mock assertion locks
	// the contract against future regression.
	lineItemRepo.AssertNotCalled(t, "Delete", mock.Anything, mock.Anything, mock.Anything)
}

// TestDeleteLineItem_AcceptsSameCourseID — happy path. Course 10's own
// line item 42 deletes cleanly; repo Delete fires with the caller's
// account_id (1) per the F-012 widening.
func TestDeleteLineItem_AcceptsSameCourseID(t *testing.T) {
	lineItemRepo := new(mocks.MockLTILineItemRepository)
	lineItemRepo.On("FindByID", mock.Anything, uint(42)).Return(&models.LTILineItem{
		ID:       42,
		CourseID: 10, // matches URL :course_id
		Label:    "in-tenant target",
	}, nil)
	lineItemRepo.On("Delete", mock.Anything, uint(42), uint(1)).Return(nil)

	agsService := newAGSServiceForTest(lineItemRepo)
	h := newLTIHandlerForTest(agsService)

	app := testutil.SetupTestApp()
	app.Delete("/lti/courses/:course_id/line_items/:id",
		ltiParentTieAuthStub(1, "TeacherEnrollment"),
		h.DeleteLineItem)

	resp := testutil.MakeRequest(app, http.MethodDelete, "/lti/courses/10/line_items/42", nil)
	assert.Equal(t, http.StatusNoContent, resp.StatusCode)
	lineItemRepo.AssertCalled(t, "Delete", mock.Anything, uint(42), uint(1))
}
