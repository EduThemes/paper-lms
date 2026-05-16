package mocks

import (
	"context"

	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/repository"
	"github.com/stretchr/testify/mock"
)

// MockCourseRepository mocks repository.CourseRepository
type MockCourseRepository struct {
	mock.Mock
}

func (m *MockCourseRepository) Create(ctx context.Context, course *models.Course) error {
	args := m.Called(ctx, course)
	return args.Error(0)
}

func (m *MockCourseRepository) FindByID(ctx context.Context, id, accountID uint) (*models.Course, error) {
	args := m.Called(ctx, id, accountID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.Course), args.Error(1)
}

func (m *MockCourseRepository) FindBySISCourseID(ctx context.Context, sisCourseID string) (*models.Course, error) {
	args := m.Called(ctx, sisCourseID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.Course), args.Error(1)
}

func (m *MockCourseRepository) Update(ctx context.Context, course *models.Course) error {
	args := m.Called(ctx, course)
	return args.Error(0)
}

func (m *MockCourseRepository) Delete(ctx context.Context, id uint) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

func (m *MockCourseRepository) List(ctx context.Context, accountID uint, params repository.PaginationParams) (*repository.PaginatedResult[models.Course], error) {
	args := m.Called(ctx, accountID, params)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*repository.PaginatedResult[models.Course]), args.Error(1)
}

func (m *MockCourseRepository) ListByUserID(ctx context.Context, userID, accountID uint, params repository.PaginationParams) (*repository.PaginatedResult[models.Course], error) {
	args := m.Called(ctx, userID, accountID, params)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*repository.PaginatedResult[models.Course]), args.Error(1)
}
