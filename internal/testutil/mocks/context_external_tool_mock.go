package mocks

import (
	"context"

	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/repository"
	"github.com/stretchr/testify/mock"
)

// MockContextExternalToolRepository implements repository.ContextExternalToolRepository for testing.
type MockContextExternalToolRepository struct {
	mock.Mock
}

func (m *MockContextExternalToolRepository) Create(ctx context.Context, tool *models.ContextExternalTool) error {
	args := m.Called(ctx, tool)
	return args.Error(0)
}

func (m *MockContextExternalToolRepository) FindByID(ctx context.Context, id, accountID uint) (*models.ContextExternalTool, error) {
	args := m.Called(ctx, id, accountID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.ContextExternalTool), args.Error(1)
}

func (m *MockContextExternalToolRepository) Update(ctx context.Context, tool *models.ContextExternalTool) error {
	args := m.Called(ctx, tool)
	return args.Error(0)
}

func (m *MockContextExternalToolRepository) Delete(ctx context.Context, id uint) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

func (m *MockContextExternalToolRepository) ListByContext(ctx context.Context, contextType string, contextID uint, params repository.PaginationParams) (*repository.PaginatedResult[models.ContextExternalTool], error) {
	args := m.Called(ctx, contextType, contextID, params)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*repository.PaginatedResult[models.ContextExternalTool]), args.Error(1)
}
