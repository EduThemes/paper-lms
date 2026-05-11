package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"math"
	"strconv"
	"time"

	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/repository"
)

// SubmissionGradedCallback is invoked asynchronously after a submission is
// successfully graded. Callbacks receive a detached context and the graded
// submission's ID. Implementations MUST NOT block grading and MUST NOT panic;
// panics are recovered and logged but never propagated.
type SubmissionGradedCallback func(ctx context.Context, submissionID uint)

// recoverFromPanic recovers from a panic in an OnGraded callback, logging the
// panic value with the originating callback name. Used as `defer
// recoverFromPanic("...")` inside the goroutine that fires each callback so a
// crashing callback never crashes the grading flow.
func recoverFromPanic(label string) {
	if r := recover(); r != nil {
		slog.Error("panic recovered in callback", "label", label, "panic", r)
	}
}

type SubmissionService struct {
	submissionRepo         repository.SubmissionRepository
	assignmentRepo         repository.AssignmentRepository
	enrollmentRepo         repository.EnrollmentRepository
	latePolicyRepo         repository.LatePolicyRepository
	courseRepo             repository.CourseRepository
	gradingPeriodGroupRepo repository.GradingPeriodGroupRepository
	gradingPeriodRepo      repository.GradingPeriodRepository
	groupMembershipRepo    repository.GroupMembershipRepository

	// onGradedCallbacks fire (in goroutines) after a successful Grade(...).
	// Registered via OnGraded; never invoked in tests unless explicitly wired.
	onGradedCallbacks []SubmissionGradedCallback
}

// OnGraded registers a callback to fire after a successful grade write. The
// callback runs in a fresh goroutine with a detached context.Background(), so
// it MUST be self-contained (don't rely on the request's context.Cancel).
// Multiple registrations stack; order is registration order.
func (s *SubmissionService) OnGraded(cb SubmissionGradedCallback) {
	s.onGradedCallbacks = append(s.onGradedCallbacks, cb)
}

// fireOnGraded runs all registered callbacks in goroutines with a detached
// context. Panics are recovered. Errors from callbacks (if any) are the
// callback's responsibility to log — the signature returns no error.
func (s *SubmissionService) fireOnGraded(submissionID uint) {
	for _, cb := range s.onGradedCallbacks {
		go func(cb SubmissionGradedCallback) {
			defer recoverFromPanic("submission OnGraded callback")
			cb(context.Background(), submissionID)
		}(cb)
	}
}

func NewSubmissionService(
	submissionRepo repository.SubmissionRepository,
	assignmentRepo repository.AssignmentRepository,
	enrollmentRepo repository.EnrollmentRepository,
	latePolicyRepo repository.LatePolicyRepository,
	courseRepo repository.CourseRepository,
	gradingPeriodGroupRepo repository.GradingPeriodGroupRepository,
	gradingPeriodRepo repository.GradingPeriodRepository,
	groupMembershipRepo repository.GroupMembershipRepository,
) *SubmissionService {
	return &SubmissionService{
		submissionRepo:         submissionRepo,
		assignmentRepo:         assignmentRepo,
		enrollmentRepo:         enrollmentRepo,
		latePolicyRepo:         latePolicyRepo,
		courseRepo:             courseRepo,
		gradingPeriodGroupRepo: gradingPeriodGroupRepo,
		gradingPeriodRepo:      gradingPeriodRepo,
		groupMembershipRepo:    groupMembershipRepo,
	}
}

func (s *SubmissionService) Create(ctx context.Context, submission *models.Submission) error {
	// Validate assignment exists
	assignment, err := s.assignmentRepo.FindByID(ctx, submission.AssignmentID)
	if err != nil {
		return errors.New("assignment not found")
	}

	if submission.SubmissionType == nil || *submission.SubmissionType == "" {
		return errors.New("submission_type is required")
	}

	now := time.Now()
	submission.SubmittedAt = &now
	submission.Attempt = 1
	submission.WorkflowState = "submitted"

	// Check if late
	if assignment.DueAt != nil && now.After(*assignment.DueAt) {
		submission.Late = true
	}

	// Check for existing submission and increment attempt
	existing, _ := s.submissionRepo.FindByAssignmentAndUser(ctx, submission.AssignmentID, submission.UserID)
	if existing != nil {
		existing.SubmissionType = submission.SubmissionType
		existing.Body = submission.Body
		existing.URL = submission.URL
		existing.Attachments = submission.Attachments
		existing.SubmittedAt = &now
		existing.Attempt = existing.Attempt + 1
		existing.Late = submission.Late
		existing.WorkflowState = "submitted"
		*submission = *existing
		if err := s.submissionRepo.Update(ctx, submission); err != nil {
			return err
		}
	} else {
		if err := s.submissionRepo.Create(ctx, submission); err != nil {
			return err
		}
	}

	// For group assignments, create/update submissions for all other group members
	if assignment.GroupCategoryID != nil && *assignment.GroupCategoryID > 0 {
		s.createGroupSubmissions(ctx, assignment, submission)
	}

	return nil
}

func (s *SubmissionService) GetByAssignmentAndUser(ctx context.Context, assignmentID, userID uint) (*models.Submission, error) {
	return s.submissionRepo.FindByAssignmentAndUser(ctx, assignmentID, userID)
}

func (s *SubmissionService) ListByAssignment(ctx context.Context, assignmentID uint, params repository.PaginationParams) (*repository.PaginatedResult[models.Submission], error) {
	return s.submissionRepo.ListByAssignmentID(ctx, assignmentID, params)
}

func (s *SubmissionService) BulkListByCourse(ctx context.Context, courseID uint, params repository.PaginationParams) (*repository.PaginatedResult[models.Submission], error) {
	return s.submissionRepo.BulkListByCourse(ctx, courseID, params)
}

func (s *SubmissionService) PostGradesByAssignment(ctx context.Context, assignmentID uint, postedAt *time.Time) error {
	return s.submissionRepo.PostGradesByAssignment(ctx, assignmentID, postedAt)
}

func (s *SubmissionService) Grade(ctx context.Context, assignmentID, userID, graderID uint, postedGrade string) (*models.Submission, error) {
	score, err := strconv.ParseFloat(postedGrade, 64)
	if err != nil {
		return nil, fmt.Errorf("invalid grade value: %s", postedGrade)
	}

	// Check if grading period is closed for this assignment
	if closed, periodTitle := s.isGradingPeriodClosed(ctx, assignmentID); closed {
		return nil, fmt.Errorf("grading period %q is closed — grades cannot be modified", periodTitle)
	}

	now := time.Now()

	// Apply late policy deductions if applicable
	score = s.applyLateDeduction(ctx, assignmentID, userID, score)

	gradeStr := strconv.FormatFloat(score, 'f', -1, 64)

	submission, err := s.submissionRepo.FindByAssignmentAndUser(ctx, assignmentID, userID)
	if err != nil {
		// No existing submission — create one (teacher grading without student submission)
		submission = &models.Submission{
			AssignmentID:  assignmentID,
			UserID:        userID,
			Score:         &score,
			Grade:         &gradeStr,
			GradedAt:      &now,
			GraderID:      &graderID,
			WorkflowState: "graded",
		}
		if createErr := s.submissionRepo.Create(ctx, submission); createErr != nil {
			return nil, createErr
		}
	} else {
		submission.Score = &score
		submission.Grade = &gradeStr
		submission.GradedAt = &now
		submission.GraderID = &graderID
		submission.WorkflowState = "graded"

		if err := s.submissionRepo.Update(ctx, submission); err != nil {
			return nil, err
		}
	}

	// For group assignments, apply the same grade to all other group members
	assignment, aErr := s.assignmentRepo.FindByID(ctx, assignmentID)
	if aErr == nil && assignment.GroupCategoryID != nil && *assignment.GroupCategoryID > 0 {
		s.gradeGroupMembers(ctx, assignment, userID, graderID, score, gradeStr, &now)
	}

	// Fire post-grade callbacks (mastery paths, etc.) asynchronously. Failures
	// in callbacks must never block grading or surface as errors here.
	s.fireOnGraded(submission.ID)

	return submission, nil
}

// getGroupMemberIDs returns the user IDs of all members in the same group as the given user
// within the specified group category, excluding the given user.
func (s *SubmissionService) getGroupMemberIDs(ctx context.Context, userID, groupCategoryID uint) []uint {
	if s.groupMembershipRepo == nil {
		return nil
	}

	// Find the user's group in this category
	group, err := s.groupMembershipRepo.FindUserGroupInCategory(ctx, userID, groupCategoryID)
	if err != nil {
		return nil
	}

	// List all members of that group
	memberships, err := s.groupMembershipRepo.ListByGroupID(ctx, group.ID, repository.PaginationParams{Page: 1, PerPage: 1000})
	if err != nil {
		return nil
	}

	var memberIDs []uint
	for _, m := range memberships.Items {
		if m.UserID != userID && m.WorkflowState == "accepted" {
			memberIDs = append(memberIDs, m.UserID)
		}
	}
	return memberIDs
}

// createGroupSubmissions creates or updates submissions for all other group members
// when a student submits a group assignment.
func (s *SubmissionService) createGroupSubmissions(ctx context.Context, assignment *models.Assignment, submission *models.Submission) {
	memberIDs := s.getGroupMemberIDs(ctx, submission.UserID, *assignment.GroupCategoryID)
	for _, memberID := range memberIDs {
		existing, _ := s.submissionRepo.FindByAssignmentAndUser(ctx, submission.AssignmentID, memberID)
		if existing != nil {
			existing.SubmissionType = submission.SubmissionType
			existing.Body = submission.Body
			existing.URL = submission.URL
			existing.Attachments = submission.Attachments
			existing.SubmittedAt = submission.SubmittedAt
			existing.Attempt = existing.Attempt + 1
			existing.Late = submission.Late
			existing.WorkflowState = "submitted"
			_ = s.submissionRepo.Update(ctx, existing)
		} else {
			memberSubmission := &models.Submission{
				AssignmentID:   submission.AssignmentID,
				UserID:         memberID,
				SubmissionType: submission.SubmissionType,
				Body:           submission.Body,
				URL:            submission.URL,
				Attachments:    submission.Attachments,
				SubmittedAt:    submission.SubmittedAt,
				Attempt:        1,
				Late:           submission.Late,
				WorkflowState:  "submitted",
			}
			_ = s.submissionRepo.Create(ctx, memberSubmission)
		}
	}
}

// gradeGroupMembers applies the same grade to all other group members
// when grading a group assignment submission.
func (s *SubmissionService) gradeGroupMembers(ctx context.Context, assignment *models.Assignment, gradedUserID, graderID uint, score float64, gradeStr string, gradedAt *time.Time) {
	memberIDs := s.getGroupMemberIDs(ctx, gradedUserID, *assignment.GroupCategoryID)
	for _, memberID := range memberIDs {
		// Apply per-member late deduction (each member's lateness may differ if overrides exist)
		memberScore := s.applyLateDeduction(ctx, assignment.ID, memberID, score)
		memberGradeStr := strconv.FormatFloat(memberScore, 'f', -1, 64)

		existing, _ := s.submissionRepo.FindByAssignmentAndUser(ctx, assignment.ID, memberID)
		if existing != nil {
			existing.Score = &memberScore
			existing.Grade = &memberGradeStr
			existing.GradedAt = gradedAt
			existing.GraderID = &graderID
			existing.WorkflowState = "graded"
			_ = s.submissionRepo.Update(ctx, existing)
		} else {
			memberSubmission := &models.Submission{
				AssignmentID:  assignment.ID,
				UserID:        memberID,
				Score:         &memberScore,
				Grade:         &memberGradeStr,
				GradedAt:      gradedAt,
				GraderID:      &graderID,
				WorkflowState: "graded",
			}
			_ = s.submissionRepo.Create(ctx, memberSubmission)
		}
	}
}

// applyLateDeduction applies late submission deduction based on the course's late policy.
// Returns the adjusted score (or original score if no deduction applies).
func (s *SubmissionService) applyLateDeduction(ctx context.Context, assignmentID, userID uint, score float64) float64 {
	// Get the submission to check if it's late
	submission, err := s.submissionRepo.FindByAssignmentAndUser(ctx, assignmentID, userID)
	if err != nil || !submission.Late {
		return score
	}

	// Get the assignment to determine course and due date
	assignment, err := s.assignmentRepo.FindByID(ctx, assignmentID)
	if err != nil || assignment.DueAt == nil {
		return score
	}

	// Get the course's late policy
	policy, err := s.latePolicyRepo.FindByCourseID(ctx, assignment.CourseID)
	if err != nil || !policy.LateSubmissionDeductionEnabled {
		return score
	}

	// Calculate how many intervals late
	submittedAt := submission.SubmittedAt
	if submittedAt == nil {
		return score
	}

	lateDuration := submittedAt.Sub(*assignment.DueAt)
	if lateDuration <= 0 {
		return score
	}

	var intervals float64
	switch policy.LateSubmissionInterval {
	case "hour":
		intervals = math.Ceil(lateDuration.Hours())
	default: // "day"
		intervals = math.Ceil(lateDuration.Hours() / 24)
	}

	// Apply deduction: deduction is a percentage per interval
	totalDeductionPct := intervals * policy.LateSubmissionDeduction
	if totalDeductionPct > 100 {
		totalDeductionPct = 100
	}

	adjustedScore := score * (1 - totalDeductionPct/100)

	// Apply minimum percent floor if enabled
	if policy.LateSubmissionMinimumPercentEnabled && assignment.PointsPossible != nil && *assignment.PointsPossible > 0 {
		minScore := *assignment.PointsPossible * (policy.LateSubmissionMinimumPercent / 100)
		if adjustedScore < minScore {
			adjustedScore = minScore
		}
	}

	// Never go below 0
	if adjustedScore < 0 {
		adjustedScore = 0
	}

	return adjustedScore
}

// isGradingPeriodClosed checks whether the assignment falls within a closed grading period.
// Returns (true, periodTitle) if closed, (false, "") if open or no grading periods configured.
func (s *SubmissionService) isGradingPeriodClosed(ctx context.Context, assignmentID uint) (bool, string) {
	assignment, err := s.assignmentRepo.FindByID(ctx, assignmentID)
	if err != nil {
		return false, ""
	}

	// Use assignment due date to determine which grading period applies
	dueAt := assignment.DueAt
	if dueAt == nil {
		// No due date means no grading period applies
		return false, ""
	}

	course, err := s.courseRepo.FindByID(ctx, assignment.CourseID)
	if err != nil {
		return false, ""
	}

	// Find grading period groups for the course's account
	groups, err := s.gradingPeriodGroupRepo.ListByAccountID(ctx, course.AccountID, repository.PaginationParams{Page: 1, PerPage: 100})
	if err != nil || len(groups.Items) == 0 {
		return false, ""
	}

	now := time.Now()

	for _, group := range groups.Items {
		if group.WorkflowState != "active" {
			continue
		}
		periods, err := s.gradingPeriodRepo.ListByGroupID(ctx, group.ID)
		if err != nil {
			continue
		}
		for _, period := range periods {
			if period.WorkflowState != "active" {
				continue
			}
			// Check if assignment due date falls within this period
			if dueAt.After(period.StartDate) && (dueAt.Before(period.EndDate) || dueAt.Equal(period.EndDate)) {
				// Period contains this assignment — check if it's closed
				if period.IsClosed {
					return true, period.Title
				}
				// Check CloseDate if set
				if period.CloseDate != nil && now.After(*period.CloseDate) {
					return true, period.Title
				}
			}
		}
	}

	return false, ""
}
