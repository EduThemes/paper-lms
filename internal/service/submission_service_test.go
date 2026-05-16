package service_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/repository"
	"github.com/EduThemes/paper-lms/internal/service"
	"github.com/EduThemes/paper-lms/internal/testutil/mocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func ptrString(s string) *string     { return &s }
func ptrFloat64(f float64) *float64  { return &f }
func ptrTime(t time.Time) *time.Time { return &t }

func TestCreate_NewSubmission(t *testing.T) {
	assignmentRepo := new(mocks.MockAssignmentRepository)
	submissionRepo := new(mocks.MockSubmissionRepository)
	enrollmentRepo := new(mocks.MockEnrollmentRepository)

	assignmentRepo.On("FindByID", mock.Anything, uint(1), uint(0)).
		Return(&models.Assignment{ID: 1, CourseID: 1, Name: "HW1"}, nil)
	submissionRepo.On("FindByAssignmentAndUser", mock.Anything, uint(1), uint(10), uint(0)).
		Return(nil, errors.New("not found"))
	submissionRepo.On("Create", mock.Anything, mock.AnythingOfType("*models.Submission")).
		Return(nil)

	latePolicyRepo := new(mocks.MockLatePolicyRepository)
	courseRepo := new(mocks.MockCourseRepository)
	gradingPeriodGroupRepo := new(mocks.MockGradingPeriodGroupRepository)
	gradingPeriodRepo := new(mocks.MockGradingPeriodRepository)
	svc := service.NewSubmissionService(submissionRepo, assignmentRepo, enrollmentRepo, latePolicyRepo, courseRepo, gradingPeriodGroupRepo, gradingPeriodRepo, nil)
	sub := &models.Submission{
		AssignmentID:   1,
		UserID:         10,
		SubmissionType: ptrString("online_text_entry"),
		Body:           ptrString("My submission"),
	}

	err := svc.Create(context.Background(), sub)

	assert.NoError(t, err)
	assert.Equal(t, 1, sub.Attempt)
	assert.Equal(t, "submitted", sub.WorkflowState)
	assert.NotNil(t, sub.SubmittedAt)
	submissionRepo.AssertCalled(t, "Create", mock.Anything, mock.AnythingOfType("*models.Submission"))
}

func TestCreate_MissingType(t *testing.T) {
	assignmentRepo := new(mocks.MockAssignmentRepository)
	submissionRepo := new(mocks.MockSubmissionRepository)
	enrollmentRepo := new(mocks.MockEnrollmentRepository)

	assignmentRepo.On("FindByID", mock.Anything, uint(1), uint(0)).
		Return(&models.Assignment{ID: 1, CourseID: 1, Name: "HW1"}, nil)

	latePolicyRepo := new(mocks.MockLatePolicyRepository)
	courseRepo := new(mocks.MockCourseRepository)
	gradingPeriodGroupRepo := new(mocks.MockGradingPeriodGroupRepository)
	gradingPeriodRepo := new(mocks.MockGradingPeriodRepository)
	svc := service.NewSubmissionService(submissionRepo, assignmentRepo, enrollmentRepo, latePolicyRepo, courseRepo, gradingPeriodGroupRepo, gradingPeriodRepo, nil)

	// Test with nil SubmissionType
	sub := &models.Submission{
		AssignmentID: 1,
		UserID:       10,
	}
	err := svc.Create(context.Background(), sub)
	assert.Error(t, err)
	assert.Equal(t, "submission_type is required", err.Error())

	// Test with empty string SubmissionType
	sub2 := &models.Submission{
		AssignmentID:   1,
		UserID:         10,
		SubmissionType: ptrString(""),
	}
	err = svc.Create(context.Background(), sub2)
	assert.Error(t, err)
	assert.Equal(t, "submission_type is required", err.Error())
}

func TestCreate_AssignmentNotFound(t *testing.T) {
	assignmentRepo := new(mocks.MockAssignmentRepository)
	submissionRepo := new(mocks.MockSubmissionRepository)
	enrollmentRepo := new(mocks.MockEnrollmentRepository)

	assignmentRepo.On("FindByID", mock.Anything, uint(999), uint(0)).
		Return(nil, errors.New("record not found"))

	latePolicyRepo := new(mocks.MockLatePolicyRepository)
	courseRepo := new(mocks.MockCourseRepository)
	gradingPeriodGroupRepo := new(mocks.MockGradingPeriodGroupRepository)
	gradingPeriodRepo := new(mocks.MockGradingPeriodRepository)
	svc := service.NewSubmissionService(submissionRepo, assignmentRepo, enrollmentRepo, latePolicyRepo, courseRepo, gradingPeriodGroupRepo, gradingPeriodRepo, nil)
	sub := &models.Submission{
		AssignmentID:   999,
		UserID:         10,
		SubmissionType: ptrString("online_text_entry"),
	}

	err := svc.Create(context.Background(), sub)

	assert.Error(t, err)
	assert.Equal(t, "assignment not found", err.Error())
}

func TestCreate_LateSubmission(t *testing.T) {
	pastDue := time.Now().Add(-24 * time.Hour)

	assignmentRepo := new(mocks.MockAssignmentRepository)
	submissionRepo := new(mocks.MockSubmissionRepository)
	enrollmentRepo := new(mocks.MockEnrollmentRepository)

	assignmentRepo.On("FindByID", mock.Anything, uint(1), uint(0)).
		Return(&models.Assignment{ID: 1, CourseID: 1, Name: "HW1", DueAt: &pastDue}, nil)
	submissionRepo.On("FindByAssignmentAndUser", mock.Anything, uint(1), uint(10), uint(0)).
		Return(nil, errors.New("not found"))
	submissionRepo.On("Create", mock.Anything, mock.AnythingOfType("*models.Submission")).
		Return(nil)

	latePolicyRepo := new(mocks.MockLatePolicyRepository)
	courseRepo := new(mocks.MockCourseRepository)
	gradingPeriodGroupRepo := new(mocks.MockGradingPeriodGroupRepository)
	gradingPeriodRepo := new(mocks.MockGradingPeriodRepository)
	svc := service.NewSubmissionService(submissionRepo, assignmentRepo, enrollmentRepo, latePolicyRepo, courseRepo, gradingPeriodGroupRepo, gradingPeriodRepo, nil)
	sub := &models.Submission{
		AssignmentID:   1,
		UserID:         10,
		SubmissionType: ptrString("online_text_entry"),
	}

	err := svc.Create(context.Background(), sub)

	assert.NoError(t, err)
	assert.True(t, sub.Late, "expected submission to be marked as late")
}

func TestCreate_OnTimeSubmission(t *testing.T) {
	futureDue := time.Now().Add(48 * time.Hour)

	assignmentRepo := new(mocks.MockAssignmentRepository)
	submissionRepo := new(mocks.MockSubmissionRepository)
	enrollmentRepo := new(mocks.MockEnrollmentRepository)

	assignmentRepo.On("FindByID", mock.Anything, uint(1), uint(0)).
		Return(&models.Assignment{ID: 1, CourseID: 1, Name: "HW1", DueAt: &futureDue}, nil)
	submissionRepo.On("FindByAssignmentAndUser", mock.Anything, uint(1), uint(10), uint(0)).
		Return(nil, errors.New("not found"))
	submissionRepo.On("Create", mock.Anything, mock.AnythingOfType("*models.Submission")).
		Return(nil)

	latePolicyRepo := new(mocks.MockLatePolicyRepository)
	courseRepo := new(mocks.MockCourseRepository)
	gradingPeriodGroupRepo := new(mocks.MockGradingPeriodGroupRepository)
	gradingPeriodRepo := new(mocks.MockGradingPeriodRepository)
	svc := service.NewSubmissionService(submissionRepo, assignmentRepo, enrollmentRepo, latePolicyRepo, courseRepo, gradingPeriodGroupRepo, gradingPeriodRepo, nil)
	sub := &models.Submission{
		AssignmentID:   1,
		UserID:         10,
		SubmissionType: ptrString("online_text_entry"),
	}

	err := svc.Create(context.Background(), sub)

	assert.NoError(t, err)
	assert.False(t, sub.Late, "expected submission to not be marked as late")
}

func TestCreate_NoDueDate(t *testing.T) {
	assignmentRepo := new(mocks.MockAssignmentRepository)
	submissionRepo := new(mocks.MockSubmissionRepository)
	enrollmentRepo := new(mocks.MockEnrollmentRepository)

	assignmentRepo.On("FindByID", mock.Anything, uint(1), uint(0)).
		Return(&models.Assignment{ID: 1, CourseID: 1, Name: "HW1", DueAt: nil}, nil)
	submissionRepo.On("FindByAssignmentAndUser", mock.Anything, uint(1), uint(10), uint(0)).
		Return(nil, errors.New("not found"))
	submissionRepo.On("Create", mock.Anything, mock.AnythingOfType("*models.Submission")).
		Return(nil)

	latePolicyRepo := new(mocks.MockLatePolicyRepository)
	courseRepo := new(mocks.MockCourseRepository)
	gradingPeriodGroupRepo := new(mocks.MockGradingPeriodGroupRepository)
	gradingPeriodRepo := new(mocks.MockGradingPeriodRepository)
	svc := service.NewSubmissionService(submissionRepo, assignmentRepo, enrollmentRepo, latePolicyRepo, courseRepo, gradingPeriodGroupRepo, gradingPeriodRepo, nil)
	sub := &models.Submission{
		AssignmentID:   1,
		UserID:         10,
		SubmissionType: ptrString("online_text_entry"),
	}

	err := svc.Create(context.Background(), sub)

	assert.NoError(t, err)
	assert.False(t, sub.Late, "expected submission with no due date to not be late")
}

func TestCreate_ResubmissionIncrementsAttempt(t *testing.T) {
	now := time.Now()
	existingSub := &models.Submission{
		ID:             5,
		AssignmentID:   1,
		UserID:         10,
		SubmissionType: ptrString("online_text_entry"),
		Body:           ptrString("First attempt"),
		Attempt:        1,
		SubmittedAt:    &now,
		WorkflowState:  "submitted",
	}

	assignmentRepo := new(mocks.MockAssignmentRepository)
	submissionRepo := new(mocks.MockSubmissionRepository)
	enrollmentRepo := new(mocks.MockEnrollmentRepository)

	assignmentRepo.On("FindByID", mock.Anything, uint(1), uint(0)).
		Return(&models.Assignment{ID: 1, CourseID: 1, Name: "HW1"}, nil)
	submissionRepo.On("FindByAssignmentAndUser", mock.Anything, uint(1), uint(10), uint(0)).
		Return(existingSub, nil)
	submissionRepo.On("Update", mock.Anything, mock.AnythingOfType("*models.Submission")).
		Return(nil)

	latePolicyRepo := new(mocks.MockLatePolicyRepository)
	courseRepo := new(mocks.MockCourseRepository)
	gradingPeriodGroupRepo := new(mocks.MockGradingPeriodGroupRepository)
	gradingPeriodRepo := new(mocks.MockGradingPeriodRepository)
	svc := service.NewSubmissionService(submissionRepo, assignmentRepo, enrollmentRepo, latePolicyRepo, courseRepo, gradingPeriodGroupRepo, gradingPeriodRepo, nil)
	sub := &models.Submission{
		AssignmentID:   1,
		UserID:         10,
		SubmissionType: ptrString("online_text_entry"),
		Body:           ptrString("Second attempt"),
	}

	err := svc.Create(context.Background(), sub)

	assert.NoError(t, err)
	submissionRepo.AssertCalled(t, "Update", mock.Anything, mock.AnythingOfType("*models.Submission"))
	assert.Equal(t, 2, sub.Attempt, "expected attempt to be incremented to 2")
	assert.Equal(t, "submitted", sub.WorkflowState)
	assert.Equal(t, uint(5), sub.ID, "expected submission to retain the original ID")
}

func TestGetByAssignmentAndUser(t *testing.T) {
	expected := &models.Submission{
		ID:            1,
		AssignmentID:  10,
		UserID:        20,
		WorkflowState: "submitted",
		Attempt:       1,
	}

	submissionRepo := new(mocks.MockSubmissionRepository)
	assignmentRepo := new(mocks.MockAssignmentRepository)
	enrollmentRepo := new(mocks.MockEnrollmentRepository)

	submissionRepo.On("FindByAssignmentAndUser", mock.Anything, uint(10), uint(20), uint(0)).
		Return(expected, nil)

	latePolicyRepo := new(mocks.MockLatePolicyRepository)
	courseRepo := new(mocks.MockCourseRepository)
	gradingPeriodGroupRepo := new(mocks.MockGradingPeriodGroupRepository)
	gradingPeriodRepo := new(mocks.MockGradingPeriodRepository)
	svc := service.NewSubmissionService(submissionRepo, assignmentRepo, enrollmentRepo, latePolicyRepo, courseRepo, gradingPeriodGroupRepo, gradingPeriodRepo, nil)
	result, err := svc.GetByAssignmentAndUser(context.Background(), 10, 20, 0)

	assert.NoError(t, err)
	assert.Equal(t, expected, result)
	submissionRepo.AssertExpectations(t)
}

func TestListByAssignment(t *testing.T) {
	expected := &repository.PaginatedResult[models.Submission]{
		Items: []models.Submission{
			{ID: 1, AssignmentID: 10, UserID: 20},
			{ID: 2, AssignmentID: 10, UserID: 21},
		},
		TotalCount: 2,
		Page:       1,
		PerPage:    10,
	}

	submissionRepo := new(mocks.MockSubmissionRepository)
	assignmentRepo := new(mocks.MockAssignmentRepository)
	enrollmentRepo := new(mocks.MockEnrollmentRepository)

	submissionRepo.On("ListByAssignmentID", mock.Anything, uint(10), mock.AnythingOfType("repository.PaginationParams")).
		Return(expected, nil)

	latePolicyRepo := new(mocks.MockLatePolicyRepository)
	courseRepo := new(mocks.MockCourseRepository)
	gradingPeriodGroupRepo := new(mocks.MockGradingPeriodGroupRepository)
	gradingPeriodRepo := new(mocks.MockGradingPeriodRepository)
	svc := service.NewSubmissionService(submissionRepo, assignmentRepo, enrollmentRepo, latePolicyRepo, courseRepo, gradingPeriodGroupRepo, gradingPeriodRepo, nil)
	result, err := svc.ListByAssignment(context.Background(), 10, repository.PaginationParams{Page: 1, PerPage: 10})

	assert.NoError(t, err)
	assert.Equal(t, int64(2), result.TotalCount)
	assert.Len(t, result.Items, 2)
	submissionRepo.AssertExpectations(t)
}

func TestGrade_Success(t *testing.T) {
	existing := &models.Submission{
		ID:            1,
		AssignmentID:  10,
		UserID:        20,
		Attempt:       1,
		WorkflowState: "submitted",
	}

	submissionRepo := new(mocks.MockSubmissionRepository)
	assignmentRepo := new(mocks.MockAssignmentRepository)
	enrollmentRepo := new(mocks.MockEnrollmentRepository)

	// Mock for grading period check (no due date → no period enforcement)
	assignmentRepo.On("FindByID", mock.Anything, uint(10), uint(0)).
		Return(&models.Assignment{ID: 10, CourseID: 1, Name: "HW1"}, nil)
	submissionRepo.On("FindByAssignmentAndUser", mock.Anything, uint(10), uint(20), uint(0)).
		Return(existing, nil)
	submissionRepo.On("Update", mock.Anything, mock.AnythingOfType("*models.Submission")).
		Return(nil)

	latePolicyRepo := new(mocks.MockLatePolicyRepository)
	courseRepo := new(mocks.MockCourseRepository)
	gradingPeriodGroupRepo := new(mocks.MockGradingPeriodGroupRepository)
	gradingPeriodRepo := new(mocks.MockGradingPeriodRepository)
	svc := service.NewSubmissionService(submissionRepo, assignmentRepo, enrollmentRepo, latePolicyRepo, courseRepo, gradingPeriodGroupRepo, gradingPeriodRepo, nil)
	result, err := svc.Grade(context.Background(), 10, 20, 99, "95")

	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.NotNil(t, result.Score)
	assert.Equal(t, 95.0, *result.Score)
	assert.NotNil(t, result.Grade)
	assert.Equal(t, "95", *result.Grade)
	assert.Equal(t, "graded", result.WorkflowState)
	assert.NotNil(t, result.GradedAt)
	assert.NotNil(t, result.GraderID)
	assert.Equal(t, uint(99), *result.GraderID)
}

func TestGrade_InvalidGrade(t *testing.T) {
	existing := &models.Submission{
		ID:            1,
		AssignmentID:  10,
		UserID:        20,
		WorkflowState: "submitted",
	}

	submissionRepo := new(mocks.MockSubmissionRepository)
	assignmentRepo := new(mocks.MockAssignmentRepository)
	enrollmentRepo := new(mocks.MockEnrollmentRepository)

	submissionRepo.On("FindByAssignmentAndUser", mock.Anything, uint(10), uint(20), uint(0)).
		Return(existing, nil)

	latePolicyRepo := new(mocks.MockLatePolicyRepository)
	courseRepo := new(mocks.MockCourseRepository)
	gradingPeriodGroupRepo := new(mocks.MockGradingPeriodGroupRepository)
	gradingPeriodRepo := new(mocks.MockGradingPeriodRepository)
	svc := service.NewSubmissionService(submissionRepo, assignmentRepo, enrollmentRepo, latePolicyRepo, courseRepo, gradingPeriodGroupRepo, gradingPeriodRepo, nil)
	result, err := svc.Grade(context.Background(), 10, 20, 99, "abc")

	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "invalid grade")
}

func TestGrade_NotFound_CreatesSubmission(t *testing.T) {
	submissionRepo := new(mocks.MockSubmissionRepository)
	assignmentRepo := new(mocks.MockAssignmentRepository)
	enrollmentRepo := new(mocks.MockEnrollmentRepository)

	// Mock for grading period check (no due date → no period enforcement)
	assignmentRepo.On("FindByID", mock.Anything, uint(10), uint(0)).
		Return(&models.Assignment{ID: 10, CourseID: 1, Name: "HW1"}, nil)
	// No existing submission — Grade() should create one
	submissionRepo.On("FindByAssignmentAndUser", mock.Anything, uint(10), uint(20), uint(0)).
		Return(nil, errors.New("record not found"))
	submissionRepo.On("Create", mock.Anything, mock.AnythingOfType("*models.Submission")).
		Return(nil)

	latePolicyRepo := new(mocks.MockLatePolicyRepository)
	courseRepo := new(mocks.MockCourseRepository)
	gradingPeriodGroupRepo := new(mocks.MockGradingPeriodGroupRepository)
	gradingPeriodRepo := new(mocks.MockGradingPeriodRepository)
	svc := service.NewSubmissionService(submissionRepo, assignmentRepo, enrollmentRepo, latePolicyRepo, courseRepo, gradingPeriodGroupRepo, gradingPeriodRepo, nil)
	result, err := svc.Grade(context.Background(), 10, 20, 99, "95")

	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, "graded", result.WorkflowState)
	assert.NotNil(t, result.Score)
	assert.Equal(t, 95.0, *result.Score)
}
