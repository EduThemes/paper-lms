package mocks

import (
	"context"

	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/repository"
	"github.com/stretchr/testify/mock"
)

// MockBlueprintTemplateRepository mocks repository.BlueprintTemplateRepository.
type MockBlueprintTemplateRepository struct {
	mock.Mock
}

func (m *MockBlueprintTemplateRepository) Create(ctx context.Context, template *models.BlueprintTemplate) error {
	args := m.Called(ctx, template)
	return args.Error(0)
}

func (m *MockBlueprintTemplateRepository) FindByID(ctx context.Context, id uint) (*models.BlueprintTemplate, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.BlueprintTemplate), args.Error(1)
}

func (m *MockBlueprintTemplateRepository) FindByCourseID(ctx context.Context, courseID uint) (*models.BlueprintTemplate, error) {
	args := m.Called(ctx, courseID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.BlueprintTemplate), args.Error(1)
}

func (m *MockBlueprintTemplateRepository) Update(ctx context.Context, template *models.BlueprintTemplate) error {
	args := m.Called(ctx, template)
	return args.Error(0)
}

func (m *MockBlueprintTemplateRepository) Delete(ctx context.Context, id, accountID uint) error {
	args := m.Called(ctx, id, accountID)
	return args.Error(0)
}

func (m *MockBlueprintTemplateRepository) ListByCourseID(ctx context.Context, courseID uint, params repository.PaginationParams) (*repository.PaginatedResult[models.BlueprintTemplate], error) {
	args := m.Called(ctx, courseID, params)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*repository.PaginatedResult[models.BlueprintTemplate]), args.Error(1)
}
