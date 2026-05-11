package postgres

import (
	"context"

	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/repository"
	"gorm.io/gorm"
)

type contentMigrationRepo struct {
	db *gorm.DB
}

func NewContentMigrationRepository(db *gorm.DB) repository.ContentMigrationRepository {
	return &contentMigrationRepo{db: db}
}

func (r *contentMigrationRepo) Create(ctx context.Context, migration *models.ContentMigration) error {
	return r.db.WithContext(ctx).Create(migration).Error
}

func (r *contentMigrationRepo) FindByID(ctx context.Context, id uint) (*models.ContentMigration, error) {
	var migration models.ContentMigration
	if err := r.db.WithContext(ctx).First(&migration, id).Error; err != nil {
		return nil, err
	}
	return &migration, nil
}

func (r *contentMigrationRepo) Update(ctx context.Context, migration *models.ContentMigration) error {
	return r.db.WithContext(ctx).Save(migration).Error
}

func (r *contentMigrationRepo) Delete(ctx context.Context, id uint) error {
	return r.db.WithContext(ctx).Model(&models.ContentMigration{}).Where("id = ?", id).Update("workflow_state", "deleted").Error
}

func (r *contentMigrationRepo) ListByCourseID(ctx context.Context, courseID uint, params repository.PaginationParams) (*repository.PaginatedResult[models.ContentMigration], error) {
	var migrations []models.ContentMigration
	var count int64

	query := r.db.WithContext(ctx).Model(&models.ContentMigration{}).Where("course_id = ? AND workflow_state != ?", courseID, "deleted")
	query.Count(&count)

	offset := (params.Page - 1) * params.PerPage
	if err := query.Offset(offset).Limit(params.PerPage).Order("created_at DESC").Find(&migrations).Error; err != nil {
		return nil, err
	}

	return &repository.PaginatedResult[models.ContentMigration]{
		Items:      migrations,
		TotalCount: count,
		Page:       params.Page,
		PerPage:    params.PerPage,
	}, nil
}
