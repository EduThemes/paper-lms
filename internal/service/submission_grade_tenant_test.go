package service_test

// F-016: SubmissionService.Grade must refuse to score a submission
// whose assignment lives outside the grader's tenant.
//
// Pre-fix Grade passed accountID=0 through every repo call, which let
// a teacher in course A submit
//   PUT /courses/A/assignments/<id from tenant B>/submissions/<user>
// and rewrite (or CREATE) a graded submission for any student in any
// tenant.

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"github.com/EduThemes/paper-lms/internal/service"
	"github.com/EduThemes/paper-lms/internal/testutil/mocks"
)

// TestGrade_CrossTenant_ReturnsErr — the assignment lookup runs under
// the grader's tenant; a miss = cross-tenant attempt = ErrGradeCrossTenant.
// Handler maps this to 404.
func TestGrade_CrossTenant_ReturnsErr(t *testing.T) {
	submissionRepo := new(mocks.MockSubmissionRepository)
	assignmentRepo := new(mocks.MockAssignmentRepository)
	enrollmentRepo := new(mocks.MockEnrollmentRepository)
	latePolicyRepo := new(mocks.MockLatePolicyRepository)
	courseRepo := new(mocks.MockCourseRepository)
	gradingPeriodGroupRepo := new(mocks.MockGradingPeriodGroupRepository)
	gradingPeriodRepo := new(mocks.MockGradingPeriodRepository)

	// Assignment lookup misses under the grader's tenant (tenant A=1)
	// — the assignment lives in tenant B (or doesn't exist).
	assignmentRepo.On("FindByID", mock.Anything, uint(10), uint(1)).
		Return(nil, errors.New("not found"))

	svc := service.NewSubmissionService(submissionRepo, assignmentRepo, enrollmentRepo, latePolicyRepo, courseRepo, gradingPeriodGroupRepo, gradingPeriodRepo, nil)
	result, err := svc.Grade(context.Background(), 10, 20, 99, 1 /* callerAccountID */, "95")

	assert.Error(t, err)
	assert.Equal(t, service.ErrGradeCrossTenant, err)
	assert.Nil(t, result)

	// CRITICAL: the submission MUST NOT be created or updated. The
	// pre-fix path would have created a graded submission for the
	// cross-tenant user — the very leak F-016 catches.
	submissionRepo.AssertNotCalled(t, "FindByAssignmentAndUser", mock.Anything, mock.Anything, mock.Anything, mock.Anything)
	submissionRepo.AssertNotCalled(t, "Create", mock.Anything, mock.Anything)
	submissionRepo.AssertNotCalled(t, "Update", mock.Anything, mock.Anything)
}
