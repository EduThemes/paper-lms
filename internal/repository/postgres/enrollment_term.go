package postgres

import (
	"context"
	"time"

	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/repository"
	"gorm.io/gorm"
)

type EnrollmentTermRepository struct {
	db *gorm.DB
}

func NewEnrollmentTermRepository(db *gorm.DB) *EnrollmentTermRepository {
	return &EnrollmentTermRepository{db: db}
}

func (r *EnrollmentTermRepository) Create(ctx context.Context, term *models.EnrollmentTerm) error {
	return r.db.WithContext(ctx).Create(term).Error
}

// FindByID loads a live term. accountID scopes the read to one tenant;
// 0 is the internal "no scope" escape hatch and is not used by handlers.
func (r *EnrollmentTermRepository) FindByID(ctx context.Context, id, accountID uint) (*models.EnrollmentTerm, error) {
	var term models.EnrollmentTerm
	q := r.db.WithContext(ctx).Where("id = ? AND workflow_state != ?", id, "deleted")
	if accountID != 0 {
		q = q.Where("account_id = ?", accountID)
	}
	if err := q.First(&term).Error; err != nil {
		return nil, err
	}
	return &term, nil
}

func (r *EnrollmentTermRepository) Update(ctx context.Context, term *models.EnrollmentTerm) error {
	return r.db.WithContext(ctx).Save(term).Error
}

// Delete soft-deletes a term within accountID (0 = unscoped, internal).
func (r *EnrollmentTermRepository) Delete(ctx context.Context, id, accountID uint) error {
	q := r.db.WithContext(ctx).Model(&models.EnrollmentTerm{}).Where("id = ?", id)
	if accountID != 0 {
		q = q.Where("account_id = ?", accountID)
	}
	return q.Update("workflow_state", "deleted").Error
}

func (r *EnrollmentTermRepository) ListByAccountID(ctx context.Context, accountID uint, params repository.PaginationParams) (*repository.PaginatedResult[models.EnrollmentTerm], error) {
	var terms []models.EnrollmentTerm
	var count int64

	query := r.db.WithContext(ctx).Model(&models.EnrollmentTerm{}).Where("account_id = ? AND workflow_state != ?", accountID, "deleted")
	query.Count(&count)

	offset := (params.Page - 1) * params.PerPage
	if err := query.Offset(offset).Limit(params.PerPage).Order("start_at DESC, id ASC").Find(&terms).Error; err != nil {
		return nil, err
	}

	return &repository.PaginatedResult[models.EnrollmentTerm]{
		Items:      terms,
		TotalCount: count,
		Page:       params.Page,
		PerPage:    params.PerPage,
	}, nil
}

// FindBySISTermID — SIS term ids are unique per account (000091).
func (r *EnrollmentTermRepository) FindBySISTermID(ctx context.Context, sisTermID string, accountID uint) (*models.EnrollmentTerm, error) {
	var term models.EnrollmentTerm
	if err := r.db.WithContext(ctx).Where("sis_term_id = ? AND account_id = ? AND workflow_state != ?", sisTermID, accountID, "deleted").First(&term).Error; err != nil {
		return nil, err
	}
	return &term, nil
}

func (r *EnrollmentTermRepository) FindCurrentTerm(ctx context.Context, accountID uint) (*models.EnrollmentTerm, error) {
	var term models.EnrollmentTerm
	now := time.Now()
	if err := r.db.WithContext(ctx).
		Where("account_id = ? AND workflow_state != ? AND start_at <= ? AND end_at >= ?", accountID, "deleted", now, now).
		Order("start_at DESC").
		First(&term).Error; err != nil {
		return nil, err
	}
	return &term, nil
}

func (r *EnrollmentTermRepository) ListActive(ctx context.Context, accountID uint, params repository.PaginationParams) (*repository.PaginatedResult[models.EnrollmentTerm], error) {
	var terms []models.EnrollmentTerm
	var count int64

	query := r.db.WithContext(ctx).Model(&models.EnrollmentTerm{}).Where("account_id = ? AND workflow_state = ?", accountID, "active")
	query.Count(&count)

	offset := (params.Page - 1) * params.PerPage
	if err := query.Offset(offset).Limit(params.PerPage).Order("start_at DESC, id ASC").Find(&terms).Error; err != nil {
		return nil, err
	}

	return &repository.PaginatedResult[models.EnrollmentTerm]{
		Items:      terms,
		TotalCount: count,
		Page:       params.Page,
		PerPage:    params.PerPage,
	}, nil
}
