package mocks

import (
	"context"

	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/repository"
	"github.com/stretchr/testify/mock"
)

// MockPageRepository mocks repository.PageRepository
type MockPageRepository struct {
	mock.Mock
}

func (m *MockPageRepository) Create(ctx context.Context, page *models.WikiPage) error {
	args := m.Called(ctx, page)
	return args.Error(0)
}

func (m *MockPageRepository) FindByID(ctx context.Context, id, accountID uint) (*models.WikiPage, error) {
	args := m.Called(ctx, id, accountID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.WikiPage), args.Error(1)
}

func (m *MockPageRepository) FindByCourseAndURL(ctx context.Context, courseID uint, url string) (*models.WikiPage, error) {
	args := m.Called(ctx, courseID, url)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.WikiPage), args.Error(1)
}

func (m *MockPageRepository) Update(ctx context.Context, page *models.WikiPage) error {
	args := m.Called(ctx, page)
	return args.Error(0)
}

func (m *MockPageRepository) Delete(ctx context.Context, id uint) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

func (m *MockPageRepository) ListByCourseID(ctx context.Context, courseID uint, params repository.PaginationParams) (*repository.PaginatedResult[models.WikiPage], error) {
	args := m.Called(ctx, courseID, params)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*repository.PaginatedResult[models.WikiPage]), args.Error(1)
}

func (m *MockPageRepository) FindPublicByCourseAndURL(ctx context.Context, courseID uint, url string) (*models.WikiPage, error) {
	args := m.Called(ctx, courseID, url)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.WikiPage), args.Error(1)
}
