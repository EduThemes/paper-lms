package postgres

import (
	"context"
	"time"

	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/repository"
	"gorm.io/gorm"
)

type moduleRepo struct {
	db *gorm.DB
}

func NewModuleRepository(db *gorm.DB) repository.ModuleRepository {
	return &moduleRepo{db: db}
}

func (r *moduleRepo) Create(ctx context.Context, module *models.ContextModule) error {
	return r.db.WithContext(ctx).Create(module).Error
}

func (r *moduleRepo) FindByID(ctx context.Context, id, accountID uint) (*models.ContextModule, error) {
	var module models.ContextModule
	q := r.db.WithContext(ctx).Preload("Items", func(db *gorm.DB) *gorm.DB {
		return db.Order("position ASC")
	})
	if accountID != 0 {
		// Scope through the parent course's account_id.
		q = q.Where("course_id IN (SELECT id FROM courses WHERE account_id = ?)", accountID)
	}
	if err := q.First(&module, id).Error; err != nil {
		return nil, err
	}
	return &module, nil
}

func (r *moduleRepo) Update(ctx context.Context, module *models.ContextModule) error {
	return r.db.WithContext(ctx).Save(module).Error
}

func (r *moduleRepo) Delete(ctx context.Context, id uint) error {
	return r.db.WithContext(ctx).Model(&models.ContextModule{}).Where("id = ?", id).Update("workflow_state", "deleted").Error
}

func (r *moduleRepo) ListByCourseID(ctx context.Context, courseID uint, params repository.PaginationParams) (*repository.PaginatedResult[models.ContextModule], error) {
	var modules []models.ContextModule
	var count int64

	query := r.db.WithContext(ctx).Model(&models.ContextModule{}).Where("course_id = ? AND workflow_state != ?", courseID, "deleted")
	query.Count(&count)

	offset := (params.Page - 1) * params.PerPage
	if err := query.Preload("Items", func(db *gorm.DB) *gorm.DB {
		return db.Where("workflow_state != ?", "deleted").Order("position ASC")
	}).Offset(offset).Limit(params.PerPage).Order("position ASC").Find(&modules).Error; err != nil {
		return nil, err
	}

	return &repository.PaginatedResult[models.ContextModule]{
		Items:      modules,
		TotalCount: count,
		Page:       params.Page,
		PerPage:    params.PerPage,
	}, nil
}

func (r *moduleRepo) FindActiveByDateRange(ctx context.Context, courseID uint, date time.Time) (*models.ContextModule, error) {
	var module models.ContextModule
	if err := r.db.WithContext(ctx).
		Where("course_id = ? AND workflow_state != ? AND unlock_at <= ? AND (end_at IS NULL OR end_at >= ?)", courseID, "deleted", date, date).
		Order("position ASC").
		First(&module).Error; err != nil {
		return nil, err
	}
	return &module, nil
}

func (r *moduleRepo) ReorderModules(ctx context.Context, courseID uint, moduleIDs []uint) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		for i, id := range moduleIDs {
			if err := tx.Model(&models.ContextModule{}).
				Where("id = ? AND course_id = ?", id, courseID).
				Update("position", i+1).Error; err != nil {
				return err
			}
		}
		return nil
	})
}
