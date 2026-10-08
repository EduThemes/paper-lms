package repository

import (
	"context"

	"github.com/EduThemes/paper-lms/internal/domain/models"
)

// GradingUIRepository backs the grading-UI features: submission attempt
// history, comment editing, moderated grading, custom grade statuses and
// peer-review housekeeping. It is a separate interface so the existing
// submission / comment / peer-review repositories (and their mocks) stay
// unchanged.
//
// Tenant scoping: every lookup that takes an accountID scopes through the
// row's parent chain (assignment -> course -> account). 0 means "no tenant
// scope" and is reserved for internal callers that already verified the
// parent row under the caller's tenant.
type GradingUIRepository interface {
	// Submission attempt history.
	CreateSubmissionVersion(ctx context.Context, v *models.SubmissionVersion) error
	ListSubmissionVersions(ctx context.Context, submissionID uint) ([]models.SubmissionVersion, error)
	ListSubmissionVersionsByAssignment(ctx context.Context, assignmentID uint) ([]models.SubmissionVersion, error)

	// Submission rows for an assignment (all, unpaginated) and the
	// anonymous-id lookup.
	ListSubmissionsByAssignment(ctx context.Context, assignmentID uint) ([]models.Submission, error)
	FindSubmissionByAnonymousID(ctx context.Context, assignmentID uint, anonymousID string, accountID uint) (*models.Submission, error)
	UpdateSubmissionColumns(ctx context.Context, submissionID uint, cols map[string]interface{}) error

	// Submission comments.
	FindSubmissionComment(ctx context.Context, id, accountID uint) (*models.SubmissionComment, error)
	UpdateSubmissionComment(ctx context.Context, c *models.SubmissionComment) error
	DeleteSubmissionComment(ctx context.Context, id uint) error
	ListSubmissionCommentsByGroupCommentID(ctx context.Context, groupCommentID string) ([]models.SubmissionComment, error)

	// Moderated grading.
	FindModerationGrader(ctx context.Context, assignmentID, userID uint) (*models.ModerationGrader, error)
	ListModerationGraders(ctx context.Context, assignmentID uint) ([]models.ModerationGrader, error)
	SaveModerationGrader(ctx context.Context, g *models.ModerationGrader) error
	FindProvisionalGrade(ctx context.Context, submissionID, scorerID uint, final bool) (*models.ProvisionalGrade, error)
	FindProvisionalGradeByID(ctx context.Context, id uint) (*models.ProvisionalGrade, error)
	SaveProvisionalGrade(ctx context.Context, pg *models.ProvisionalGrade) error
	ListProvisionalGradesBySubmissions(ctx context.Context, submissionIDs []uint) ([]models.ProvisionalGrade, error)
	FindModerationSelection(ctx context.Context, assignmentID, studentID uint) (*models.ModerationSelection, error)
	SaveModerationSelection(ctx context.Context, s *models.ModerationSelection) error
	ListModerationSelections(ctx context.Context, assignmentID uint) ([]models.ModerationSelection, error)
	SetAssignmentGradesPublished(ctx context.Context, assignmentID uint) error

	// Custom grade statuses.
	ListCustomGradeStatuses(ctx context.Context, accountID uint, includeDeleted bool) ([]models.CustomGradeStatus, error)
	FindCustomGradeStatus(ctx context.Context, id, accountID uint) (*models.CustomGradeStatus, error)
	CreateCustomGradeStatus(ctx context.Context, s *models.CustomGradeStatus) error
	UpdateCustomGradeStatus(ctx context.Context, s *models.CustomGradeStatus) error
	ClearCustomGradeStatus(ctx context.Context, statusID uint) error

	// Peer reviews.
	DeletePeerReview(ctx context.Context, id uint) error
}
