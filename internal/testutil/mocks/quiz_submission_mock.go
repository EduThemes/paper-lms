package mocks

import (
	"context"

	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/repository"
	"github.com/stretchr/testify/mock"
)

// MockQuizSubmissionRepository mocks repository.QuizSubmissionRepository
type MockQuizSubmissionRepository struct {
	mock.Mock
}

func (m *MockQuizSubmissionRepository) Create(ctx context.Context, submission *models.QuizSubmission) error {
	args := m.Called(ctx, submission)
	return args.Error(0)
}

func (m *MockQuizSubmissionRepository) FindByID(ctx context.Context, id uint) (*models.QuizSubmission, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.QuizSubmission), args.Error(1)
}

func (m *MockQuizSubmissionRepository) Update(ctx context.Context, submission *models.QuizSubmission) error {
	args := m.Called(ctx, submission)
	return args.Error(0)
}

func (m *MockQuizSubmissionRepository) FindByQuizAndUser(ctx context.Context, quizID, userID uint) (*models.QuizSubmission, error) {
	args := m.Called(ctx, quizID, userID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.QuizSubmission), args.Error(1)
}

func (m *MockQuizSubmissionRepository) ListByUserAndQuizIDs(ctx context.Context, userID uint, quizIDs []uint) ([]models.QuizSubmission, error) {
	args := m.Called(ctx, userID, quizIDs)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]models.QuizSubmission), args.Error(1)
}

func (m *MockQuizSubmissionRepository) ListByQuizID(ctx context.Context, quizID uint, params repository.PaginationParams) (*repository.PaginatedResult[models.QuizSubmission], error) {
	args := m.Called(ctx, quizID, params)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*repository.PaginatedResult[models.QuizSubmission]), args.Error(1)
}

func (m *MockQuizSubmissionRepository) ListCompletedByQuizID(ctx context.Context, quizID uint) ([]models.QuizSubmission, error) {
	args := m.Called(ctx, quizID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]models.QuizSubmission), args.Error(1)
}
