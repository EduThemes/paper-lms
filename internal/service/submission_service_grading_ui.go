package service

import (
	"context"
	"sort"
	"strings"

	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/repository"
)

// CourseStudent is one active student of a course.
type CourseStudent struct {
	UserID uint
	Name   string
}

// CourseStudents lists the course's students (one entry per user, by
// enrollment order, the same set SpeedGrader shows), tenant-scoped by
// accountID.
func (s *SubmissionService) CourseStudents(ctx context.Context, courseID, accountID uint) ([]CourseStudent, error) {
	res, err := s.enrollmentRepo.ListByCourseID(ctx, courseID, accountID, repository.PaginationParams{Page: 1, PerPage: 1000})
	if err != nil {
		return nil, err
	}
	seen := map[uint]bool{}
	out := make([]CourseStudent, 0, len(res.Items))
	for _, e := range res.Items {
		if e.Type != "StudentEnrollment" || seen[e.UserID] {
			continue
		}
		seen[e.UserID] = true
		name := ""
		if e.User != nil {
			name = e.User.Name
		}
		out = append(out, CourseStudent{UserID: e.UserID, Name: name})
	}
	return out, nil
}

// SortForAnonymity orders students the way SpeedGrader numbers them
// under anonymous grading ("Student 1", "Student 2", ...): a stable
// per-assignment pseudo-random order that does not follow enrollment.
func SortForAnonymity(assignmentID uint, students []CourseStudent) {
	sort.SliceStable(students, func(i, j int) bool {
		return anonymousSortKey(assignmentID, students[i].UserID) < anonymousSortKey(assignmentID, students[j].UserID)
	})
}

// CourseGradingPeriods returns the grading periods that apply to a
// course, for the gradebook's grading-period filter: the active periods
// of the period set linked to the course's term, the same set the course
// grade calculator, final grade overrides and the student Grades page
// use. accountID scopes the course.
func (s *SubmissionService) CourseGradingPeriods(ctx context.Context, courseID, accountID uint) ([]models.GradingPeriod, error) {
	course, err := s.courseRepo.FindByID(ctx, courseID, accountID)
	if err != nil || course == nil {
		return nil, ErrAssignmentUnavailable
	}
	out := make([]models.GradingPeriod, 0)
	if s.periodResolver == nil {
		return out, nil
	}
	periods, _, err := s.periodResolver.CoursePeriods(ctx, course.ID)
	if err != nil {
		return nil, err
	}
	out = append(out, periods...)
	sort.SliceStable(out, func(i, j int) bool { return out[i].StartDate.Before(out[j].StartDate) })
	return out, nil
}

// RecordExternalToolActivity turns an LTI tool's report that the student
// submitted (AGS activityProgress Submitted / Completed) into a
// basic_lti_launch submission on an external_tool assignment, the way
// Canvas does, unless the student has already submitted. It is a no-op
// for assignments that are not external_tool ones. Lock windows and
// attempt limits still apply (an error means no submission was made).
func (s *SubmissionService) RecordExternalToolActivity(ctx context.Context, assignment *models.Assignment, userID uint) error {
	if assignment == nil || !assignmentHasType(assignment, "external_tool") {
		return nil
	}
	if existing, err := s.submissionRepo.FindByAssignmentAndUser(ctx, assignment.ID, userID, 0); err == nil && existing != nil && existing.SubmittedAt != nil {
		return nil
	}
	_, err := s.RecordExternalToolSubmission(ctx, assignment, userID, "")
	return err
}

// assignmentHasType reports whether t is one of a's submission types.
func assignmentHasType(a *models.Assignment, t string) bool {
	for _, st := range strings.Split(a.SubmissionTypes, ",") {
		if strings.TrimSpace(st) == t {
			return true
		}
	}
	return false
}
