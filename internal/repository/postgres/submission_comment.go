package postgres

import (
	"context"

	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/repository"
	"gorm.io/gorm"
)

type submissionCommentRepo struct {
	db *gorm.DB
}

func NewSubmissionCommentRepository(db *gorm.DB) repository.SubmissionCommentRepository {
	return &submissionCommentRepo{db: db}
}

func (r *submissionCommentRepo) Create(ctx context.Context, comment *models.SubmissionComment) error {
	return r.db.WithContext(ctx).Create(comment).Error
}

func (r *submissionCommentRepo) ListBySubmissionID(ctx context.Context, submissionID, accountID uint) ([]models.SubmissionComment, error) {
	var comments []models.SubmissionComment
	q := r.db.WithContext(ctx).Where("submission_id = ?", submissionID)
	if accountID != 0 {
		// Scope through submission->assignment->course (deep 3-level subquery).
		q = q.Where("submission_id IN (SELECT id FROM submissions WHERE assignment_id IN (SELECT id FROM assignments WHERE course_id IN (SELECT id FROM courses WHERE account_id = ?)))", accountID)
	}
	if err := q.Order("created_at ASC").Find(&comments).Error; err != nil {
		return nil, err
	}
	return comments, nil
}
