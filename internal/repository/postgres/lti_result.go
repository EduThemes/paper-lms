package postgres

import (
	"context"

	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/repository"
	"gorm.io/gorm"
)

type ltiResultRepo struct {
	db *gorm.DB
}

func NewLTIResultRepository(db *gorm.DB) repository.LTIResultRepository {
	return &ltiResultRepo{db: db}
}

func (r *ltiResultRepo) Create(ctx context.Context, result *models.LTIResult) error {
	return r.db.WithContext(ctx).Create(result).Error
}

func (r *ltiResultRepo) FindByID(ctx context.Context, id uint) (*models.LTIResult, error) {
	var result models.LTIResult
	if err := r.db.WithContext(ctx).First(&result, id).Error; err != nil {
		return nil, err
	}
	return &result, nil
}

func (r *ltiResultRepo) Upsert(ctx context.Context, result *models.LTIResult) error {
	var existing models.LTIResult
	err := r.db.WithContext(ctx).Where("line_item_id = ? AND user_id = ?", result.LineItemID, result.UserID).First(&existing).Error
	if err == gorm.ErrRecordNotFound {
		return r.db.WithContext(ctx).Create(result).Error
	}
	if err != nil {
		return err
	}

	existing.ResultScore = result.ResultScore
	existing.ResultMaximum = result.ResultMaximum
	existing.Comment = result.Comment
	existing.ActivityProgress = result.ActivityProgress
	existing.GradingProgress = result.GradingProgress
	existing.Timestamp = result.Timestamp

	result.ID = existing.ID
	return r.db.WithContext(ctx).Save(&existing).Error
}

func (r *ltiResultRepo) ListByLineItem(ctx context.Context, lineItemID uint, params repository.PaginationParams) (*repository.PaginatedResult[models.LTIResult], error) {
	var results []models.LTIResult
	var count int64

	query := r.db.WithContext(ctx).Model(&models.LTIResult{}).Where("line_item_id = ?", lineItemID)
	query.Count(&count)

	offset := (params.Page - 1) * params.PerPage
	if err := query.Offset(offset).Limit(params.PerPage).Order("id ASC").Find(&results).Error; err != nil {
		return nil, err
	}

	return &repository.PaginatedResult[models.LTIResult]{
		Items:      results,
		TotalCount: count,
		Page:       params.Page,
		PerPage:    params.PerPage,
	}, nil
}
