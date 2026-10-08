package postgres

import (
	"context"
	"time"

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

func (r *pageViewRepo) CountByUserAndContext(ctx context.Context, userID uint, contextType string, contextID uint) (int64, error) {
	var total int64
	err := r.db.WithContext(ctx).Model(&models.PageView{}).
		Where("user_id = ? AND context_type = ? AND context_id = ?", userID, contextType, contextID).
		Count(&total).Error
	return total, err
}

func (r *pageViewRepo) SumInteractionByUserAndContext(ctx context.Context, userID uint, contextType string, contextID uint) (int64, error) {
	var total int64
	err := r.db.WithContext(ctx).Model(&models.PageView{}).
		Select("COALESCE(SUM(interaction_seconds), 0)").
		Where("user_id = ? AND context_type = ? AND context_id = ?", userID, contextType, contextID).
		Scan(&total).Error
	return total, err
}

// courseStudentsFilter limits page views to the course's students: course
// analytics reports student activity, not the instructors' own browsing.
const courseStudentsFilter = `page_views.context_type = 'Course' AND page_views.context_id = ?
	AND page_views.user_id IN (
		SELECT e.user_id FROM enrollments e
		WHERE e.course_id = ? AND e.type = 'StudentEnrollment' AND e.workflow_state <> 'deleted'
	)`

func (r *pageViewRepo) courseStudentViews(ctx context.Context, courseID, userID uint) *gorm.DB {
	q := r.db.WithContext(ctx).Model(&models.PageView{}).Where(courseStudentsFilter, courseID, courseID)
	if userID != 0 {
		q = q.Where("page_views.user_id = ?", userID)
	}
	return q
}

func (r *pageViewRepo) ActivityByDay(ctx context.Context, courseID, userID uint, since *time.Time) ([]repository.PageViewDayCount, error) {
	q := r.courseStudentViews(ctx, courseID, userID)
	if since != nil {
		q = q.Where("page_views.created_at >= ?", *since)
	}
	var scanned []struct {
		Day            string
		Views          int64
		Participations int64
	}
	err := q.Select(`to_char(page_views.created_at AT TIME ZONE 'UTC', 'YYYY-MM-DD') AS day,
			COUNT(*) FILTER (WHERE NOT COALESCE(page_views.participated, false)) AS views,
			COUNT(*) FILTER (WHERE COALESCE(page_views.participated, false)) AS participations`).
		Group("day").Order("day ASC").Scan(&scanned).Error
	rows := make([]repository.PageViewDayCount, len(scanned))
	for i, s := range scanned {
		rows[i] = repository.PageViewDayCount{Date: s.Day, Views: s.Views, Participations: s.Participations}
	}
	return rows, err
}

func (r *pageViewRepo) TotalsByUser(ctx context.Context, courseID uint) (map[uint]repository.PageViewUserTotals, error) {
	var rows []struct {
		UserID             uint
		Views              int64
		Participations     int64
		InteractionSeconds float64
		LastViewedAt       *time.Time
	}
	err := r.courseStudentViews(ctx, courseID, 0).
		Select(`page_views.user_id AS user_id,
			COUNT(*) FILTER (WHERE NOT COALESCE(page_views.participated, false)) AS views,
			COUNT(*) FILTER (WHERE COALESCE(page_views.participated, false)) AS participations,
			COALESCE(SUM(page_views.interaction_seconds), 0) AS interaction_seconds,
			MAX(page_views.created_at) AS last_viewed_at`).
		Group("page_views.user_id").Scan(&rows).Error
	out := make(map[uint]repository.PageViewUserTotals, len(rows))
	for _, row := range rows {
		out[row.UserID] = repository.PageViewUserTotals{
			Views:              row.Views,
			Participations:     row.Participations,
			InteractionSeconds: int64(row.InteractionSeconds),
			LastViewedAt:       row.LastViewedAt,
		}
	}
	return out, err
}

func (r *pageViewRepo) CategoryCounts(ctx context.Context, courseID, userID uint) ([]repository.PageViewCategoryCount, error) {
	var rows []repository.PageViewCategoryCount
	err := r.courseStudentViews(ctx, courseID, userID).
		Select(`COALESCE(NULLIF(page_views.asset_category, ''), 'other') AS category,
			COUNT(*) FILTER (WHERE NOT COALESCE(page_views.participated, false)) AS views,
			COUNT(*) FILTER (WHERE COALESCE(page_views.participated, false)) AS participations`).
		Group("category").Order("views DESC, category ASC").Scan(&rows).Error
	if rows == nil {
		rows = []repository.PageViewCategoryCount{}
	}
	return rows, err
}

func (r *pageViewRepo) ListByUserAndContext(ctx context.Context, userID uint, contextType string, contextID uint, params repository.PaginationParams) (*repository.PaginatedResult[models.PageView], error) {
	if params.Page < 1 {
		params.Page = 1
	}
	if params.PerPage < 1 {
		params.PerPage = 50
	}
	q := r.db.WithContext(ctx).Model(&models.PageView{}).
		Where("user_id = ? AND context_type = ? AND context_id = ?", userID, contextType, contextID)
	var count int64
	if err := q.Count(&count).Error; err != nil {
		return nil, err
	}
	var items []models.PageView
	if err := q.Order("created_at DESC, id DESC").Offset((params.Page - 1) * params.PerPage).Limit(params.PerPage).Find(&items).Error; err != nil {
		return nil, err
	}
	return &repository.PaginatedResult[models.PageView]{Items: items, TotalCount: count, Page: params.Page, PerPage: params.PerPage}, nil
}

func (r *pageViewRepo) UpdateInteraction(ctx context.Context, id, userID uint, seconds int) error {
	res := r.db.WithContext(ctx).Model(&models.PageView{}).
		Where("id = ? AND user_id = ?", id, userID).
		Update("interaction_seconds", gorm.Expr("GREATEST(COALESCE(interaction_seconds, 0), ?)", seconds))
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

func (r *pageViewRepo) TouchEnrollmentActivity(ctx context.Context, userID, courseID uint) error {
	return r.db.WithContext(ctx).Exec(
		`UPDATE enrollments SET last_activity_at = now()
		 WHERE user_id = ? AND course_id = ? AND workflow_state <> 'deleted'
		   AND (last_activity_at IS NULL OR last_activity_at < now() - interval '5 minutes')`,
		userID, courseID).Error
}
