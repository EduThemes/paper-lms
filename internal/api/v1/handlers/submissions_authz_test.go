package handlers_test

// SEC-002/003/004: the submission read endpoints (GetSubmission,
// ListSubmissions, ListCourseSubmissions) are mounted behind `enrolled`, which
// only requires *some* active enrollment. Before the fix they performed no
// owner/role check, so any enrolled student could read every classmate's
// submission, score, grade, and attachments (a FERPA-grade IDOR). These tests
// lock the contract: a non-owner, non-staff student is confined to their own
// submissions; course staff (teacher/TA/admin) see the whole class.

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/gofiber/fiber/v2"
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

// setupSubmissionAuthz wires a SubmissionHandler with a per-test auth identity
// (caller user_id + course enrollment role). observerService and auditService
// are nil — the security contract under test does not depend on them (a
// non-owner, non-staff, non-observer caller must be refused).
func setupSubmissionAuthz(callerUserID uint, enrollmentType string) (*fiber.App, *mocks.MockSubmissionRepository) {
	submissionRepo := new(mocks.MockSubmissionRepository)
	assignmentRepo := new(mocks.MockAssignmentRepository)
	enrollmentRepo := new(mocks.MockEnrollmentRepository)
	latePolicyRepo := new(mocks.MockLatePolicyRepository)
	courseRepo := new(mocks.MockCourseRepository)
	gpgRepo := new(mocks.MockGradingPeriodGroupRepository)
	gpRepo := new(mocks.MockGradingPeriodRepository)
	commentRepo := new(mocks.MockSubmissionCommentRepository)
	attachmentRepo := new(mocks.MockAttachmentRepository)
	userRepo := new(mocks.MockUserRepository)

	svc := service.NewSubmissionService(submissionRepo, assignmentRepo, enrollmentRepo, latePolicyRepo, courseRepo, gpgRepo, gpRepo, nil)
	h := handlers.NewSubmissionHandler(svc, commentRepo, attachmentRepo, userRepo, assignmentRepo, nil, nil, nil, nil, nil)

	app := testutil.SetupTestApp()
	app.Use(func(c *fiber.Ctx) error {
		c.Locals("user_id", callerUserID)
		c.Locals("account_id", uint(1))
		if enrollmentType != "" {
			c.Locals("enrollment_type", enrollmentType)
		}
		return c.Next()
	})
	app.Use(middleware.PaginationParams())
	app.Get("/api/v1/courses/:course_id/assignments/:assignment_id/submissions/:user_id", h.GetSubmission)
	app.Get("/api/v1/courses/:course_id/assignments/:assignment_id/submissions", h.ListSubmissions)
	app.Get("/api/v1/courses/:course_id/submissions", h.ListCourseSubmissions)
	return app, submissionRepo
}

func decodeJSONArray(t *testing.T, resp *http.Response) []map[string]interface{} {
	t.Helper()
	var out []map[string]interface{}
	_ = json.NewDecoder(resp.Body).Decode(&out)
	return out
}

// SEC-002: a student must not read another student's submission.
func TestGetSubmission_RejectsOtherStudent(t *testing.T) {
	app, submissionRepo := setupSubmissionAuthz(1, "StudentEnrollment")
	submissionRepo.On("FindByAssignmentAndUser", mock.Anything, uint(1), uint(2), uint(1)).
		Return(&models.Submission{ID: 5, AssignmentID: 1, UserID: 2}, nil)

	resp := testutil.MakeRequest(app, http.MethodGet, "/api/v1/courses/10/assignments/1/submissions/2", nil)
	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
}

// SEC-002: a student may read their own submission.
func TestGetSubmission_AllowsOwner(t *testing.T) {
	app, submissionRepo := setupSubmissionAuthz(2, "StudentEnrollment")
	submissionRepo.On("FindByAssignmentAndUser", mock.Anything, uint(1), uint(2), uint(1)).
		Return(&models.Submission{ID: 5, AssignmentID: 1, UserID: 2}, nil)

	resp := testutil.MakeRequest(app, http.MethodGet, "/api/v1/courses/10/assignments/1/submissions/2", nil)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
}

// SEC-002: course staff may read any student's submission.
func TestGetSubmission_AllowsTeacher(t *testing.T) {
	app, submissionRepo := setupSubmissionAuthz(9, "TeacherEnrollment")
	submissionRepo.On("FindByAssignmentAndUser", mock.Anything, uint(1), uint(2), uint(1)).
		Return(&models.Submission{ID: 5, AssignmentID: 1, UserID: 2}, nil)

	resp := testutil.MakeRequest(app, http.MethodGet, "/api/v1/courses/10/assignments/1/submissions/2", nil)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
}

// SEC-004: a student listing an assignment's submissions sees only their own.
func TestListSubmissions_StudentSeesOnlyOwn(t *testing.T) {
	app, submissionRepo := setupSubmissionAuthz(1, "StudentEnrollment")
	submissionRepo.On("ListByAssignmentID", mock.Anything, uint(1), mock.Anything).
		Return(&repository.PaginatedResult[models.Submission]{
			Items: []models.Submission{
				{ID: 1, AssignmentID: 1, UserID: 1},
				{ID: 2, AssignmentID: 1, UserID: 2},
			},
			TotalCount: 2, Page: 1, PerPage: 10,
		}, nil)

	resp := testutil.MakeRequest(app, http.MethodGet, "/api/v1/courses/10/assignments/1/submissions", nil)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	items := decodeJSONArray(t, resp)
	assert.Len(t, items, 1)
	if len(items) == 1 {
		assert.EqualValues(t, 1, items[0]["user_id"])
	}
}

// SEC-004: course staff listing an assignment's submissions see the whole class.
func TestListSubmissions_TeacherSeesAll(t *testing.T) {
	app, submissionRepo := setupSubmissionAuthz(9, "TeacherEnrollment")
	submissionRepo.On("ListByAssignmentID", mock.Anything, uint(1), mock.Anything).
		Return(&repository.PaginatedResult[models.Submission]{
			Items: []models.Submission{
				{ID: 1, AssignmentID: 1, UserID: 1},
				{ID: 2, AssignmentID: 1, UserID: 2},
			},
			TotalCount: 2, Page: 1, PerPage: 10,
		}, nil)

	resp := testutil.MakeRequest(app, http.MethodGet, "/api/v1/courses/10/assignments/1/submissions", nil)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Len(t, decodeJSONArray(t, resp), 2)
}

// SEC-003: a student listing course submissions without ?user_id is confined to
// their own rows (previously this returned every student's submissions).
func TestListCourseSubmissions_StudentDefaultsToOwn(t *testing.T) {
	app, submissionRepo := setupSubmissionAuthz(1, "StudentEnrollment")
	submissionRepo.On("BulkListByCourse", mock.Anything, uint(10), mock.Anything).
		Return(&repository.PaginatedResult[models.Submission]{
			Items: []models.Submission{
				{ID: 1, UserID: 1},
				{ID: 2, UserID: 2},
				{ID: 3, UserID: 3},
			},
			TotalCount: 3, Page: 1, PerPage: 10,
		}, nil)

	resp := testutil.MakeRequest(app, http.MethodGet, "/api/v1/courses/10/submissions", nil)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	items := decodeJSONArray(t, resp)
	assert.Len(t, items, 1)
	if len(items) == 1 {
		assert.EqualValues(t, 1, items[0]["user_id"])
	}
}
