package repository

import (
	"context"

	"github.com/EduThemes/paper-lms/internal/domain/models"
)

type LearningOutcomeGroupRepository interface {
	Create(ctx context.Context, group *models.LearningOutcomeGroup) error
	// 13.1.D — tenant scope via context_type branch (Account direct,
	// Course via parent courses.account_id).
	FindByID(ctx context.Context, id, accountID uint) (*models.LearningOutcomeGroup, error)
	Update(ctx context.Context, group *models.LearningOutcomeGroup) error
	Delete(ctx context.Context, id uint) error
	ListByContext(ctx context.Context, contextType string, contextID, accountID uint, params PaginationParams) (*PaginatedResult[models.LearningOutcomeGroup], error)
	FindRootGroup(ctx context.Context, contextType string, contextID, accountID uint) (*models.LearningOutcomeGroup, error)
}

type LearningOutcomeRepository interface {
	Create(ctx context.Context, outcome *models.LearningOutcome) error
	// 13.1.D — tenant scope. Outcomes at Account level are shareable
	// across every course in the same tenant; the polymorphic branch
	// enforces "Account → direct match, Course → JOIN through courses".
	FindByID(ctx context.Context, id, accountID uint) (*models.LearningOutcome, error)
	Update(ctx context.Context, outcome *models.LearningOutcome) error
	Delete(ctx context.Context, id uint) error
	ListByGroupID(ctx context.Context, groupID, accountID uint, params PaginationParams) (*PaginatedResult[models.LearningOutcome], error)
	ListByContext(ctx context.Context, contextType string, contextID, accountID uint, params PaginationParams) (*PaginatedResult[models.LearningOutcome], error)
}

type LearningOutcomeResultRepository interface {
	Create(ctx context.Context, result *models.LearningOutcomeResult) error
	FindByID(ctx context.Context, id uint) (*models.LearningOutcomeResult, error)
	Update(ctx context.Context, result *models.LearningOutcomeResult) error
	// Upsert writes a result row keyed on
	// (user_id, learning_outcome_id, associated_asset_type, associated_asset_id)
	// and returns the row's Mastery value as it was BEFORE the write.
	// priorMastery is nil if no prior row existed or the prior row's
	// Mastery was nil. The implementation must serialize concurrent
	// writes to the same composite (the postgres impl uses a single
	// transaction with SELECT … FOR UPDATE) so that the
	// LearningOutcomeService.OnMasteryCrossed transition detector can
	// trust the returned value as the atomic pre-write state.
	Upsert(ctx context.Context, result *models.LearningOutcomeResult) (priorMastery *bool, err error)
	ListByOutcomeID(ctx context.Context, outcomeID uint, params PaginationParams) (*PaginatedResult[models.LearningOutcomeResult], error)
	ListByUserAndContext(ctx context.Context, userID uint, contextType string, contextID uint) ([]models.LearningOutcomeResult, error)
	// ListByUserAndOutcomeIDs is the snapshot loader's targeted read for
	// OutcomeMastery predicates. Returns every recorded result for the
	// given outcome set; the mastery package consumes them via its
	// per-method calculators.
	ListByUserAndOutcomeIDs(ctx context.Context, userID uint, outcomeIDs []uint) ([]models.LearningOutcomeResult, error)
}

type OutcomeAlignmentRepository interface {
	Create(ctx context.Context, alignment *models.OutcomeAlignment) error
	Delete(ctx context.Context, id uint) error
	// 13.1.D — accountID, when non-zero, filters alignments to those whose
	// course (or whose assignment's course) belongs to caller's tenant.
	ListByAssignmentID(ctx context.Context, assignmentID, accountID uint) ([]models.OutcomeAlignment, error)
	ListByCourseID(ctx context.Context, courseID, accountID uint) ([]models.OutcomeAlignment, error)
}
