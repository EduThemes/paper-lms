package repository

import (
	"context"
	"time"

	"github.com/EduThemes/paper-lms/internal/domain/models"
)

// CourseUserFilter narrows GET /courses/:course_id/users (Canvas
// courses#users).
type CourseUserFilter struct {
	// EnrollmentTypes are model enrollment types (StudentEnrollment, ...).
	// Empty means every type.
	EnrollmentTypes []string
	// EnrollmentStates are enrollment workflow states. Empty means
	// active + invited (Canvas's default).
	EnrollmentStates []string
	UserIDs          []uint
	SearchTerm       string
	// NameSearchOnly limits SearchTerm to the user's names. Set for callers
	// who may not see login ids, SIS ids or emails (students, observers):
	// matching on those fields would let them confirm a classmate's email
	// or SIS id one guess at a time.
	NameSearchOnly bool
	// Sort is one of "username" (default), "email", "sis_id", "id".
	Sort string
}

// AccountCourseFilter narrows GET /accounts/:account_id/courses (Canvas
// accounts#courses_api).
type AccountCourseFilter struct {
	SearchTerm       string
	EnrollmentTermID *uint
	// Published: nil = any, true = available/completed, false = not yet
	// published.
	Published *bool
	// Completed: nil = any, true = only concluded, false = exclude
	// concluded.
	Completed *bool
	// States are Canvas course states (created, claimed, available,
	// completed, deleted). Empty means every state except deleted.
	States []string
	// WithEnrollments: nil = any, true = only courses with at least one
	// active enrollment, false = only courses with none.
	WithEnrollments *bool
	// ByTeachers restricts to courses where one of these users teaches.
	ByTeachers []uint
	// Sort is "course_name" (default), "sis_course_id", "teacher",
	// "term", "created_at" or "id"; Order is "asc" or "desc".
	Sort  string
	Order string
}

// AccountUserFilter narrows GET /accounts/:account_id/users.
type AccountUserFilter struct {
	SearchTerm string
	// Sort is "username" (default), "email", "sis_id", "last_login" or
	// "id"; Order is "asc" or "desc".
	Sort  string
	Order string
}

// StudentSubmissionFilter narrows GET /courses/:course_id/students/
// submissions (Canvas submissions_api#for_students).
type StudentSubmissionFilter struct {
	// StudentIDs restricts to these users; empty means every student of
	// the course (the caller must be staff).
	StudentIDs    []uint
	AssignmentIDs []uint
	// WorkflowState filters on submissions.workflow_state when non-empty.
	WorkflowState  string
	SubmittedSince *time.Time
	GradedSince    *time.Time
	// IncludeUnpublished keeps submissions of unpublished assignments
	// (staff only).
	IncludeUnpublished bool
	// Order is "id" (default) or "graded_at"; OrderDirection "ascending"
	// or "descending".
	Order          string
	OrderDirection string
}

// CanvasAPIRepository backs the Canvas-compatible collection endpoints
// scripts and integrations call (G137/G138). Every method is scoped to a
// tenant via a trailing accountID; 0 means unscoped and is reserved for
// platform super admins.
type CanvasAPIRepository interface {
	// ResolveCanvasID turns a Canvas SIS-style alternate id into a row id.
	ResolveCanvasID(ctx context.Context, kind, value string, accountID uint) (uint, error)

	ListCourseUsers(ctx context.Context, courseID uint, f CourseUserFilter, params PaginationParams, accountID uint) (*PaginatedResult[models.User], error)
	// CourseEnrollmentsForUsers returns the enrollments of userIDs in the
	// course, restricted to states (empty = active + invited).
	CourseEnrollmentsForUsers(ctx context.Context, courseID uint, userIDs []uint, states []string, accountID uint) ([]models.Enrollment, error)

	ListAccountCourses(ctx context.Context, f AccountCourseFilter, params PaginationParams, accountID uint) (*PaginatedResult[models.Course], error)
	// TeachersForCourses returns active teachers keyed by course id.
	TeachersForCourses(ctx context.Context, courseIDs []uint, accountID uint) (map[uint][]models.User, error)
	// StudentCountsForCourses returns active student counts keyed by
	// course id.
	StudentCountsForCourses(ctx context.Context, courseIDs []uint, accountID uint) (map[uint]int64, error)
	// TermsByID returns the enrollment terms with these ids.
	TermsByID(ctx context.Context, ids []uint, accountID uint) (map[uint]models.EnrollmentTerm, error)
	// UserEnrollmentsInCourses returns userID's non-deleted enrollments in
	// these courses, keyed by course id.
	UserEnrollmentsInCourses(ctx context.Context, userID uint, courseIDs []uint, accountID uint) (map[uint][]models.Enrollment, error)

	ListAccountUsers(ctx context.Context, f AccountUserFilter, params PaginationParams, accountID uint) (*PaginatedResult[models.User], error)

	ListStudentSubmissions(ctx context.Context, courseID uint, f StudentSubmissionFilter, params PaginationParams, accountID uint) (*PaginatedResult[models.Submission], error)
	// AssignmentsByID returns the course's assignments with these ids.
	AssignmentsByID(ctx context.Context, courseID uint, ids []uint, accountID uint) (map[uint]models.Assignment, error)
	// UsersByID returns the users with these ids.
	UsersByID(ctx context.Context, ids []uint, accountID uint) (map[uint]models.User, error)
}
