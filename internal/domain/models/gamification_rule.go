package models

import (
	"time"

	"gorm.io/datatypes"
)

// GamificationScopeType mirrors the gamification_scope_type Postgres enum
// declared in migration 000033. Values track the org hierarchy a rule or
// currency lives in. Resolution order for currency lookups (Wave 2):
// section > course > school > district > site.
type GamificationScopeType string

const (
	ScopeSite     GamificationScopeType = "site"
	ScopeDistrict GamificationScopeType = "district"
	ScopeSchool   GamificationScopeType = "school"
	ScopeCourse   GamificationScopeType = "course"
	ScopeSection  GamificationScopeType = "section"
)

// GamificationAudience mirrors the gamification_audience Postgres enum.
// Drives pedagogical defaults — K-5 is the safest superset.
type GamificationAudience string

const (
	AudienceK5       GamificationAudience = "k5"
	AudienceM68      GamificationAudience = "m68"
	AudienceH912     GamificationAudience = "h912"
	AudienceHigherEd GamificationAudience = "higher_ed"
	AudienceCorp     GamificationAudience = "corp"
	AudiencePro      GamificationAudience = "pro"
)

// GamificationRule binds a trigger_event to a condition_set and a list of
// effects. The condition_set is a recursive AND/OR/N_OF_M predicate tree
// stored as JSONB; the predicate evaluator (service/gamification/
// predicates) walks it. The effects array is ordered; each effect runs
// in sequence and may short-circuit on error.
//
// trigger_event variants (JSONB):
//
//	{"kind":"OnEvent",        "verb":"completed", "object_type":"Quiz"}
//	{"kind":"OnSchedule",     "cron":"0 3 * * *"}
//	{"kind":"OnManualTrigger","handle":"award_xp"}
//
// max_per_window JSONB: {"window":"day"|"week"|"lifetime","count":N}.
//
// ScopeType is stored as the Postgres enum (`gamification_scope_type`)
// declared in 000033. The Go field is a plain string so AutoMigrate
// doesn't try to fabricate a type tag the schema-parity diff would
// otherwise flag as type drift. Indexes are owned by the SQL chain.
type GamificationRule struct {
	ID              uint                 `json:"id" gorm:"primaryKey"`
	TenantID        uint                 `json:"tenant_id" gorm:"not null"`
	ScopeType       GamificationScopeType `json:"scope_type" gorm:"not null;type:text"`
	ScopeID         uint                 `json:"scope_id" gorm:"not null"`
	AudienceLevel   GamificationAudience  `json:"audience_level" gorm:"not null;type:text"`
	Name            string               `json:"name" gorm:"not null"`
	Description     string               `json:"description"`
	Enabled         bool                 `json:"enabled" gorm:"not null;default:true"`
	TriggerEvent    datatypes.JSON       `json:"trigger_event" gorm:"type:jsonb;not null"`
	ConditionSet    datatypes.JSON       `json:"condition_set" gorm:"type:jsonb;not null"`
	Effects         datatypes.JSON       `json:"effects" gorm:"type:jsonb;not null"`
	CooldownSeconds *int                 `json:"cooldown_seconds,omitempty"`
	MaxPerWindow    datatypes.JSON       `json:"max_per_window,omitempty" gorm:"type:jsonb"`
	CreatedBy       *uint                `json:"created_by,omitempty"`
	CreatedAt       time.Time            `json:"created_at" gorm:"not null;default:now()"`
	UpdatedAt       time.Time            `json:"updated_at" gorm:"not null;default:now()"`
}

func (GamificationRule) TableName() string { return "gamification_rules" }

// GamificationRuleEvaluation is the audit trail of one rule firing for
// one user at one moment. predicate_state captures the snapshot used so
// a teacher can ask "why didn't this rule fire?" weeks later.
//
// Composite uniqueness on (rule_id, user_id, evaluated_at) is enforced
// at the SQL chain (uniq_gam_eval_rule_user_time). The Go field is
// surrogate-keyed (ID bigserial) for clean repository ergonomics.
type GamificationRuleEvaluation struct {
	ID                uint           `json:"id" gorm:"primaryKey"`
	RuleID            uint           `json:"rule_id" gorm:"not null"`
	UserID            uint           `json:"user_id" gorm:"not null"`
	EvaluatedAt       time.Time      `json:"evaluated_at" gorm:"not null;default:now()"`
	PredicateState    datatypes.JSON `json:"predicate_state,omitempty" gorm:"type:jsonb"`
	Result            bool           `json:"result" gorm:"not null"`
	EffectsFired      datatypes.JSON `json:"effects_fired,omitempty" gorm:"type:jsonb"`
	TriggeringEventID *uint          `json:"triggering_event_id,omitempty"`
}

func (GamificationRuleEvaluation) TableName() string { return "gamification_rule_evaluations" }
