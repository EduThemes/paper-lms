package postgres

import (
	"context"

	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/repository"
	"gorm.io/gorm"
)

type ltiLineItemRepo struct {
	db *gorm.DB
}

func NewLTILineItemRepository(db *gorm.DB) repository.LTILineItemRepository {
	return &ltiLineItemRepo{db: db}
}

func (r *ltiLineItemRepo) Create(ctx context.Context, item *models.LTILineItem) error {
	return r.db.WithContext(ctx).Create(item).Error
}

func (r *ltiLineItemRepo) FindByID(ctx context.Context, id uint) (*models.LTILineItem, error) {
	var item models.LTILineItem
	if err := r.db.WithContext(ctx).First(&item, id).Error; err != nil {
		return nil, err
	}
	return &item, nil
}

func (r *ltiLineItemRepo) Update(ctx context.Context, item *models.LTILineItem) error {
	return r.db.WithContext(ctx).Save(item).Error
}

func (r *ltiLineItemRepo) Delete(ctx context.Context, id uint) error {
	return r.db.WithContext(ctx).Delete(&models.LTILineItem{}, id).Error
}

func (r *ltiLineItemRepo) ListByCourse(ctx context.Context, courseID uint, params repository.PaginationParams) (*repository.PaginatedResult[models.LTILineItem], error) {
	var items []models.LTILineItem
	var count int64

	query := r.db.WithContext(ctx).Model(&models.LTILineItem{}).Where("course_id = ?", courseID)
	query.Count(&count)

	offset := (params.Page - 1) * params.PerPage
	if err := query.Offset(offset).Limit(params.PerPage).Order("id ASC").Find(&items).Error; err != nil {
		return nil, err
	}

	return &repository.PaginatedResult[models.LTILineItem]{
		Items:      items,
		TotalCount: count,
		Page:       params.Page,
		PerPage:    params.PerPage,
	}, nil
}

func (r *ltiLineItemRepo) FindByAssignmentID(ctx context.Context, assignmentID uint) (*models.LTILineItem, error) {
	var item models.LTILineItem
	if err := r.db.WithContext(ctx).Where("assignment_id = ?", assignmentID).First(&item).Error; err != nil {
		return nil, err
	}
	return &item, nil
}
