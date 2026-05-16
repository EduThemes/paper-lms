package mocks

import (
	"context"

	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/repository"
	"github.com/stretchr/testify/mock"
)

// MockRubricRepository mocks repository.RubricRepository.
type MockRubricRepository struct {
	mock.Mock
}

func (m *MockRubricRepository) Create(ctx context.Context, rubric *models.Rubric) error {
	return m.Called(ctx, rubric).Error(0)
}

func (m *MockRubricRepository) FindByID(ctx context.Context, id, accountID uint) (*models.Rubric, error) {
	args := m.Called(ctx, id, accountID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.Rubric), args.Error(1)
}

func (m *MockRubricRepository) Update(ctx context.Context, rubric *models.Rubric) error {
	return m.Called(ctx, rubric).Error(0)
}

func (m *MockRubricRepository) Delete(ctx context.Context, id uint) error {
	return m.Called(ctx, id).Error(0)
}

func (m *MockRubricRepository) ListByContext(ctx context.Context, contextType string, contextID, accountID uint, params repository.PaginationParams) (*repository.PaginatedResult[models.Rubric], error) {
	args := m.Called(ctx, contextType, contextID, accountID, params)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*repository.PaginatedResult[models.Rubric]), args.Error(1)
}
