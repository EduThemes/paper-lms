package postgres

import (
	"context"
	"time"

	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/repository"
	"gorm.io/gorm"
)

type discussionEntryParticipantRepo struct {
	db *gorm.DB
}

func NewDiscussionEntryParticipantRepository(db *gorm.DB) repository.DiscussionEntryParticipantRepository {
	return &discussionEntryParticipantRepo{db: db}
}

func (r *discussionEntryParticipantRepo) Create(ctx context.Context, p *models.DiscussionEntryParticipant) error {
	return r.db.WithContext(ctx).Create(p).Error
}

func (r *discussionEntryParticipantRepo) FindByEntryAndUser(ctx context.Context, entryID, userID, accountID uint) (*models.DiscussionEntryParticipant, error) {
	var p models.DiscussionEntryParticipant
	q := r.db.WithContext(ctx).
		Where("discussion_entry_id = ? AND user_id = ?", entryID, userID)
	if accountID != 0 {
		// 3-level deep: participant → entry → topic → course → account.
		q = q.Where(`discussion_entry_id IN (
			SELECT id FROM discussion_entries WHERE discussion_topic_id IN (
				SELECT id FROM discussion_topics WHERE course_id IN (
					SELECT id FROM courses WHERE account_id = ?
				)
			)
		)`, accountID)
	}
	if err := q.First(&p).Error; err != nil {
		return nil, err
	}
	return &p, nil
}

func (r *discussionEntryParticipantRepo) MarkAsRead(ctx context.Context, entryID, userID uint) error {
	now := time.Now()
	result := r.db.WithContext(ctx).
		Where("discussion_entry_id = ? AND user_id = ?", entryID, userID).
		First(&models.DiscussionEntryParticipant{})

	if result.Error != nil {
		// Record does not exist, create it
		return r.db.WithContext(ctx).Create(&models.DiscussionEntryParticipant{
			DiscussionEntryID: entryID,
			UserID:            userID,
			ReadAt:            &now,
		}).Error
	}

	// Record exists, update read_at
	return r.db.WithContext(ctx).
		Model(&models.DiscussionEntryParticipant{}).
		Where("discussion_entry_id = ? AND user_id = ?", entryID, userID).
		Update("read_at", now).Error
}

func (r *discussionEntryParticipantRepo) MarkTopicAsRead(ctx context.Context, topicID, userID uint) error {
	now := time.Now()

	// Get all entry IDs for this topic
	var entryIDs []uint
	if err := r.db.WithContext(ctx).
		Model(&models.DiscussionEntry{}).
		Where("discussion_topic_id = ? AND workflow_state != ?", topicID, "deleted").
		Pluck("id", &entryIDs).Error; err != nil {
		return err
	}

	if len(entryIDs) == 0 {
		return nil
	}

	// For each entry, upsert a participant record with read_at set
	for _, entryID := range entryIDs {
		err := r.db.WithContext(ctx).Exec(
			"INSERT INTO discussion_entry_participants (discussion_entry_id, user_id, read_at, created_at, updated_at) VALUES (?, ?, ?, ?, ?) ON CONFLICT (discussion_entry_id, user_id) DO UPDATE SET read_at = ?, updated_at = ?",
			entryID, userID, now, now, now, now, now,
		).Error
		if err != nil {
			return err
		}
	}

	return nil
}

func (r *discussionEntryParticipantRepo) CountUnreadByTopic(ctx context.Context, topicID, userID uint) (int64, error) {
	var count int64

	// Count entries in topic that either have no participant record for this user
	// or have a participant record with read_at IS NULL
	err := r.db.WithContext(ctx).
		Model(&models.DiscussionEntry{}).
		Where("discussion_topic_id = ? AND workflow_state != ?", topicID, "deleted").
		Where("id NOT IN (?)",
			r.db.Model(&models.DiscussionEntryParticipant{}).
				Select("discussion_entry_id").
				Where("user_id = ? AND read_at IS NOT NULL", userID),
		).
		Count(&count).Error

	return count, err
}

func (r *discussionEntryParticipantRepo) ListUnreadByTopic(ctx context.Context, topicID, userID uint) ([]uint, error) {
	var entryIDs []uint

	err := r.db.WithContext(ctx).
		Model(&models.DiscussionEntry{}).
		Where("discussion_topic_id = ? AND workflow_state != ?", topicID, "deleted").
		Where("id NOT IN (?)",
			r.db.Model(&models.DiscussionEntryParticipant{}).
				Select("discussion_entry_id").
				Where("user_id = ? AND read_at IS NOT NULL", userID),
		).
		Pluck("id", &entryIDs).Error

	return entryIDs, err
}
