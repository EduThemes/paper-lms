package service

import (
	"context"
	"errors"
	"fmt"
	"math"
	"strconv"
	"time"

	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/repository"
)

// AGSGrader writes a score into the gradebook through the normal grading
// path. *SubmissionService satisfies it: Grade enforces the caller's
// tenant, closed grading periods and the late policy, creates the
// submission when the student never submitted in Paper, and fires the
// post-grade callbacks (B082).
type AGSGrader interface {
	Grade(ctx context.Context, assignmentID, userID, graderID, callerAccountID uint, postedGrade string) (*models.Submission, error)
}

// AGS errors. Handlers map ErrAGSInvalid → 400, ErrAGSUserNotInCourse and
// ErrAGSGradeRefused → 422 (Canvas's status for both), ErrAGSNotFound → 404.
var (
	ErrAGSInvalid         = errors.New("invalid AGS request")
	ErrAGSNotFound        = errors.New("AGS resource not found")
	ErrAGSUserNotInCourse = errors.New("user not found in course or is not a student")
	ErrAGSGradeRefused    = errors.New("score could not be applied to the gradebook")
)

type agsError struct {
	kind error
	msg  string
}

func (e *agsError) Error() string { return e.msg }
func (e *agsError) Unwrap() error { return e.kind }

func agsErr(kind error, msg string) error { return &agsError{kind: kind, msg: msg} }

// LTIAGSService implements the LTI Assignment and Grade Services (AGS)
// specification. It manages line items and results for LTI-connected
// assignments and grade passback.
type LTIAGSService struct {
	lineItemRepo   repository.LTILineItemRepository
	resultRepo     repository.LTIResultRepository
	assignmentRepo repository.AssignmentRepository
	enrollmentRepo repository.EnrollmentRepository
	grader         AGSGrader
	// store (optional) backs the filtered line item listing and resource
	// link lookups of tool callers (G128).
	store repository.LTIAdvantageRepository
}

// SetAdvantageStore wires the LTI Advantage store (G128 filters and
// tool-created line items).
func (s *LTIAGSService) SetAdvantageStore(store repository.LTIAdvantageRepository) { s.store = store }

// ErrAGSConflict is a score older than the result already on record
// (AGS 2.0 §3.4.6: the platform must not apply it). Handlers answer 409.
var ErrAGSConflict = errors.New("a newer score has already been recorded")

// NewLTIAGSService creates a new LTIAGSService.
func NewLTIAGSService(
	lineItemRepo repository.LTILineItemRepository,
	resultRepo repository.LTIResultRepository,
	assignmentRepo repository.AssignmentRepository,
	enrollmentRepo repository.EnrollmentRepository,
	grader AGSGrader,
) *LTIAGSService {
	return &LTIAGSService{
		lineItemRepo:   lineItemRepo,
		resultRepo:     resultRepo,
		assignmentRepo: assignmentRepo,
		enrollmentRepo: enrollmentRepo,
		grader:         grader,
	}
}

// CreateLineItem creates a new LTI line item for a course. The label and
// scoreMaximum fields are required.
//
// B077: an assignmentId must name an assignment of the same course in the
// caller's tenant — otherwise a later score POST would grade another
// course's (or tenant's) submission.
func (s *LTIAGSService) CreateLineItem(ctx context.Context, courseID, accountID uint, item *models.LTILineItem) error {
	if item.Label == "" {
		return agsErr(ErrAGSInvalid, "line item label is required")
	}
	if item.ScoreMaximum <= 0 {
		return agsErr(ErrAGSInvalid, "scoreMaximum must be greater than 0")
	}
	if item.AssignmentID != nil {
		assignment, err := s.assignmentRepo.FindByID(ctx, *item.AssignmentID, accountID)
		if err != nil || assignment == nil || assignment.CourseID != courseID {
			return agsErr(ErrAGSInvalid, "assignmentId does not belong to this course")
		}
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

// DeleteLineItem removes a line item by its ID. accountID is threaded to
// the repo's tenant-scoped Delete (F-012) so a cross-tenant id soft-
// fails as a no-op rather than destructive write. accountID==0 means
// "no scope" — only auth-internal background callers should pass 0;
// handler-routed callers MUST pass callerAccountID(c).
func (s *LTIAGSService) DeleteLineItem(ctx context.Context, id, accountID uint) error {
	_, err := s.lineItemRepo.FindByID(ctx, id)
	if err != nil {
		return errors.New("line item not found")
	}
	return s.lineItemRepo.Delete(ctx, id, accountID)
}

// ListLineItems returns a paginated list of line items for the specified course.
func (s *LTIAGSService) ListLineItems(ctx context.Context, courseID uint, params repository.PaginationParams) (*repository.PaginatedResult[models.LTILineItem], error) {
	return s.lineItemRepo.ListByCourse(ctx, courseID, params)
}

var validActivityProgress = map[string]bool{
	"Initialized": true,
	"Started":     true,
	"InProgress":  true,
	"Submitted":   true,
	"Completed":   true,
}

var validGradingProgress = map[string]bool{
	"FullyGraded":   true,
	"Pending":       true,
	"PendingManual": true,
	"Failed":        true,
	"NotReady":      true,
}

// acceptGivenScore lists the gradingProgress values whose scoreGiven is
// written to the gradebook (Canvas Lti::Result::ACCEPT_GIVEN_SCORE_TYPES).
// For NotReady, Failed and Pending the score is stored on the result only.
var acceptGivenScore = map[string]bool{
	"FullyGraded":   true,
	"PendingManual": true,
}

// PostScore records an AGS score for a line item (already parent-tied to
// the URL course by the handler) and, for an assignment-linked line item,
// writes it to the gradebook.
//
//   - B077: the scored user must be an active student of the line item's
//     course, and the linked assignment must live in that course under the
//     caller's tenant. All lookups are tenant-scoped.
//   - B082: the gradebook write goes through AGSGrader (SubmissionService.
//     Grade): it creates the submission when the student never submitted
//     in Paper, refuses closed grading periods, applies the late policy and
//     fires post-grade callbacks. Only FullyGraded/PendingManual scores are
//     applied (Canvas parity); a refused grade fails the request and the
//     result is not saved.
//   - The score is scaled into the gradebook's range, as Canvas does
//     (scoreGiven * max / scoreMaximum). Canvas keeps an assignment's line
//     item scoreMaximum equal to its points_possible; Paper does not sync
//     them, so the assignment's points_possible is the target when set,
//     else the line item's scoreMaximum.
func (s *LTIAGSService) PostScore(ctx context.Context, lineItem *models.LTILineItem, accountID, graderID uint, result *models.LTIResult) error {
	if result.ActivityProgress != "" && !validActivityProgress[result.ActivityProgress] {
		return agsErr(ErrAGSInvalid, fmt.Sprintf("invalid activityProgress: %s", result.ActivityProgress))
	}
	if result.GradingProgress != "" && !validGradingProgress[result.GradingProgress] {
		return agsErr(ErrAGSInvalid, fmt.Sprintf("invalid gradingProgress: %s", result.GradingProgress))
	}
	if result.ResultScore != nil {
		if result.ResultMaximum == nil {
			return agsErr(ErrAGSInvalid, "scoreMaximum is required when scoreGiven is present")
		}
		if *result.ResultMaximum < 0 || *result.ResultScore < 0 {
			return agsErr(ErrAGSInvalid, "scoreGiven and scoreMaximum must be non-negative")
		}
	}

	enrollment, err := s.enrollmentRepo.FindByUserAndCourse(ctx, result.UserID, lineItem.CourseID, accountID)
	if err != nil || enrollment == nil || enrollment.WorkflowState != "active" || enrollment.Type != "StudentEnrollment" {
		return agsErr(ErrAGSUserNotInCourse, ErrAGSUserNotInCourse.Error())
	}

	// AGS §3.4.6: a score whose timestamp is older than the one on record
	// must not be applied (out-of-order delivery).
	if result.Timestamp != nil {
		if prev, pErr := s.resultRepo.ListByLineItem(ctx, lineItem.ID, result.UserID, repository.PaginationParams{Page: 1, PerPage: 1}); pErr == nil && prev != nil && len(prev.Items) > 0 {
			if ts := prev.Items[0].Timestamp; ts != nil && ts.After(*result.Timestamp) {
				return agsErr(ErrAGSConflict, ErrAGSConflict.Error())
			}
		}
	}

	if lineItem.AssignmentID != nil {
		assignment, aErr := s.assignmentRepo.FindByID(ctx, *lineItem.AssignmentID, accountID)
		if aErr != nil || assignment == nil || assignment.CourseID != lineItem.CourseID {
			return agsErr(ErrAGSNotFound, "line item assignment not found")
		}

		if result.ResultScore != nil && acceptGivenScore[result.GradingProgress] {
			if s.grader == nil {
				return agsErr(ErrAGSGradeRefused, "gradebook sync is not configured")
			}
			targetMax := lineItem.ScoreMaximum
			if assignment.PointsPossible != nil && *assignment.PointsPossible > 0 {
				targetMax = *assignment.PointsPossible
			}
			score := *result.ResultScore * agsScoreScale(targetMax, result.ResultMaximum)
			grade := strconv.FormatFloat(score, 'f', -1, 64)
			if _, gErr := s.grader.Grade(ctx, assignment.ID, result.UserID, graderID, accountID, grade); gErr != nil {
				return agsErr(ErrAGSGradeRefused, gErr.Error())
			}
		}
	}

	result.LineItemID = lineItem.ID
	if result.Timestamp == nil {
		now := time.Now()
		result.Timestamp = &now
	}

	if err := s.resultRepo.Upsert(ctx, result); err != nil {
		return fmt.Errorf("failed to save result: %w", err)
	}

	return nil
}

// agsScoreScale mirrors Canvas's line_item_score_maximum_scale: scale the
// tool's score into the line item's range; a missing/zero maximum means
// "don't scale".
func agsScoreScale(lineItemMax float64, resultMax *float64) float64 {
	if resultMax == nil || math.Abs(*resultMax) < 1e-9 || math.IsNaN(*resultMax) {
		return 1.0
	}
	return lineItemMax / *resultMax
}

// GetResults returns a paginated list of LTI results for a given line item.
// userID != 0 restricts the list to that user's result.
func (s *LTIAGSService) GetResults(ctx context.Context, lineItemID, userID uint, params repository.PaginationParams) (*repository.PaginatedResult[models.LTIResult], error) {
	// Verify the line item exists
	_, err := s.lineItemRepo.FindByID(ctx, lineItemID)
	if err != nil {
		return nil, errors.New("line item not found")
	}

	return s.resultRepo.ListByLineItem(ctx, lineItemID, userID, params)
}

// ListLineItemsFiltered lists a course's line items with the AGS filters
// (resource_link_id, resource_id, tag) and, for a tool caller, only the
// line items that tool owns (G128). Falls back to the unfiltered list
// when no store is wired.
func (s *LTIAGSService) ListLineItemsFiltered(ctx context.Context, courseID uint, filter repository.LTILineItemFilter, params repository.PaginationParams) (*repository.PaginatedResult[models.LTILineItem], error) {
	if s.store == nil {
		return s.lineItemRepo.ListByCourse(ctx, courseID, params)
	}
	return s.store.ListLineItems(ctx, courseID, filter, params)
}

// CreateToolLineItem is a tool's AGS line item POST (G128). Unless it
// names an assignment of the course, Paper creates a published
// placeholder assignment (points = scoreMaximum, due/unlock from the AGS
// dates) so the tool's scores land in the gradebook; the line item is
// owned by the tool.
func (s *LTIAGSService) CreateToolLineItem(ctx context.Context, courseID, accountID, devKeyID uint, item *models.LTILineItem) error {
	if item.Label == "" {
		return agsErr(ErrAGSInvalid, "line item label is required")
	}
	if item.ScoreMaximum <= 0 {
		return agsErr(ErrAGSInvalid, "scoreMaximum must be greater than 0")
	}
	if item.AssignmentID == nil {
		pts := item.ScoreMaximum
		a := &models.Assignment{
			CourseID: courseID, Name: item.Label, PointsPossible: &pts, GradingType: "points",
			SubmissionTypes: "none", WorkflowState: models.AssignmentPublished, Published: true,
			DueAt: item.EndDateTime, UnlockAt: item.StartDateTime,
		}
		if err := s.assignmentRepo.Create(ctx, a); err != nil {
			return fmt.Errorf("create assignment: %w", err)
		}
		aid := a.ID
		item.AssignmentID = &aid
	}
	dk := devKeyID
	item.DeveloperKeyID = &dk
	return s.CreateLineItem(ctx, courseID, accountID, item)
}

// SyncToolLineItemAssignment keeps a tool-owned line item's assignment
// in step with it: the label is the assignment name, scoreMaximum its
// points and the AGS dates its unlock/due dates (Canvas parity).
func (s *LTIAGSService) SyncToolLineItemAssignment(ctx context.Context, item *models.LTILineItem, accountID uint) error {
	if item.AssignmentID == nil || item.DeveloperKeyID == nil {
		return nil
	}
	a, err := s.assignmentRepo.FindByID(ctx, *item.AssignmentID, accountID)
	if err != nil || a == nil || a.CourseID != item.CourseID {
		return nil
	}
	pts := item.ScoreMaximum
	a.Name = item.Label
	a.PointsPossible = &pts
	if item.EndDateTime != nil {
		a.DueAt = item.EndDateTime
	}
	if item.StartDateTime != nil {
		a.UnlockAt = item.StartDateTime
	}
	return s.assignmentRepo.Update(ctx, a)
}
