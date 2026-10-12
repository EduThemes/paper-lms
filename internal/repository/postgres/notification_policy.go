package postgres

import (
	"context"
	"time"

	"github.com/EduThemes/paper-lms/internal/domain/models"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// NotificationPolicyRepository stores the per-category x per-channel
// notification frequencies (G091). Every method is keyed by the owning
// user, so one user can never read or write another's grid.
type NotificationPolicyRepository struct {
	db *gorm.DB
}

// NewNotificationPolicyRepository creates a postgres-backed policy store.
func NewNotificationPolicyRepository(db *gorm.DB) *NotificationPolicyRepository {
	return &NotificationPolicyRepository{db: db}
}

// ListByUser returns every explicit policy row of the user.
func (r *NotificationPolicyRepository) ListByUser(ctx context.Context, userID uint) ([]models.NotificationPolicy, error) {
	var rows []models.NotificationPolicy
	err := r.db.WithContext(ctx).
		Where("user_id = ?", userID).
		Order("communication_channel_id ASC, category ASC").
		Find(&rows).Error
	return rows, err
}

// Upsert sets the frequency of one (user, channel, category) cell.
func (r *NotificationPolicyRepository) Upsert(ctx context.Context, p *models.NotificationPolicy) error {
	now := time.Now()
	p.CreatedAt = now
	p.UpdatedAt = now
	return r.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "user_id"}, {Name: "communication_channel_id"}, {Name: "category"}},
		DoUpdates: clause.Assignments(map[string]any{"frequency": p.Frequency, "updated_at": now}),
	}).Create(p).Error
}

// Delete removes one explicit cell so it falls back to the defaults.
func (r *NotificationPolicyRepository) Delete(ctx context.Context, userID, channelID uint, category string) error {
	return r.db.WithContext(ctx).
		Where("user_id = ? AND communication_channel_id = ? AND category = ?", userID, channelID, category).
		Delete(&models.NotificationPolicy{}).Error
}

// DeleteByChannel drops every policy of a channel (the channel was removed).
func (r *NotificationPolicyRepository) DeleteByChannel(ctx context.Context, userID, channelID uint) error {
	return r.db.WithContext(ctx).
		Where("user_id = ? AND communication_channel_id = ?", userID, channelID).
		Delete(&models.NotificationPolicy{}).Error
}
