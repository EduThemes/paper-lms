package postgres

import (
	"context"

	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/repository"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// --- DiscussionCheckpointRepository ---

type discussionCheckpointRepo struct {
	db *gorm.DB
}

func NewDiscussionCheckpointRepository(db *gorm.DB) repository.DiscussionCheckpointRepository {
	return &discussionCheckpointRepo{db: db}
}

func (r *discussionCheckpointRepo) Create(ctx context.Context, c *models.DiscussionCheckpoint) error {
	return r.db.WithContext(ctx).Create(c).Error
}

func (r *discussionCheckpointRepo) FindByID(ctx context.Context, id uint) (*models.DiscussionCheckpoint, error) {
	var cp models.DiscussionCheckpoint
	if err := r.db.WithContext(ctx).First(&cp, id).Error; err != nil {
		return nil, err
	}
	return &cp, nil
}

func (r *discussionCheckpointRepo) Update(ctx context.Context, c *models.DiscussionCheckpoint) error {
	return r.db.WithContext(ctx).Save(c).Error
}

func (r *discussionCheckpointRepo) Delete(ctx context.Context, id uint) error {
	return r.db.WithContext(ctx).
		Model(&models.DiscussionCheckpoint{}).
		Where("id = ?", id).
		Update("workflow_state", "deleted").Error
}

func (r *discussionCheckpointRepo) ListByTopicID(ctx context.Context, topicID uint) ([]models.DiscussionCheckpoint, error) {
	var checkpoints []models.DiscussionCheckpoint
	if err := r.db.WithContext(ctx).
		Where("discussion_topic_id = ? AND workflow_state != ?", topicID, "deleted").
		Order("checkpoint_type ASC, due_at ASC").
		Find(&checkpoints).Error; err != nil {
		return nil, err
	}
	return checkpoints, nil
}

func (r *discussionCheckpointRepo) DeleteByTopicID(ctx context.Context, topicID uint) error {
	return r.db.WithContext(ctx).
		Model(&models.DiscussionCheckpoint{}).
		Where("discussion_topic_id = ?", topicID).
		Update("workflow_state", "deleted").Error
}

// --- DiscussionCheckpointSubmissionRepository ---

type discussionCheckpointSubmissionRepo struct {
	db *gorm.DB
}

func NewDiscussionCheckpointSubmissionRepository(db *gorm.DB) repository.DiscussionCheckpointSubmissionRepository {
	return &discussionCheckpointSubmissionRepo{db: db}
}

func (r *discussionCheckpointSubmissionRepo) UpsertSubmission(ctx context.Context, s *models.DiscussionCheckpointSubmission) error {
	return r.db.WithContext(ctx).
		Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "discussion_checkpoint_id"}, {Name: "user_id"}},
			DoUpdates: clause.AssignmentColumns([]string{"reply_count", "status", "completed_at", "updated_at"}),
		}).
		Create(s).Error
}

func (r *discussionCheckpointSubmissionRepo) FindByCheckpointAndUser(ctx context.Context, checkpointID, userID, accountID uint) (*models.DiscussionCheckpointSubmission, error) {
	var sub models.DiscussionCheckpointSubmission
	q := r.db.WithContext(ctx).Where("discussion_checkpoint_id = ? AND user_id = ?", checkpointID, userID)
	if accountID != 0 {
		// Scope through checkpoint->topic->course (deep 3-level subquery).
		q = q.Where("discussion_checkpoint_id IN (SELECT id FROM discussion_checkpoints WHERE discussion_topic_id IN (SELECT id FROM discussion_topics WHERE course_id IN (SELECT id FROM courses WHERE account_id = ?)))", accountID)
	}
	if err := q.First(&sub).Error; err != nil {
		return nil, err
	}
	return &sub, nil
}

func (r *discussionCheckpointSubmissionRepo) ListByCheckpoint(ctx context.Context, checkpointID, accountID uint) ([]models.DiscussionCheckpointSubmission, error) {
	var subs []models.DiscussionCheckpointSubmission
	q := r.db.WithContext(ctx).Where("discussion_checkpoint_id = ?", checkpointID)
	if accountID != 0 {
		q = q.Where("discussion_checkpoint_id IN (SELECT id FROM discussion_checkpoints WHERE discussion_topic_id IN (SELECT id FROM discussion_topics WHERE course_id IN (SELECT id FROM courses WHERE account_id = ?)))", accountID)
	}
	if err := q.Find(&subs).Error; err != nil {
		return nil, err
	}
	return subs, nil
}

func (r *discussionCheckpointSubmissionRepo) ListByUserAndTopic(ctx context.Context, topicID, userID, accountID uint) ([]models.DiscussionCheckpointSubmission, error) {
	var subs []models.DiscussionCheckpointSubmission
	q := r.db.WithContext(ctx).
		Joins("JOIN discussion_checkpoints dc ON dc.id = discussion_checkpoint_submissions.discussion_checkpoint_id").
		Where("dc.discussion_topic_id = ? AND discussion_checkpoint_submissions.user_id = ?", topicID, userID)
	if accountID != 0 {
		q = q.Where("dc.discussion_topic_id IN (SELECT id FROM discussion_topics WHERE course_id IN (SELECT id FROM courses WHERE account_id = ?))", accountID)
	}
	if err := q.Find(&subs).Error; err != nil {
		return nil, err
	}
	return subs, nil
}
