package service

import (
	"context"
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

// AssignPeerReviews automatically assigns peer reviews for an assignment.
// Each student who has submitted gets `count` reviewers from the pool of other submitters.
// accountID scopes the parent assignment to a single tenant — 0 means no tenant scope
// (internal callers only).
func (s *PeerReviewService) AssignPeerReviews(ctx context.Context, courseID, assignmentID uint, count int, accountID uint) ([]models.PeerReview, error) {
	if count <= 0 {
		return nil, errors.New("peer review count must be positive")
	}

	// Get all student enrollments
	enrollments, err := s.enrollmentRepo.ListByCourseID(ctx, courseID, repository.PaginationParams{Page: 1, PerPage: 1000})
	if err != nil {
		return nil, err
	}

	// Get student user IDs who have submitted
	var submitterIDs []uint
	for _, e := range enrollments.Items {
		if e.Type != "StudentEnrollment" {
			continue
		}
		sub, err := s.submissionRepo.FindByAssignmentAndUser(ctx, assignmentID, e.UserID, accountID)
		if err == nil && sub != nil && sub.WorkflowState == "submitted" {
			submitterIDs = append(submitterIDs, e.UserID)
		}
	}

	if len(submitterIDs) < 2 {
		return nil, errors.New("not enough submissions for peer review")
	}

	// Delete existing assignments
	_ = s.peerReviewRepo.DeleteByAssignment(ctx, assignmentID, accountID)

	// Assign reviews: each student reviews `count` other students
	var created []models.PeerReview
	for _, reviewerID := range submitterIDs {
		// Build pool of other submitters
		var pool []uint
		for _, id := range submitterIDs {
			if id != reviewerID {
				pool = append(pool, id)
			}
		}

		// Shuffle and pick up to `count`
		rand.Shuffle(len(pool), func(i, j int) { pool[i], pool[j] = pool[j], pool[i] })
		n := count
		if n > len(pool) {
			n = len(pool)
		}

		for _, revieweeID := range pool[:n] {
			// Get the submission for the reviewee
			sub, _ := s.submissionRepo.FindByAssignmentAndUser(ctx, assignmentID, revieweeID, accountID)
			submissionID := uint(0)
			if sub != nil {
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

func (s *PeerReviewService) SubmitReview(ctx context.Context, reviewID uint, score float64, comments string, accountID uint) (*models.PeerReview, error) {
	pr, err := s.peerReviewRepo.FindByID(ctx, reviewID, accountID)
	if err != nil {
		return nil, errors.New("peer review not found")
	}

	pr.Score = &score
	pr.Comments = comments
	pr.WorkflowState = "completed"

	if err := s.peerReviewRepo.Update(ctx, pr); err != nil {
		return nil, err
	}

	return pr, nil
}

func (s *PeerReviewService) GetByID(ctx context.Context, id, accountID uint) (*models.PeerReview, error) {
	return s.peerReviewRepo.FindByID(ctx, id, accountID)
}
