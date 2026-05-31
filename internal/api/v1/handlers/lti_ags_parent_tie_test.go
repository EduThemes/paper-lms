package handlers_test

// SEC / Gap-1 (LTI AGS): the line-item GET / PUT / score / results handlers must
// enforce the same parent-tie that DeleteLineItem already does. Before the fix
// they resolved a line item by :id alone, so an actor in course A could read or
// overwrite grades on a line item owned by course B in a different tenant
// (cross-tenant grade read + write, with an attacker-controlled userId on
// PostScore). These tests lock the parent-tie (item.CourseID == URL :course_id,
// else 404) on all four handlers. The helpers ltiParentTieAuthStub,
// newAGSServiceForTest, and newLTIHandlerForTest are shared with
// lti_delete_parent_tie_test.go (same package).

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/testutil"
	"github.com/EduThemes/paper-lms/internal/testutil/mocks"
)

func TestGetLineItem_RejectsCrossCourseID(t *testing.T) {
	lineItemRepo := new(mocks.MockLTILineItemRepository)
	lineItemRepo.On("FindByID", mock.Anything, uint(42)).
		Return(&models.LTILineItem{ID: 42, CourseID: 99}, nil) // ← belongs to another course
	h := newLTIHandlerForTest(newAGSServiceForTest(lineItemRepo))

	app := testutil.SetupTestApp()
	app.Get("/lti/courses/:course_id/line_items/:id", ltiParentTieAuthStub(1, "TeacherEnrollment"), h.GetLineItem)

	resp := testutil.MakeRequest(app, http.MethodGet, "/lti/courses/10/line_items/42", nil)
	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
}

func TestGetLineItem_AcceptsSameCourseID(t *testing.T) {
	lineItemRepo := new(mocks.MockLTILineItemRepository)
	lineItemRepo.On("FindByID", mock.Anything, uint(42)).
		Return(&models.LTILineItem{ID: 42, CourseID: 10, Label: "ok"}, nil)
	h := newLTIHandlerForTest(newAGSServiceForTest(lineItemRepo))

	app := testutil.SetupTestApp()
	app.Get("/lti/courses/:course_id/line_items/:id", ltiParentTieAuthStub(1, "TeacherEnrollment"), h.GetLineItem)

	resp := testutil.MakeRequest(app, http.MethodGet, "/lti/courses/10/line_items/42", nil)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
}

func TestUpdateLineItem_RejectsCrossCourseID(t *testing.T) {
	lineItemRepo := new(mocks.MockLTILineItemRepository)
	lineItemRepo.On("FindByID", mock.Anything, uint(42)).
		Return(&models.LTILineItem{ID: 42, CourseID: 99}, nil)
	h := newLTIHandlerForTest(newAGSServiceForTest(lineItemRepo))

	app := testutil.SetupTestApp()
	app.Put("/lti/courses/:course_id/line_items/:id", ltiParentTieAuthStub(1, "TeacherEnrollment"), h.UpdateLineItem)

	resp := testutil.MakeRequest(app, http.MethodPut, "/lti/courses/10/line_items/42",
		testutil.JSONBody(map[string]interface{}{"label": "x"}))
	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
	lineItemRepo.AssertNotCalled(t, "Update", mock.Anything, mock.Anything)
}

func TestUpdateLineItem_AcceptsSameCourseID(t *testing.T) {
	lineItemRepo := new(mocks.MockLTILineItemRepository)
	lineItemRepo.On("FindByID", mock.Anything, uint(42)).
		Return(&models.LTILineItem{ID: 42, CourseID: 10, Label: "orig", ScoreMaximum: 100}, nil)
	lineItemRepo.On("Update", mock.Anything, mock.AnythingOfType("*models.LTILineItem")).Return(nil)
	h := newLTIHandlerForTest(newAGSServiceForTest(lineItemRepo))

	app := testutil.SetupTestApp()
	app.Put("/lti/courses/:course_id/line_items/:id", ltiParentTieAuthStub(1, "TeacherEnrollment"), h.UpdateLineItem)

	resp := testutil.MakeRequest(app, http.MethodPut, "/lti/courses/10/line_items/42",
		testutil.JSONBody(map[string]interface{}{"label": "updated"}))
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	lineItemRepo.AssertCalled(t, "Update", mock.Anything, mock.AnythingOfType("*models.LTILineItem"))
}

func TestPostScore_RejectsCrossCourseID(t *testing.T) {
	lineItemRepo := new(mocks.MockLTILineItemRepository)
	lineItemRepo.On("FindByID", mock.Anything, uint(42)).
		Return(&models.LTILineItem{ID: 42, CourseID: 99}, nil)
	h := newLTIHandlerForTest(newAGSServiceForTest(lineItemRepo))

	app := testutil.SetupTestApp()
	app.Post("/lti/courses/:course_id/line_items/:id/scores", ltiParentTieAuthStub(1, "TeacherEnrollment"), h.PostScore)

	resp := testutil.MakeRequest(app, http.MethodPost, "/lti/courses/10/line_items/42/scores",
		testutil.JSONBody(map[string]interface{}{
			"userId": 5, "scoreGiven": 9, "activityProgress": "Completed", "gradingProgress": "FullyGraded",
		}))
	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
}

func TestGetResults_RejectsCrossCourseID(t *testing.T) {
	lineItemRepo := new(mocks.MockLTILineItemRepository)
	lineItemRepo.On("FindByID", mock.Anything, uint(42)).
		Return(&models.LTILineItem{ID: 42, CourseID: 99}, nil)
	h := newLTIHandlerForTest(newAGSServiceForTest(lineItemRepo))

	app := testutil.SetupTestApp()
	app.Get("/lti/courses/:course_id/line_items/:id/results", ltiParentTieAuthStub(1, "TeacherEnrollment"), h.GetResults)

	resp := testutil.MakeRequest(app, http.MethodGet, "/lti/courses/10/line_items/42/results", nil)
	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
}
