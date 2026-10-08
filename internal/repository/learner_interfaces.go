package repository

import (
	"context"
	"errors"
	"time"

	"github.com/EduThemes/paper-lms/internal/domain/models"
)

// ErrLearnerNotFound is returned by LearnerRepository lookups that match no
// row (or a row owned by someone else — callers map both to a 404).
var ErrLearnerNotFound = errors.New("not found")

// LearnerRepository is the read model behind the student / observer
// experience: dashboard cards, the planner, the activity stream, missing
// work, the student grade summary, observer alerts and profile links. It
// reads across several aggregates in a handful of set-based queries so a
// dashboard load is not one query per course.
//
// Every course-keyed method takes the course ids the caller already
// resolved from the user's own tenant-scoped enrollments; none of them
// accepts a course id straight from request input.
type LearnerRepository interface {
	// Courses
	CoursesByIDs(ctx context.Context, ids []uint, accountID uint) ([]models.Course, error)
	EnrollmentTermsByIDs(ctx context.Context, ids []uint) ([]models.EnrollmentTerm, error)
	GradingPeriodsForGroup(ctx context.Context, groupID uint) ([]models.GradingPeriod, error)

	// Dashboard course preferences
	ListCoursePreferences(ctx context.Context, userID uint) ([]models.UserCoursePreference, error)
	GetCoursePreference(ctx context.Context, userID, courseID uint) (*models.UserCoursePreference, error)
	SaveCoursePreference(ctx context.Context, pref *models.UserCoursePreference) error

	// Plannables
	PublishedAssignments(ctx context.Context, courseIDs []uint) ([]models.Assignment, error)
	PublishedQuizzes(ctx context.Context, courseIDs []uint) ([]models.Quiz, error)
	AssignmentGroups(ctx context.Context, courseID uint) ([]models.AssignmentGroup, error)
	// OverrideDueDates returns, per assignment, the due dates of every
	// active override that applies to the user (as an ad-hoc student or
	// through one of sectionIDs). An entry with a nil element is an
	// override without a due date.
	OverrideDueDates(ctx context.Context, userID uint, sectionIDs, assignmentIDs []uint) (map[uint][]*time.Time, error)
	SubmissionsForUser(ctx context.Context, userID uint, assignmentIDs []uint) ([]models.Submission, error)
	LatestQuizSubmissions(ctx context.Context, userID uint, quizIDs []uint) (map[uint]models.QuizSubmission, error)
	DiscussionTopicsForAssignments(ctx context.Context, assignmentIDs []uint) (map[uint]uint, error)

	// Activity
	RecentlyPostedSubmissions(ctx context.Context, userID uint, courseIDs []uint, since time.Time) ([]models.Submission, error)
	RecentCommentsOnSubmissions(ctx context.Context, userID uint, courseIDs []uint, since time.Time) ([]SubmissionCommentActivity, error)
	RecentDiscussionTopics(ctx context.Context, courseIDs []uint, since time.Time) ([]models.DiscussionTopic, error)
	RecentAnnouncements(ctx context.Context, courseIDs []uint, since time.Time) ([]models.Announcement, error)

	// Grade summary
	SubmissionComments(ctx context.Context, submissionIDs []uint) ([]models.SubmissionComment, error)
	RubricAssessmentsForUser(ctx context.Context, userID uint, assignmentIDs []uint) ([]RubricAssessmentWithRubric, error)
	UsersByIDs(ctx context.Context, ids []uint) ([]models.User, error)

	// Observer alerts
	ListAlertThresholds(ctx context.Context, observerID uint, studentID *uint) ([]models.ObserverAlertThreshold, error)
	ListAllAlertThresholds(ctx context.Context) ([]models.ObserverAlertThreshold, error)
	GetAlertThreshold(ctx context.Context, id, observerID uint) (*models.ObserverAlertThreshold, error)
	SaveAlertThreshold(ctx context.Context, t *models.ObserverAlertThreshold) error
	DeleteAlertThreshold(ctx context.Context, id, observerID uint) error
	TouchAlertThreshold(ctx context.Context, id uint, at time.Time) error
	// CreateAlertIfAbsent inserts the alert unless one already exists for
	// the same (threshold, context); created reports whether it was new.
	CreateAlertIfAbsent(ctx context.Context, a *models.ObserverAlert) (created bool, err error)
	// ClearHandledAlert removes a read / dismissed alert for the context so
	// the condition can alert again the next time it occurs (course grade
	// alerts re-arm once the grade recovers). Unread alerts are kept.
	ClearHandledAlert(ctx context.Context, thresholdID uint, contextType string, contextID uint) error
	ListAlerts(ctx context.Context, observerID uint, studentID *uint, limit int) ([]models.ObserverAlert, error)
	CountUnreadAlerts(ctx context.Context, observerID uint) (int64, error)
	GetAlert(ctx context.Context, id, observerID uint) (*models.ObserverAlert, error)
	UpdateAlertState(ctx context.Context, id uint, state models.ObserverAlertWorkflow) error

	// Profile links
	ListProfileLinks(ctx context.Context, userID uint) ([]models.UserProfileLink, error)
	ReplaceProfileLinks(ctx context.Context, userID uint, links []models.UserProfileLink) error
	// UsersShareCourse reports whether a and b hold an active enrollment
	// in a common course.
	UsersShareCourse(ctx context.Context, a, b uint) (bool, error)
}

// SubmissionCommentActivity is a comment on one of the user's submissions,
// with the ids the activity stream needs to link to it.
type SubmissionCommentActivity struct {
	CommentID    uint
	SubmissionID uint
	AssignmentID uint
	CourseID     uint
	AuthorID     uint
	Comment      string
	CreatedAt    time.Time
}

// RubricAssessmentWithRubric pairs a grading rubric assessment with the
// rubric's criteria and the association's display flags.
type RubricAssessmentWithRubric struct {
	AssignmentID   uint
	Assessment     models.RubricAssessment
	RubricTitle    string
	RubricData     string
	PointsPossible float64
	HidePoints     bool
	HideScoreTotal bool
}
