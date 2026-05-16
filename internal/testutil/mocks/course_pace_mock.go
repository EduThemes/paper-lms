package mocks

import (
	"context"

	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/repository"
	"github.com/stretchr/testify/mock"
)

// MockCoursePaceRepository implements repository.CoursePaceRepository for testing.
type MockCoursePaceRepository struct {
	mock.Mock
}

func (m *MockCoursePaceRepository) Create(ctx context.Context, pace *models.CoursePace) error {
	args := m.Called(ctx, pace)
	return args.Error(0)
}

func (m *MockCoursePaceRepository) FindByID(ctx context.Context, id, accountID uint) (*models.CoursePace, error) {
	args := m.Called(ctx, id, accountID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.CoursePace), args.Error(1)
}

func (m *MockCoursePaceRepository) Update(ctx context.Context, pace *models.CoursePace) error {
	args := m.Called(ctx, pace)
	return args.Error(0)
}

func (m *MockCoursePaceRepository) Delete(ctx context.Context, id uint) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

func (m *MockCoursePaceRepository) FindByCourseID(ctx context.Context, courseID uint) (*models.CoursePace, error) {
	args := m.Called(ctx, courseID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.CoursePace), args.Error(1)
}

func (m *MockCoursePaceRepository) FindByUserID(ctx context.Context, courseID, userID uint) (*models.CoursePace, error) {
	args := m.Called(ctx, courseID, userID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.CoursePace), args.Error(1)
}

func (m *MockCoursePaceRepository) FindBySectionID(ctx context.Context, courseID, sectionID uint) (*models.CoursePace, error) {
	args := m.Called(ctx, courseID, sectionID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.CoursePace), args.Error(1)
}

func (m *MockCoursePaceRepository) ListByCourseID(ctx context.Context, courseID uint, params repository.PaginationParams) (*repository.PaginatedResult[models.CoursePace], error) {
	args := m.Called(ctx, courseID, params)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*repository.PaginatedResult[models.CoursePace]), args.Error(1)
}

// MockCoursePaceModuleItemRepository implements repository.CoursePaceModuleItemRepository for testing.
type MockCoursePaceModuleItemRepository struct {
	mock.Mock
}

func (m *MockCoursePaceModuleItemRepository) Create(ctx context.Context, item *models.CoursePaceModuleItem) error {
	args := m.Called(ctx, item)
	return args.Error(0)
}

func (m *MockCoursePaceModuleItemRepository) FindByID(ctx context.Context, id, accountID uint) (*models.CoursePaceModuleItem, error) {
	args := m.Called(ctx, id, accountID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.CoursePaceModuleItem), args.Error(1)
}

func (m *MockCoursePaceModuleItemRepository) Update(ctx context.Context, item *models.CoursePaceModuleItem) error {
	args := m.Called(ctx, item)
	return args.Error(0)
}

func (m *MockCoursePaceModuleItemRepository) Delete(ctx context.Context, id uint) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

func (m *MockCoursePaceModuleItemRepository) ListByPaceID(ctx context.Context, paceID uint) ([]models.CoursePaceModuleItem, error) {
	args := m.Called(ctx, paceID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]models.CoursePaceModuleItem), args.Error(1)
}

func (m *MockCoursePaceModuleItemRepository) BulkUpsert(ctx context.Context, items []models.CoursePaceModuleItem) error {
	args := m.Called(ctx, items)
	return args.Error(0)
}
