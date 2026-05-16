package mocks

import (
	"context"

	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/repository"
	"github.com/stretchr/testify/mock"
)

// MockQuizRepository mocks repository.QuizRepository
type MockQuizRepository struct {
	mock.Mock
}

func (m *MockQuizRepository) Create(ctx context.Context, quiz *models.Quiz) error {
	args := m.Called(ctx, quiz)
	return args.Error(0)
}

func (m *MockQuizRepository) FindByID(ctx context.Context, id, accountID uint) (*models.Quiz, error) {
	args := m.Called(ctx, id, accountID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.Quiz), args.Error(1)
}

func (m *MockQuizRepository) Update(ctx context.Context, quiz *models.Quiz) error {
	args := m.Called(ctx, quiz)
	return args.Error(0)
}

func (m *MockQuizRepository) Delete(ctx context.Context, id uint) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

func (m *MockQuizRepository) ListByCourseID(ctx context.Context, courseID uint, params repository.PaginationParams) (*repository.PaginatedResult[models.Quiz], error) {
	args := m.Called(ctx, courseID, params)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*repository.PaginatedResult[models.Quiz]), args.Error(1)
}
