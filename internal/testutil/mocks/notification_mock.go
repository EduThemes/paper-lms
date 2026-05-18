package mocks

import (
	"context"

	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/repository"
	"github.com/stretchr/testify/mock"
)

// MockNotificationRepository mocks repository.NotificationRepository.
// All tenant-keyed methods accept accountID per 13.1.D — test setups
// MUST stub with the matching accountID value (or use
// mock.AnythingOfType("uint")) so cross-tenant cases surface as the
// not-found error path.
type MockNotificationRepository struct {
	mock.Mock
}

func (m *MockNotificationRepository) Create(ctx context.Context, notification *models.Notification) error {
	args := m.Called(ctx, notification)
	return args.Error(0)
}

func (m *MockNotificationRepository) FindByID(ctx context.Context, id, accountID uint) (*models.Notification, error) {
	args := m.Called(ctx, id, accountID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.Notification), args.Error(1)
}

func (m *MockNotificationRepository) Update(ctx context.Context, notification *models.Notification) error {
	args := m.Called(ctx, notification)
	return args.Error(0)
}

func (m *MockNotificationRepository) Delete(ctx context.Context, id uint) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

func (m *MockNotificationRepository) ListByUserID(ctx context.Context, userID, accountID uint, params repository.PaginationParams) (*repository.PaginatedResult[models.Notification], error) {
	args := m.Called(ctx, userID, accountID, params)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*repository.PaginatedResult[models.Notification]), args.Error(1)
}

func (m *MockNotificationRepository) MarkAsRead(ctx context.Context, userID, notificationID, accountID uint) error {
	args := m.Called(ctx, userID, notificationID, accountID)
	return args.Error(0)
}

func (m *MockNotificationRepository) MarkAllAsRead(ctx context.Context, userID, accountID uint) error {
	args := m.Called(ctx, userID, accountID)
	return args.Error(0)
}

func (m *MockNotificationRepository) ListUnreadByUserID(ctx context.Context, userID, accountID uint, params repository.PaginationParams) (*repository.PaginatedResult[models.Notification], error) {
	args := m.Called(ctx, userID, accountID, params)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*repository.PaginatedResult[models.Notification]), args.Error(1)
}

// MockNotificationPreferenceRepository mocks
// repository.NotificationPreferenceRepository. Preferences are
// user-keyed (not account-keyed); kept here alongside the
// NotificationRepository mock to keep the messaging-domain mocks
// co-located.
type MockNotificationPreferenceRepository struct {
	mock.Mock
}

func (m *MockNotificationPreferenceRepository) Create(ctx context.Context, prefs *models.NotificationPreference) error {
	args := m.Called(ctx, prefs)
	return args.Error(0)
}

func (m *MockNotificationPreferenceRepository) FindByUserID(ctx context.Context, userID uint) (*models.NotificationPreference, error) {
	args := m.Called(ctx, userID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.NotificationPreference), args.Error(1)
}

func (m *MockNotificationPreferenceRepository) Update(ctx context.Context, prefs *models.NotificationPreference) error {
	args := m.Called(ctx, prefs)
	return args.Error(0)
}

func (m *MockNotificationPreferenceRepository) Delete(ctx context.Context, userID uint) error {
	args := m.Called(ctx, userID)
	return args.Error(0)
}
