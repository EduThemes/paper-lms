package repository

import (
	"context"

	"github.com/EduThemes/paper-lms/internal/domain/models"
)

// DiscussionCheckpointRepository persists Canvas-compatible discussion
// checkpoints (multi-deadline thread participation requirements).
type DiscussionCheckpointRepository interface {
	Create(ctx context.Context, checkpoint *models.DiscussionCheckpoint) error
	// FindByID — 13.1.D: tenant-scoped via the checkpoint → topic →
	// course chain. accountID==0 means "no scope" (internal callers only).
	FindByID(ctx context.Context, id, accountID uint) (*models.DiscussionCheckpoint, error)
	Update(ctx context.Context, checkpoint *models.DiscussionCheckpoint) error
	Delete(ctx context.Context, id uint) error
	ListByTopicID(ctx context.Context, topicID, accountID uint) ([]models.DiscussionCheckpoint, error)
	DeleteByTopicID(ctx context.Context, topicID uint) error
}

// DiscussionCheckpointSubmissionRepository tracks per-user progress
// against discussion checkpoints.
type DiscussionCheckpointSubmissionRepository interface {
	UpsertSubmission(ctx context.Context, submission *models.DiscussionCheckpointSubmission) error
	// 13.1.D — tenant scope via checkpoint->topic->course. 0 means no tenant scope (internal callers only).
	FindByCheckpointAndUser(ctx context.Context, checkpointID, userID, accountID uint) (*models.DiscussionCheckpointSubmission, error)
	ListByCheckpoint(ctx context.Context, checkpointID, accountID uint) ([]models.DiscussionCheckpointSubmission, error)
	ListByUserAndTopic(ctx context.Context, topicID, userID, accountID uint) ([]models.DiscussionCheckpointSubmission, error)
}
