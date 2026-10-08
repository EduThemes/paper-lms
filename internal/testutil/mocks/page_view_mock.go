package mocks

import (
	"context"
	"time"

	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/repository"
	"github.com/stretchr/testify/mock"
)

// MockPageViewRepository mocks repository.PageViewRepository
type MockPageViewRepository struct {
	mock.Mock
}

func (m *MockPageViewRepository) Create(ctx context.Context, pageView *models.PageView) error {
	args := m.Called(ctx, pageView)
	return args.Error(0)
}

func (m *MockPageViewRepository) FindByID(ctx context.Context, id, accountID uint) (*models.PageView, error) {
	args := m.Called(ctx, id, accountID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.PageView), args.Error(1)
}

func (m *MockPageViewRepository) ListByUserID(ctx context.Context, userID, accountID uint, params repository.PaginationParams) (*repository.PaginatedResult[models.PageView], error) {
	args := m.Called(ctx, userID, accountID, params)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*repository.PaginatedResult[models.PageView]), args.Error(1)
}

func (m *MockPageViewRepository) CountByContextGrouped(ctx context.Context, contextType string, contextID uint) ([]map[string]interface{}, error) {
	args := m.Called(ctx, contextType, contextID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]map[string]interface{}), args.Error(1)
}

func (m *MockPageViewRepository) SumInteractionByUserAndContext(ctx context.Context, userID uint, contextType string, contextID uint) (int64, error) {
	args := m.Called(ctx, userID, contextType, contextID)
	return args.Get(0).(int64), args.Error(1)
}

func (m *MockPageViewRepository) CountByUserAndContext(ctx context.Context, userID uint, contextType string, contextID uint) (int64, error) {
	args := m.Called(ctx, userID, contextType, contextID)
	return args.Get(0).(int64), args.Error(1)
}

func (m *MockPageViewRepository) ActivityByDay(ctx context.Context, courseID, userID uint, since *time.Time) ([]repository.PageViewDayCount, error) {
	args := m.Called(ctx, courseID, userID, since)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]repository.PageViewDayCount), args.Error(1)
}

func (m *MockPageViewRepository) TotalsByUser(ctx context.Context, courseID uint) (map[uint]repository.PageViewUserTotals, error) {
	args := m.Called(ctx, courseID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(map[uint]repository.PageViewUserTotals), args.Error(1)
}

func (m *MockPageViewRepository) CategoryCounts(ctx context.Context, courseID, userID uint) ([]repository.PageViewCategoryCount, error) {
	args := m.Called(ctx, courseID, userID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]repository.PageViewCategoryCount), args.Error(1)
}

func (m *MockPageViewRepository) ListByUserAndContext(ctx context.Context, userID uint, contextType string, contextID uint, params repository.PaginationParams) (*repository.PaginatedResult[models.PageView], error) {
	args := m.Called(ctx, userID, contextType, contextID, params)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*repository.PaginatedResult[models.PageView]), args.Error(1)
}

func (m *MockPageViewRepository) UpdateInteraction(ctx context.Context, id, userID uint, seconds int) error {
	args := m.Called(ctx, id, userID, seconds)
	return args.Error(0)
}

func (m *MockPageViewRepository) TouchEnrollmentActivity(ctx context.Context, userID, courseID uint) error {
	args := m.Called(ctx, userID, courseID)
	return args.Error(0)
}
