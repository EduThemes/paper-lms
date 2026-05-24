package service_test

// F-047: SubmissionService.Create must enforce assignment.UnlockAt /
// LockAt. The pre-fix path flagged late submissions (past due_at) but
// accepted them; lock_at was ignored entirely, so students could
// submit indefinitely after the assignment closed.

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/service"
	"github.com/EduThemes/paper-lms/internal/testutil/mocks"
)

func newSubmissionServiceForLockTests(assignmentRepo *mocks.MockAssignmentRepository) (*service.SubmissionService, *mocks.MockSubmissionRepository) {
	submissionRepo := new(mocks.MockSubmissionRepository)
	enrollmentRepo := new(mocks.MockEnrollmentRepository)
	latePolicyRepo := new(mocks.MockLatePolicyRepository)
	courseRepo := new(mocks.MockCourseRepository)
	gradingPeriodGroupRepo := new(mocks.MockGradingPeriodGroupRepository)
	gradingPeriodRepo := new(mocks.MockGradingPeriodRepository)
	svc := service.NewSubmissionService(submissionRepo, assignmentRepo, enrollmentRepo, latePolicyRepo, courseRepo, gradingPeriodGroupRepo, gradingPeriodRepo, nil)
	return svc, submissionRepo
}

// TestSubmissionCreate_LockAtPast_Returns409Error — F-047 contract.
func TestSubmissionCreate_LockAtPast_Returns409Error(t *testing.T) {
	assignmentRepo := new(mocks.MockAssignmentRepository)
	past := time.Now().Add(-1 * time.Hour)
	assignmentRepo.On("FindByID", mock.Anything, uint(1), uint(0)).Return(&models.Assignment{
		ID:       1,
		CourseID: 1,
		LockAt:   &past,
	}, nil)

	svc, submissionRepo := newSubmissionServiceForLockTests(assignmentRepo)

	subType := "online_text_entry"
	sub := &models.Submission{AssignmentID: 1, UserID: 2, SubmissionType: &subType}
	err := svc.Create(context.Background(), sub)
	assert.Equal(t, service.ErrSubmissionLocked, err)

	// The submission table MUST NOT be touched on a locked assignment.
	submissionRepo.AssertNotCalled(t, "FindByAssignmentAndUser", mock.Anything, mock.Anything, mock.Anything, mock.Anything)
	submissionRepo.AssertNotCalled(t, "Create", mock.Anything, mock.Anything)
}

// TestSubmissionCreate_BeforeUnlockAt_Returns409Error — F-047 contract.
func TestSubmissionCreate_BeforeUnlockAt_Returns409Error(t *testing.T) {
	assignmentRepo := new(mocks.MockAssignmentRepository)
	future := time.Now().Add(1 * time.Hour)
	assignmentRepo.On("FindByID", mock.Anything, uint(1), uint(0)).Return(&models.Assignment{
		ID:       1,
		CourseID: 1,
		UnlockAt: &future,
	}, nil)

	svc, submissionRepo := newSubmissionServiceForLockTests(assignmentRepo)

	subType := "online_text_entry"
	sub := &models.Submission{AssignmentID: 1, UserID: 2, SubmissionType: &subType}
	err := svc.Create(context.Background(), sub)
	assert.Equal(t, service.ErrSubmissionNotYetUnlocked, err)

	submissionRepo.AssertNotCalled(t, "FindByAssignmentAndUser", mock.Anything, mock.Anything, mock.Anything, mock.Anything)
	submissionRepo.AssertNotCalled(t, "Create", mock.Anything, mock.Anything)
}

// TestSubmissionCreate_WithinWindow_Succeeds — happy path: no
// unlock_at / lock_at fields set → the gate is a no-op.
func TestSubmissionCreate_WithinWindow_Succeeds(t *testing.T) {
	assignmentRepo := new(mocks.MockAssignmentRepository)
	assignmentRepo.On("FindByID", mock.Anything, uint(1), uint(0)).Return(&models.Assignment{
		ID:       1,
		CourseID: 1,
	}, nil)

	svc, submissionRepo := newSubmissionServiceForLockTests(assignmentRepo)
	// No existing submission → Create path
	submissionRepo.On("FindByAssignmentAndUser", mock.Anything, uint(1), uint(2), uint(0)).Return(nil, assert.AnError)
	submissionRepo.On("Create", mock.Anything, mock.AnythingOfType("*models.Submission")).Return(nil)

	subType := "online_text_entry"
	sub := &models.Submission{AssignmentID: 1, UserID: 2, SubmissionType: &subType}
	err := svc.Create(context.Background(), sub)
	assert.NoError(t, err)
}
