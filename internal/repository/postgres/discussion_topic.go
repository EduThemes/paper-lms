package postgres

import (
	"context"

	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/repository"
	"gorm.io/gorm"
)

type discussionTopicRepo struct {
	db *gorm.DB
}

func NewDiscussionTopicRepository(db *gorm.DB) repository.DiscussionTopicRepository {
	return &discussionTopicRepo{db: db}
}

func (r *discussionTopicRepo) Create(ctx context.Context, topic *models.DiscussionTopic) error {
	return r.db.WithContext(ctx).Create(topic).Error
}

func (r *discussionTopicRepo) FindByID(ctx context.Context, id, accountID uint) (*models.DiscussionTopic, error) {
	var topic models.DiscussionTopic
	q := r.db.WithContext(ctx)
	if accountID != 0 {
		// Scope through the parent course's account_id.
		q = q.Where("course_id IN (SELECT id FROM courses WHERE account_id = ?)", accountID)
	}
	if err := q.First(&topic, id).Error; err != nil {
		return nil, err
	}
	return &topic, nil
}

func (r *discussionTopicRepo) Update(ctx context.Context, topic *models.DiscussionTopic) error {
	return r.db.WithContext(ctx).Save(topic).Error
}

func (r *discussionTopicRepo) Delete(ctx context.Context, id uint) error {
	return r.db.WithContext(ctx).Model(&models.DiscussionTopic{}).Where("id = ?", id).Update("workflow_state", "deleted").Error
}

func (r *discussionTopicRepo) ListByCourseID(ctx context.Context, courseID, accountID uint, params repository.PaginationParams) (*repository.PaginatedResult[models.DiscussionTopic], error) {
	var topics []models.DiscussionTopic
	var count int64

	query := r.db.WithContext(ctx).Model(&models.DiscussionTopic{}).Where("course_id = ? AND workflow_state != ?", courseID, "deleted")
	if accountID != 0 {
		query = query.Where("course_id IN (SELECT id FROM courses WHERE account_id = ?)", accountID)
	}
	query.Count(&count)

	offset := (params.Page - 1) * params.PerPage
	if err := query.Offset(offset).Limit(params.PerPage).Order("pinned DESC, created_at DESC").Find(&topics).Error; err != nil {
		return nil, err
	}

	return &repository.PaginatedResult[models.DiscussionTopic]{
		Items:      topics,
		TotalCount: count,
		Page:       params.Page,
		PerPage:    params.PerPage,
	}, nil
}
