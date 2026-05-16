package mocks

import (
	"context"

	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/repository"
	"github.com/stretchr/testify/mock"
)

// MockQuizQuestionRepository mocks repository.QuizQuestionRepository
type MockQuizQuestionRepository struct {
	mock.Mock
}

func (m *MockQuizQuestionRepository) Create(ctx context.Context, question *models.QuizQuestion) error {
	args := m.Called(ctx, question)
	return args.Error(0)
}

func (m *MockQuizQuestionRepository) FindByID(ctx context.Context, id uint) (*models.QuizQuestion, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.QuizQuestion), args.Error(1)
}

func (m *MockQuizQuestionRepository) Update(ctx context.Context, question *models.QuizQuestion) error {
	args := m.Called(ctx, question)
	return args.Error(0)
}

func (m *MockQuizQuestionRepository) Delete(ctx context.Context, id uint) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

func (m *MockQuizQuestionRepository) ListByQuizID(ctx context.Context, quizID uint, params repository.PaginationParams) (*repository.PaginatedResult[models.QuizQuestion], error) {
	args := m.Called(ctx, quizID, params)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*repository.PaginatedResult[models.QuizQuestion]), args.Error(1)
}

func (m *MockQuizQuestionRepository) ListByGroupID(ctx context.Context, groupID uint) ([]models.QuizQuestion, error) {
	args := m.Called(ctx, groupID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]models.QuizQuestion), args.Error(1)
}

func (m *MockQuizQuestionRepository) FindByIDs(ctx context.Context, ids []uint) ([]models.QuizQuestion, error) {
	args := m.Called(ctx, ids)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]models.QuizQuestion), args.Error(1)
}
