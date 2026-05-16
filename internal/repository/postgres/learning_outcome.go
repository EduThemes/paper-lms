package postgres

import (
	"context"

	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/repository"
	"gorm.io/gorm"
)

type learningOutcomeRepo struct {
	db *gorm.DB
}

func NewLearningOutcomeRepository(db *gorm.DB) repository.LearningOutcomeRepository {
	return &learningOutcomeRepo{db: db}
}

func (r *learningOutcomeRepo) Create(ctx context.Context, outcome *models.LearningOutcome) error {
	return r.db.WithContext(ctx).Create(outcome).Error
}

func (r *learningOutcomeRepo) FindByID(ctx context.Context, id, accountID uint) (*models.LearningOutcome, error) {
	var outcome models.LearningOutcome
	q := r.db.WithContext(ctx)
	if accountID != 0 {
		// Polymorphic context: Account-level outcomes are intentionally
		// cross-course-shareable WITHIN a tenant; both branches scope to
		// the SAME account_id.
		q = q.Where(`
			(context_type = 'Account' AND context_id = ?)
			OR (context_type = 'Course' AND context_id IN (SELECT id FROM courses WHERE account_id = ?))
		`, accountID, accountID)
	}
	if err := q.First(&outcome, id).Error; err != nil {
		return nil, err
	}
	return &outcome, nil
}

func (r *learningOutcomeRepo) Update(ctx context.Context, outcome *models.LearningOutcome) error {
	return r.db.WithContext(ctx).Save(outcome).Error
}

func (r *learningOutcomeRepo) Delete(ctx context.Context, id uint) error {
	return r.db.WithContext(ctx).Model(&models.LearningOutcome{}).Where("id = ?", id).Update("workflow_state", "deleted").Error
}

func (r *learningOutcomeRepo) ListByGroupID(ctx context.Context, groupID, accountID uint, params repository.PaginationParams) (*repository.PaginatedResult[models.LearningOutcome], error) {
	var outcomes []models.LearningOutcome
	var count int64

	query := r.db.WithContext(ctx).Model(&models.LearningOutcome{}).Where("outcome_group_id = ? AND workflow_state != ?", groupID, "deleted")
	if accountID != 0 {
		// Outcomes inherit the tenant boundary of their parent group's
		// context. Filter on the outcome row's context_type/context_id
		// because that's what's directly indexable.
		query = query.Where(`
			(context_type = 'Account' AND context_id = ?)
			OR (context_type = 'Course' AND context_id IN (SELECT id FROM courses WHERE account_id = ?))
		`, accountID, accountID)
	}
	query.Count(&count)

	offset := (params.Page - 1) * params.PerPage
	if err := query.Offset(offset).Limit(params.PerPage).Order("created_at ASC").Find(&outcomes).Error; err != nil {
		return nil, err
	}

	return &repository.PaginatedResult[models.LearningOutcome]{
		Items:      outcomes,
		TotalCount: count,
		Page:       params.Page,
		PerPage:    params.PerPage,
	}, nil
}

func (r *learningOutcomeRepo) ListByContext(ctx context.Context, contextType string, contextID, accountID uint, params repository.PaginationParams) (*repository.PaginatedResult[models.LearningOutcome], error) {
	var outcomes []models.LearningOutcome
	var count int64

	query := r.db.WithContext(ctx).Model(&models.LearningOutcome{}).Where("context_type = ? AND context_id = ? AND workflow_state != ?", contextType, contextID, "deleted")
	if accountID != 0 {
		switch contextType {
		case "Course":
			query = query.Where("context_id IN (SELECT id FROM courses WHERE account_id = ?)", accountID)
		case "Account":
			query = query.Where("context_id = ?", accountID)
		}
	}
	query.Count(&count)

	offset := (params.Page - 1) * params.PerPage
	if err := query.Offset(offset).Limit(params.PerPage).Order("created_at ASC").Find(&outcomes).Error; err != nil {
		return nil, err
	}

	return &repository.PaginatedResult[models.LearningOutcome]{
		Items:      outcomes,
		TotalCount: count,
		Page:       params.Page,
		PerPage:    params.PerPage,
	}, nil
}
