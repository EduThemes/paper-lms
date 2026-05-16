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

// pageViewTenantFilter is the polymorphic tenant filter for page_views.
// PageViews can attach to any contextable entity (Course, Account, Group,
// User). Apply identical branch logic to FindByID and ListByUserID so
// the high-volume read path stays predictable. accountID==0 disables.
const pageViewTenantFilter = `
	(context_type = 'Course' AND context_id IN (SELECT id FROM courses WHERE account_id = ?))
	OR (context_type = 'Account' AND context_id = ?)
	OR (context_type = 'Group' AND context_id IN (
		SELECT g.id FROM groups g
		WHERE (g.context_type = 'Course' AND g.context_id IN (SELECT id FROM courses WHERE account_id = ?))
		   OR (g.context_type = 'Account' AND g.context_id = ?)
	))
	OR (context_type = 'User' AND context_id IN (SELECT id FROM users WHERE account_id = ?))
`

func (r *pageViewRepo) FindByID(ctx context.Context, id, accountID uint) (*models.PageView, error) {
	var pageView models.PageView
	q := r.db.WithContext(ctx)
	if accountID != 0 {
		q = q.Where(pageViewTenantFilter, accountID, accountID, accountID, accountID, accountID)
	}
	if err := q.First(&pageView, id).Error; err != nil {
		return nil, err
	}
	return &pageView, nil
}

func (r *pageViewRepo) ListByUserID(ctx context.Context, userID, accountID uint, params repository.PaginationParams) (*repository.PaginatedResult[models.PageView], error) {
	var pageViews []models.PageView
	var count int64

	query := r.db.WithContext(ctx).Model(&models.PageView{}).Where("user_id = ?", userID)
	if accountID != 0 {
		query = query.Where(pageViewTenantFilter, accountID, accountID, accountID, accountID, accountID)
	}
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
