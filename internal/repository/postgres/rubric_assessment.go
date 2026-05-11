package postgres

import (
	"context"

	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/repository"
	"gorm.io/gorm"
)

type rubricAssessmentRepo struct {
	db *gorm.DB
}

func NewRubricAssessmentRepository(db *gorm.DB) repository.RubricAssessmentRepository {
	return &rubricAssessmentRepo{db: db}
}

func (r *rubricAssessmentRepo) Create(ctx context.Context, assessment *models.RubricAssessment) error {
	return r.db.WithContext(ctx).Create(assessment).Error
}

func (r *rubricAssessmentRepo) FindByID(ctx context.Context, id uint) (*models.RubricAssessment, error) {
	var assessment models.RubricAssessment
	if err := r.db.WithContext(ctx).First(&assessment, id).Error; err != nil {
		return nil, err
	}
	return &assessment, nil
}

func (r *rubricAssessmentRepo) Update(ctx context.Context, assessment *models.RubricAssessment) error {
	return r.db.WithContext(ctx).Save(assessment).Error
}

func (r *rubricAssessmentRepo) Delete(ctx context.Context, id uint) error {
	return r.db.WithContext(ctx).Delete(&models.RubricAssessment{}, id).Error
}

func (r *rubricAssessmentRepo) FindByUserAndAssociation(ctx context.Context, userID, assessorID, rubricAssocID uint) (*models.RubricAssessment, error) {
	var assessment models.RubricAssessment
	if err := r.db.WithContext(ctx).Where("user_id = ? AND assessor_id = ? AND rubric_association_id = ?", userID, assessorID, rubricAssocID).First(&assessment).Error; err != nil {
		return nil, err
	}
	return &assessment, nil
}

func (r *rubricAssessmentRepo) ListByAssociationID(ctx context.Context, rubricAssocID uint, params repository.PaginationParams) (*repository.PaginatedResult[models.RubricAssessment], error) {
	var assessments []models.RubricAssessment
	var count int64

	query := r.db.WithContext(ctx).Model(&models.RubricAssessment{}).Where("rubric_association_id = ? AND workflow_state != ?", rubricAssocID, "deleted")
	query.Count(&count)

	offset := (params.Page - 1) * params.PerPage
	if err := query.Offset(offset).Limit(params.PerPage).Order("created_at DESC").Find(&assessments).Error; err != nil {
		return nil, err
	}

	return &repository.PaginatedResult[models.RubricAssessment]{
		Items:      assessments,
		TotalCount: count,
		Page:       params.Page,
		PerPage:    params.PerPage,
	}, nil
}
