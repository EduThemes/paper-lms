package mocks

import (
	"context"

	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/repository"
	"github.com/stretchr/testify/mock"
)

// MockCollaborationRepository mocks repository.CollaborationRepository
type MockCollaborationRepository struct {
	mock.Mock
}

func (m *MockCollaborationRepository) Create(ctx context.Context, collaboration *models.Collaboration) error {
	args := m.Called(ctx, collaboration)
	return args.Error(0)
}

func (m *MockCollaborationRepository) FindByID(ctx context.Context, id, accountID uint) (*models.Collaboration, error) {
	args := m.Called(ctx, id, accountID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.Collaboration), args.Error(1)
}

func (m *MockCollaborationRepository) Update(ctx context.Context, collaboration *models.Collaboration) error {
	args := m.Called(ctx, collaboration)
	return args.Error(0)
}

func (m *MockCollaborationRepository) Delete(ctx context.Context, id uint) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

func (m *MockCollaborationRepository) ListByContext(ctx context.Context, contextType string, contextID, accountID uint, params repository.PaginationParams) (*repository.PaginatedResult[models.Collaboration], error) {
	args := m.Called(ctx, contextType, contextID, accountID, params)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*repository.PaginatedResult[models.Collaboration]), args.Error(1)
}
