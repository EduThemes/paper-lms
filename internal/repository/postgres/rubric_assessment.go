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

// Delete removes the rubric assessment row.
// F-012 widening — accountID, when non-zero, restricts the delete to
// assessments whose rubric belongs to caller's tenant (Account context
// → direct match, Course context → JOIN through courses.account_id).
// accountID==0 is the auth-internal contract documented on
// internal/repository/postgres/user.go.
func (r *rubricAssessmentRepo) Delete(ctx context.Context, id, accountID uint) error {
	q := r.db.WithContext(ctx).Where("id = ?", id)
	if accountID != 0 {
		q = q.Where(`
			rubric_id IN (
				SELECT id FROM rubrics
				WHERE (context_type = 'Account' AND context_id = ?)
				   OR (context_type = 'Course' AND context_id IN (SELECT id FROM courses WHERE account_id = ?))
			)
		`, accountID, accountID)
	}
	return q.Delete(&models.RubricAssessment{}).Error
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
