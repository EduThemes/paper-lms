package repository

import (
	"context"
	"errors"

	"github.com/EduThemes/paper-lms/internal/domain/models"
)

type CourseRepository interface {
	Create(ctx context.Context, course *models.Course) error
	// FindByID — 13.1.D: tenant-scoped. accountID==0 means "no scope"
	// and is permitted only from internal callers that have already
	// validated tenant ownership upstream (e.g. background workers).
	// Handler-layer callers MUST pass the caller's account_id.
	FindByID(ctx context.Context, id, accountID uint) (*models.Course, error)
	FindBySISCourseID(ctx context.Context, sisCourseID string) (*models.Course, error)
	Update(ctx context.Context, course *models.Course) error
	Delete(ctx context.Context, id uint) error
	List(ctx context.Context, accountID uint, params PaginationParams) (*PaginatedResult[models.Course], error)
	ListByUserID(ctx context.Context, userID, accountID uint, params PaginationParams) (*PaginatedResult[models.Course], error)
}

type SectionRepository interface {
	Create(ctx context.Context, section *models.CourseSection) error
	FindByID(ctx context.Context, id uint) (*models.CourseSection, error)
	FindBySISSectionID(ctx context.Context, sisSectionID string) (*models.CourseSection, error)
	ListByCourseID(ctx context.Context, courseID uint, params PaginationParams) (*PaginatedResult[models.CourseSection], error)
}

type EnrollmentRepository interface {
	Create(ctx context.Context, enrollment *models.Enrollment) error
	FindByID(ctx context.Context, id uint) (*models.Enrollment, error)
	Update(ctx context.Context, enrollment *models.Enrollment) error
	ListByCourseID(ctx context.Context, courseID uint, params PaginationParams) (*PaginatedResult[models.Enrollment], error)
	ListByUserID(ctx context.Context, userID uint) ([]models.Enrollment, error)
	FindByUserAndCourse(ctx context.Context, userID, courseID uint) (*models.Enrollment, error)
	CountByCourseIDs(ctx context.Context, courseIDs []uint) (map[uint]int64, error)
	// ListActiveStudentUserIDsByCourse (W3-A) returns user_ids of active
	// StudentEnrollment rows for a course — the leaderboard candidate
	// set. Uses idx_enrollments_course_active (migration 000042).
	ListActiveStudentUserIDsByCourse(ctx context.Context, courseID uint) ([]uint, error)
	// ListActiveStudentEnrollmentsByCourse (W3-B) returns full
	// Enrollment rows for the same set — needed when the caller
	// also has to read per-enrollment pseudonym fields rather than
	// just user_ids.
	ListActiveStudentEnrollmentsByCourse(ctx context.Context, courseID uint) ([]models.Enrollment, error)
	// UpdatePseudonymForSelf (W3-B) writes a learner-chosen pseudonym
	// to their enrollment row in the given course. Returns
	// repository.ErrPseudonymTaken on UNIQUE collision so the handler
	// can map it to a 409.
	UpdatePseudonymForSelf(ctx context.Context, userID, courseID uint, poolCode, name string) error
}

// ErrPseudonymTaken indicates that another active enrollment in the
// same course already has the requested pseudonym name in the same
// pool. The handler maps this to 409 so the picker UI can offer the
// learner a re-roll.
var ErrPseudonymTaken = errors.New("pseudonym already taken in this course pool")
