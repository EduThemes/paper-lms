package service

import (
	"context"
	"time"

	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/repository"
)

// CoursePeriodResolver returns the active grading periods of a course's
// grading period set (the set linked to its enrollment term) and whether
// the set is weighted.
type CoursePeriodResolver interface {
	CoursePeriods(ctx context.Context, courseID uint) (periods []models.GradingPeriod, weighted bool, err error)
}

// ResolveGradingScheme returns the grading scheme for an assignment (a
// letter_grade / gpa_scale assignment may pick its own) or, with a nil
// assignment, the course: the course's selected scheme, else its newest
// course scheme, else the default scale. A scheme is only honoured when
// it belongs to the course or the course's account.
func ResolveGradingScheme(ctx context.Context, repo repository.GradingStandardRepository, course *models.Course, a *models.Assignment) *GradingScheme {
	if repo == nil {
		return DefaultGradingScheme()
	}
	try := func(id *uint) *GradingScheme {
		if id == nil || *id == 0 {
			return nil
		}
		gs, err := repo.FindByID(ctx, *id)
		if err != nil || gs == nil || gs.WorkflowState == "deleted" || !StandardVisibleToCourse(gs, course) {
			return nil
		}
		return SchemeFromStandard(gs)
	}
	if a != nil {
		if sch := try(a.GradingStandardID); sch != nil {
			return sch
		}
	}
	if course != nil {
		if sch := try(course.GradingStandardID); sch != nil {
			return sch
		}
		if gs, err := repo.FindActiveByCourse(ctx, course.ID); err == nil && gs != nil {
			if sch := SchemeFromStandard(gs); sch != nil {
				return sch
			}
		}
	}
	return DefaultGradingScheme()
}

// StandardVisibleToCourse reports whether a grading standard may be used
// by the course: its own schemes and its account's.
func StandardVisibleToCourse(gs *models.GradingStandard, course *models.Course) bool {
	if gs == nil || course == nil {
		return false
	}
	switch gs.ContextType {
	case "Course":
		return gs.ContextID == course.ID
	case "Account":
		return gs.ContextID == course.AccountID
	}
	return false
}

// markSubmitted refreshes the status of a just-submitted submission:
// handing work in clears a manual "missing" status, and lateness follows
// the student's effective due date (or the manual late status).
func markSubmitted(a *models.Assignment, sub *models.Submission, dueAt *time.Time, at time.Time) {
	if sub.LatePolicyStatus != nil && *sub.LatePolicyStatus == models.LatePolicyStatusMissing {
		sub.LatePolicyStatus = nil
	}
	t := ComputeTardiness(a, sub, dueAt, at)
	sub.Late = t.Late
	sub.Missing = false
}

// recordAttempt stores the submission's current attempt in its history.
func (s *SubmissionService) recordAttempt(ctx context.Context, sub *models.Submission) {
	if s.policyRepo == nil || sub == nil || sub.ID == 0 || sub.SubmittedAt == nil {
		return
	}
	attempt := sub.Attempt
	if attempt < 1 {
		attempt = 1
	}
	v := &models.SubmissionVersion{
		SubmissionID:   sub.ID,
		Attempt:        attempt,
		AssignmentID:   sub.AssignmentID,
		UserID:         sub.UserID,
		SubmissionType: sub.SubmissionType,
		Body:           sub.Body,
		URL:            sub.URL,
		Attachments:    sub.Attachments,
		SubmittedAt:    sub.SubmittedAt,
		Late:           sub.Late,
		Score:          sub.Score,
		Grade:          sub.Grade,
		EnteredScore:   sub.EnteredScore,
		GraderID:       sub.GraderID,
		GradedAt:       sub.GradedAt,
	}
	_ = s.policyRepo.UpsertSubmissionVersion(ctx, v)
}

// recordAttemptGrade copies a grade onto the graded attempt's history row.
func (s *SubmissionService) recordAttemptGrade(ctx context.Context, sub *models.Submission) {
	if s.policyRepo == nil || sub == nil {
		return
	}
	_ = s.policyRepo.UpdateSubmissionVersionGrade(ctx, sub)
}

// resolveScheme returns the scheme for grading the assignment.
func (s *SubmissionService) resolveScheme(ctx context.Context, a *models.Assignment) *GradingScheme {
	if a == nil || s.gradingStandardRepo == nil {
		return DefaultGradingScheme()
	}
	var course *models.Course
	if s.courseRepo != nil {
		if c, err := s.courseRepo.FindByID(ctx, a.CourseID, 0); err == nil {
			course = c
		}
	}
	return ResolveGradingScheme(ctx, s.gradingStandardRepo, course, a)
}

// isGradingPeriodClosedFor reports whether the student's effective due
// date on the assignment falls in a closed grading period of the
// course's own grading period set (G025). Without a resolver it falls
// back to the legacy account-wide check.
func (s *SubmissionService) isGradingPeriodClosedFor(ctx context.Context, a *models.Assignment, userID uint) (bool, string) {
	if a == nil {
		return false, ""
	}
	if s.periodResolver == nil {
		return s.isGradingPeriodClosed(ctx, a)
	}
	periods, _, err := s.periodResolver.CoursePeriods(ctx, a.CourseID)
	if err != nil || len(periods) == 0 {
		return false, ""
	}
	due, _, _ := s.EffectiveDates(ctx, a, userID)
	p := GradingPeriodForDate(periods, due)
	if GradingPeriodClosed(p, time.Now()) {
		return true, p.Title
	}
	return false, ""
}
