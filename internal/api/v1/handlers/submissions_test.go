package handlers_test

import (
	"errors"
	"net/http"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/EduThemes/paper-lms/internal/api/v1/handlers"
	"github.com/EduThemes/paper-lms/internal/api/v1/middleware"
	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/repository"
	"github.com/EduThemes/paper-lms/internal/service"
	"github.com/EduThemes/paper-lms/internal/testutil"
	"github.com/EduThemes/paper-lms/internal/testutil/mocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// setupSubmissionTest wires up the submission handler with mocks and returns
// the Fiber app and all mock repositories for assertion.
func setupSubmissionTest() (
	app *fiber.App,
	submissionRepo *mocks.MockSubmissionRepository,
	assignmentRepo *mocks.MockAssignmentRepository,
	enrollmentRepo *mocks.MockEnrollmentRepository,
	commentRepo *mocks.MockSubmissionCommentRepository,
	userRepo *mocks.MockUserRepository,
) {
	submissionRepo = new(mocks.MockSubmissionRepository)
	assignmentRepo = new(mocks.MockAssignmentRepository)
	enrollmentRepo = new(mocks.MockEnrollmentRepository)
	commentRepo = new(mocks.MockSubmissionCommentRepository)
	userRepo = new(mocks.MockUserRepository)

	attachmentRepo := new(mocks.MockAttachmentRepository)
	latePolicyRepo := new(mocks.MockLatePolicyRepository)
	courseRepo := new(mocks.MockCourseRepository)
	gradingPeriodGroupRepo := new(mocks.MockGradingPeriodGroupRepository)
	gradingPeriodRepo := new(mocks.MockGradingPeriodRepository)
	submissionService := service.NewSubmissionService(submissionRepo, assignmentRepo, enrollmentRepo, latePolicyRepo, courseRepo, gradingPeriodGroupRepo, gradingPeriodRepo, nil)
	handler := handlers.NewSubmissionHandler(submissionService, commentRepo, attachmentRepo, userRepo, assignmentRepo, nil, nil, nil, nil, nil)

	app = testutil.SetupTestApp()

	// Auth middleware stub: set user_id in Locals for all requests.
	app.Use(func(c *fiber.Ctx) error {
		c.Locals("user_id", uint(1))
		c.Locals("account_id", uint(1))
		return c.Next()
	})

	// Pagination middleware so handlers can call middleware.GetPagination.
	app.Use(middleware.PaginationParams())

	// Register the submission routes matching the real router.
	courses := app.Group("/api/v1/courses/:course_id/assignments/:assignment_id")
	courses.Post("/submissions", handler.CreateSubmission)
	courses.Get("/submissions", handler.ListSubmissions)
	courses.Get("/submissions/:user_id", handler.GetSubmission)
	courses.Put("/submissions/:user_id", handler.UpdateSubmission)
	courses.Post("/submissions/:user_id/comments", handler.CreateSubmissionComment)
	courses.Get("/submissions/:user_id/comments", handler.ListSubmissionComments)

	return
}

func TestCreateSubmission_Success(t *testing.T) {
	app, submissionRepo, assignmentRepo, _, _, _ := setupSubmissionTest()

	assignment := &models.Assignment{
		ID:       1,
		CourseID: 1,
		Name:     "Essay 1",
	}
	assignmentRepo.On("FindByID", mock.Anything, uint(1), uint(0)).Return(assignment, nil)

	// No existing submission for this user+assignment.
	submissionRepo.On("FindByAssignmentAndUser", mock.Anything, uint(1), uint(1), uint(0)).Return(nil, errors.New("not found"))
	submissionRepo.On("Create", mock.Anything, mock.AnythingOfType("*models.Submission")).Return(nil)

	body := testutil.JSONBody(map[string]interface{}{
		"submission": map[string]interface{}{
			"submission_type": "online_text_entry",
			"body":            "Here is my essay.",
		},
	})

	resp := testutil.MakeRequest(app, http.MethodPost, "/api/v1/courses/1/assignments/1/submissions", body)

	assert.Equal(t, http.StatusCreated, resp.StatusCode)

	result, err := testutil.ParseJSONMap(resp)
	require.NoError(t, err)
	assert.Equal(t, "online_text_entry", result["submission_type"])
	assert.Equal(t, "submitted", result["workflow_state"])

	submissionRepo.AssertExpectations(t)
	assignmentRepo.AssertExpectations(t)
}

func TestCreateSubmission_InvalidAssignmentID(t *testing.T) {
	app, _, _, _, _, _ := setupSubmissionTest()

	body := testutil.JSONBody(map[string]interface{}{
		"submission": map[string]interface{}{
			"submission_type": "online_text_entry",
			"body":            "My essay",
		},
	})

	resp := testutil.MakeRequest(app, http.MethodPost, "/api/v1/courses/1/assignments/abc/submissions", body)

	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)

	result, err := testutil.ParseJSONMap(resp)
	require.NoError(t, err)
	errs := result["errors"].([]interface{})
	firstErr := errs[0].(map[string]interface{})
	assert.Equal(t, "Invalid assignment ID", firstErr["message"])
}

func TestListSubmissions(t *testing.T) {
	app, submissionRepo, _, _, _, _ := setupSubmissionTest()

	subType := "online_text_entry"
	now := time.Now()
	paginatedResult := &repository.PaginatedResult[models.Submission]{
		Items: []models.Submission{
			{
				ID:             1,
				AssignmentID:   1,
				UserID:         1,
				SubmissionType: &subType,
				SubmittedAt:    &now,
				Attempt:        1,
				WorkflowState:  "submitted",
			},
			{
				ID:             2,
				AssignmentID:   1,
				UserID:         2,
				SubmissionType: &subType,
				SubmittedAt:    &now,
				Attempt:        1,
				WorkflowState:  "submitted",
			},
		},
		TotalCount: 2,
		Page:       1,
		PerPage:    10,
	}

	submissionRepo.On("ListByAssignmentID", mock.Anything, uint(1), mock.AnythingOfType("repository.PaginationParams")).Return(paginatedResult, nil)

	resp := testutil.MakeRequest(app, http.MethodGet, "/api/v1/courses/1/assignments/1/submissions", nil)

	assert.Equal(t, http.StatusOK, resp.StatusCode)

	result, err := testutil.ParseJSONArray(resp)
	require.NoError(t, err)
	assert.Len(t, result, 2)

	submissionRepo.AssertExpectations(t)
}

func TestGetSubmission_Success(t *testing.T) {
	app, submissionRepo, _, _, _, _ := setupSubmissionTest()

	subType := "online_text_entry"
	now := time.Now()
	submission := &models.Submission{
		ID:             1,
		AssignmentID:   1,
		UserID:         1,
		SubmissionType: &subType,
		SubmittedAt:    &now,
		Attempt:        1,
		WorkflowState:  "submitted",
	}

	// GetSubmission threads callerAccountID(c)=1 from the test's authStub.
	submissionRepo.On("FindByAssignmentAndUser", mock.Anything, uint(1), uint(1), uint(1)).Return(submission, nil)

	resp := testutil.MakeRequest(app, http.MethodGet, "/api/v1/courses/1/assignments/1/submissions/1", nil)

	assert.Equal(t, http.StatusOK, resp.StatusCode)

	result, err := testutil.ParseJSONMap(resp)
	require.NoError(t, err)
	assert.Equal(t, float64(1), result["user_id"])
	assert.Equal(t, "submitted", result["workflow_state"])

	submissionRepo.AssertExpectations(t)
}

func TestGetSubmission_NotFound(t *testing.T) {
	app, submissionRepo, _, _, _, _ := setupSubmissionTest()

	submissionRepo.On("FindByAssignmentAndUser", mock.Anything, uint(1), uint(999), uint(1)).Return(nil, errors.New("not found"))

	resp := testutil.MakeRequest(app, http.MethodGet, "/api/v1/courses/1/assignments/1/submissions/999", nil)

	assert.Equal(t, http.StatusNotFound, resp.StatusCode)

	result, err := testutil.ParseJSONMap(resp)
	require.NoError(t, err)
	errs := result["errors"].([]interface{})
	firstErr := errs[0].(map[string]interface{})
	assert.Contains(t, firstErr["message"], "not found")

	submissionRepo.AssertExpectations(t)
}

func TestUpdateSubmission_Grade(t *testing.T) {
	app, submissionRepo, assignmentRepo, _, _, _ := setupSubmissionTest()

	subType := "online_text_entry"
	now := time.Now()
	submission := &models.Submission{
		ID:             1,
		AssignmentID:   1,
		UserID:         2,
		SubmissionType: &subType,
		SubmittedAt:    &now,
		Attempt:        1,
		WorkflowState:  "submitted",
	}

	// Grade calls isGradingPeriodClosed → assignmentRepo.FindByID (returns no DueAt, so period check skips)
	assignmentRepo.On("FindByID", mock.Anything, uint(1), uint(0)).Return(&models.Assignment{ID: 1, CourseID: 1, Name: "Essay 1"}, nil)
	// Grade calls FindByAssignmentAndUser, then Update.
	submissionRepo.On("FindByAssignmentAndUser", mock.Anything, uint(1), uint(2), uint(0)).Return(submission, nil)
	submissionRepo.On("Update", mock.Anything, mock.AnythingOfType("*models.Submission")).Return(nil)

	body := testutil.JSONBody(map[string]interface{}{
		"submission": map[string]interface{}{
			"posted_grade": "95",
		},
	})

	resp := testutil.MakeRequest(app, http.MethodPut, "/api/v1/courses/1/assignments/1/submissions/2", body)

	assert.Equal(t, http.StatusOK, resp.StatusCode)

	result, err := testutil.ParseJSONMap(resp)
	require.NoError(t, err)
	assert.Equal(t, "graded", result["workflow_state"])
	assert.Equal(t, float64(95), result["score"])

	submissionRepo.AssertExpectations(t)
}

func TestUpdateSubmission_ExcusedPersists(t *testing.T) {
	app, submissionRepo, _, _, _, _ := setupSubmissionTest()

	subType := "online_text_entry"
	now := time.Now()
	submission := &models.Submission{
		ID:             1,
		AssignmentID:   1,
		UserID:         2,
		SubmissionType: &subType,
		SubmittedAt:    &now,
		Attempt:        1,
		WorkflowState:  "submitted",
		Excused:        false,
	}

	// SetExcused threads callerAccountID(c)=1 from the test's auth stub.
	// PATCH excused=true: Find returns the row, then Update is called with Excused=true.
	submissionRepo.On("FindByAssignmentAndUser", mock.Anything, uint(1), uint(2), uint(1)).Return(submission, nil).Once()
	submissionRepo.On("Update", mock.Anything, mock.MatchedBy(func(s *models.Submission) bool {
		return s.Excused == true
	})).Return(nil).Once()

	body := testutil.JSONBody(map[string]interface{}{
		"submission": map[string]interface{}{
			"excused": true,
		},
	})
	resp := testutil.MakeRequest(app, http.MethodPut, "/api/v1/courses/1/assignments/1/submissions/2", body)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	result, err := testutil.ParseJSONMap(resp)
	require.NoError(t, err)
	assert.Equal(t, true, result["excused"], "PATCH response must reflect excused=true")

	// PATCH excused=false: same submission (now Excused=true from the previous
	// Update) is returned, then Update is called with Excused=false.
	submissionRepo.On("FindByAssignmentAndUser", mock.Anything, uint(1), uint(2), uint(1)).Return(submission, nil).Once()
	submissionRepo.On("Update", mock.Anything, mock.MatchedBy(func(s *models.Submission) bool {
		return s.Excused == false
	})).Return(nil).Once()

	body2 := testutil.JSONBody(map[string]interface{}{
		"submission": map[string]interface{}{
			"excused": false,
		},
	})
	resp2 := testutil.MakeRequest(app, http.MethodPut, "/api/v1/courses/1/assignments/1/submissions/2", body2)
	assert.Equal(t, http.StatusOK, resp2.StatusCode)

	result2, err := testutil.ParseJSONMap(resp2)
	require.NoError(t, err)
	assert.Equal(t, false, result2["excused"], "PATCH response must reflect excused=false")

	submissionRepo.AssertExpectations(t)
}

func TestCreateSubmissionComment(t *testing.T) {
	app, submissionRepo, _, _, commentRepo, userRepo := setupSubmissionTest()

	subType := "online_text_entry"
	submission := &models.Submission{
		ID:             1,
		AssignmentID:   1,
		UserID:         2,
		SubmissionType: &subType,
		WorkflowState:  "submitted",
	}

	// CreateSubmissionComment threads callerAccountID(c)=1 from the test's authStub.
	submissionRepo.On("FindByAssignmentAndUser", mock.Anything, uint(1), uint(2), uint(1)).Return(submission, nil)
	commentRepo.On("Create", mock.Anything, mock.AnythingOfType("*models.SubmissionComment")).Return(nil)
	userRepo.On("FindByID", mock.Anything, uint(1), uint(1)).Return(&models.User{ID: 1, Name: "Test Teacher"}, nil)

	body := testutil.JSONBody(map[string]interface{}{
		"comment": map[string]interface{}{
			"text_comment": "Great work on this essay!",
		},
	})

	resp := testutil.MakeRequest(app, http.MethodPost, "/api/v1/courses/1/assignments/1/submissions/2/comments", body)

	assert.Equal(t, http.StatusCreated, resp.StatusCode)

	result, err := testutil.ParseJSONMap(resp)
	require.NoError(t, err)
	assert.Equal(t, "Great work on this essay!", result["comment"])
	assert.Equal(t, float64(1), result["author_id"])
	assert.Equal(t, "Test Teacher", result["author_name"])

	commentRepo.AssertExpectations(t)
	submissionRepo.AssertExpectations(t)
}

func TestListSubmissionComments(t *testing.T) {
	app, submissionRepo, _, _, commentRepo, userRepo := setupSubmissionTest()

	subType := "online_text_entry"
	submission := &models.Submission{
		ID:             1,
		AssignmentID:   1,
		UserID:         2,
		SubmissionType: &subType,
		WorkflowState:  "submitted",
	}

	// ListSubmissionComments threads callerAccountID(c)=1.
	submissionRepo.On("FindByAssignmentAndUser", mock.Anything, uint(1), uint(2), uint(1)).Return(submission, nil)

	comments := []models.SubmissionComment{
		{ID: 1, SubmissionID: 1, AuthorID: 1, Comment: "Good job!"},
		{ID: 2, SubmissionID: 1, AuthorID: 3, Comment: "Needs revision."},
	}

	commentRepo.On("ListBySubmissionID", mock.Anything, uint(1), uint(1)).Return(comments, nil)
	userRepo.On("FindByID", mock.Anything, uint(1), uint(1)).Return(&models.User{ID: 1, Name: "Teacher One"}, nil)
	userRepo.On("FindByID", mock.Anything, uint(3), uint(1)).Return(&models.User{ID: 3, Name: "Teacher Two"}, nil)

	resp := testutil.MakeRequest(app, http.MethodGet, "/api/v1/courses/1/assignments/1/submissions/2/comments", nil)

	assert.Equal(t, http.StatusOK, resp.StatusCode)

	result, err := testutil.ParseJSONArray(resp)
	require.NoError(t, err)
	assert.Len(t, result, 2)
	assert.Equal(t, "Good job!", result[0]["comment"])
	assert.Equal(t, "Teacher One", result[0]["author_name"])
	assert.Equal(t, "Needs revision.", result[1]["comment"])
	assert.Equal(t, "Teacher Two", result[1]["author_name"])

	commentRepo.AssertExpectations(t)
	submissionRepo.AssertExpectations(t)
}
