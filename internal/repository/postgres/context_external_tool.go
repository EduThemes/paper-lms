package postgres

import (
	"context"

	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/repository"
	"gorm.io/gorm"
)

type contextExternalToolRepo struct {
	db *gorm.DB
}

func NewContextExternalToolRepository(db *gorm.DB) repository.ContextExternalToolRepository {
	return &contextExternalToolRepo{db: db}
}

func (r *contextExternalToolRepo) Create(ctx context.Context, tool *models.ContextExternalTool) error {
	return r.db.WithContext(ctx).Create(tool).Error
}

func (r *contextExternalToolRepo) FindByID(ctx context.Context, id, accountID uint) (*models.ContextExternalTool, error) {
	var tool models.ContextExternalTool
	q := r.db.WithContext(ctx).Where("workflow_state != ?", "deleted")
	if accountID != 0 {
		// Context-polymorphic tenant scope:
		//  Course → context_id→courses.account_id
		//  Account → context_id IS account_id
		q = q.Where(`
			(context_type = 'Course' AND context_id IN (SELECT id FROM courses WHERE account_id = ?))
			OR (context_type = 'Account' AND context_id = ?)
		`, accountID, accountID)
	}
	if err := q.First(&tool, id).Error; err != nil {
		return nil, err
	}
	return &tool, nil
}

func (r *contextExternalToolRepo) Update(ctx context.Context, tool *models.ContextExternalTool) error {
	return r.db.WithContext(ctx).Save(tool).Error
}

func (r *contextExternalToolRepo) Delete(ctx context.Context, id uint) error {
	return r.db.WithContext(ctx).Model(&models.ContextExternalTool{}).Where("id = ?", id).Update("workflow_state", "deleted").Error
}

func (r *contextExternalToolRepo) ListByContext(ctx context.Context, contextType string, contextID uint, params repository.PaginationParams) (*repository.PaginatedResult[models.ContextExternalTool], error) {
	var tools []models.ContextExternalTool
	var count int64

	query := r.db.WithContext(ctx).Model(&models.ContextExternalTool{}).Where("context_type = ? AND context_id = ? AND workflow_state != ?", contextType, contextID, "deleted")
	query.Count(&count)

	offset := (params.Page - 1) * params.PerPage
	if err := query.Offset(offset).Limit(params.PerPage).Order("id ASC").Find(&tools).Error; err != nil {
		return nil, err
	}

	return &repository.PaginatedResult[models.ContextExternalTool]{
		Items:      tools,
		TotalCount: count,
		Page:       params.Page,
		PerPage:    params.PerPage,
	}, nil
}
