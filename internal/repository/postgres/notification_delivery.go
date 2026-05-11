package postgres

import (
	"context"
	"time"

	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/repository"
	"gorm.io/gorm"
)

// NotificationDeliveryRepository defines the data access methods for notification deliveries.
type NotificationDeliveryRepository interface {
	Create(ctx context.Context, delivery *models.NotificationDelivery) error
	FindByID(ctx context.Context, id uint) (*models.NotificationDelivery, error)
	Update(ctx context.Context, delivery *models.NotificationDelivery) error
	Delete(ctx context.Context, id uint) error
	ListByUserID(ctx context.Context, userID uint, params repository.PaginationParams) (*repository.PaginatedResult[models.NotificationDelivery], error)
	ListPending(ctx context.Context, now time.Time) ([]models.NotificationDelivery, error)
	ListPendingByDigestType(ctx context.Context, digestType string, now time.Time) ([]models.NotificationDelivery, error)
	UpdateStatus(ctx context.Context, id uint, status string) error
	IncrementRetry(ctx context.Context, id uint, lastError string) error
	ListByNotificationID(ctx context.Context, notificationID uint) ([]models.NotificationDelivery, error)
	CountByStatus(ctx context.Context) (map[string]int64, error)
	ListFailed(ctx context.Context) ([]models.NotificationDelivery, error)
	ListByUserIDAndStatus(ctx context.Context, userID uint, status string, params repository.PaginationParams) (*repository.PaginatedResult[models.NotificationDelivery], error)
}

type notificationDeliveryRepo struct {
	db *gorm.DB
}

// NewNotificationDeliveryRepository creates a new NotificationDeliveryRepository backed by PostgreSQL.
func NewNotificationDeliveryRepository(db *gorm.DB) NotificationDeliveryRepository {
	return &notificationDeliveryRepo{db: db}
}

func (r *notificationDeliveryRepo) Create(ctx context.Context, delivery *models.NotificationDelivery) error {
	return r.db.WithContext(ctx).Create(delivery).Error
}

func (r *notificationDeliveryRepo) FindByID(ctx context.Context, id uint) (*models.NotificationDelivery, error) {
	var delivery models.NotificationDelivery
	if err := r.db.WithContext(ctx).First(&delivery, id).Error; err != nil {
		return nil, err
	}
	return &delivery, nil
}

func (r *notificationDeliveryRepo) Update(ctx context.Context, delivery *models.NotificationDelivery) error {
	return r.db.WithContext(ctx).Save(delivery).Error
}

func (r *notificationDeliveryRepo) Delete(ctx context.Context, id uint) error {
	return r.db.WithContext(ctx).Delete(&models.NotificationDelivery{}, id).Error
}

func (r *notificationDeliveryRepo) ListByUserID(ctx context.Context, userID uint, params repository.PaginationParams) (*repository.PaginatedResult[models.NotificationDelivery], error) {
	var deliveries []models.NotificationDelivery
	var count int64

	query := r.db.WithContext(ctx).Model(&models.NotificationDelivery{}).Where("user_id = ?", userID)
	query.Count(&count)

	offset := (params.Page - 1) * params.PerPage
	if err := query.Offset(offset).Limit(params.PerPage).Order("created_at DESC").Find(&deliveries).Error; err != nil {
		return nil, err
	}

	return &repository.PaginatedResult[models.NotificationDelivery]{
		Items:      deliveries,
		TotalCount: count,
		Page:       params.Page,
		PerPage:    params.PerPage,
	}, nil
}

func (r *notificationDeliveryRepo) ListByUserIDAndStatus(ctx context.Context, userID uint, status string, params repository.PaginationParams) (*repository.PaginatedResult[models.NotificationDelivery], error) {
	var deliveries []models.NotificationDelivery
	var count int64

	query := r.db.WithContext(ctx).Model(&models.NotificationDelivery{}).
		Where("user_id = ? AND delivery_status = ?", userID, status)
	query.Count(&count)

	offset := (params.Page - 1) * params.PerPage
	if err := query.Offset(offset).Limit(params.PerPage).Order("created_at DESC").Find(&deliveries).Error; err != nil {
		return nil, err
	}

	return &repository.PaginatedResult[models.NotificationDelivery]{
		Items:      deliveries,
		TotalCount: count,
		Page:       params.Page,
		PerPage:    params.PerPage,
	}, nil
}

// ListPending fetches all deliveries where scheduled_for <= now AND status is pending or queued,
// ordered by scheduled_for ascending (oldest first).
func (r *notificationDeliveryRepo) ListPending(ctx context.Context, now time.Time) ([]models.NotificationDelivery, error) {
	var deliveries []models.NotificationDelivery
	if err := r.db.WithContext(ctx).
		Where("scheduled_for <= ? AND delivery_status IN ?", now, []string{"pending", "queued"}).
		Order("scheduled_for ASC").
		Find(&deliveries).Error; err != nil {
		return nil, err
	}
	return deliveries, nil
}

// ListPendingByDigestType fetches pending deliveries for a specific digest type.
func (r *notificationDeliveryRepo) ListPendingByDigestType(ctx context.Context, digestType string, now time.Time) ([]models.NotificationDelivery, error) {
	var deliveries []models.NotificationDelivery
	if err := r.db.WithContext(ctx).
		Where("digest_type = ? AND scheduled_for <= ? AND delivery_status IN ?", digestType, now, []string{"pending", "queued"}).
		Order("scheduled_for ASC").
		Find(&deliveries).Error; err != nil {
		return nil, err
	}
	return deliveries, nil
}

func (r *notificationDeliveryRepo) UpdateStatus(ctx context.Context, id uint, status string) error {
	return r.db.WithContext(ctx).Model(&models.NotificationDelivery{}).
		Where("id = ?", id).
		Update("delivery_status", status).Error
}

func (r *notificationDeliveryRepo) IncrementRetry(ctx context.Context, id uint, lastError string) error {
	return r.db.WithContext(ctx).Model(&models.NotificationDelivery{}).
		Where("id = ?", id).
		Updates(map[string]interface{}{
			"retry_count":     gorm.Expr("retry_count + 1"),
			"last_error":      lastError,
			"delivery_status": "failed",
		}).Error
}

func (r *notificationDeliveryRepo) ListByNotificationID(ctx context.Context, notificationID uint) ([]models.NotificationDelivery, error) {
	var deliveries []models.NotificationDelivery
	if err := r.db.WithContext(ctx).
		Where("notification_id = ?", notificationID).
		Order("created_at DESC").
		Find(&deliveries).Error; err != nil {
		return nil, err
	}
	return deliveries, nil
}

// CountByStatus returns a map of delivery_status -> count for the admin dashboard.
func (r *notificationDeliveryRepo) CountByStatus(ctx context.Context) (map[string]int64, error) {
	type statusCount struct {
		DeliveryStatus string
		Count          int64
	}
	var results []statusCount

	if err := r.db.WithContext(ctx).Model(&models.NotificationDelivery{}).
		Select("delivery_status, COUNT(*) as count").
		Group("delivery_status").
		Find(&results).Error; err != nil {
		return nil, err
	}

	counts := make(map[string]int64)
	for _, r := range results {
		counts[r.DeliveryStatus] = r.Count
	}
	return counts, nil
}

// ListFailed returns deliveries where status = failed AND retry_count < max_retries.
func (r *notificationDeliveryRepo) ListFailed(ctx context.Context) ([]models.NotificationDelivery, error) {
	var deliveries []models.NotificationDelivery
	if err := r.db.WithContext(ctx).
		Where("delivery_status = ? AND retry_count < max_retries", "failed").
		Order("created_at ASC").
		Find(&deliveries).Error; err != nil {
		return nil, err
	}
	return deliveries, nil
}
