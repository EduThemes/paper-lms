package postgres

import (
	"context"
	"errors"

	"github.com/EduThemes/paper-lms/internal/domain/models"
	"gorm.io/gorm"
)

// GamificationBadgeAwardRepo persists (user, badge) issuances. The
// uniq_gam_badge_award constraint (migration 000041) gives us atomic
// idempotency: re-awarding is a no-op via ON CONFLICT DO NOTHING.
type GamificationBadgeAwardRepo struct {
	db *gorm.DB
}

func NewGamificationBadgeAwardRepository(db *gorm.DB) *GamificationBadgeAwardRepo {
	return &GamificationBadgeAwardRepo{db: db}
}

// Award inserts a (user, badge) row, or no-ops if one already exists.
// The bool return tells the caller whether a NEW award actually
// happened — a future W2-E hook can fan out a `badge.earned` event on
// the first-time-only edge. We use Raw rather than gorm.Create here to
// (a) avoid GORM's bool-default elision on AwardedBy = nil and (b) keep
// the ON CONFLICT semantics explicit + readable.
func (r *GamificationBadgeAwardRepo) Award(ctx context.Context, award *models.GamificationBadgeAward) (bool, error) {
	const insertSQL = `
		INSERT INTO gamification_badge_awards
			(user_id, badge_id, awarded_at, awarded_by, evidence_event_id)
		VALUES
			(?, ?, COALESCE(?, now()), ?, ?)
		ON CONFLICT ON CONSTRAINT uniq_gam_badge_award DO NOTHING
		RETURNING id, awarded_at`
	var awardedAt = award.AwardedAt
	if awardedAt.IsZero() {
		// Pass nil to COALESCE → DB now() so the timestamp is server-side.
		awardedAt = awardedAt // sentinel; we pass nil interface below
	}
	var awardedAtArg any
	if !award.AwardedAt.IsZero() {
		awardedAtArg = award.AwardedAt
	}
	row := r.db.WithContext(ctx).Raw(insertSQL,
		award.UserID, award.BadgeID, awardedAtArg,
		award.AwardedBy, award.EvidenceEventID,
	).Row()
	if err := row.Scan(&award.ID, &award.AwardedAt); err != nil {
		// sql.ErrNoRows on ON CONFLICT DO NOTHING means the (user, badge)
		// already existed — surface that as `created=false, err=nil` so
		// idempotent callers (the AwardBadge effect, manual-award handler
		// on a re-submitted form) don't surface a phantom error.
		if errors.Is(err, gorm.ErrRecordNotFound) || err.Error() == "sql: no rows in result set" {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

// Revoke removes a (user, badge) award. Returns nil even when no row
// existed (idempotent — admins re-clicking should not error).
func (r *GamificationBadgeAwardRepo) Revoke(ctx context.Context, userID, badgeID uint) error {
	return r.db.WithContext(ctx).
		Where("user_id = ? AND badge_id = ?", userID, badgeID).
		Delete(&models.GamificationBadgeAward{}).Error
}

// ListForUser returns every badge issuance for one learner, most recent
// first. Powers /profile/badges.
func (r *GamificationBadgeAwardRepo) ListForUser(ctx context.Context, userID uint) ([]models.GamificationBadgeAward, error) {
	var awards []models.GamificationBadgeAward
	err := r.db.WithContext(ctx).
		Where("user_id = ?", userID).
		Order("awarded_at DESC, id DESC").
		Find(&awards).Error
	return awards, err
}

func (r *GamificationBadgeAwardRepo) FindByUserAndBadge(ctx context.Context, userID, badgeID uint) (*models.GamificationBadgeAward, error) {
	var award models.GamificationBadgeAward
	err := r.db.WithContext(ctx).
		Where("user_id = ? AND badge_id = ?", userID, badgeID).
		First(&award).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &award, nil
}
