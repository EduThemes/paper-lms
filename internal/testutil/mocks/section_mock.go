package mocks

import (
	"context"

	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/repository"
	"github.com/stretchr/testify/mock"
)

// MockSectionRepository mocks repository.SectionRepository
type MockSectionRepository struct {
	mock.Mock
}

func (m *MockSectionRepository) Create(ctx context.Context, section *models.CourseSection) error {
	args := m.Called(ctx, section)
	return args.Error(0)
}

func (m *MockSectionRepository) FindByID(ctx context.Context, id uint) (*models.CourseSection, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.CourseSection), args.Error(1)
}

func (m *MockSectionRepository) FindBySISSectionID(ctx context.Context, sisSectionID string) (*models.CourseSection, error) {
	args := m.Called(ctx, sisSectionID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.CourseSection), args.Error(1)
}

func (m *MockSectionRepository) ListByCourseID(ctx context.Context, courseID uint, params repository.PaginationParams) (*repository.PaginatedResult[models.CourseSection], error) {
	args := m.Called(ctx, courseID, params)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*repository.PaginatedResult[models.CourseSection]), args.Error(1)
}
