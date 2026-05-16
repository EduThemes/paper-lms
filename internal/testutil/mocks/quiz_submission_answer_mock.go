package mocks

import (
	"context"

	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/stretchr/testify/mock"
)

// MockQuizSubmissionAnswerRepository mocks repository.QuizSubmissionAnswerRepository
type MockQuizSubmissionAnswerRepository struct {
	mock.Mock
}

func (m *MockQuizSubmissionAnswerRepository) Create(ctx context.Context, answer *models.QuizSubmissionAnswer) error {
	args := m.Called(ctx, answer)
	return args.Error(0)
}

func (m *MockQuizSubmissionAnswerRepository) BulkCreate(ctx context.Context, answers []models.QuizSubmissionAnswer) error {
	args := m.Called(ctx, answers)
	return args.Error(0)
}

func (m *MockQuizSubmissionAnswerRepository) FindByID(ctx context.Context, id uint) (*models.QuizSubmissionAnswer, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.QuizSubmissionAnswer), args.Error(1)
}

func (m *MockQuizSubmissionAnswerRepository) Update(ctx context.Context, answer *models.QuizSubmissionAnswer) error {
	args := m.Called(ctx, answer)
	return args.Error(0)
}

func (m *MockQuizSubmissionAnswerRepository) ListBySubmissionID(ctx context.Context, submissionID uint) ([]models.QuizSubmissionAnswer, error) {
	args := m.Called(ctx, submissionID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]models.QuizSubmissionAnswer), args.Error(1)
}

func (m *MockQuizSubmissionAnswerRepository) FindBySubmissionAndQuestion(ctx context.Context, submissionID, questionID uint) (*models.QuizSubmissionAnswer, error) {
	args := m.Called(ctx, submissionID, questionID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.QuizSubmissionAnswer), args.Error(1)
}

func (m *MockQuizSubmissionAnswerRepository) ListBySubmissionIDs(ctx context.Context, submissionIDs []uint) ([]models.QuizSubmissionAnswer, error) {
	args := m.Called(ctx, submissionIDs)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]models.QuizSubmissionAnswer), args.Error(1)
}
