package postgres

import (
	"context"
	"errors"
	"strings"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/EduThemes/paper-lms/internal/domain/models"
)

// GamificationLeaderboardSnapshotRepo persists weekly ranked-window
// snapshots. Writes are idempotent against the UNIQUE on
// (scope_type, scope_id, currency_type_id, window_kind, window_end).
type GamificationLeaderboardSnapshotRepo struct {
	db *gorm.DB
}

func NewGamificationLeaderboardSnapshotRepository(db *gorm.DB) *GamificationLeaderboardSnapshotRepo {
	return &GamificationLeaderboardSnapshotRepo{db: db}
}

// Upsert writes the snapshot row. Returns created=true only when the
// row was actually inserted (i.e., no conflict on the window UNIQUE).
// Returning created lets the CLI distinguish "wrote 12 snapshots" from
// "noticed 12 windows were already covered."
//
// Implementation: GORM's clause.OnConflict with DoNothing + the
// RowsAffected reading from the resulting tx, which is 0 on conflict
// and 1 on insert.
func (r *GamificationLeaderboardSnapshotRepo) Upsert(ctx context.Context, snap *models.GamificationLeaderboardSnapshot) (bool, error) {
	tx := r.db.WithContext(ctx).
		Clauses(clause.OnConflict{
			Columns: []clause.Column{
				{Name: "scope_type"},
				{Name: "scope_id"},
				{Name: "currency_type_id"},
				{Name: "window_kind"},
				{Name: "window_end"},
			},
			DoNothing: true,
		}).
		Create(snap)
	if err := tx.Error; err != nil {
		// A constraint shape we don't expect (e.g., CHECK violation on
		// window_kind) bubbles up as the driver-shaped error.
		// Translate the common case so callers can match it cleanly.
		if isCheckViolation(err) {
			return false, errors.New("snapshot violates a CHECK constraint (likely window_kind or window_end > window_start)")
		}
		return false, err
	}
	return tx.RowsAffected > 0, nil
}

// FindByWindow returns the snapshot for the exact window tuple, or
// (nil, nil) if no snapshot exists. The "exact match" semantics are
// intentional — operationally, you ask "do we have the 2026-05-12
// 00:00 UTC weekly snapshot for course 42 / xp?" and either get it or
// know to fall back to live compute.
func (r *GamificationLeaderboardSnapshotRepo) FindByWindow(
	ctx context.Context,
	scopeType models.GamificationScopeType,
	scopeID, currencyTypeID uint,
	kind string,
	windowEnd time.Time,
) (*models.GamificationLeaderboardSnapshot, error) {
	var row models.GamificationLeaderboardSnapshot
	err := r.db.WithContext(ctx).
		Where("scope_type = ? AND scope_id = ? AND currency_type_id = ? AND window_kind = ? AND window_end = ?",
			scopeType, scopeID, currencyTypeID, kind, windowEnd).
		First(&row).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &row, nil
}

// isCheckViolation matches Postgres CHECK constraint errors. Same
// pattern as enrollment.go's isUniqueViolation — the driver string is
// stable across recent pg versions and the path is exercised in tests.
func isCheckViolation(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return strings.Contains(msg, "check constraint") || strings.Contains(msg, "SQLSTATE 23514")
}
