package service

import (
	"context"
	"sort"

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

// CourseGradingPeriods returns the active grading periods that apply to
// a course (its account's active grading period groups), for the
// gradebook's grading-period filter. accountID scopes the course.
func (s *SubmissionService) CourseGradingPeriods(ctx context.Context, courseID, accountID uint) ([]models.GradingPeriod, error) {
	course, err := s.courseRepo.FindByID(ctx, courseID, accountID)
	if err != nil || course == nil {
		return nil, ErrAssignmentUnavailable
	}
	out := make([]models.GradingPeriod, 0)
	if s.gradingPeriodGroupRepo == nil || s.gradingPeriodRepo == nil {
		return out, nil
	}
	groups, err := s.gradingPeriodGroupRepo.ListByAccountID(ctx, course.AccountID, repository.PaginationParams{Page: 1, PerPage: 100})
	if err != nil || groups == nil {
		return out, nil
	}
	for _, g := range groups.Items {
		if g.WorkflowState != "active" {
			continue
		}
		periods, err := s.gradingPeriodRepo.ListByGroupID(ctx, g.ID, 0)
		if err != nil {
			continue
		}
		for _, p := range periods {
			if p.WorkflowState == "active" {
				out = append(out, p)
			}
		}
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].StartDate.Before(out[j].StartDate) })
	return out, nil
}
