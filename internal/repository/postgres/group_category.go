package postgres

import (
	"context"

	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/repository"
	"gorm.io/gorm"
)

type groupCategoryRepo struct {
	db *gorm.DB
}

func NewGroupCategoryRepository(db *gorm.DB) repository.GroupCategoryRepository {
	return &groupCategoryRepo{db: db}
}

func (r *groupCategoryRepo) Create(ctx context.Context, category *models.GroupCategory) error {
	return r.db.WithContext(ctx).Create(category).Error
}

func (r *groupCategoryRepo) FindByID(ctx context.Context, id uint) (*models.GroupCategory, error) {
	var category models.GroupCategory
	if err := r.db.WithContext(ctx).First(&category, id).Error; err != nil {
		return nil, err
	}
	return &category, nil
}

func (r *groupCategoryRepo) Update(ctx context.Context, category *models.GroupCategory) error {
	return r.db.WithContext(ctx).Save(category).Error
}

func (r *groupCategoryRepo) Delete(ctx context.Context, id uint) error {
	return r.db.WithContext(ctx).Model(&models.GroupCategory{}).Where("id = ?", id).Update("workflow_state", "deleted").Error
}

func (r *groupCategoryRepo) ListByCourseID(ctx context.Context, courseID uint, params repository.PaginationParams) (*repository.PaginatedResult[models.GroupCategory], error) {
	var categories []models.GroupCategory
	var count int64

	query := r.db.WithContext(ctx).Model(&models.GroupCategory{}).Where("course_id = ? AND workflow_state != ?", courseID, "deleted")
	query.Count(&count)

	offset := (params.Page - 1) * params.PerPage
	if err := query.Offset(offset).Limit(params.PerPage).Order("id ASC").Find(&categories).Error; err != nil {
		return nil, err
	}

	return &repository.PaginatedResult[models.GroupCategory]{
		Items:      categories,
		TotalCount: count,
		Page:       params.Page,
		PerPage:    params.PerPage,
	}, nil
}

func (r *groupCategoryRepo) ListByAccountID(ctx context.Context, accountID uint, params repository.PaginationParams) (*repository.PaginatedResult[models.GroupCategory], error) {
	var categories []models.GroupCategory
	var count int64

	query := r.db.WithContext(ctx).Model(&models.GroupCategory{}).Where("account_id = ? AND workflow_state != ?", accountID, "deleted")
	query.Count(&count)

	offset := (params.Page - 1) * params.PerPage
	if err := query.Offset(offset).Limit(params.PerPage).Order("id ASC").Find(&categories).Error; err != nil {
		return nil, err
	}

	return &repository.PaginatedResult[models.GroupCategory]{
		Items:      categories,
		TotalCount: count,
		Page:       params.Page,
		PerPage:    params.PerPage,
	}, nil
}
