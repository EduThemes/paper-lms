package postgres

import (
	"context"

	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/repository"
	"gorm.io/gorm"
)

// assignmentTenantScope is the canonical 2-level subquery that gates
// peer_reviews rows to a single tenant via the parent
// assignment -> course -> account_id chain. A cross-tenant assignment_id
// fails the filter and the row is not returned (handler surfaces 404),
// preserving the 13.1.E existence-leak contract.
const assignmentTenantScope = "assignment_id IN (SELECT id FROM assignments WHERE course_id IN (SELECT id FROM courses WHERE account_id = ?))"

type peerReviewRepo struct {
	db *gorm.DB
}

func NewPeerReviewRepository(db *gorm.DB) repository.PeerReviewRepository {
	return &peerReviewRepo{db: db}
}

func (r *peerReviewRepo) Create(ctx context.Context, pr *models.PeerReview) error {
	return r.db.WithContext(ctx).Create(pr).Error
}

func (r *peerReviewRepo) FindByID(ctx context.Context, id, accountID uint) (*models.PeerReview, error) {
	var pr models.PeerReview
	q := r.db.WithContext(ctx)
	if accountID != 0 {
		q = q.Where(assignmentTenantScope, accountID)
	}
	if err := q.First(&pr, id).Error; err != nil {
		return nil, err
	}
	return &pr, nil
}

func (r *peerReviewRepo) Update(ctx context.Context, pr *models.PeerReview) error {
	return r.db.WithContext(ctx).Save(pr).Error
}

func (r *peerReviewRepo) ListByAssignment(ctx context.Context, assignmentID, accountID uint) ([]models.PeerReview, error) {
	var reviews []models.PeerReview
	q := r.db.WithContext(ctx).Where("assignment_id = ?", assignmentID)
	if accountID != 0 {
		q = q.Where(assignmentTenantScope, accountID)
	}
	if err := q.Order("id ASC").Find(&reviews).Error; err != nil {
		return nil, err
	}
	return reviews, nil
}

func (r *peerReviewRepo) ListByReviewer(ctx context.Context, assignmentID, reviewerID, accountID uint) ([]models.PeerReview, error) {
	var reviews []models.PeerReview
	q := r.db.WithContext(ctx).Where("assignment_id = ? AND reviewer_id = ?", assignmentID, reviewerID)
	if accountID != 0 {
		q = q.Where(assignmentTenantScope, accountID)
	}
	if err := q.Find(&reviews).Error; err != nil {
		return nil, err
	}
	return reviews, nil
}

func (r *peerReviewRepo) FindByAssignmentAndReviewerAndReviewee(ctx context.Context, assignmentID, reviewerID, revieweeID, accountID uint) (*models.PeerReview, error) {
	var pr models.PeerReview
	q := r.db.WithContext(ctx).Where("assignment_id = ? AND reviewer_id = ? AND reviewee_id = ?", assignmentID, reviewerID, revieweeID)
	if accountID != 0 {
		q = q.Where(assignmentTenantScope, accountID)
	}
	if err := q.First(&pr).Error; err != nil {
		return nil, err
	}
	return &pr, nil
}

func (r *peerReviewRepo) DeleteByAssignment(ctx context.Context, assignmentID, accountID uint) error {
	q := r.db.WithContext(ctx).Where("assignment_id = ?", assignmentID)
	if accountID != 0 {
		q = q.Where(assignmentTenantScope, accountID)
	}
	return q.Delete(&models.PeerReview{}).Error
}
