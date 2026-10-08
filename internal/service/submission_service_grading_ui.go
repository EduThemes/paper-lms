package service

import (
	"context"
	"sort"

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
