package postgres

import (
	"context"

	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/repository"
	"gorm.io/gorm"
)

type discussionEntryRepo struct {
	db *gorm.DB
}

func NewDiscussionEntryRepository(db *gorm.DB) repository.DiscussionEntryRepository {
	return &discussionEntryRepo{db: db}
}

func (r *discussionEntryRepo) Create(ctx context.Context, entry *models.DiscussionEntry) error {
	return r.db.WithContext(ctx).Create(entry).Error
}

func (r *discussionEntryRepo) FindByID(ctx context.Context, id uint) (*models.DiscussionEntry, error) {
	var entry models.DiscussionEntry
	if err := r.db.WithContext(ctx).First(&entry, id).Error; err != nil {
		return nil, err
	}
	return &entry, nil
}

func (r *discussionEntryRepo) Update(ctx context.Context, entry *models.DiscussionEntry) error {
	return r.db.WithContext(ctx).Save(entry).Error
}

func (r *discussionEntryRepo) Delete(ctx context.Context, id uint) error {
	return r.db.WithContext(ctx).Model(&models.DiscussionEntry{}).Where("id = ?", id).Update("workflow_state", "deleted").Error
}

func (r *discussionEntryRepo) ListByTopicID(ctx context.Context, topicID uint, params repository.PaginationParams) (*repository.PaginatedResult[models.DiscussionEntry], error) {
	var entries []models.DiscussionEntry
	var count int64

	query := r.db.WithContext(ctx).Model(&models.DiscussionEntry{}).Where("discussion_topic_id = ? AND workflow_state != ?", topicID, "deleted")
	query.Count(&count)

	offset := (params.Page - 1) * params.PerPage
	if err := query.Offset(offset).Limit(params.PerPage).Order("created_at ASC").Find(&entries).Error; err != nil {
		return nil, err
	}

	return &repository.PaginatedResult[models.DiscussionEntry]{
		Items:      entries,
		TotalCount: count,
		Page:       params.Page,
		PerPage:    params.PerPage,
	}, nil
}

func (r *discussionEntryRepo) ListReplies(ctx context.Context, entryID uint, params repository.PaginationParams) (*repository.PaginatedResult[models.DiscussionEntry], error) {
	var entries []models.DiscussionEntry
	var count int64

	query := r.db.WithContext(ctx).Model(&models.DiscussionEntry{}).Where("parent_id = ? AND workflow_state != ?", entryID, "deleted")
	query.Count(&count)

	offset := (params.Page - 1) * params.PerPage
	if err := query.Offset(offset).Limit(params.PerPage).Order("created_at ASC").Find(&entries).Error; err != nil {
		return nil, err
	}

	return &repository.PaginatedResult[models.DiscussionEntry]{
		Items:      entries,
		TotalCount: count,
		Page:       params.Page,
		PerPage:    params.PerPage,
	}, nil
}

func (r *discussionEntryRepo) ListAllByTopicID(ctx context.Context, topicID uint) ([]models.DiscussionEntry, error) {
	var entries []models.DiscussionEntry
	if err := r.db.WithContext(ctx).Where("discussion_topic_id = ? AND workflow_state != ?", topicID, "deleted").Order("created_at ASC").Find(&entries).Error; err != nil {
		return nil, err
	}
	return entries, nil
}
