package mocks

import (
	"context"

	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/repository"
	"github.com/stretchr/testify/mock"
)

// MockEnrollmentRepository mocks repository.EnrollmentRepository
type MockEnrollmentRepository struct {
	mock.Mock
}

func (m *MockEnrollmentRepository) Create(ctx context.Context, enrollment *models.Enrollment) error {
	args := m.Called(ctx, enrollment)
	return args.Error(0)
}

func (m *MockEnrollmentRepository) FindByID(ctx context.Context, id, accountID uint) (*models.Enrollment, error) {
	args := m.Called(ctx, id, accountID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.Enrollment), args.Error(1)
}

func (m *MockEnrollmentRepository) Update(ctx context.Context, enrollment *models.Enrollment) error {
	args := m.Called(ctx, enrollment)
	return args.Error(0)
}

func (m *MockEnrollmentRepository) ListByCourseID(ctx context.Context, courseID, accountID uint, params repository.PaginationParams) (*repository.PaginatedResult[models.Enrollment], error) {
	args := m.Called(ctx, courseID, accountID, params)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*repository.PaginatedResult[models.Enrollment]), args.Error(1)
}

func (m *MockEnrollmentRepository) ListByUserID(ctx context.Context, userID, accountID uint) ([]models.Enrollment, error) {
	args := m.Called(ctx, userID, accountID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]models.Enrollment), args.Error(1)
}

func (m *MockEnrollmentRepository) FindByUserAndCourse(ctx context.Context, userID, courseID, accountID uint) (*models.Enrollment, error) {
	args := m.Called(ctx, userID, courseID, accountID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.Enrollment), args.Error(1)
}

func (m *MockEnrollmentRepository) CountByCourseIDs(ctx context.Context, courseIDs []uint, accountID uint) (map[uint]int64, error) {
	args := m.Called(ctx, courseIDs, accountID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(map[uint]int64), args.Error(1)
}

func (m *MockEnrollmentRepository) ListActiveStudentUserIDsByCourse(ctx context.Context, courseID, accountID uint) ([]uint, error) {
	args := m.Called(ctx, courseID, accountID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]uint), args.Error(1)
}

func (m *MockEnrollmentRepository) ListActiveStudentEnrollmentsByCourse(ctx context.Context, courseID, accountID uint) ([]models.Enrollment, error) {
	args := m.Called(ctx, courseID, accountID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]models.Enrollment), args.Error(1)
}

func (m *MockEnrollmentRepository) UpdatePseudonymForSelf(ctx context.Context, userID, courseID, accountID uint, poolCode, name string) error {
	args := m.Called(ctx, userID, courseID, accountID, poolCode, name)
	return args.Error(0)
}
