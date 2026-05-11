package postgres

import (
	"context"
	"time"

	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/repository"
	"gorm.io/gorm"
)

type discussionTopicParticipantRepo struct {
	db *gorm.DB
}

func NewDiscussionTopicParticipantRepository(db *gorm.DB) repository.DiscussionTopicParticipantRepository {
	return &discussionTopicParticipantRepo{db: db}
}

func (r *discussionTopicParticipantRepo) Upsert(ctx context.Context, p *models.DiscussionTopicParticipant) error {
	now := time.Now()
	return r.db.WithContext(ctx).Exec(
		"INSERT INTO discussion_topic_participants (discussion_topic_id, user_id, subscribed, forced_read_state, last_read_at, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?) ON CONFLICT (discussion_topic_id, user_id) DO UPDATE SET subscribed = ?, forced_read_state = ?, last_read_at = ?, updated_at = ?",
		p.DiscussionTopicID, p.UserID, p.Subscribed, p.ForcedReadState, p.LastReadAt, now, now,
		p.Subscribed, p.ForcedReadState, p.LastReadAt, now,
	).Error
}

func (r *discussionTopicParticipantRepo) FindByTopicAndUser(ctx context.Context, topicID, userID uint) (*models.DiscussionTopicParticipant, error) {
	var p models.DiscussionTopicParticipant
	if err := r.db.WithContext(ctx).
		Where("discussion_topic_id = ? AND user_id = ?", topicID, userID).
		First(&p).Error; err != nil {
		return nil, err
	}
	return &p, nil
}

func (r *discussionTopicParticipantRepo) ListByTopicID(ctx context.Context, topicID uint) ([]models.DiscussionTopicParticipant, error) {
	var participants []models.DiscussionTopicParticipant
	if err := r.db.WithContext(ctx).
		Where("discussion_topic_id = ?", topicID).
		Order("created_at ASC").
		Find(&participants).Error; err != nil {
		return nil, err
	}
	return participants, nil
}

func (r *discussionTopicParticipantRepo) UpdateSubscription(ctx context.Context, topicID, userID uint, subscribed bool) error {
	now := time.Now()
	return r.db.WithContext(ctx).Exec(
		"INSERT INTO discussion_topic_participants (discussion_topic_id, user_id, subscribed, created_at, updated_at) VALUES (?, ?, ?, ?, ?) ON CONFLICT (discussion_topic_id, user_id) DO UPDATE SET subscribed = ?, updated_at = ?",
		topicID, userID, subscribed, now, now, subscribed, now,
	).Error
}
