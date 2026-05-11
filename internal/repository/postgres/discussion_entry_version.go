package postgres

import (
	"context"

	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/repository"
	"gorm.io/gorm"
)

type discussionEntryVersionRepo struct {
	db *gorm.DB
}

func NewDiscussionEntryVersionRepository(db *gorm.DB) repository.DiscussionEntryVersionRepository {
	return &discussionEntryVersionRepo{db: db}
}

func (r *discussionEntryVersionRepo) Create(ctx context.Context, v *models.DiscussionEntryVersion) error {
	return r.db.WithContext(ctx).Create(v).Error
}

func (r *discussionEntryVersionRepo) ListByEntryID(ctx context.Context, entryID uint) ([]models.DiscussionEntryVersion, error) {
	var versions []models.DiscussionEntryVersion
	if err := r.db.WithContext(ctx).
		Where("discussion_entry_id = ?", entryID).
		Order("version ASC").
		Find(&versions).Error; err != nil {
		return nil, err
	}
	return versions, nil
}

func (r *discussionEntryVersionRepo) CountByEntryID(ctx context.Context, entryID uint) (int64, error) {
	var count int64
	err := r.db.WithContext(ctx).
		Model(&models.DiscussionEntryVersion{}).
		Where("discussion_entry_id = ?", entryID).
		Count(&count).Error
	return count, err
}
