package handlers_test

import (
	"net/http"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/EduThemes/paper-lms/internal/api/v1/handlers"
	"github.com/EduThemes/paper-lms/internal/api/v1/middleware"
	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/service"
	"github.com/EduThemes/paper-lms/internal/testutil"
	"github.com/EduThemes/paper-lms/internal/testutil/mocks"
)

// TestGetSubmission_FiresLogPIIAccess is the Wave C.3 lock for the read
// side: a teacher fetching a student's submission must emit a
// pii_access_log row with the student as subject. Regression here would
// re-open the audit's "LogPIIAccess defined, never called" finding.
func TestGetSubmission_FiresLogPIIAccess(t *testing.T) {
	submissionRepo := new(mocks.MockSubmissionRepository)
	assignmentRepo := new(mocks.MockAssignmentRepository)
	enrollmentRepo := new(mocks.MockEnrollmentRepository)
	commentRepo := new(mocks.MockSubmissionCommentRepository)
	userRepo := new(mocks.MockUserRepository)
	attachmentRepo := new(mocks.MockAttachmentRepository)
	latePolicyRepo := new(mocks.MockLatePolicyRepository)
	courseRepo := new(mocks.MockCourseRepository)
	gradingPeriodGroupRepo := new(mocks.MockGradingPeriodGroupRepository)
	gradingPeriodRepo := new(mocks.MockGradingPeriodRepository)
	piiLogRepo := new(mocks.MockPIIAccessLogRepository)

	submissionService := service.NewSubmissionService(submissionRepo, assignmentRepo, enrollmentRepo, latePolicyRepo, courseRepo, gradingPeriodGroupRepo, gradingPeriodRepo, nil)
	// auditLogRepo + gradeChangeLogRepo are nil — LogPIIAccess only
	// touches piiLogRepo. LogEvent is a no-op when auditLogRepo is nil.
	auditService := service.NewAuditService(nil, nil, piiLogRepo)
	handler := handlers.NewSubmissionHandler(submissionService, commentRepo, attachmentRepo, userRepo, assignmentRepo, nil, nil, nil, nil, auditService)

	app := testutil.SetupTestApp()
	app.Use(func(c *fiber.Ctx) error {
		c.Locals("user_id", uint(7))
		c.Locals("account_id", uint(1))
		return c.Next()
	})
	app.Use(middleware.PaginationParams())

	courses := app.Group("/api/v1/courses/:course_id/assignments/:assignment_id")
	courses.Get("/submissions/:user_id", handler.GetSubmission)

	subType := "online_text_entry"
	now := time.Now()
	submission := &models.Submission{
		ID:             99,
		AssignmentID:   1,
		UserID:         123,
		SubmissionType: &subType,
		SubmittedAt:    &now,
		Attempt:        1,
		WorkflowState:  "submitted",
	}
	submissionRepo.On("FindByAssignmentAndUser", mock.Anything, uint(1), uint(123), uint(1)).Return(submission, nil)

	// The audit lock: the handler must Create exactly one PIIAccessLog
	// with accessor=7 (the caller) and student=123 (the submission's
	// owner) for the "submission" data_field on "submissions" resource.
	piiLogRepo.On("Create", mock.Anything, mock.MatchedBy(func(log *models.PIIAccessLog) bool {
		return log.AccessorID == 7 &&
			log.StudentID == 123 &&
			log.AccessType == "read" &&
			log.DataField == "submission" &&
			log.Resource == "submissions" &&
			log.ResourceID == 99
	})).Return(nil).Once()

	resp := testutil.MakeRequest(app, http.MethodGet, "/api/v1/courses/1/assignments/1/submissions/123", nil)

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	result, err := testutil.ParseJSONMap(resp)
	require.NoError(t, err)
	assert.Equal(t, float64(123), result["user_id"])

	piiLogRepo.AssertExpectations(t)
	submissionRepo.AssertExpectations(t)
}
