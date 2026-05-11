package repository

import (
	"context"

	"github.com/EduThemes/paper-lms/internal/domain/models"
)

// DiscussionCheckpointRepository persists Canvas-compatible discussion
// checkpoints (multi-deadline thread participation requirements).
type DiscussionCheckpointRepository interface {
	Create(ctx context.Context, checkpoint *models.DiscussionCheckpoint) error
	FindByID(ctx context.Context, id uint) (*models.DiscussionCheckpoint, error)
	Update(ctx context.Context, checkpoint *models.DiscussionCheckpoint) error
	Delete(ctx context.Context, id uint) error
	ListByTopicID(ctx context.Context, topicID uint) ([]models.DiscussionCheckpoint, error)
	DeleteByTopicID(ctx context.Context, topicID uint) error
}

// DiscussionCheckpointSubmissionRepository tracks per-user progress
// against discussion checkpoints.
type DiscussionCheckpointSubmissionRepository interface {
	UpsertSubmission(ctx context.Context, submission *models.DiscussionCheckpointSubmission) error
	FindByCheckpointAndUser(ctx context.Context, checkpointID, userID uint) (*models.DiscussionCheckpointSubmission, error)
	ListByCheckpoint(ctx context.Context, checkpointID uint) ([]models.DiscussionCheckpointSubmission, error)
	ListByUserAndTopic(ctx context.Context, topicID, userID uint) ([]models.DiscussionCheckpointSubmission, error)
}
