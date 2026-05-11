package postgres

import (
	"context"

	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/repository"
	"gorm.io/gorm"
)

type pageViewRepo struct {
	db *gorm.DB
}

func NewPageViewRepository(db *gorm.DB) repository.PageViewRepository {
	return &pageViewRepo{db: db}
}

func (r *pageViewRepo) Create(ctx context.Context, pageView *models.PageView) error {
	return r.db.WithContext(ctx).Create(pageView).Error
}

func (r *pageViewRepo) FindByID(ctx context.Context, id uint) (*models.PageView, error) {
	var pageView models.PageView
	if err := r.db.WithContext(ctx).First(&pageView, id).Error; err != nil {
		return nil, err
	}
	return &pageView, nil
}

func (r *pageViewRepo) ListByUserID(ctx context.Context, userID uint, params repository.PaginationParams) (*repository.PaginatedResult[models.PageView], error) {
	var pageViews []models.PageView
	var count int64

	query := r.db.WithContext(ctx).Model(&models.PageView{}).Where("user_id = ?", userID)
	query.Count(&count)

	offset := (params.Page - 1) * params.PerPage
	if err := query.Offset(offset).Limit(params.PerPage).Order("created_at DESC").Find(&pageViews).Error; err != nil {
		return nil, err
	}

	return &repository.PaginatedResult[models.PageView]{
		Items:      pageViews,
		TotalCount: count,
		Page:       params.Page,
		PerPage:    params.PerPage,
	}, nil
}

func (r *pageViewRepo) CountByContextGrouped(ctx context.Context, contextType string, contextID uint) ([]map[string]interface{}, error) {
	var results []map[string]interface{}

	rows, err := r.db.WithContext(ctx).Model(&models.PageView{}).
		Select("DATE(created_at) as date, COUNT(*) as views").
		Where("context_type = ? AND context_id = ?", contextType, contextID).
		Group("DATE(created_at)").
		Order("date ASC").
		Rows()
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var date string
		var views int64
		if err := rows.Scan(&date, &views); err != nil {
			return nil, err
		}
		results = append(results, map[string]interface{}{
			"date":  date,
			"views": views,
		})
	}

	if results == nil {
		results = []map[string]interface{}{}
	}

	return results, nil
}

func (r *pageViewRepo) SumInteractionByUserAndContext(ctx context.Context, userID uint, contextType string, contextID uint) (int64, error) {
	var total int64
	err := r.db.WithContext(ctx).Model(&models.PageView{}).
		Select("COALESCE(SUM(interaction_seconds), 0)").
		Where("user_id = ? AND context_type = ? AND context_id = ?", userID, contextType, contextID).
		Scan(&total).Error
	return total, err
}
