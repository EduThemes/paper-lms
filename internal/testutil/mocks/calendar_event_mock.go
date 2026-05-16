package mocks

import (
	"context"
	"time"

	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/repository"
	"github.com/stretchr/testify/mock"
)

// MockCalendarEventRepository implements repository.CalendarEventRepository for testing.
type MockCalendarEventRepository struct {
	mock.Mock
}

func (m *MockCalendarEventRepository) Create(ctx context.Context, event *models.CalendarEvent) error {
	args := m.Called(ctx, event)
	return args.Error(0)
}

func (m *MockCalendarEventRepository) FindByID(ctx context.Context, id, accountID uint) (*models.CalendarEvent, error) {
	args := m.Called(ctx, id, accountID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.CalendarEvent), args.Error(1)
}

func (m *MockCalendarEventRepository) Update(ctx context.Context, event *models.CalendarEvent) error {
	args := m.Called(ctx, event)
	return args.Error(0)
}

func (m *MockCalendarEventRepository) Delete(ctx context.Context, id uint) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

func (m *MockCalendarEventRepository) ListByContext(ctx context.Context, contextType string, contextID uint, params repository.PaginationParams) (*repository.PaginatedResult[models.CalendarEvent], error) {
	args := m.Called(ctx, contextType, contextID, params)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*repository.PaginatedResult[models.CalendarEvent]), args.Error(1)
}

func (m *MockCalendarEventRepository) ListByContextAndDateRange(ctx context.Context, contextType string, contextID uint, startAt, endAt time.Time) ([]models.CalendarEvent, error) {
	args := m.Called(ctx, contextType, contextID, startAt, endAt)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]models.CalendarEvent), args.Error(1)
}
