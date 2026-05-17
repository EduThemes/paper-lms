package repository

import (
	"context"
	"time"

	"github.com/EduThemes/paper-lms/internal/domain/models"
)

type SubmissionRepository interface {
	Create(ctx context.Context, submission *models.Submission) error
	// 13.1.D — tenant scope via parent assignment->course. 0 means no tenant scope (internal callers only).
	FindByID(ctx context.Context, id, accountID uint) (*models.Submission, error)
	FindByIDs(ctx context.Context, ids []uint) ([]models.Submission, error)
	// 13.x.2.1 — tenant-scoped via parent assignment->course->account_id.
	// 0 means no tenant scope (internal callers only).
	FindByAssignmentAndUser(ctx context.Context, assignmentID, userID, accountID uint) (*models.Submission, error)
	FindByAssignmentAndUserIDs(ctx context.Context, assignmentID uint, userIDs []uint) ([]models.Submission, error)
	// ListByUserAndAssignmentIDs is the snapshot loader's targeted read:
	// pulls one user's submissions for a small set of assignments at once,
	// avoiding the N round-trips a per-assignment loop would cost.
	ListByUserAndAssignmentIDs(ctx context.Context, userID uint, assignmentIDs []uint) ([]models.Submission, error)
	Update(ctx context.Context, submission *models.Submission) error
	ListByAssignmentID(ctx context.Context, assignmentID uint, params PaginationParams) (*PaginatedResult[models.Submission], error)
	ListByUserAndCourse(ctx context.Context, userID, courseID uint) ([]models.Submission, error)
	BulkListByCourse(ctx context.Context, courseID uint, params PaginationParams) (*PaginatedResult[models.Submission], error)
	PostGradesByAssignment(ctx context.Context, assignmentID uint, postedAt *time.Time) error
	RunInTransaction(ctx context.Context, fn func(txRepo SubmissionRepository) error) error
}

type SubmissionCommentRepository interface {
	Create(ctx context.Context, comment *models.SubmissionComment) error
	// 13.1.D — tenant scope via submission->assignment->course. 0 means no tenant scope (internal callers only).
	ListBySubmissionID(ctx context.Context, submissionID, accountID uint) ([]models.SubmissionComment, error)
}

type PeerReviewRepository interface {
	Create(ctx context.Context, pr *models.PeerReview) error
	FindByID(ctx context.Context, id uint) (*models.PeerReview, error)
	Update(ctx context.Context, pr *models.PeerReview) error
	ListByAssignment(ctx context.Context, assignmentID uint) ([]models.PeerReview, error)
	ListByReviewer(ctx context.Context, assignmentID, reviewerID uint) ([]models.PeerReview, error)
	FindByAssignmentAndReviewerAndReviewee(ctx context.Context, assignmentID, reviewerID, revieweeID uint) (*models.PeerReview, error)
	DeleteByAssignment(ctx context.Context, assignmentID uint) error
}

type DocumentAnnotationRepository interface {
	Create(ctx context.Context, annotation *models.DocumentAnnotation) error
	// 13.1.D — tenant scope via submission->assignment->course. 0 means no tenant scope (internal callers only).
	FindByID(ctx context.Context, id, accountID uint) (*models.DocumentAnnotation, error)
	Update(ctx context.Context, annotation *models.DocumentAnnotation) error
	Delete(ctx context.Context, id uint) error
	ListBySubmissionID(ctx context.Context, submissionID uint, params PaginationParams) (*PaginatedResult[models.DocumentAnnotation], error)
	ListBySubmissionAndPage(ctx context.Context, submissionID uint, pageNumber int) ([]models.DocumentAnnotation, error)
	CountBySubmissionID(ctx context.Context, submissionID uint) (int64, error)
	ListReplies(ctx context.Context, parentAnnotationID uint) ([]models.DocumentAnnotation, error)
	Resolve(ctx context.Context, annotationID uint, resolvedByUserID uint) error
	Unresolve(ctx context.Context, annotationID uint) error
}

type CommentBankItemRepository interface {
	Create(ctx context.Context, item *models.CommentBankItem) error
	FindByID(ctx context.Context, id uint) (*models.CommentBankItem, error)
	Update(ctx context.Context, item *models.CommentBankItem) error
	Delete(ctx context.Context, id uint) error
	ListByUserID(ctx context.Context, userID uint, params PaginationParams) (*PaginatedResult[models.CommentBankItem], error)
	SearchByUser(ctx context.Context, userID uint, query string) ([]models.CommentBankItem, error)
}
