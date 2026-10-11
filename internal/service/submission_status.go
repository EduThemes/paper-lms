package service

// Late / missing statuses and the missing-submission policy (G022),
// following Canvas's Submission::Tardiness rules:
//
//   - excused work is never late or missing;
//   - a manually set late_policy_status wins (late / missing; extended and
//     none clear both);
//   - otherwise work is late when submitted after the student's
//     effective due date, and missing when nothing was submitted, the
//     due date has passed, the assignment expects an online submission,
//     and no grader has graded it.
//
// The missing policy grades a missing submission with
// points_possible * (100 - missing_submission_deduction)% when the
// course late policy enables it and the submission has no score yet.

import (
	"context"
	"errors"
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/EduThemes/paper-lms/internal/domain/models"
)

// noSubmissionTypes are submission types that do not expect the student
// to hand anything in online.
var noSubmissionTypes = map[string]bool{
	"none": true, "on_paper": true, "not_graded": true, "wiki_page": true, "external_tool": true,
}

// ExpectsSubmission reports whether an assignment expects online work
// (Canvas Assignment#expects_submission?, plus quizzes).
func ExpectsSubmission(a *models.Assignment) bool {
	if a == nil || strings.TrimSpace(a.SubmissionTypes) == "" {
		return false
	}
	for _, t := range strings.Split(a.SubmissionTypes, ",") {
		t = strings.TrimSpace(t)
		if t != "" && !noSubmissionTypes[t] {
			return true
		}
	}
	return false
}

// TardinessResult is the computed late/missing status of one submission.
type TardinessResult struct {
	Late        bool
	Missing     bool
	Extended    bool
	SecondsLate int64
}

// ComputeTardiness applies the Canvas rules to sub against the student's
// effective due date.
func ComputeTardiness(a *models.Assignment, sub *models.Submission, dueAt *time.Time, now time.Time) TardinessResult {
	var t TardinessResult
	if sub == nil || sub.Excused {
		return t
	}
	status := ""
	if sub.LatePolicyStatus != nil {
		status = *sub.LatePolicyStatus
	}
	switch status {
	case models.LatePolicyStatusLate:
		t.Late = true
		if sub.SecondsLateOverride != nil && *sub.SecondsLateOverride > 0 {
			t.SecondsLate = *sub.SecondsLateOverride
		}
		return t
	case models.LatePolicyStatusMissing:
		t.Missing = true
		return t
	case models.LatePolicyStatusExtended:
		t.Extended = true
		return t
	case models.LatePolicyStatusNone:
		return t
	}
	if dueAt != nil {
		at := now
		if sub.SubmittedAt != nil {
			at = *sub.SubmittedAt
		}
		if at.After(*dueAt) {
			t.SecondsLate = int64(at.Sub(*dueAt) / time.Second)
		}
	}
	if sub.SubmittedAt != nil {
		t.Late = t.SecondsLate > 0
		return t
	}
	if sub.GraderID != nil && sub.Score != nil {
		return t // graded by an instructor: not missing
	}
	t.Missing = t.SecondsLate > 0 && ExpectsSubmission(a)
	return t
}

// LateDeductionFor is the late-policy deduction for an entered score
// given the student's effective due date: the bulk twin of
// SubmissionService.latePointsDeducted, used when due dates or the late
// policy change after grading (Canvas LatePolicyApplicator). A manual
// "late" status uses the seconds-late override; none / extended /
// missing take no deduction; otherwise lateness is submitted_at past the
// effective due date.
func LateDeductionFor(policy *models.LatePolicy, a *models.Assignment, sub *models.Submission, entered float64, dueAt *time.Time) float64 {
	if policy == nil || a == nil || sub == nil || sub.Excused {
		return 0
	}
	gradingType := a.GradingType
	if gradingType == "" {
		gradingType = "points"
	}
	if !pointDeductibleGradingTypes[gradingType] || a.PointsPossible == nil || *a.PointsPossible <= 0 {
		return 0
	}
	possible := *a.PointsPossible
	if sub.LatePolicyStatus != nil {
		if *sub.LatePolicyStatus != models.LatePolicyStatusLate || sub.SecondsLateOverride == nil || *sub.SecondsLateOverride <= 0 {
			return 0
		}
		return LatePointsDeducted(policy, entered, possible, time.Duration(*sub.SecondsLateOverride)*time.Second)
	}
	if sub.SubmittedAt == nil || dueAt == nil {
		return 0
	}
	lateFor := sub.SubmittedAt.Sub(*dueAt)
	if lateFor <= 0 {
		return 0
	}
	return LatePointsDeducted(policy, entered, possible, lateFor)
}

// MissingPoints is Canvas LatePolicy#points_for_missing.
func MissingPoints(policy *models.LatePolicy, a *models.Assignment) (float64, bool) {
	if policy == nil || !policy.MissingSubmissionDeductionEnabled || a == nil {
		return 0, false
	}
	possible := 0.0
	if a.PointsPossible != nil {
		possible = *a.PointsPossible
	}
	gradingType := a.GradingType
	if gradingType == "not_graded" {
		return 0, false
	}
	deducted := possible * policy.MissingSubmissionDeduction / 100
	if gradingType == "pass_fail" {
		deducted = possible
	}
	return math.Round((possible-deducted)*100) / 100, true
}

// ErrInvalidLateStatus is returned for an unknown late_policy_status.
var ErrInvalidLateStatus = errors.New("late_policy_status must be none, missing, late or extended")

// SetLatePolicyStatus records an instructor-chosen status on a student's
// submission (creating the row when the student has none), recomputes
// the late deduction from the entered score, and applies the missing
// policy to an ungraded submission marked missing. status "" clears the
// manual status (back to computed).
func (s *SubmissionService) SetLatePolicyStatus(ctx context.Context, assignmentID, userID, graderID, accountID uint, status string, secondsLate *int64) (*models.Submission, error) {
	if status != "" && !models.ValidLatePolicyStatus(status) {
		return nil, ErrInvalidLateStatus
	}
	assignment, err := s.assignmentRepo.FindByID(ctx, assignmentID, accountID)
	if err != nil {
		return nil, ErrGradeCrossTenant
	}
	if closed, periodTitle := s.isGradingPeriodClosedFor(ctx, assignment, userID); closed {
		return nil, fmt.Errorf("grading period %q is closed — grades cannot be modified", periodTitle)
	}
	sub, findErr := s.submissionRepo.FindByAssignmentAndUser(ctx, assignmentID, userID, accountID)
	isNew := findErr != nil || sub == nil
	if isNew {
		sub = &models.Submission{AssignmentID: assignmentID, UserID: userID, WorkflowState: models.SubmissionUnsubmitted}
	}
	oldGrade, oldScore := sub.Grade, sub.Score

	if status == "" {
		sub.LatePolicyStatus = nil
	} else {
		st := status
		sub.LatePolicyStatus = &st
		// Canvas: a status replaces excused.
		sub.Excused = false
	}
	sub.SecondsLateOverride = nil
	if status == models.LatePolicyStatusLate && secondsLate != nil && *secondsLate > 0 {
		v := *secondsLate
		sub.SecondsLateOverride = &v
	}

	dueAt, _, _ := s.EffectiveDates(ctx, assignment, userID)
	sub.CachedDueDate = dueAt
	now := time.Now()
	t := ComputeTardiness(assignment, sub, dueAt, now)
	sub.Late, sub.Missing = t.Late, t.Missing

	switch {
	case sub.EnteredScore != nil:
		// Re-derive the late deduction from what the grader typed.
		s.applyGrade(ctx, assignment, sub, *sub.EnteredScore, graderID, now)
		sub.Late, sub.Missing = t.Late, t.Missing
	case t.Missing && sub.Score == nil:
		s.applyMissingPolicy(ctx, assignment, sub, now)
	}

	if isNew {
		if err := s.submissionRepo.Create(ctx, sub); err != nil {
			return nil, err
		}
	} else if err := s.submissionRepo.Update(ctx, sub); err != nil {
		return nil, err
	}
	if sub.Score != nil && (oldScore == nil || *oldScore != *sub.Score) {
		s.logGradeChange(ctx, assignment, sub, graderID, oldGrade, oldScore)
		s.fireIfPosted(sub)
	}
	return sub, nil
}

// applyMissingPolicy grades a missing, ungraded submission per the course
// late policy. It never overrides an existing score. Returns whether a
// score was applied.
func (s *SubmissionService) applyMissingPolicy(ctx context.Context, assignment *models.Assignment, sub *models.Submission, now time.Time) bool {
	if sub == nil || sub.Score != nil || sub.Excused || s.latePolicyRepo == nil {
		return false
	}
	policy, err := s.latePolicyRepo.FindByCourseID(ctx, assignment.CourseID)
	if err != nil || policy == nil {
		return false
	}
	points, ok := MissingPoints(policy, assignment)
	if !ok {
		return false
	}
	applyMissingScore(assignment, s.resolveScheme(ctx, assignment), sub, points, now)
	return true
}

// applyMissingScore writes a missing-policy score: graded, no grader (it
// stays "missing"), posted unless the assignment posts manually.
func applyMissingScore(assignment *models.Assignment, scheme *GradingScheme, sub *models.Submission, points float64, now time.Time) {
	score := points
	entered := points
	grade := FormatGrade(assignment, scheme, points, "")
	sub.Score = &score
	sub.EnteredScore = &entered
	sub.Grade = &grade
	g2 := grade
	sub.EnteredGrade = &g2
	sub.PointsDeducted = nil
	sub.WorkflowState = models.SubmissionGraded
	at := now
	sub.GradedAt = &at
	if !assignment.UsesManualPostPolicy() && sub.PostedAt == nil {
		posted := now
		sub.PostedAt = &posted
	}
}
