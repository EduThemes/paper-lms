package models

import "time"

// GamificationBadge is an admin/instructor-authored achievement. Wave 2
// W2-D ships these as internal-only by design: K-12 mode (the safest
// superset) has no production-grade Open Badges wallet for under-13
// learners, so badges stay server-side. The `InternalOnly=true` default
// stakes out W5's OB 3.0 export pivot — flipping a per-badge value to
// false is the explicit signal that admin/parent consent is on file for
// 3rd-party wallet export.
//
// Rules reference badges by `Code` (e.g., "first_quiz_passed"), same
// resolution pattern as GamificationCurrencyType. The uniqueness
// constraint `uniq_gam_badge_scope_code` (migration 000041) lives on
// (tenant_id, scope_type, scope_id, code).
//
// Indexes are owned by the SQL chain; this model declares none.
type GamificationBadge struct {
	ID          uint                  `json:"id" gorm:"column:id;primaryKey"`
	TenantID    uint                  `json:"tenant_id" gorm:"not null"`
	ScopeType   GamificationScopeType `json:"scope_type" gorm:"not null;type:text"`
	ScopeID     uint                  `json:"scope_id" gorm:"not null"`
	Code        string                `json:"code" gorm:"not null"`
	Name        string                `json:"name" gorm:"not null"`
	Description string                `json:"description"`
	Icon        string                `json:"icon"`
	ImageURL    string                `json:"image_url"`
	Color       string                `json:"color"`
	// InternalOnly default TRUE per SYNTHESIS §5. No `default:` GORM tag —
	// the repo Create writes every column explicitly so this bool never
	// hits the bool-default elision class (W2-A lesson).
	InternalOnly bool `json:"internal_only"`
	SystemOwned  bool `json:"system_owned"`
	// AudienceLevel is informational today (e.g., 'k5', 'm68', 'h912',
	// 'higher_ed', 'corp', 'pro'). Wave 3's audience-filter rules can
	// consult this; W2-D's CRUD just round-trips it. Aligned to the
	// gamification_audience enum in migration 000050 (F1.11 closeout).
	// Nullable — badges without an explicit audience apply broadly.
	// The pointer is load-bearing: GORM serializes a `string` as ''
	// not NULL, and the column shape post-000050 is nullable enum, so
	// pointer-or-NULL is the only correct Go-side shape.
	AudienceLevel *GamificationAudience `json:"audience_level,omitempty" gorm:"type:text"`
	CreatedBy     *uint                 `json:"created_by,omitempty"`
	CreatedAt     time.Time             `json:"created_at" gorm:"not null;default:now()"`
	UpdatedAt     time.Time             `json:"updated_at" gorm:"not null;default:now()"`
}

func (GamificationBadge) TableName() string { return "gamification_badges" }

// GamificationBadgeAward is one (user, badge) issuance. The unique
// constraint `uniq_gam_badge_award` enforces "a user holds each badge at
// most once"; the W2-D AwardBadge effect leans on this via
// INSERT ... ON CONFLICT DO NOTHING for atomic idempotency.
//
// EvidenceEventID is the optional pointer to the gamification_events row
// that *caused* this award (a rule-fired AwardBadge sets it; a manual
// admin grant leaves it nil). Powers the audit trail in
// /profile/badges → "earned for: [event]".
type GamificationBadgeAward struct {
	ID              uint      `json:"id" gorm:"column:id;primaryKey"`
	UserID          uint      `json:"user_id" gorm:"not null"`
	BadgeID         uint      `json:"badge_id" gorm:"not null"`
	AwardedAt       time.Time `json:"awarded_at" gorm:"not null;default:now()"`
	AwardedBy       *uint     `json:"awarded_by,omitempty"`
	EvidenceEventID *uint     `json:"evidence_event_id,omitempty"`
}

func (GamificationBadgeAward) TableName() string { return "gamification_badge_awards" }
