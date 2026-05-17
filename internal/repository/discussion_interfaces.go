package repository

import (
	"context"

	"github.com/EduThemes/paper-lms/internal/domain/models"
)

type DiscussionTopicRepository interface {
	Create(ctx context.Context, topic *models.DiscussionTopic) error
	// FindByID — 13.1.D: tenant-scoped via the parent course's account_id.
	// accountID==0 means "no scope" and is permitted only from internal
	// callers that have already validated tenant ownership upstream (e.g.
	// background workers, service-internal hops). Handler-layer callers
	// MUST pass the caller's account_id.
	FindByID(ctx context.Context, id, accountID uint) (*models.DiscussionTopic, error)
	Update(ctx context.Context, topic *models.DiscussionTopic) error
	Delete(ctx context.Context, id uint) error
	ListByCourseID(ctx context.Context, courseID, accountID uint, params PaginationParams) (*PaginatedResult[models.DiscussionTopic], error)
}

type DiscussionEntryRepository interface {
	Create(ctx context.Context, entry *models.DiscussionEntry) error
	// FindByID — 13.1.D: tenant-scoped via the entry → topic → course
	// chain. accountID==0 means "no scope" (internal callers only).
	FindByID(ctx context.Context, id, accountID uint) (*models.DiscussionEntry, error)
	Update(ctx context.Context, entry *models.DiscussionEntry) error
	Delete(ctx context.Context, id uint) error
	ListByTopicID(ctx context.Context, topicID, accountID uint, params PaginationParams) (*PaginatedResult[models.DiscussionEntry], error)
	ListReplies(ctx context.Context, entryID, accountID uint, params PaginationParams) (*PaginatedResult[models.DiscussionEntry], error)
	ListAllByTopicID(ctx context.Context, topicID, accountID uint) ([]models.DiscussionEntry, error)
}

type DiscussionEntryRatingRepository interface {
	Upsert(ctx context.Context, rating *models.DiscussionEntryRating) error
	Delete(ctx context.Context, entryID uint, userID uint) error
}
