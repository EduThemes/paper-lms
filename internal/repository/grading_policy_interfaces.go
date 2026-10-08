package repository

import (
	"context"
	"time"

	"github.com/EduThemes/paper-lms/internal/domain/models"
)

// GradingPolicyRepository holds the bulk reads the grade calculator and
// due-date resolver need, plus the grading-policy tables (final grade
// overrides, submission versions). Every course-keyed method expects a
// courseID the caller already loaded under its tenant.
type GradingPolicyRepository interface {
	ListCourseAssignments(ctx context.Context, courseID uint) ([]models.Assignment, error)
	ListCourseGroups(ctx context.Context, courseID uint) ([]models.AssignmentGroup, error)
	// ListCourseSubmissions returns the course's submissions, limited to
	// userIDs when non-empty.
	ListCourseSubmissions(ctx context.Context, courseID uint, userIDs []uint) ([]models.Submission, error)
	ListAssignmentSubmissions(ctx context.Context, assignmentID uint) ([]models.Submission, error)
	ListCourseOverrides(ctx context.Context, courseID uint) ([]models.AssignmentOverride, error)
	ListCourseOverrideStudents(ctx context.Context, courseID uint) ([]models.AssignmentOverrideStudent, error)
	// ListCourseStudents returns the course's (non-deleted, non-rejected)
	// student enrollments with their users.
	ListCourseStudents(ctx context.Context, courseID uint) ([]models.Enrollment, error)
	// ListCourseAccommodations returns accommodations in effect now for
	// the users in the course (course-specific or global).
	ListCourseAccommodations(ctx context.Context, courseID uint, userIDs []uint) ([]models.StudentAccommodation, error)
	// CourseGradingPeriods returns the grading period set linked to the
	// course's enrollment term and its active periods (nil, nil when the
	// course has none).
	CourseGradingPeriods(ctx context.Context, course *models.Course) (*models.GradingPeriodGroup, []models.GradingPeriod, error)

	ListScoreOverrides(ctx context.Context, courseID uint) ([]models.CourseScoreOverride, error)
	UpsertScoreOverride(ctx context.Context, o *models.CourseScoreOverride) error
	DeleteScoreOverride(ctx context.Context, courseID, userID, gradingPeriodID uint) error

	// UpsertSubmissionVersion writes the (submission_id, attempt) row.
	UpsertSubmissionVersion(ctx context.Context, v *models.SubmissionVersion) error
	// UpdateSubmissionVersionGrade copies a grade onto the attempt's row.
	UpdateSubmissionVersionGrade(ctx context.Context, sub *models.Submission) error
	ListSubmissionVersions(ctx context.Context, submissionID uint) ([]models.SubmissionVersion, error)

	// SetAssignmentsPostPolicy sets post_policy on every non-deleted,
	// non-anonymous assignment of the course.
	SetAssignmentsPostPolicy(ctx context.Context, courseID uint, policy string) error
	SetCoursePostManually(ctx context.Context, courseID uint, postManually bool) error
	// SetSubmissionsPostedAt sets posted_at on the assignment's
	// submissions with the given ids.
	SetSubmissionsPostedAt(ctx context.Context, assignmentID uint, submissionIDs []uint, postedAt *time.Time) error

	// UpdateSubmissionStatus persists the computed due-date fields.
	UpdateSubmissionStatus(ctx context.Context, sub *models.Submission) error
	// ListAssignmentsDueBetween returns published assignments of active
	// courses whose own due date, or an active override's, falls in
	// [from, to].
	ListAssignmentsDueBetween(ctx context.Context, from, to time.Time) ([]models.Assignment, error)
	FindCourse(ctx context.Context, courseID uint) (*models.Course, error)
	FindLatePolicy(ctx context.Context, courseID uint) (*models.LatePolicy, error)
	CreateSubmission(ctx context.Context, sub *models.Submission) error
}
