package postgres

import (
	"context"

	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/repository"
	"gorm.io/gorm"
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

func (r *learningOutcomeResultRepo) Upsert(ctx context.Context, result *models.LearningOutcomeResult) error {
	var existing models.LearningOutcomeResult
	err := r.db.WithContext(ctx).Where("user_id = ? AND learning_outcome_id = ? AND associated_asset_type = ? AND associated_asset_id = ?",
		result.UserID, result.LearningOutcomeID, result.AssociatedAssetType, result.AssociatedAssetID).First(&existing).Error
	if err == gorm.ErrRecordNotFound {
		return r.db.WithContext(ctx).Create(result).Error
	}
	if err != nil {
		return err
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
	return r.db.WithContext(ctx).Save(&existing).Error
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

func (r *learningOutcomeResultRepo) FindByUserOutcomeAsset(ctx context.Context, userID, outcomeID uint, assetType string, assetID uint) (*models.LearningOutcomeResult, error) {
	var existing models.LearningOutcomeResult
	if err := r.db.WithContext(ctx).
		Where("user_id = ? AND learning_outcome_id = ? AND associated_asset_type = ? AND associated_asset_id = ?",
			userID, outcomeID, assetType, assetID).
		First(&existing).Error; err != nil {
		return nil, err
	}
	return &existing, nil
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
