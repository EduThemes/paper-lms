package service

import (
	"context"

	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/repository"
)

// CanvasAPIService serves the Canvas-compatible collection endpoints that
// scripts and integrations depend on (G138): course rosters, account
// course and user lists, and the multi-student submissions feed. Every
// method takes the tenant as a trailing accountID.
type CanvasAPIService struct {
	repo repository.CanvasAPIRepository
}

// NewCanvasAPIService wires the service.
func NewCanvasAPIService(repo repository.CanvasAPIRepository) *CanvasAPIService {
	return &CanvasAPIService{repo: repo}
}

// CourseUser is one row of GET /courses/:course_id/users.
type CourseUser struct {
	User        models.User
	Enrollments []models.Enrollment
}

// ListCourseUsers returns a page of the course roster, each user with
// their enrollments in the course (filtered to the same states).
func (s *CanvasAPIService) ListCourseUsers(ctx context.Context, courseID uint, f repository.CourseUserFilter, params repository.PaginationParams, accountID uint) ([]CourseUser, int64, error) {
	page, err := s.repo.ListCourseUsers(ctx, courseID, f, params, accountID)
	if err != nil {
		return nil, 0, err
	}
	ids := make([]uint, len(page.Items))
	for i, u := range page.Items {
		ids[i] = u.ID
	}
	enrollments, err := s.repo.CourseEnrollmentsForUsers(ctx, courseID, ids, f.EnrollmentStates, accountID)
	if err != nil {
		return nil, 0, err
	}
	byUser := map[uint][]models.Enrollment{}
	for _, e := range enrollments {
		if len(f.EnrollmentTypes) > 0 && !canvasContainsString(f.EnrollmentTypes, e.Type) {
			continue
		}
		byUser[e.UserID] = append(byUser[e.UserID], e)
	}
	out := make([]CourseUser, len(page.Items))
	for i, u := range page.Items {
		out[i] = CourseUser{User: u, Enrollments: byUser[u.ID]}
	}
	return out, page.TotalCount, nil
}

// AccountCourse is one row of GET /accounts/:account_id/courses with the
// optional include[] data resolved.
type AccountCourse struct {
	Course        models.Course
	Term          *models.EnrollmentTerm
	Teachers      []models.User
	TotalStudents int64
	// Enrollments are the EnrollmentsFor user's own enrollments.
	Enrollments []models.Enrollment
}

// AccountCourseIncludes selects the optional per-course data to load.
type AccountCourseIncludes struct {
	Term          bool
	Teachers      bool
	TotalStudents bool
	// EnrollmentsFor, when non-zero, loads that user's enrollments in
	// each course (Canvas's course "enrollments" array).
	EnrollmentsFor uint
}

// ListAccountCourses returns a page of the account's courses.
func (s *CanvasAPIService) ListAccountCourses(ctx context.Context, f repository.AccountCourseFilter, inc AccountCourseIncludes, params repository.PaginationParams, accountID uint) ([]AccountCourse, int64, error) {
	page, err := s.repo.ListAccountCourses(ctx, f, params, accountID)
	if err != nil {
		return nil, 0, err
	}
	out, err := s.decorateCourses(ctx, page.Items, inc, accountID)
	if err != nil {
		return nil, 0, err
	}
	return out, page.TotalCount, nil
}

// DecorateCourses resolves include[] data for courses loaded elsewhere
// (the course list and show endpoints).
func (s *CanvasAPIService) DecorateCourses(ctx context.Context, courses []models.Course, inc AccountCourseIncludes, accountID uint) ([]AccountCourse, error) {
	return s.decorateCourses(ctx, courses, inc, accountID)
}

func (s *CanvasAPIService) decorateCourses(ctx context.Context, courses []models.Course, inc AccountCourseIncludes, accountID uint) ([]AccountCourse, error) {
	ids := make([]uint, len(courses))
	var termIDs []uint
	for i, c := range courses {
		ids[i] = c.ID
		if c.EnrollmentTermID != nil {
			termIDs = append(termIDs, *c.EnrollmentTermID)
		}
	}
	var (
		terms    map[uint]models.EnrollmentTerm
		teachers map[uint][]models.User
		counts   map[uint]int64
		mine     map[uint][]models.Enrollment
		err      error
	)
	if inc.EnrollmentsFor != 0 {
		if mine, err = s.repo.UserEnrollmentsInCourses(ctx, inc.EnrollmentsFor, ids, accountID); err != nil {
			return nil, err
		}
	}
	if inc.Term {
		if terms, err = s.repo.TermsByID(ctx, termIDs, accountID); err != nil {
			return nil, err
		}
	}
	if inc.Teachers {
		if teachers, err = s.repo.TeachersForCourses(ctx, ids, accountID); err != nil {
			return nil, err
		}
	}
	if inc.TotalStudents {
		if counts, err = s.repo.StudentCountsForCourses(ctx, ids, accountID); err != nil {
			return nil, err
		}
	}
	out := make([]AccountCourse, len(courses))
	for i, c := range courses {
		row := AccountCourse{Course: c, Teachers: teachers[c.ID], TotalStudents: counts[c.ID], Enrollments: mine[c.ID]}
		if c.EnrollmentTermID != nil {
			if t, ok := terms[*c.EnrollmentTermID]; ok {
				t := t
				row.Term = &t
			}
		}
		out[i] = row
	}
	return out, nil
}

// ListAccountUsers returns a page of the account's users.
func (s *CanvasAPIService) ListAccountUsers(ctx context.Context, f repository.AccountUserFilter, params repository.PaginationParams, accountID uint) (*repository.PaginatedResult[models.User], error) {
	return s.repo.ListAccountUsers(ctx, f, params, accountID)
}

// StudentSubmissions is a page of GET /courses/:course_id/students/
// submissions with the rows' assignments and users for include[].
type StudentSubmissions struct {
	Submissions []models.Submission
	Total       int64
	Assignments map[uint]models.Assignment
	Users       map[uint]models.User
}

// ListStudentSubmissions returns a page of submissions of the course's
// students, plus the assignments and users the page references.
func (s *CanvasAPIService) ListStudentSubmissions(ctx context.Context, courseID uint, f repository.StudentSubmissionFilter, params repository.PaginationParams, accountID uint) (*StudentSubmissions, error) {
	page, err := s.repo.ListStudentSubmissions(ctx, courseID, f, params, accountID)
	if err != nil {
		return nil, err
	}
	var aIDs, uIDs []uint
	seenA, seenU := map[uint]bool{}, map[uint]bool{}
	for _, sub := range page.Items {
		if !seenA[sub.AssignmentID] {
			seenA[sub.AssignmentID] = true
			aIDs = append(aIDs, sub.AssignmentID)
		}
		if !seenU[sub.UserID] {
			seenU[sub.UserID] = true
			uIDs = append(uIDs, sub.UserID)
		}
	}
	assignments, err := s.repo.AssignmentsByID(ctx, courseID, aIDs, accountID)
	if err != nil {
		return nil, err
	}
	users, err := s.repo.UsersByID(ctx, uIDs, accountID)
	if err != nil {
		return nil, err
	}
	return &StudentSubmissions{Submissions: page.Items, Total: page.TotalCount, Assignments: assignments, Users: users}, nil
}

// ResolveCanvasID resolves a Canvas SIS-style alternate id (G137).
func (s *CanvasAPIService) ResolveCanvasID(ctx context.Context, kind, value string, accountID uint) (uint, error) {
	return s.repo.ResolveCanvasID(ctx, kind, value, accountID)
}

func canvasContainsString(list []string, v string) bool {
	for _, s := range list {
		if s == v {
			return true
		}
	}
	return false
}
