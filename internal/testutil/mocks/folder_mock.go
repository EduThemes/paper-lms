package mocks

import (
	"context"

	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/repository"
	"github.com/stretchr/testify/mock"
)

// MockFolderRepository mocks repository.FolderRepository
type MockFolderRepository struct {
	mock.Mock
}

func (m *MockFolderRepository) Create(ctx context.Context, folder *models.Folder) error {
	args := m.Called(ctx, folder)
	return args.Error(0)
}

func (m *MockFolderRepository) FindByID(ctx context.Context, id, accountID uint) (*models.Folder, error) {
	args := m.Called(ctx, id, accountID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.Folder), args.Error(1)
}

func (m *MockFolderRepository) Update(ctx context.Context, folder *models.Folder) error {
	args := m.Called(ctx, folder)
	return args.Error(0)
}

func (m *MockFolderRepository) Delete(ctx context.Context, id uint) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

func (m *MockFolderRepository) ListByContext(ctx context.Context, contextType string, contextID uint, parentFolderID *uint, params repository.PaginationParams) (*repository.PaginatedResult[models.Folder], error) {
	args := m.Called(ctx, contextType, contextID, parentFolderID, params)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*repository.PaginatedResult[models.Folder]), args.Error(1)
}

func (m *MockFolderRepository) FindRootFolder(ctx context.Context, contextType string, contextID uint) (*models.Folder, error) {
	args := m.Called(ctx, contextType, contextID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.Folder), args.Error(1)
}
