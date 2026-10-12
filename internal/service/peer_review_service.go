package service

import (
	"context"
	"encoding/json"
	"errors"
	"math/rand"

	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/repository"
)

type PeerReviewService struct {
	peerReviewRepo repository.PeerReviewRepository
	submissionRepo repository.SubmissionRepository
	enrollmentRepo repository.EnrollmentRepository
}

func NewPeerReviewService(
	peerReviewRepo repository.PeerReviewRepository,
	submissionRepo repository.SubmissionRepository,
	enrollmentRepo repository.EnrollmentRepository,
) *PeerReviewService {
	return &PeerReviewService{
		peerReviewRepo: peerReviewRepo,
		submissionRepo: submissionRepo,
		enrollmentRepo: enrollmentRepo,
	}
}

// ErrPeerReviewNotFound is returned when a review does not exist in the
// caller's tenant or is not assigned to the caller. Handlers map it to 404.
var ErrPeerReviewNotFound = errors.New("peer review not found")

// hasSubmitted reports whether sub represents turned-in work. A graded or
// pending-review submission was submitted too; only the workflow state
// "submitted" used to count, which dropped already-graded students from
// the reviewer pool.
func hasSubmitted(sub *models.Submission) bool {
	return sub != nil && sub.SubmittedAt != nil && sub.WorkflowState != models.SubmissionUnsubmitted
}

// AssignPeerReviews automatically assigns peer reviews for an assignment.
// Each student who has submitted gets `count` reviewers from the pool of other submitters.
// Completed reviews are kept (they are student work) and count toward the
// reviewer's quota; only incomplete assignments are redrawn.
// accountID scopes the parent assignment to a single tenant — 0 means no tenant scope
// (internal callers only).
func (s *PeerReviewService) AssignPeerReviews(ctx context.Context, courseID, assignmentID uint, count int, accountID uint) ([]models.PeerReview, error) {
	if count <= 0 {
		return nil, errors.New("peer review count must be positive")
	}

	// Get all student enrollments
	enrollments, err := s.enrollmentRepo.ListByCourseID(ctx, courseID, 0, repository.PaginationParams{Page: 1, PerPage: 1000})
	if err != nil {
		return nil, err
	}

	// Get student user IDs who have submitted
	var submitterIDs []uint
	submissionByUser := make(map[uint]*models.Submission)
	for _, e := range enrollments.Items {
		if e.Type != "StudentEnrollment" {
			continue
		}
		sub, err := s.submissionRepo.FindByAssignmentAndUser(ctx, assignmentID, e.UserID, accountID)
		if err == nil && hasSubmitted(sub) {
			submitterIDs = append(submitterIDs, e.UserID)
			submissionByUser[e.UserID] = sub
		}
	}

	if len(submitterIDs) < 2 {
		return nil, errors.New("not enough submissions for peer review")
	}

	// Keep completed reviews; drop the incomplete ones that get redrawn.
	completedPairs := make(map[[2]uint]bool)
	completedPerReviewer := make(map[uint]int)
	var kept []models.PeerReview
	if existing, err := s.peerReviewRepo.ListByAssignment(ctx, assignmentID, accountID); err == nil {
		for _, pr := range existing {
			if pr.WorkflowState == "completed" {
				completedPairs[[2]uint{pr.ReviewerID, pr.RevieweeID}] = true
				completedPerReviewer[pr.ReviewerID]++
				kept = append(kept, pr)
			}
		}
	}
	if err := s.peerReviewRepo.DeleteIncompleteByAssignment(ctx, assignmentID, accountID); err != nil {
		return nil, err
	}

	// Assign reviews: each student reviews `count` other students
	created := kept
	for _, reviewerID := range submitterIDs {
		need := count - completedPerReviewer[reviewerID]
		if need <= 0 {
			continue
		}
		// Build pool of other submitters this reviewer hasn't reviewed yet.
		var pool []uint
		for _, id := range submitterIDs {
			if id != reviewerID && !completedPairs[[2]uint{reviewerID, id}] {
				pool = append(pool, id)
			}
		}

		// Shuffle and pick up to `need`
		rand.Shuffle(len(pool), func(i, j int) { pool[i], pool[j] = pool[j], pool[i] })
		if need > len(pool) {
			need = len(pool)
		}

		for _, revieweeID := range pool[:need] {
			submissionID := uint(0)
			if sub := submissionByUser[revieweeID]; sub != nil {
				submissionID = sub.ID
			}

			pr := models.PeerReview{
				AssignmentID:  assignmentID,
				SubmissionID:  submissionID,
				ReviewerID:    reviewerID,
				RevieweeID:    revieweeID,
				WorkflowState: "assigned",
			}
			if err := s.peerReviewRepo.Create(ctx, &pr); err != nil {
				return nil, err
			}
			created = append(created, pr)
		}
	}

	return created, nil
}

func (s *PeerReviewService) ListByAssignment(ctx context.Context, assignmentID, accountID uint) ([]models.PeerReview, error) {
	return s.peerReviewRepo.ListByAssignment(ctx, assignmentID, accountID)
}

func (s *PeerReviewService) ListByReviewer(ctx context.Context, assignmentID, reviewerID, accountID uint) ([]models.PeerReview, error) {
	return s.peerReviewRepo.ListByReviewer(ctx, assignmentID, reviewerID, accountID)
}

// SubmissionForReview returns the reviewee's submission a review points
// at (tenant-scoped).
func (s *PeerReviewService) SubmissionForReview(ctx context.Context, pr *models.PeerReview, accountID uint) (*models.Submission, error) {
	if pr == nil {
		return nil, ErrPeerReviewNotFound
	}
	if pr.SubmissionID != 0 {
		if sub, err := s.submissionRepo.FindByID(ctx, pr.SubmissionID, accountID); err == nil && sub != nil {
			return sub, nil
		}
	}
	return s.submissionRepo.FindByAssignmentAndUser(ctx, pr.AssignmentID, pr.RevieweeID, accountID)
}

// SubmitReview records the caller's review. B018: the review must be
// assigned to callerID — any other user (even in the same tenant) gets
// ErrPeerReviewNotFound, the same as a missing review.
func (s *PeerReviewService) SubmitReview(ctx context.Context, reviewID, callerID uint, score float64, comments string, accountID uint) (*models.PeerReview, error) {
	pr, err := s.peerReviewRepo.FindByID(ctx, reviewID, accountID)
	if err != nil || pr == nil || callerID == 0 || pr.ReviewerID != callerID {
		return nil, ErrPeerReviewNotFound
	}
	if score < 0 {
		return nil, errors.New("score must not be negative")
	}

	pr.Score = &score
	pr.Comments = comments
	pr.WorkflowState = "completed"

	if err := s.peerReviewRepo.Update(ctx, pr); err != nil {
		return nil, err
	}

	return pr, nil
}

// SubmitRubricReview records a rubric-based peer assessment. rubricData
// maps criterion ids to {points, comments}; when score is nil the review
// score is the sum of the criterion points.
func (s *PeerReviewService) SubmitRubricReview(ctx context.Context, reviewID, callerID uint, score *float64, comments string, rubricData map[string]RubricCriterionScore, accountID uint) (*models.PeerReview, error) {
	pr, err := s.peerReviewRepo.FindByID(ctx, reviewID, accountID)
	if err != nil || pr == nil || callerID == 0 || pr.ReviewerID != callerID {
		return nil, ErrPeerReviewNotFound
	}
	total := 0.0
	for _, c := range rubricData {
		if c.Points < 0 {
			return nil, errors.New("rubric points must not be negative")
		}
		total += c.Points
	}
	if score == nil {
		if len(rubricData) == 0 {
			return nil, errors.New("a score or rubric assessment is required")
		}
		score = &total
	}
	if *score < 0 {
		return nil, errors.New("score must not be negative")
	}
	if len(rubricData) > 0 {
		clean := make(map[string]RubricCriterionScore, len(rubricData))
		for k, v := range rubricData {
			v.Comments = SanitizePlainText(v.Comments)
			clean[k] = v
		}
		raw, err := json.Marshal(clean)
		if err != nil {
			return nil, err
		}
		str := string(raw)
		pr.RubricData = &str
	}
	pr.Score = score
	pr.Comments = SanitizePlainText(comments)
	pr.WorkflowState = "completed"
	if err := s.peerReviewRepo.Update(ctx, pr); err != nil {
		return nil, err
	}
	return pr, nil
}

// RubricCriterionScore is one criterion of a rubric-based peer review.
type RubricCriterionScore struct {
	Points   float64 `json:"points"`
	Comments string  `json:"comments,omitempty"`
}

func (s *PeerReviewService) GetByID(ctx context.Context, id, accountID uint) (*models.PeerReview, error) {
	return s.peerReviewRepo.FindByID(ctx, id, accountID)
}
