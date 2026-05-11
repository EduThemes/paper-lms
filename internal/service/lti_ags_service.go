package service

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/repository"
)

// LTIAGSService implements the LTI Assignment and Grade Services (AGS)
// specification. It manages line items and results for LTI-connected
// assignments and grade passback.
type LTIAGSService struct {
	lineItemRepo   repository.LTILineItemRepository
	resultRepo     repository.LTIResultRepository
	submissionRepo repository.SubmissionRepository
	assignmentRepo repository.AssignmentRepository
}

// NewLTIAGSService creates a new LTIAGSService.
func NewLTIAGSService(
	lineItemRepo repository.LTILineItemRepository,
	resultRepo repository.LTIResultRepository,
	submissionRepo repository.SubmissionRepository,
	assignmentRepo repository.AssignmentRepository,
) *LTIAGSService {
	return &LTIAGSService{
		lineItemRepo:   lineItemRepo,
		resultRepo:     resultRepo,
		submissionRepo: submissionRepo,
		assignmentRepo: assignmentRepo,
	}
}

// CreateLineItem creates a new LTI line item for a course. The label and
// scoreMaximum fields are required.
func (s *LTIAGSService) CreateLineItem(ctx context.Context, courseID uint, item *models.LTILineItem) error {
	if item.Label == "" {
		return errors.New("line item label is required")
	}
	if item.ScoreMaximum <= 0 {
		return errors.New("scoreMaximum must be greater than 0")
	}

	item.CourseID = courseID
	return s.lineItemRepo.Create(ctx, item)
}

// GetLineItem retrieves a line item by its ID.
func (s *LTIAGSService) GetLineItem(ctx context.Context, id uint) (*models.LTILineItem, error) {
	item, err := s.lineItemRepo.FindByID(ctx, id)
	if err != nil {
		return nil, errors.New("line item not found")
	}
	return item, nil
}

// UpdateLineItem updates an existing line item. The label and scoreMaximum
// are validated before persisting.
func (s *LTIAGSService) UpdateLineItem(ctx context.Context, item *models.LTILineItem) error {
	if item.Label == "" {
		return errors.New("line item label is required")
	}
	if item.ScoreMaximum <= 0 {
		return errors.New("scoreMaximum must be greater than 0")
	}

	// Verify the line item exists
	_, err := s.lineItemRepo.FindByID(ctx, item.ID)
	if err != nil {
		return errors.New("line item not found")
	}

	return s.lineItemRepo.Update(ctx, item)
}

// DeleteLineItem removes a line item by its ID.
func (s *LTIAGSService) DeleteLineItem(ctx context.Context, id uint) error {
	_, err := s.lineItemRepo.FindByID(ctx, id)
	if err != nil {
		return errors.New("line item not found")
	}
	return s.lineItemRepo.Delete(ctx, id)
}

// ListLineItems returns a paginated list of line items for the specified course.
func (s *LTIAGSService) ListLineItems(ctx context.Context, courseID uint, params repository.PaginationParams) (*repository.PaginatedResult[models.LTILineItem], error) {
	return s.lineItemRepo.ListByCourse(ctx, courseID, params)
}

// PostScore upserts an LTI result (score) for a line item. If the line item
// is linked to a Canvas assignment (via AssignmentID), the corresponding
// submission is also updated with the computed score.
func (s *LTIAGSService) PostScore(ctx context.Context, lineItemID uint, result *models.LTIResult) error {
	// Validate the line item exists
	lineItem, err := s.lineItemRepo.FindByID(ctx, lineItemID)
	if err != nil {
		return errors.New("line item not found")
	}

	// Validate activity progress
	validActivityProgress := map[string]bool{
		"Initialized": true,
		"Started":     true,
		"InProgress":  true,
		"Submitted":   true,
		"Completed":   true,
	}
	if result.ActivityProgress != "" && !validActivityProgress[result.ActivityProgress] {
		return fmt.Errorf("invalid activityProgress: %s", result.ActivityProgress)
	}

	// Validate grading progress
	validGradingProgress := map[string]bool{
		"FullyGraded":   true,
		"Pending":       true,
		"PendingManual": true,
		"Failed":        true,
		"NotReady":      true,
	}
	if result.GradingProgress != "" && !validGradingProgress[result.GradingProgress] {
		return fmt.Errorf("invalid gradingProgress: %s", result.GradingProgress)
	}

	// Set the line item reference
	result.LineItemID = lineItemID

	// Set the timestamp if not provided
	if result.Timestamp == nil {
		now := time.Now()
		result.Timestamp = &now
	}

	// Upsert the result
	if err := s.resultRepo.Upsert(ctx, result); err != nil {
		return fmt.Errorf("failed to save result: %w", err)
	}

	// If the line item is linked to a Canvas assignment, also update the
	// submission score so grades are reflected in the gradebook.
	if lineItem.AssignmentID != nil && result.ResultScore != nil && result.ResultMaximum != nil && *result.ResultMaximum > 0 {
		if err := s.syncSubmissionScore(ctx, *lineItem.AssignmentID, result); err != nil {
			// Log but don't fail the score post if submission sync fails
			_ = err
		}
	}

	return nil
}

// syncSubmissionScore updates the Canvas submission for an assignment with the
// score computed from the LTI result. The score is scaled to the assignment's
// points_possible: score = (resultScore / resultMaximum) * pointsPossible.
func (s *LTIAGSService) syncSubmissionScore(ctx context.Context, assignmentID uint, result *models.LTIResult) error {
	submission, err := s.submissionRepo.FindByAssignmentAndUser(ctx, assignmentID, result.UserID)
	if err != nil {
		// No submission exists yet; nothing to sync
		return nil
	}

	// Compute the scaled score
	if result.ResultScore != nil && result.ResultMaximum != nil && *result.ResultMaximum > 0 {
		// Scale to assignment's actual points_possible (default 100 if not set)
		pointsPossible := 100.0
		if assignment, aErr := s.assignmentRepo.FindByID(ctx, assignmentID); aErr == nil && assignment.PointsPossible != nil && *assignment.PointsPossible > 0 {
			pointsPossible = *assignment.PointsPossible
		}
		score := *result.ResultScore / *result.ResultMaximum * pointsPossible
		submission.Score = &score

		gradeStr := fmt.Sprintf("%.2f", score)
		submission.Grade = &gradeStr

		now := time.Now()
		submission.GradedAt = &now
		submission.WorkflowState = "graded"

		return s.submissionRepo.Update(ctx, submission)
	}

	return nil
}

// GetResults returns a paginated list of LTI results for a given line item.
func (s *LTIAGSService) GetResults(ctx context.Context, lineItemID uint, params repository.PaginationParams) (*repository.PaginatedResult[models.LTIResult], error) {
	// Verify the line item exists
	_, err := s.lineItemRepo.FindByID(ctx, lineItemID)
	if err != nil {
		return nil, errors.New("line item not found")
	}

	return s.resultRepo.ListByLineItem(ctx, lineItemID, params)
}
