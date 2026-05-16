package mocks

import (
	"context"
	"time"

	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/repository"
	"github.com/stretchr/testify/mock"
)

// MockSubmissionRepository mocks repository.SubmissionRepository
type MockSubmissionRepository struct {
	mock.Mock
}

func (m *MockSubmissionRepository) Create(ctx context.Context, submission *models.Submission) error {
	args := m.Called(ctx, submission)
	return args.Error(0)
}

func (m *MockSubmissionRepository) FindByID(ctx context.Context, id uint) (*models.Submission, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.Submission), args.Error(1)
}

func (m *MockSubmissionRepository) FindByAssignmentAndUser(ctx context.Context, assignmentID, userID uint) (*models.Submission, error) {
	args := m.Called(ctx, assignmentID, userID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.Submission), args.Error(1)
}

func (m *MockSubmissionRepository) Update(ctx context.Context, submission *models.Submission) error {
	args := m.Called(ctx, submission)
	return args.Error(0)
}

func (m *MockSubmissionRepository) ListByAssignmentID(ctx context.Context, assignmentID uint, params repository.PaginationParams) (*repository.PaginatedResult[models.Submission], error) {
	args := m.Called(ctx, assignmentID, params)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*repository.PaginatedResult[models.Submission]), args.Error(1)
}

func (m *MockSubmissionRepository) ListByUserAndCourse(ctx context.Context, userID, courseID uint) ([]models.Submission, error) {
	args := m.Called(ctx, userID, courseID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]models.Submission), args.Error(1)
}

func (m *MockSubmissionRepository) ListByUserAndAssignmentIDs(ctx context.Context, userID uint, assignmentIDs []uint) ([]models.Submission, error) {
	args := m.Called(ctx, userID, assignmentIDs)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]models.Submission), args.Error(1)
}

func (m *MockSubmissionRepository) BulkListByCourse(ctx context.Context, courseID uint, params repository.PaginationParams) (*repository.PaginatedResult[models.Submission], error) {
	args := m.Called(ctx, courseID, params)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*repository.PaginatedResult[models.Submission]), args.Error(1)
}

func (m *MockSubmissionRepository) PostGradesByAssignment(ctx context.Context, assignmentID uint, postedAt *time.Time) error {
	args := m.Called(ctx, assignmentID, postedAt)
	return args.Error(0)
}

func (m *MockSubmissionRepository) FindByIDs(ctx context.Context, ids []uint) ([]models.Submission, error) {
	args := m.Called(ctx, ids)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]models.Submission), args.Error(1)
}

func (m *MockSubmissionRepository) FindByAssignmentAndUserIDs(ctx context.Context, assignmentID uint, userIDs []uint) ([]models.Submission, error) {
	args := m.Called(ctx, assignmentID, userIDs)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]models.Submission), args.Error(1)
}

func (m *MockSubmissionRepository) RunInTransaction(ctx context.Context, fn func(txRepo repository.SubmissionRepository) error) error {
	return fn(m)
}
