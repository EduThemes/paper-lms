package postgres

import (
	"context"

	"github.com/EduThemes/paper-lms/internal/domain/models"
	"gorm.io/gorm"
)

// ContentViewRepo persists per-user content-view aggregates.
type ContentViewRepo struct {
	db *gorm.DB
}

func NewContentViewRepository(db *gorm.DB) *ContentViewRepo {
	return &ContentViewRepo{db: db}
}

// IncrementView upserts the (user_id, object_type, object_id) row,
// incrementing view_count and total_seconds and bumping last_viewed_at.
// First-time inserts use the row's `created_at` semantics (default NOW())
// for first_viewed_at. The upsert target is the
// uniq_content_views_user_object constraint defined in migration 000036.
//
// The implementation uses a raw INSERT … ON CONFLICT … DO UPDATE so the
// increment is atomic — concurrent IncrementView calls against the same
// (user, content) tuple linearize at the DB level rather than racing on
// SELECT-then-UPDATE in Go.
func (r *ContentViewRepo) IncrementView(ctx context.Context, userID uint, objectType string, objectID uint, durationSeconds int64) error {
	const stmt = `
		INSERT INTO content_views
			(user_id, object_type, object_id, view_count, total_seconds, first_viewed_at, last_viewed_at)
		VALUES
			(?, ?, ?, 1, ?, NOW(), NOW())
		ON CONFLICT ON CONSTRAINT uniq_content_views_user_object DO UPDATE SET
			view_count    = content_views.view_count + 1,
			total_seconds = content_views.total_seconds + EXCLUDED.total_seconds,
			last_viewed_at = NOW()
	`
	return r.db.WithContext(ctx).Exec(stmt, userID, objectType, objectID, durationSeconds).Error
}

// ListByUserAndObjectIDs is the read path the snapshot loader uses:
// fetches every aggregate row for (userID, objectType) whose object_id
// is in the given slice. Returns an empty slice (no error) when nothing
// matches.
func (r *ContentViewRepo) ListByUserAndObjectIDs(ctx context.Context, userID uint, objectType string, objectIDs []uint) ([]models.ContentView, error) {
	if len(objectIDs) == 0 {
		return nil, nil
	}
	var views []models.ContentView
	err := r.db.WithContext(ctx).
		Where("user_id = ? AND object_type = ? AND object_id IN ?", userID, objectType, objectIDs).
		Find(&views).Error
	return views, err
}

// GetByUserAndObject returns nil (no error) when no row exists; treat
// that as zero views.
func (r *ContentViewRepo) GetByUserAndObject(ctx context.Context, userID uint, objectType string, objectID uint) (*models.ContentView, error) {
	var view models.ContentView
	err := r.db.WithContext(ctx).
		Where("user_id = ? AND object_type = ? AND object_id = ?", userID, objectType, objectID).
		First(&view).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, err
	}
	return &view, nil
}

