package repository

import (
	"context"

	"github.com/EduThemes/paper-lms/internal/domain/models"
)

// EnrollmentAccessReader is an OPTIONAL extension of EnrollmentRepository
// implemented by the Postgres enrollment repository. The course access
// path (PermissionMiddleware, ResourceAuthorizer) type-asserts for it and
// falls back to FindByUserAndCourse when it is absent (test doubles), so
// adding it did not widen the core interface every mock implements.
type EnrollmentAccessReader interface {
	// LoadCourseAccess returns every enrollment the user holds in the
	// course that can still grant access (active, invited, completed,
	// inactive), their sections and the course's term. accountID scopes
	// through courses.account_id (0 = no tenant scope; auth-internal).
	LoadCourseAccess(ctx context.Context, userID, courseID, accountID uint) (*models.CourseAccessData, error)
	// UserInCourseSections reports whether the user has a non-deleted
	// enrollment in the course in one of sectionIDs (0 = no section) or
	// is one of its staff (teacher/TA/designer, visible to everyone).
	UserInCourseSections(ctx context.Context, userID, courseID uint, sectionIDs []uint) (bool, error)
	// UserIDsInSections returns the distinct user ids with a non-deleted
	// enrollment in the course in one of sectionIDs (0 = no section),
	// plus the course's staff.
	UserIDsInSections(ctx context.Context, courseID uint, sectionIDs []uint) ([]uint, error)
}

// EnrollmentListFilter narrows a course roster listing.
type EnrollmentListFilter struct {
	// States limits workflow_state (empty = every state but deleted).
	States []string
	// SectionIDs limits to these sections (0 = no section). Nil = all.
	SectionIDs []uint
	// AlwaysIncludeTypes keeps enrollments of these types even when
	// they fall outside SectionIDs (a section-limited student still
	// sees the course's teachers).
	AlwaysIncludeTypes []string
}

// EnrollmentAdminRepository is the enrollment-lifecycle surface used by
// EnrollmentLifecycleService and SectionService. Implemented by the same
// Postgres repository as EnrollmentRepository.
type EnrollmentAdminRepository interface {
	EnrollmentAccessReader
	// FindInCourse loads one enrollment of the course (any state),
	// tenant-scoped; gorm.ErrRecordNotFound when it's elsewhere.
	FindInCourse(ctx context.Context, id, courseID, accountID uint) (*models.Enrollment, error)
	// ListForUserInCourse returns the user's non-deleted enrollments in
	// the course, any state.
	ListForUserInCourse(ctx context.Context, userID, courseID, accountID uint) ([]models.Enrollment, error)
	// ListByCourseFiltered is the roster listing with state/section
	// filters.
	ListByCourseFiltered(ctx context.Context, courseID, accountID uint, f EnrollmentListFilter, params PaginationParams) (*PaginatedResult[models.Enrollment], error)
	// ListInvitedForUser returns the user's pending course invitations
	// with Course preloaded.
	ListInvitedForUser(ctx context.Context, userID, accountID uint) ([]models.Enrollment, error)
	// CountLiveInSection counts non-deleted enrollments in a section.
	CountLiveInSection(ctx context.Context, sectionID uint) (int64, error)
	// Save writes the full row.
	Save(ctx context.Context, e *models.Enrollment) error
}

// SectionAdminRepository is the section-management surface (G109/G108).
type SectionAdminRepository interface {
	// FindInAccount loads a section whose course is in accountID
	// (0 = no tenant scope; super_admin / internal callers only).
	FindInAccount(ctx context.Context, id, accountID uint) (*models.CourseSection, error)
	// ListByCourse returns every non-deleted section of a course.
	ListByCourse(ctx context.Context, courseID uint) ([]models.CourseSection, error)
	Save(ctx context.Context, section *models.CourseSection) error
	// MoveToCourse cross-lists (or de-cross-lists) a section in one
	// transaction: the section's course_id and nonxlist_course_id are set
	// and every enrollment in the section follows it to toCourseID.
	MoveToCourse(ctx context.Context, section *models.CourseSection, toCourseID uint, nonxlistCourseID *uint) error
}
