package models

import (
	"time"

	"gorm.io/datatypes"
)

// GamificationLeaderboardSnapshot freezes the ranked list at a window
// close (Sunday 00:00 UTC for the v1 weekly cadence). Reads served from
// this table avoid re-walking the wallet ledger for past windows; the
// current open window stays live-compute.
//
// `Payload` is JSONB containing []SnapshotRow sorted by Rank ASC.
// Reading is single-row; pagination happens in-process over the JSON
// array. See migration 000045 for the constraint rationale.
//
// FERPA contract: the snapshot captures the opt-out set as of
// window-close. The HANDLER re-applies `FilterPublicLeaderboardCandidates`
// at read time so a learner who opts out *after* a snapshot is stored
// still vanishes from peer views. The snapshot row is not editable; the
// re-filter is the correctness mechanism.
type GamificationLeaderboardSnapshot struct {
	ID             uint                  `json:"id" gorm:"column:id;primaryKey"`
	ScopeType      GamificationScopeType `json:"scope_type" gorm:"not null;type:text"`
	ScopeID        uint                  `json:"scope_id" gorm:"not null"`
	CurrencyTypeID uint                  `json:"currency_type_id" gorm:"not null"`
	WindowKind     string                `json:"window_kind" gorm:"not null"`
	WindowStart    time.Time             `json:"window_start" gorm:"not null"`
	WindowEnd      time.Time             `json:"window_end" gorm:"not null"`
	ComputedAt     time.Time             `json:"computed_at" gorm:"not null;default:now()"`
	Payload        datatypes.JSON        `json:"payload" gorm:"type:jsonb;not null"`
}

func (GamificationLeaderboardSnapshot) TableName() string {
	return "gamification_leaderboard_snapshots"
}

// SnapshotRow is the per-learner shape inside Snapshot.Payload. Kept
// minimal: user_id + rank + lifetime_earned is what every read path
// needs; name lookup happens against the live users table at render
// time so a learner who legally changes their name doesn't display the
// old version in a historical view.
type SnapshotRow struct {
	UserID         uint  `json:"user_id"`
	Rank           int   `json:"rank"`
	LifetimeEarned int64 `json:"lifetime_earned"`
}

// SnapshotWindowKind is the window-cadence enum mirror. v1 ships only
// 'weekly'; the CHECK constraint in the migration matches.
type SnapshotWindowKind string

const (
	WindowKindWeekly SnapshotWindowKind = "weekly"
)
