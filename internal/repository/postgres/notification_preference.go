package postgres

import (
	"context"

	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/repository"
	"gorm.io/gorm"
)

type notificationPreferenceRepo struct {
	db *gorm.DB
}

func NewNotificationPreferenceRepository(db *gorm.DB) repository.NotificationPreferenceRepository {
	return &notificationPreferenceRepo{db: db}
}

func (r *notificationPreferenceRepo) Create(ctx context.Context, prefs *models.NotificationPreference) error {
	return r.db.WithContext(ctx).Create(prefs).Error
}

func (r *notificationPreferenceRepo) FindByUserID(ctx context.Context, userID uint) (*models.NotificationPreference, error) {
	var prefs models.NotificationPreference
	if err := r.db.WithContext(ctx).Where("user_id = ?", userID).First(&prefs).Error; err != nil {
		return nil, err
	}
	return &prefs, nil
}

func (r *notificationPreferenceRepo) Update(ctx context.Context, prefs *models.NotificationPreference) error {
	return r.db.WithContext(ctx).Save(prefs).Error
}

func (r *notificationPreferenceRepo) Delete(ctx context.Context, userID uint) error {
	return r.db.WithContext(ctx).Where("user_id = ?", userID).Delete(&models.NotificationPreference{}).Error
}
