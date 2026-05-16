package mocks

import (
	"context"

	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/repository"
	"github.com/stretchr/testify/mock"
)

// MockAttachmentRepository mocks repository.AttachmentRepository
type MockAttachmentRepository struct {
	mock.Mock
}

func (m *MockAttachmentRepository) Create(ctx context.Context, attachment *models.Attachment) error {
	args := m.Called(ctx, attachment)
	return args.Error(0)
}

func (m *MockAttachmentRepository) FindByID(ctx context.Context, id, accountID uint) (*models.Attachment, error) {
	args := m.Called(ctx, id, accountID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.Attachment), args.Error(1)
}

func (m *MockAttachmentRepository) Update(ctx context.Context, attachment *models.Attachment) error {
	args := m.Called(ctx, attachment)
	return args.Error(0)
}

func (m *MockAttachmentRepository) Delete(ctx context.Context, id uint) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

func (m *MockAttachmentRepository) ListByContext(ctx context.Context, contextType string, contextID uint, params repository.PaginationParams) (*repository.PaginatedResult[models.Attachment], error) {
	args := m.Called(ctx, contextType, contextID, params)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*repository.PaginatedResult[models.Attachment]), args.Error(1)
}

func (m *MockAttachmentRepository) ListByFolderID(ctx context.Context, folderID uint, params repository.PaginationParams) (*repository.PaginatedResult[models.Attachment], error) {
	args := m.Called(ctx, folderID, params)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*repository.PaginatedResult[models.Attachment]), args.Error(1)
}
