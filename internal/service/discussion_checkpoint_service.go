package service

import (
	"context"
	"errors"
	"math"
	"time"

	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/repository"
)

// pointsTolerance is used when comparing float sums against the parent
// assignment's points_possible (avoids float-equality false negatives).
const pointsTolerance = 0.001

// DiscussionCheckpointService provides business logic for Canvas-compatible
// discussion checkpoints (multi-deadline thread participation).
type DiscussionCheckpointService struct {
	checkpointRepo repository.DiscussionCheckpointRepository
	submissionRepo repository.DiscussionCheckpointSubmissionRepository
	topicRepo      repository.DiscussionTopicRepository
	entryRepo      repository.DiscussionEntryRepository
	assignmentRepo repository.AssignmentRepository
}

func NewDiscussionCheckpointService(
	checkpointRepo repository.DiscussionCheckpointRepository,
	submissionRepo repository.DiscussionCheckpointSubmissionRepository,
	topicRepo repository.DiscussionTopicRepository,
	entryRepo repository.DiscussionEntryRepository,
	assignmentRepo repository.AssignmentRepository,
) *DiscussionCheckpointService {
	return &DiscussionCheckpointService{
		checkpointRepo: checkpointRepo,
		submissionRepo: submissionRepo,
		topicRepo:      topicRepo,
		entryRepo:      entryRepo,
		assignmentRepo: assignmentRepo,
	}
}

// CreateCheckpoints creates one or more checkpoints for a topic atomically
// (replacing any existing checkpoints). Validates that the set includes
// exactly one reply_to_topic and one reply_to_entry checkpoint and that
// the points sum matches the parent assignment's points_possible.
func (s *DiscussionCheckpointService) CreateCheckpoints(
	ctx context.Context,
	topicID uint,
	checkpoints []*models.DiscussionCheckpoint,
) ([]models.DiscussionCheckpoint, error) {
	if len(checkpoints) == 0 {
		return nil, errors.New("at least one checkpoint is required")
	}

	topic, err := s.topicRepo.FindByID(ctx, topicID)
	if err != nil {
		return nil, errors.New("discussion topic not found")
	}

	if err := validateCheckpointSet(checkpoints); err != nil {
		return nil, err
	}

	// If the topic is graded (has an assignment), the checkpoint points
	// must sum to the assignment's points_possible — Canvas requires this.
	if topic.AssignmentID != nil {
		assignment, err := s.assignmentRepo.FindByID(ctx, *topic.AssignmentID)
		if err != nil {
			return nil, errors.New("parent assignment not found")
		}
		var assignmentPoints float64
		if assignment.PointsPossible != nil {
			assignmentPoints = *assignment.PointsPossible
		}
		var sum float64
		for _, cp := range checkpoints {
			sum += cp.PointsPossible
		}
		if math.Abs(sum-assignmentPoints) > pointsTolerance {
			return nil, errors.New("checkpoint points must sum to the assignment's points_possible")
		}
	}

	// Replace existing checkpoints (soft-delete old ones).
	if err := s.checkpointRepo.DeleteByTopicID(ctx, topicID); err != nil {
		return nil, err
	}

	created := make([]models.DiscussionCheckpoint, 0, len(checkpoints))
	for _, cp := range checkpoints {
		cp.DiscussionTopicID = topicID
		if cp.WorkflowState == "" {
			cp.WorkflowState = "active"
		}
		if cp.CheckpointType == models.CheckpointTypeReplyToTopic {
			// initial-post checkpoint always counts as a single required reply.
			cp.RequiredReplies = 1
		}
		if err := s.checkpointRepo.Create(ctx, cp); err != nil {
			return nil, err
		}
		created = append(created, *cp)
	}
	return created, nil
}

// UpdateCheckpoint updates a single checkpoint. Note: doesn't re-validate
// the points sum — clients should typically re-call CreateCheckpoints to
// replace the full set when changing point distribution.
func (s *DiscussionCheckpointService) UpdateCheckpoint(ctx context.Context, cp *models.DiscussionCheckpoint) error {
	if cp.CheckpointType != models.CheckpointTypeReplyToTopic &&
		cp.CheckpointType != models.CheckpointTypeReplyToEntry {
		return errors.New("invalid checkpoint_type")
	}
	if cp.PointsPossible < 0 {
		return errors.New("points_possible cannot be negative")
	}
	if cp.CheckpointType == models.CheckpointTypeReplyToEntry && cp.RequiredReplies < 1 {
		return errors.New("required_replies must be >= 1 for reply_to_entry")
	}
	return s.checkpointRepo.Update(ctx, cp)
}

func (s *DiscussionCheckpointService) DeleteCheckpoint(ctx context.Context, id uint) error {
	return s.checkpointRepo.Delete(ctx, id)
}

func (s *DiscussionCheckpointService) ListCheckpoints(ctx context.Context, topicID uint) ([]models.DiscussionCheckpoint, error) {
	return s.checkpointRepo.ListByTopicID(ctx, topicID)
}

// UserCheckpointProgress is a per-checkpoint progress view for a user.
type UserCheckpointProgress struct {
	Checkpoint  models.DiscussionCheckpoint `json:"checkpoint"`
	ReplyCount  int                         `json:"reply_count"`
	Required    int                         `json:"required"`
	Status      string                      `json:"status"`
	CompletedAt *time.Time                  `json:"completed_at"`
}

// EvaluateUserProgress recomputes a user's progress against every
// checkpoint on the topic by counting their entries, then upserts the
// resulting submission rows. Returns the latest progress view.
func (s *DiscussionCheckpointService) EvaluateUserProgress(
	ctx context.Context,
	topicID, userID uint,
) ([]UserCheckpointProgress, error) {
	checkpoints, err := s.checkpointRepo.ListByTopicID(ctx, topicID)
	if err != nil {
		return nil, err
	}
	if len(checkpoints) == 0 {
		return []UserCheckpointProgress{}, nil
	}

	// Pull all entries this user wrote on this topic; we partition them
	// into "initial post" (parent_id IS NULL) and "reply-to-entry" buckets.
	entries, err := s.entryRepo.ListAllByTopicID(ctx, topicID)
	if err != nil {
		return nil, err
	}
	var initialPosts, replyEntries int
	for _, e := range entries {
		if e.UserID != userID || e.WorkflowState == "deleted" {
			continue
		}
		if e.ParentID == nil {
			initialPosts++
		} else {
			replyEntries++
		}
	}

	now := time.Now()
	out := make([]UserCheckpointProgress, 0, len(checkpoints))
	for i := range checkpoints {
		cp := checkpoints[i]
		var count, required int
		switch cp.CheckpointType {
		case models.CheckpointTypeReplyToTopic:
			count = initialPosts
			required = 1
		case models.CheckpointTypeReplyToEntry:
			count = replyEntries
			required = cp.RequiredReplies
			if required < 1 {
				required = 1
			}
		}

		status := models.CheckpointStatusNotStarted
		var completedAt *time.Time
		switch {
		case count >= required:
			status = models.CheckpointStatusCompleted
			t := now
			completedAt = &t
		case count > 0:
			status = models.CheckpointStatusInProgress
		}

		sub := &models.DiscussionCheckpointSubmission{
			DiscussionCheckpointID: cp.ID,
			UserID:                 userID,
			ReplyCount:             count,
			Status:                 status,
			CompletedAt:            completedAt,
		}
		if err := s.submissionRepo.UpsertSubmission(ctx, sub); err != nil {
			return nil, err
		}

		out = append(out, UserCheckpointProgress{
			Checkpoint:  cp,
			ReplyCount:  count,
			Required:    required,
			Status:      status,
			CompletedAt: completedAt,
		})
	}
	return out, nil
}

// validateCheckpointSet enforces Canvas's invariants on a checkpoint set.
func validateCheckpointSet(checkpoints []*models.DiscussionCheckpoint) error {
	var sawTopic, sawEntry bool
	for _, cp := range checkpoints {
		switch cp.CheckpointType {
		case models.CheckpointTypeReplyToTopic:
			if sawTopic {
				return errors.New("only one reply_to_topic checkpoint is allowed")
			}
			sawTopic = true
		case models.CheckpointTypeReplyToEntry:
			if sawEntry {
				return errors.New("only one reply_to_entry checkpoint is allowed")
			}
			if cp.RequiredReplies < 1 {
				return errors.New("required_replies must be >= 1 for reply_to_entry")
			}
			sawEntry = true
		default:
			return errors.New("checkpoint_type must be reply_to_topic or reply_to_entry")
		}
		if cp.PointsPossible < 0 {
			return errors.New("points_possible cannot be negative")
		}
	}
	if !sawTopic || !sawEntry {
		return errors.New("checkpoint set must include both reply_to_topic and reply_to_entry")
	}
	return nil
}
