package postgres

import (
	"context"
	"time"

	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/repository"
	"gorm.io/gorm"
)

// AnnouncementReadReceiptRepository defines the persistence methods for read receipts.
type AnnouncementReadReceiptRepository interface {
	Create(ctx context.Context, receipt *models.AnnouncementReadReceipt) error
	FindByAnnouncementAndUser(ctx context.Context, announcementID, userID uint) (*models.AnnouncementReadReceipt, error)
	FindByAnnouncementIDsAndUser(ctx context.Context, announcementIDs []uint, userID uint) ([]models.AnnouncementReadReceipt, error)
	ListByAnnouncementID(ctx context.Context, announcementID uint, params repository.PaginationParams) (*repository.PaginatedResult[models.AnnouncementReadReceipt], error)
	CountReadByAnnouncementID(ctx context.Context, announcementID uint) (int64, error)
	CountAcknowledgedByAnnouncementID(ctx context.Context, announcementID uint) (int64, error)
	MarkRead(ctx context.Context, announcementID, userID uint) error
	MarkAcknowledged(ctx context.Context, announcementID, userID uint) error
	BulkMarkRead(ctx context.Context, announcementID uint, userIDs []uint) error
}

type announcementReadReceiptRepo struct {
	db *gorm.DB
}

// NewAnnouncementReadReceiptRepository creates a new postgres-backed read receipt repository.
func NewAnnouncementReadReceiptRepository(db *gorm.DB) AnnouncementReadReceiptRepository {
	return &announcementReadReceiptRepo{db: db}
}

func (r *announcementReadReceiptRepo) Create(ctx context.Context, receipt *models.AnnouncementReadReceipt) error {
	return r.db.WithContext(ctx).Create(receipt).Error
}

func (r *announcementReadReceiptRepo) FindByAnnouncementAndUser(ctx context.Context, announcementID, userID uint) (*models.AnnouncementReadReceipt, error) {
	var receipt models.AnnouncementReadReceipt
	if err := r.db.WithContext(ctx).
		Where("announcement_id = ? AND user_id = ?", announcementID, userID).
		First(&receipt).Error; err != nil {
		return nil, err
	}
	return &receipt, nil
}

func (r *announcementReadReceiptRepo) FindByAnnouncementIDsAndUser(ctx context.Context, announcementIDs []uint, userID uint) ([]models.AnnouncementReadReceipt, error) {
	if len(announcementIDs) == 0 {
		return nil, nil
	}
	var receipts []models.AnnouncementReadReceipt
	if err := r.db.WithContext(ctx).
		Where("announcement_id IN ? AND user_id = ?", announcementIDs, userID).
		Find(&receipts).Error; err != nil {
		return nil, err
	}
	return receipts, nil
}

func (r *announcementReadReceiptRepo) ListByAnnouncementID(ctx context.Context, announcementID uint, params repository.PaginationParams) (*repository.PaginatedResult[models.AnnouncementReadReceipt], error) {
	var receipts []models.AnnouncementReadReceipt
	var count int64

	query := r.db.WithContext(ctx).Model(&models.AnnouncementReadReceipt{}).
		Where("announcement_id = ?", announcementID)
	query.Count(&count)

	offset := (params.Page - 1) * params.PerPage
	if err := query.Offset(offset).Limit(params.PerPage).Order("read_at DESC").Find(&receipts).Error; err != nil {
		return nil, err
	}

	return &repository.PaginatedResult[models.AnnouncementReadReceipt]{
		Items:      receipts,
		TotalCount: count,
		Page:       params.Page,
		PerPage:    params.PerPage,
	}, nil
}

func (r *announcementReadReceiptRepo) CountReadByAnnouncementID(ctx context.Context, announcementID uint) (int64, error) {
	var count int64
	err := r.db.WithContext(ctx).Model(&models.AnnouncementReadReceipt{}).
		Where("announcement_id = ?", announcementID).
		Count(&count).Error
	return count, err
}

func (r *announcementReadReceiptRepo) CountAcknowledgedByAnnouncementID(ctx context.Context, announcementID uint) (int64, error) {
	var count int64
	err := r.db.WithContext(ctx).Model(&models.AnnouncementReadReceipt{}).
		Where("announcement_id = ? AND acknowledged = ?", announcementID, true).
		Count(&count).Error
	return count, err
}

func (r *announcementReadReceiptRepo) MarkRead(ctx context.Context, announcementID, userID uint) error {
	now := time.Now()

	// Check if receipt already exists
	var existing models.AnnouncementReadReceipt
	result := r.db.WithContext(ctx).
		Where("announcement_id = ? AND user_id = ?", announcementID, userID).
		First(&existing)

	if result.Error != nil {
		// Record does not exist, create it
		return r.db.WithContext(ctx).Create(&models.AnnouncementReadReceipt{
			AnnouncementID: announcementID,
			UserID:         userID,
			ReadAt:         now,
		}).Error
	}

	// Already read, no-op
	return nil
}

func (r *announcementReadReceiptRepo) MarkAcknowledged(ctx context.Context, announcementID, userID uint) error {
	now := time.Now()

	// Check if receipt already exists
	var existing models.AnnouncementReadReceipt
	result := r.db.WithContext(ctx).
		Where("announcement_id = ? AND user_id = ?", announcementID, userID).
		First(&existing)

	if result.Error != nil {
		// Record does not exist, create with both read and acknowledged
		return r.db.WithContext(ctx).Create(&models.AnnouncementReadReceipt{
			AnnouncementID: announcementID,
			UserID:         userID,
			ReadAt:         now,
			Acknowledged:   true,
			AcknowledgedAt: &now,
		}).Error
	}

	// Record exists, update acknowledged fields
	return r.db.WithContext(ctx).
		Model(&models.AnnouncementReadReceipt{}).
		Where("announcement_id = ? AND user_id = ?", announcementID, userID).
		Updates(map[string]interface{}{
			"acknowledged":    true,
			"acknowledged_at": now,
		}).Error
}

func (r *announcementReadReceiptRepo) BulkMarkRead(ctx context.Context, announcementID uint, userIDs []uint) error {
	if len(userIDs) == 0 {
		return nil
	}

	now := time.Now()
	for _, userID := range userIDs {
		if err := r.MarkRead(ctx, announcementID, userID); err != nil {
			_ = now // suppress unused if MarkRead handles time internally
			return err
		}
	}
	return nil
}
