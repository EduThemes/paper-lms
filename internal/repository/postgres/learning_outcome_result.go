package postgres

import (
	"context"
	"errors"

	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/repository"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type learningOutcomeResultRepo struct {
	db *gorm.DB
}

func NewLearningOutcomeResultRepository(db *gorm.DB) repository.LearningOutcomeResultRepository {
	return &learningOutcomeResultRepo{db: db}
}

func (r *learningOutcomeResultRepo) Create(ctx context.Context, result *models.LearningOutcomeResult) error {
	return r.db.WithContext(ctx).Create(result).Error
}

func (r *learningOutcomeResultRepo) FindByID(ctx context.Context, id uint) (*models.LearningOutcomeResult, error) {
	var result models.LearningOutcomeResult
	if err := r.db.WithContext(ctx).First(&result, id).Error; err != nil {
		return nil, err
	}
	return &result, nil
}

func (r *learningOutcomeResultRepo) Update(ctx context.Context, result *models.LearningOutcomeResult) error {
	return r.db.WithContext(ctx).Save(result).Error
}

// Upsert writes a result row, returning the row's mastery value as it was
// BEFORE the write. priorMastery is nil when no prior row existed (or when
// the prior row's Mastery was nil); otherwise it points to a copy of the
// prior bool. The lookup-then-write runs in a single transaction with
// SELECT … FOR UPDATE on the existing row to serialize concurrent updates
// to the same (user, outcome, asset) composite — preventing the
// check-then-act race that would otherwise let two concurrent
// CreateResult calls each observe "not yet mastered" and both fire the
// OnMasteryCrossed callback.
//
// The residual race is on INSERT (two concurrent writes both finding no
// row and both calling Create). Closing that fully needs a UNIQUE index on
// (user_id, learning_outcome_id, associated_asset_type, associated_asset_id)
// + ON CONFLICT semantics — a Sprint D-3 migration.
func (r *learningOutcomeResultRepo) Upsert(ctx context.Context, result *models.LearningOutcomeResult) (priorMastery *bool, err error) {
	err = r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var existing models.LearningOutcomeResult
		findErr := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("user_id = ? AND learning_outcome_id = ? AND associated_asset_type = ? AND associated_asset_id = ?",
				result.UserID, result.LearningOutcomeID, result.AssociatedAssetType, result.AssociatedAssetID).
			First(&existing).Error
		if errors.Is(findErr, gorm.ErrRecordNotFound) {
			// No prior row; create. priorMastery stays nil.
			return tx.Create(result).Error
		}
		if findErr != nil {
			return findErr
		}

		// Snapshot prior mastery before mutating the existing row.
		if existing.Mastery != nil {
			prior := *existing.Mastery
			priorMastery = &prior
		}

		existing.Score = result.Score
		existing.Possible = result.Possible
		existing.Mastery = result.Mastery
		existing.Percent = result.Percent
		existing.Attempt = result.Attempt
		existing.AssessedAt = result.AssessedAt
		existing.SubmittedAt = result.SubmittedAt
		existing.Title = result.Title
		existing.ContextType = result.ContextType
		existing.ContextID = result.ContextID

		result.ID = existing.ID
		return tx.Save(&existing).Error
	})
	if err != nil {
		return nil, err
	}
	return priorMastery, nil
}

func (r *learningOutcomeResultRepo) ListByOutcomeID(ctx context.Context, outcomeID uint, params repository.PaginationParams) (*repository.PaginatedResult[models.LearningOutcomeResult], error) {
	var results []models.LearningOutcomeResult
	var count int64

	query := r.db.WithContext(ctx).Model(&models.LearningOutcomeResult{}).Where("learning_outcome_id = ?", outcomeID)
	query.Count(&count)

	offset := (params.Page - 1) * params.PerPage
	if err := query.Offset(offset).Limit(params.PerPage).Order("created_at DESC").Find(&results).Error; err != nil {
		return nil, err
	}

	return &repository.PaginatedResult[models.LearningOutcomeResult]{
		Items:      results,
		TotalCount: count,
		Page:       params.Page,
		PerPage:    params.PerPage,
	}, nil
}

func (r *learningOutcomeResultRepo) ListByUserAndContext(ctx context.Context, userID uint, contextType string, contextID uint) ([]models.LearningOutcomeResult, error) {
	var results []models.LearningOutcomeResult
	if err := r.db.WithContext(ctx).Where("user_id = ? AND context_type = ? AND context_id = ?", userID, contextType, contextID).Order("created_at DESC").Find(&results).Error; err != nil {
		return nil, err
	}
	return results, nil
}

func (r *learningOutcomeResultRepo) ListByUserAndOutcomeIDs(ctx context.Context, userID uint, outcomeIDs []uint) ([]models.LearningOutcomeResult, error) {
	if len(outcomeIDs) == 0 {
		return nil, nil
	}
	var results []models.LearningOutcomeResult
	if err := r.db.WithContext(ctx).
		Where("user_id = ? AND learning_outcome_id IN ?", userID, outcomeIDs).
		Order("assessed_at ASC").
		Find(&results).Error; err != nil {
		return nil, err
	}
	return results, nil
}
