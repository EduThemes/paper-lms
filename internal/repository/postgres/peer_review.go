package postgres

import (
	"context"

	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/repository"
	"gorm.io/gorm"
)

type peerReviewRepo struct {
	db *gorm.DB
}

func NewPeerReviewRepository(db *gorm.DB) repository.PeerReviewRepository {
	return &peerReviewRepo{db: db}
}

func (r *peerReviewRepo) Create(ctx context.Context, pr *models.PeerReview) error {
	return r.db.WithContext(ctx).Create(pr).Error
}

func (r *peerReviewRepo) FindByID(ctx context.Context, id uint) (*models.PeerReview, error) {
	var pr models.PeerReview
	if err := r.db.WithContext(ctx).First(&pr, id).Error; err != nil {
		return nil, err
	}
	return &pr, nil
}

func (r *peerReviewRepo) Update(ctx context.Context, pr *models.PeerReview) error {
	return r.db.WithContext(ctx).Save(pr).Error
}

func (r *peerReviewRepo) ListByAssignment(ctx context.Context, assignmentID uint) ([]models.PeerReview, error) {
	var reviews []models.PeerReview
	if err := r.db.WithContext(ctx).Where("assignment_id = ?", assignmentID).Order("id ASC").Find(&reviews).Error; err != nil {
		return nil, err
	}
	return reviews, nil
}

func (r *peerReviewRepo) ListByReviewer(ctx context.Context, assignmentID, reviewerID uint) ([]models.PeerReview, error) {
	var reviews []models.PeerReview
	if err := r.db.WithContext(ctx).Where("assignment_id = ? AND reviewer_id = ?", assignmentID, reviewerID).Find(&reviews).Error; err != nil {
		return nil, err
	}
	return reviews, nil
}

func (r *peerReviewRepo) FindByAssignmentAndReviewerAndReviewee(ctx context.Context, assignmentID, reviewerID, revieweeID uint) (*models.PeerReview, error) {
	var pr models.PeerReview
	if err := r.db.WithContext(ctx).Where("assignment_id = ? AND reviewer_id = ? AND reviewee_id = ?", assignmentID, reviewerID, revieweeID).First(&pr).Error; err != nil {
		return nil, err
	}
	return &pr, nil
}

func (r *peerReviewRepo) DeleteByAssignment(ctx context.Context, assignmentID uint) error {
	return r.db.WithContext(ctx).Where("assignment_id = ?", assignmentID).Delete(&models.PeerReview{}).Error
}
