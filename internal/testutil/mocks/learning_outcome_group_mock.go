package mocks

import (
	"context"

	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/repository"
	"github.com/stretchr/testify/mock"
)

// MockLearningOutcomeGroupRepository mocks repository.LearningOutcomeGroupRepository.
type MockLearningOutcomeGroupRepository struct {
	mock.Mock
}

func (m *MockLearningOutcomeGroupRepository) Create(ctx context.Context, group *models.LearningOutcomeGroup) error {
	return m.Called(ctx, group).Error(0)
}

func (m *MockLearningOutcomeGroupRepository) FindByID(ctx context.Context, id, accountID uint) (*models.LearningOutcomeGroup, error) {
	args := m.Called(ctx, id, accountID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.LearningOutcomeGroup), args.Error(1)
}

func (m *MockLearningOutcomeGroupRepository) Update(ctx context.Context, group *models.LearningOutcomeGroup) error {
	return m.Called(ctx, group).Error(0)
}

func (m *MockLearningOutcomeGroupRepository) Delete(ctx context.Context, id uint) error {
	return m.Called(ctx, id).Error(0)
}

func (m *MockLearningOutcomeGroupRepository) ListByContext(ctx context.Context, contextType string, contextID, accountID uint, params repository.PaginationParams) (*repository.PaginatedResult[models.LearningOutcomeGroup], error) {
	args := m.Called(ctx, contextType, contextID, accountID, params)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*repository.PaginatedResult[models.LearningOutcomeGroup]), args.Error(1)
}

func (m *MockLearningOutcomeGroupRepository) FindRootGroup(ctx context.Context, contextType string, contextID, accountID uint) (*models.LearningOutcomeGroup, error) {
	args := m.Called(ctx, contextType, contextID, accountID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.LearningOutcomeGroup), args.Error(1)
}
