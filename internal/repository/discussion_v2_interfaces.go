package repository

import (
	"context"

	"github.com/EduThemes/paper-lms/internal/domain/models"
)

type DiscussionEntryParticipantRepository interface {
	Create(ctx context.Context, p *models.DiscussionEntryParticipant) error
	// FindByEntryAndUser — 13.1.D: tenant-scoped via the 3-level deep
	// entry → topic → course chain. accountID==0 means "no scope" and
	// is permitted only from internal callers that have already
	// validated tenant ownership upstream.
	FindByEntryAndUser(ctx context.Context, entryID, userID, accountID uint) (*models.DiscussionEntryParticipant, error)
	MarkAsRead(ctx context.Context, entryID, userID uint) error
	MarkTopicAsRead(ctx context.Context, topicID, userID uint) error // marks all entries in topic
	CountUnreadByTopic(ctx context.Context, topicID, userID uint) (int64, error)
	ListUnreadByTopic(ctx context.Context, topicID, userID uint) ([]uint, error) // returns entry IDs
}

type DiscussionTopicParticipantRepository interface {
	Upsert(ctx context.Context, p *models.DiscussionTopicParticipant) error
	FindByTopicAndUser(ctx context.Context, topicID, userID uint) (*models.DiscussionTopicParticipant, error)
	ListByTopicID(ctx context.Context, topicID uint) ([]models.DiscussionTopicParticipant, error)
	UpdateSubscription(ctx context.Context, topicID, userID uint, subscribed bool) error
}

type DiscussionEntryVersionRepository interface {
	Create(ctx context.Context, v *models.DiscussionEntryVersion) error
	ListByEntryID(ctx context.Context, entryID uint) ([]models.DiscussionEntryVersion, error)
	CountByEntryID(ctx context.Context, entryID uint) (int64, error)
}
