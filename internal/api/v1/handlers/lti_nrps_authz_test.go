package handlers_test

// Gap-2 (LTI NRPS): the roster endpoint (names, emails, LTI roles) is mounted
// behind `enrolled`, so before the fix any enrolled student could pull the
// full course roster's PII. The handler now restricts it to course staff and
// threads the caller's tenant into the service. Helpers ltiParentTieAuthStub /
// newLTIHandlerForTest are shared with lti_delete_parent_tie_test.go.

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"github.com/EduThemes/paper-lms/internal/api/v1/handlers"
	"github.com/EduThemes/paper-lms/internal/api/v1/middleware"
	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/repository"
	"github.com/EduThemes/paper-lms/internal/service"
	"github.com/EduThemes/paper-lms/internal/testutil"
	"github.com/EduThemes/paper-lms/internal/testutil/mocks"
)

func TestGetMemberships_RejectsNonStaff(t *testing.T) {
	// Student caller — must be refused before the roster is fetched.
	h := newLTIHandlerForTest(nil)

	app := testutil.SetupTestApp()
	app.Get("/lti/courses/:course_id/memberships", ltiParentTieAuthStub(1, "StudentEnrollment"), h.GetMemberships)

	resp := testutil.MakeRequest(app, http.MethodGet, "/lti/courses/10/memberships", nil)
	assert.Equal(t, http.StatusForbidden, resp.StatusCode)
}

func TestGetMemberships_AllowsStaff(t *testing.T) {
	enrollmentRepo := new(mocks.MockEnrollmentRepository)
	userRepo := new(mocks.MockUserRepository)
	// The service must be called with the caller's tenant (account 1), not 0.
	enrollmentRepo.On("ListByCourseID", mock.Anything, uint(10), uint(1), mock.Anything).
		Return(&repository.PaginatedResult[models.Enrollment]{
			Items: []models.Enrollment{}, TotalCount: 0, Page: 1, PerPage: 10,
		}, nil)
	nrps := service.NewLTINRPSService(enrollmentRepo, userRepo)
	h := handlers.NewLTIHandler(nil, nil, nrps, nil, nil, nil, nil, nil)

	app := testutil.SetupTestApp()
	app.Use(middleware.PaginationParams())
	app.Get("/lti/courses/:course_id/memberships", ltiParentTieAuthStub(1, "TeacherEnrollment"), h.GetMemberships)

	resp := testutil.MakeRequest(app, http.MethodGet, "/lti/courses/10/memberships", nil)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	enrollmentRepo.AssertCalled(t, "ListByCourseID", mock.Anything, uint(10), uint(1), mock.Anything)
}
