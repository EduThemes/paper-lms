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

func (r *submissionCommentRepo) ListBySubmissionID(ctx context.Context, submissionID uint) ([]models.SubmissionComment, error) {
	var comments []models.SubmissionComment
	if err := r.db.WithContext(ctx).Where("submission_id = ?", submissionID).Order("created_at ASC").Find(&comments).Error; err != nil {
		return nil, err
	}
	return comments, nil
}
