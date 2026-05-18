package postgres

import (
	"context"

	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/repository"
	"gorm.io/gorm"
)

type notificationRepo struct {
	db *gorm.DB
}

func NewNotificationRepository(db *gorm.DB) repository.NotificationRepository {
	return &notificationRepo{db: db}
}

// notificationTenantFilter scopes a notifications query to a tenant
// via the owning user's account_id. Notifications carry no direct
// account_id column; user_id → users.account_id is the join. The
// subquery shape matches the parent-table pattern used elsewhere
// (e.g. submissions → assignments → courses). accountID==0 disables.
const notificationTenantFilter = `user_id IN (SELECT id FROM users WHERE account_id = ?)`

func (r *notificationRepo) Create(ctx context.Context, notification *models.Notification) error {
	return r.db.WithContext(ctx).Create(notification).Error
}

func (r *notificationRepo) FindByID(ctx context.Context, id, accountID uint) (*models.Notification, error) {
	var notification models.Notification
	q := r.db.WithContext(ctx)
	if accountID != 0 {
		q = q.Where(notificationTenantFilter, accountID)
	}
	if err := q.First(&notification, id).Error; err != nil {
		return nil, err
	}
	return &notification, nil
}

func (r *notificationRepo) Update(ctx context.Context, notification *models.Notification) error {
	return r.db.WithContext(ctx).Save(notification).Error
}

func (r *notificationRepo) Delete(ctx context.Context, id uint) error {
	return r.db.WithContext(ctx).Delete(&models.Notification{}, id).Error
}

func (r *notificationRepo) ListByUserID(ctx context.Context, userID, accountID uint, params repository.PaginationParams) (*repository.PaginatedResult[models.Notification], error) {
	var notifications []models.Notification
	var count int64

	query := r.db.WithContext(ctx).Model(&models.Notification{}).Where("user_id = ?", userID)
	if accountID != 0 {
		query = query.Where(notificationTenantFilter, accountID)
	}
	query.Count(&count)

	offset := (params.Page - 1) * params.PerPage
	if err := query.Offset(offset).Limit(params.PerPage).Order("created_at DESC").Find(&notifications).Error; err != nil {
		return nil, err
	}

	return &repository.PaginatedResult[models.Notification]{
		Items:      notifications,
		TotalCount: count,
		Page:       params.Page,
		PerPage:    params.PerPage,
	}, nil
}

func (r *notificationRepo) MarkAsRead(ctx context.Context, userID, notificationID, accountID uint) error {
	q := r.db.WithContext(ctx).Model(&models.Notification{}).
		Where("id = ? AND user_id = ?", notificationID, userID)
	if accountID != 0 {
		q = q.Where(notificationTenantFilter, accountID)
	}
	return q.Update("is_read", true).Error
}

func (r *notificationRepo) MarkAllAsRead(ctx context.Context, userID, accountID uint) error {
	q := r.db.WithContext(ctx).Model(&models.Notification{}).
		Where("user_id = ? AND is_read = ?", userID, false)
	if accountID != 0 {
		q = q.Where(notificationTenantFilter, accountID)
	}
	return q.Update("is_read", true).Error
}

func (r *notificationRepo) ListUnreadByUserID(ctx context.Context, userID, accountID uint, params repository.PaginationParams) (*repository.PaginatedResult[models.Notification], error) {
	var notifications []models.Notification
	var count int64

	query := r.db.WithContext(ctx).Model(&models.Notification{}).Where("user_id = ? AND is_read = ?", userID, false)
	if accountID != 0 {
		query = query.Where(notificationTenantFilter, accountID)
	}
	query.Count(&count)

	offset := (params.Page - 1) * params.PerPage
	if err := query.Offset(offset).Limit(params.PerPage).Order("created_at DESC").Find(&notifications).Error; err != nil {
		return nil, err
	}

	return &repository.PaginatedResult[models.Notification]{
		Items:      notifications,
		TotalCount: count,
		Page:       params.Page,
		PerPage:    params.PerPage,
	}, nil
}
