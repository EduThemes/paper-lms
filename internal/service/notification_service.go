package service

import (
	"context"
	"errors"
	"time"

	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/repository"
)

type NotificationService struct {
	prefRepo  repository.NotificationPreferenceRepository
	notifRepo repository.NotificationRepository
}

func NewNotificationService(
	prefRepo repository.NotificationPreferenceRepository,
	notifRepo repository.NotificationRepository,
) *NotificationService {
	return &NotificationService{
		prefRepo:  prefRepo,
		notifRepo: notifRepo,
	}
}

func (s *NotificationService) GetOrCreatePreferences(ctx context.Context, userID uint) (*models.NotificationPreference, error) {
	prefs, err := s.prefRepo.FindByUserID(ctx, userID)
	if err == nil {
		return prefs, nil
	}

	// Not found — create with defaults
	prefs = &models.NotificationPreference{
		UserID:                userID,
		Policy:                "daily",
		NotifyNewMessage:      true,
		NotifyEventStart:      false,
		NotifySubmissionGrade: true,
		NotifyNewAnnouncement: true,
	}
	if err := s.prefRepo.Create(ctx, prefs); err != nil {
		return nil, err
	}
	return prefs, nil
}

func (s *NotificationService) UpdatePreferences(ctx context.Context, prefs *models.NotificationPreference) error {
	validPolicies := map[string]bool{
		"immediately": true,
		"daily":       true,
		"weekly":      true,
		"never":       true,
	}
	if !validPolicies[prefs.Policy] {
		return errors.New("policy must be one of: immediately, daily, weekly, never")
	}
	return s.prefRepo.Update(ctx, prefs)
}

func (s *NotificationService) CreateNotification(ctx context.Context, notification *models.Notification) error {
	if notification.Title == "" {
		return errors.New("title is required")
	}
	now := time.Now()
	notification.SentAt = &now
	return s.notifRepo.Create(ctx, notification)
}

func (s *NotificationService) GetNotification(ctx context.Context, id uint) (*models.Notification, error) {
	return s.notifRepo.FindByID(ctx, id)
}

func (s *NotificationService) ListByUser(ctx context.Context, userID uint, params repository.PaginationParams) (*repository.PaginatedResult[models.Notification], error) {
	return s.notifRepo.ListByUserID(ctx, userID, params)
}

func (s *NotificationService) ListUnreadByUser(ctx context.Context, userID uint, params repository.PaginationParams) (*repository.PaginatedResult[models.Notification], error) {
	return s.notifRepo.ListUnreadByUserID(ctx, userID, params)
}

func (s *NotificationService) MarkAsRead(ctx context.Context, userID, notificationID uint) error {
	return s.notifRepo.MarkAsRead(ctx, userID, notificationID)
}

func (s *NotificationService) MarkAllAsRead(ctx context.Context, userID uint) error {
	return s.notifRepo.MarkAllAsRead(ctx, userID)
}
