package mocks

import (
	"context"

	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/repository"
	"github.com/stretchr/testify/mock"
)

// MockPIIAccessLogRepository mocks postgres.PIIAccessLogRepository so handler
// tests can assert that PII reads emit an audit row.
type MockPIIAccessLogRepository struct {
	mock.Mock
}

func (m *MockPIIAccessLogRepository) Create(ctx context.Context, log *models.PIIAccessLog) error {
	args := m.Called(ctx, log)
	return args.Error(0)
}

func (m *MockPIIAccessLogRepository) ListByStudentID(ctx context.Context, studentID uint, params repository.PaginationParams) (*repository.PaginatedResult[models.PIIAccessLog], error) {
	args := m.Called(ctx, studentID, params)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*repository.PaginatedResult[models.PIIAccessLog]), args.Error(1)
}

func (m *MockPIIAccessLogRepository) ListByAccessorID(ctx context.Context, accessorID uint, params repository.PaginationParams) (*repository.PaginatedResult[models.PIIAccessLog], error) {
	args := m.Called(ctx, accessorID, params)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*repository.PaginatedResult[models.PIIAccessLog]), args.Error(1)
}
