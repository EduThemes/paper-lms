package repository

import (
	"context"

	"github.com/EduThemes/paper-lms/internal/domain/models"
)

type RubricRepository interface {
	Create(ctx context.Context, rubric *models.Rubric) error
	// 13.1.D — tenant scope via context_type branch: Account → direct
	// account_id match; Course → JOIN through courses.account_id.
	// Rubrics are intentionally cross-course-shareable WITHIN a tenant;
	// an Account-level rubric in tenant A is reachable from any course
	// in tenant A but never from tenant B.
	FindByID(ctx context.Context, id, accountID uint) (*models.Rubric, error)
	Update(ctx context.Context, rubric *models.Rubric) error
	Delete(ctx context.Context, id uint) error
	ListByContext(ctx context.Context, contextType string, contextID, accountID uint, params PaginationParams) (*PaginatedResult[models.Rubric], error)
}

type RubricAssociationRepository interface {
	Create(ctx context.Context, assoc *models.RubricAssociation) error
	FindByID(ctx context.Context, id uint) (*models.RubricAssociation, error)
	Update(ctx context.Context, assoc *models.RubricAssociation) error
	Delete(ctx context.Context, id uint) error
	FindByAssociation(ctx context.Context, associationID uint, associationType string) (*models.RubricAssociation, error)
}

type RubricAssessmentRepository interface {
	Create(ctx context.Context, assessment *models.RubricAssessment) error
	FindByID(ctx context.Context, id uint) (*models.RubricAssessment, error)
	Update(ctx context.Context, assessment *models.RubricAssessment) error
	Delete(ctx context.Context, id uint) error
	FindByUserAndAssociation(ctx context.Context, userID, assessorID, rubricAssocID uint) (*models.RubricAssessment, error)
	ListByAssociationID(ctx context.Context, rubricAssocID uint, params PaginationParams) (*PaginatedResult[models.RubricAssessment], error)
}
