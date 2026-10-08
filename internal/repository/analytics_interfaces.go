package repository

import (
	"context"
	"time"

	"github.com/EduThemes/paper-lms/internal/domain/models"
)

// Analytics

type PageViewRepository interface {
	Create(ctx context.Context, pageView *models.PageView) error
	// 13.1.D — tenant scope via polymorphic context_type/context_id.
	// Page views can attach to any contextable entity in any tenant.
	FindByID(ctx context.Context, id, accountID uint) (*models.PageView, error)
	ListByUserID(ctx context.Context, userID, accountID uint, params PaginationParams) (*PaginatedResult[models.PageView], error)
	CountByContextGrouped(ctx context.Context, contextType string, contextID uint) ([]map[string]interface{}, error)
	SumInteractionByUserAndContext(ctx context.Context, userID uint, contextType string, contextID uint) (int64, error)
	// CountByUserAndContext counts one user's page views in one context
	// (e.g. a course) — the per-course "page views" analytics column.
	CountByUserAndContext(ctx context.Context, userID uint, contextType string, contextID uint) (int64, error)

	// Course analytics (G154). Every aggregate counts only the course's
	// students (non-deleted StudentEnrollment rows), the way Canvas course
	// analytics reports student activity. userID 0 means every student.

	// ActivityByDay returns the course's page views and participations per
	// UTC day, oldest first, from since (nil: all time).
	ActivityByDay(ctx context.Context, courseID, userID uint, since *time.Time) ([]PageViewDayCount, error)
	// TotalsByUser returns each student's page-view totals in the course.
	TotalsByUser(ctx context.Context, courseID uint) (map[uint]PageViewUserTotals, error)
	// CategoryCounts groups the course's page views by asset_category.
	CategoryCounts(ctx context.Context, courseID, userID uint) ([]PageViewCategoryCount, error)
	// ListByUserAndContext pages one user's page views in one context,
	// newest first.
	ListByUserAndContext(ctx context.Context, userID uint, contextType string, contextID uint, params PaginationParams) (*PaginatedResult[models.PageView], error)
	// UpdateInteraction raises a page view's interaction_seconds to seconds
	// (never lowers it). Only the row's own user may update it; any other
	// row reports gorm.ErrRecordNotFound.
	UpdateInteraction(ctx context.Context, id, userID uint, seconds int) error
	// TouchEnrollmentActivity stamps enrollments.last_activity_at on the
	// user's enrollments in the course (at most once every few minutes).
	TouchEnrollmentActivity(ctx context.Context, userID, courseID uint) error
}

// PageViewDayCount is one day of course activity.
type PageViewDayCount struct {
	Date           string `json:"date"` // YYYY-MM-DD (UTC)
	Views          int64  `json:"views"`
	Participations int64  `json:"participations"`
}

// PageViewUserTotals are one student's page-view totals in a course.
type PageViewUserTotals struct {
	Views              int64      `json:"page_views"`
	Participations     int64      `json:"participations"`
	InteractionSeconds int64      `json:"interaction_seconds"`
	LastViewedAt       *time.Time `json:"last_activity_at"`
}

// PageViewCategoryCount is the course's page views of one kind of content.
type PageViewCategoryCount struct {
	Category       string `json:"category"`
	Views          int64  `json:"views"`
	Participations int64  `json:"participations"`
}
