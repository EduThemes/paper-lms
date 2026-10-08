package postgres

import (
	"context"
	"time"

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

	// A group discussion's per-group child topics are internal copies of
	// their root; course lists (index, modules, copy, export) show only
	// the root.
	query := r.db.WithContext(ctx).Model(&models.DiscussionTopic{}).Where("course_id = ? AND workflow_state != ? AND root_topic_id IS NULL", courseID, "deleted")
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

// ListStudentVisibleByCourseID is ListByCourseID restricted to the topics
// a student may open (service.DiscussionTopicVisibleToStudents):
// unpublished topics are left out, and so is any topic whose
// delayed_post_at is still in the future; a post_delayed topic shows once
// its delayed_post_at has passed.
func (r *discussionTopicRepo) ListStudentVisibleByCourseID(ctx context.Context, courseID, accountID uint, now time.Time, params repository.PaginationParams) (*repository.PaginatedResult[models.DiscussionTopic], error) {
	var topics []models.DiscussionTopic
	var count int64

	query := r.db.WithContext(ctx).Model(&models.DiscussionTopic{}).
		Where("course_id = ? AND workflow_state NOT IN ? AND root_topic_id IS NULL", courseID, []string{"deleted", "unpublished"}).
		Where("(workflow_state <> ? OR delayed_post_at IS NOT NULL)", "post_delayed").
		Where("(delayed_post_at IS NULL OR delayed_post_at <= ?)", now)
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
