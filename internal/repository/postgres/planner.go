package postgres

import (
	"context"

	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/repository"
	"gorm.io/gorm"
)

// --- PlannerNoteRepository ---

type plannerNoteRepo struct {
	db *gorm.DB
}

func NewPlannerNoteRepository(db *gorm.DB) repository.PlannerNoteRepository {
	return &plannerNoteRepo{db: db}
}

func (r *plannerNoteRepo) Create(ctx context.Context, note *models.PlannerNote) error {
	return r.db.WithContext(ctx).Create(note).Error
}

func (r *plannerNoteRepo) FindByID(ctx context.Context, id uint) (*models.PlannerNote, error) {
	var note models.PlannerNote
	if err := r.db.WithContext(ctx).First(&note, id).Error; err != nil {
		return nil, err
	}
	return &note, nil
}

func (r *plannerNoteRepo) Update(ctx context.Context, note *models.PlannerNote) error {
	return r.db.WithContext(ctx).Save(note).Error
}

func (r *plannerNoteRepo) Delete(ctx context.Context, id uint) error {
	return r.db.WithContext(ctx).Model(&models.PlannerNote{}).Where("id = ?", id).Update("workflow_state", "deleted").Error
}

func (r *plannerNoteRepo) ListByUserID(ctx context.Context, userID uint, params repository.PaginationParams) (*repository.PaginatedResult[models.PlannerNote], error) {
	var notes []models.PlannerNote
	var count int64

	query := r.db.WithContext(ctx).Model(&models.PlannerNote{}).Where("user_id = ? AND workflow_state != ?", userID, "deleted")
	query.Count(&count)

	offset := (params.Page - 1) * params.PerPage
	if err := query.Offset(offset).Limit(params.PerPage).Order("todo_date ASC, id ASC").Find(&notes).Error; err != nil {
		return nil, err
	}

	return &repository.PaginatedResult[models.PlannerNote]{
		Items:      notes,
		TotalCount: count,
		Page:       params.Page,
		PerPage:    params.PerPage,
	}, nil
}

// --- PlannerOverrideRepository ---

type plannerOverrideRepo struct {
	db *gorm.DB
}

func NewPlannerOverrideRepository(db *gorm.DB) repository.PlannerOverrideRepository {
	return &plannerOverrideRepo{db: db}
}

func (r *plannerOverrideRepo) Create(ctx context.Context, override *models.PlannerOverride) error {
	return r.db.WithContext(ctx).Create(override).Error
}

func (r *plannerOverrideRepo) FindByID(ctx context.Context, id uint) (*models.PlannerOverride, error) {
	var override models.PlannerOverride
	if err := r.db.WithContext(ctx).First(&override, id).Error; err != nil {
		return nil, err
	}
	return &override, nil
}

func (r *plannerOverrideRepo) Update(ctx context.Context, override *models.PlannerOverride) error {
	return r.db.WithContext(ctx).Save(override).Error
}

func (r *plannerOverrideRepo) Delete(ctx context.Context, id uint) error {
	return r.db.WithContext(ctx).Delete(&models.PlannerOverride{}, id).Error
}

func (r *plannerOverrideRepo) FindByUserAndPlannable(ctx context.Context, userID uint, plannableType string, plannableID uint) (*models.PlannerOverride, error) {
	var override models.PlannerOverride
	if err := r.db.WithContext(ctx).
		Where("user_id = ? AND plannable_type = ? AND plannable_id = ?", userID, plannableType, plannableID).
		First(&override).Error; err != nil {
		return nil, err
	}
	return &override, nil
}

func (r *plannerOverrideRepo) ListByUserID(ctx context.Context, userID uint) ([]models.PlannerOverride, error) {
	var overrides []models.PlannerOverride
	if err := r.db.WithContext(ctx).Where("user_id = ?", userID).Find(&overrides).Error; err != nil {
		return nil, err
	}
	return overrides, nil
}
