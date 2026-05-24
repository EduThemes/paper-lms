package postgres

import (
	"context"

	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/repository"
	"gorm.io/gorm"
)

type blueprintTemplateRepo struct {
	db *gorm.DB
}

func NewBlueprintTemplateRepository(db *gorm.DB) repository.BlueprintTemplateRepository {
	return &blueprintTemplateRepo{db: db}
}

func (r *blueprintTemplateRepo) Create(ctx context.Context, template *models.BlueprintTemplate) error {
	return r.db.WithContext(ctx).Create(template).Error
}

func (r *blueprintTemplateRepo) FindByID(ctx context.Context, id uint) (*models.BlueprintTemplate, error) {
	var template models.BlueprintTemplate
	if err := r.db.WithContext(ctx).Where("workflow_state != ?", "deleted").First(&template, id).Error; err != nil {
		return nil, err
	}
	return &template, nil
}

func (r *blueprintTemplateRepo) FindByCourseID(ctx context.Context, courseID uint) (*models.BlueprintTemplate, error) {
	var template models.BlueprintTemplate
	if err := r.db.WithContext(ctx).Where("course_id = ? AND workflow_state != ?", courseID, "deleted").First(&template).Error; err != nil {
		return nil, err
	}
	return &template, nil
}

func (r *blueprintTemplateRepo) Update(ctx context.Context, template *models.BlueprintTemplate) error {
	return r.db.WithContext(ctx).Save(template).Error
}

// Delete soft-deletes a blueprint template by setting workflow_state to
// "deleted". F-012 — accountID scopes the write to a single tenant via
// the parent course (blueprint_templates has no direct account_id column,
// so we join through courses.account_id). Pass 0 only from privileged
// internal callers. Handler callers MUST pass callerAccountID(c).
func (r *blueprintTemplateRepo) Delete(ctx context.Context, id, accountID uint) error {
	q := r.db.WithContext(ctx).Model(&models.BlueprintTemplate{}).Where("id = ?", id)
	if accountID != 0 {
		q = q.Where("course_id IN (SELECT id FROM courses WHERE account_id = ?)", accountID)
	}
	return q.Update("workflow_state", "deleted").Error
}

func (r *blueprintTemplateRepo) ListByCourseID(ctx context.Context, courseID uint, params repository.PaginationParams) (*repository.PaginatedResult[models.BlueprintTemplate], error) {
	var templates []models.BlueprintTemplate
	var count int64

	query := r.db.WithContext(ctx).Model(&models.BlueprintTemplate{}).Where("course_id = ? AND workflow_state != ?", courseID, "deleted")
	query.Count(&count)

	offset := (params.Page - 1) * params.PerPage
	if err := query.Offset(offset).Limit(params.PerPage).Order("created_at DESC").Find(&templates).Error; err != nil {
		return nil, err
	}

	return &repository.PaginatedResult[models.BlueprintTemplate]{
		Items:      templates,
		TotalCount: count,
		Page:       params.Page,
		PerPage:    params.PerPage,
	}, nil
}
