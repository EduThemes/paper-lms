package postgres

import (
	"context"

	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/repository"
	"gorm.io/gorm"
)

type pageRepo struct {
	db *gorm.DB
}

func NewPageRepository(db *gorm.DB) repository.PageRepository {
	return &pageRepo{db: db}
}

func (r *pageRepo) Create(ctx context.Context, page *models.WikiPage) error {
	return r.db.WithContext(ctx).Create(page).Error
}

func (r *pageRepo) FindByID(ctx context.Context, id uint) (*models.WikiPage, error) {
	var page models.WikiPage
	if err := r.db.WithContext(ctx).First(&page, id).Error; err != nil {
		return nil, err
	}
	return &page, nil
}

func (r *pageRepo) FindByCourseAndURL(ctx context.Context, courseID uint, url string) (*models.WikiPage, error) {
	var page models.WikiPage
	if err := r.db.WithContext(ctx).Where("course_id = ? AND url = ? AND workflow_state != ?", courseID, url, "deleted").First(&page).Error; err != nil {
		return nil, err
	}
	return &page, nil
}

func (r *pageRepo) FindPublicByCourseAndURL(ctx context.Context, courseID uint, url string) (*models.WikiPage, error) {
	var page models.WikiPage
	if err := r.db.WithContext(ctx).Where("course_id = ? AND url = ? AND public = ? AND workflow_state = ?", courseID, url, true, "active").First(&page).Error; err != nil {
		return nil, err
	}
	return &page, nil
}

func (r *pageRepo) Update(ctx context.Context, page *models.WikiPage) error {
	return r.db.WithContext(ctx).Save(page).Error
}

func (r *pageRepo) Delete(ctx context.Context, id uint) error {
	return r.db.WithContext(ctx).Model(&models.WikiPage{}).Where("id = ?", id).Update("workflow_state", "deleted").Error
}

func (r *pageRepo) ListByCourseID(ctx context.Context, courseID uint, params repository.PaginationParams) (*repository.PaginatedResult[models.WikiPage], error) {
	var pages []models.WikiPage
	var count int64

	query := r.db.WithContext(ctx).Model(&models.WikiPage{}).Where("course_id = ? AND workflow_state != ?", courseID, "deleted")
	query.Count(&count)

	offset := (params.Page - 1) * params.PerPage
	if err := query.Offset(offset).Limit(params.PerPage).Order("title ASC").Find(&pages).Error; err != nil {
		return nil, err
	}

	return &repository.PaginatedResult[models.WikiPage]{
		Items:      pages,
		TotalCount: count,
		Page:       params.Page,
		PerPage:    params.PerPage,
	}, nil
}
