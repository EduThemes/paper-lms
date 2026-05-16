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

func (r *discussionEntryRepo) FindByID(ctx context.Context, id, accountID uint) (*models.DiscussionEntry, error) {
	var entry models.DiscussionEntry
	q := r.db.WithContext(ctx)
	if accountID != 0 {
		// Scope through entry → topic → course → account.
		q = q.Where(`discussion_topic_id IN (
			SELECT id FROM discussion_topics WHERE course_id IN (
				SELECT id FROM courses WHERE account_id = ?
			)
		)`, accountID)
	}
	if err := q.First(&entry, id).Error; err != nil {
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

func (r *discussionEntryRepo) ListByTopicID(ctx context.Context, topicID, accountID uint, params repository.PaginationParams) (*repository.PaginatedResult[models.DiscussionEntry], error) {
	var entries []models.DiscussionEntry
	var count int64

	query := r.db.WithContext(ctx).Model(&models.DiscussionEntry{}).Where("discussion_topic_id = ? AND workflow_state != ?", topicID, "deleted")
	if accountID != 0 {
		query = query.Where(`discussion_topic_id IN (
			SELECT id FROM discussion_topics WHERE course_id IN (
				SELECT id FROM courses WHERE account_id = ?
			)
		)`, accountID)
	}
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

func (r *discussionEntryRepo) ListReplies(ctx context.Context, entryID, accountID uint, params repository.PaginationParams) (*repository.PaginatedResult[models.DiscussionEntry], error) {
	var entries []models.DiscussionEntry
	var count int64

	query := r.db.WithContext(ctx).Model(&models.DiscussionEntry{}).Where("parent_id = ? AND workflow_state != ?", entryID, "deleted")
	if accountID != 0 {
		query = query.Where(`discussion_topic_id IN (
			SELECT id FROM discussion_topics WHERE course_id IN (
				SELECT id FROM courses WHERE account_id = ?
			)
		)`, accountID)
	}
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

func (r *discussionEntryRepo) ListAllByTopicID(ctx context.Context, topicID, accountID uint) ([]models.DiscussionEntry, error) {
	var entries []models.DiscussionEntry
	q := r.db.WithContext(ctx).Where("discussion_topic_id = ? AND workflow_state != ?", topicID, "deleted")
	if accountID != 0 {
		q = q.Where(`discussion_topic_id IN (
			SELECT id FROM discussion_topics WHERE course_id IN (
				SELECT id FROM courses WHERE account_id = ?
			)
		)`, accountID)
	}
	if err := q.Order("created_at ASC").Find(&entries).Error; err != nil {
		return nil, err
	}
	return entries, nil
}
