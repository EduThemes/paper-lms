package repository

import (
	"context"

	"github.com/EduThemes/paper-lms/internal/domain/models"
)

type AssignmentRepository interface {
	Create(ctx context.Context, assignment *models.Assignment) error
	// 13.1.D — tenant scope via parent course's account_id.
	FindByID(ctx context.Context, id, accountID uint) (*models.Assignment, error)
	FindByIDs(ctx context.Context, ids []uint) ([]models.Assignment, error)
	Update(ctx context.Context, assignment *models.Assignment) error
	Delete(ctx context.Context, id uint) error
	ListByCourseID(ctx context.Context, courseID uint, params PaginationParams) (*PaginatedResult[models.Assignment], error)
}

type AssignmentGroupRepository interface {
	Create(ctx context.Context, group *models.AssignmentGroup) error
	// 13.1.D — tenant scope via parent course's account_id.
	FindByID(ctx context.Context, id, accountID uint) (*models.AssignmentGroup, error)
	Update(ctx context.Context, group *models.AssignmentGroup) error
	Delete(ctx context.Context, id uint) error
	ListByCourseID(ctx context.Context, courseID uint, params PaginationParams) (*PaginatedResult[models.AssignmentGroup], error)
}

type AssignmentOverrideRepository interface {
	Create(ctx context.Context, override *models.AssignmentOverride) error
	FindByID(ctx context.Context, id uint) (*models.AssignmentOverride, error)
	Update(ctx context.Context, override *models.AssignmentOverride) error
	Delete(ctx context.Context, id uint) error
	ListByAssignmentID(ctx context.Context, assignmentID uint) ([]models.AssignmentOverride, error)
}

type AssignmentOverrideStudentRepository interface {
	Create(ctx context.Context, student *models.AssignmentOverrideStudent) error
	Delete(ctx context.Context, overrideID, userID uint) error
	ListByOverrideID(ctx context.Context, overrideID uint) ([]models.AssignmentOverrideStudent, error)
	ListByUserAndAssignment(ctx context.Context, userID, assignmentID uint) ([]models.AssignmentOverrideStudent, error)
}

type LatePolicyRepository interface {
	Create(ctx context.Context, policy *models.LatePolicy) error
	FindByCourseID(ctx context.Context, courseID uint) (*models.LatePolicy, error)
	Update(ctx context.Context, policy *models.LatePolicy) error
	Delete(ctx context.Context, courseID uint) error
}
