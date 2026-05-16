package models

import (
	"time"

	"gorm.io/datatypes"
)

// GamificationCurrencyType is a user-defined currency. MyCred pattern.
// Each tenant/course/section can define unlimited currencies. Four are
// system-seeded on tenant creation (xp, gems, mastery_points, reputation)
// — those rows carry SystemOwned=true and cannot be deleted.
//
// Rules reference currencies by Code (e.g. "xp", "coins") for portability
// across tenants and rule templates; the evaluator resolves code → id at
// apply time. The (tenant_id, scope_type, scope_id, code) tuple is
// uniquely indexed (uniq_gam_currency_scope_code in 000034).
//
// FerpaClassification gates which surfaces a balance may appear on.
// mastery_points seeds with 'education_record' — the data-access layer
// will block it from public leaderboards regardless of teacher config.
//
// Indexes are owned by the SQL chain; the model declares no `index:` tags.
type GamificationCurrencyType struct {
	ID       uint `json:"id" gorm:"primaryKey"`
	TenantID uint `json:"tenant_id" gorm:"not null"`
	// ScopeType + ScopeID locate the currency in the org tree.
	// Convention (load-bearing, undocumented before F1.8): for
	// ScopeType=site rows, ScopeID == TenantID (NOT 0). The
	// SeedSystemCurrenciesForTenant seeder establishes this; handlers
	// querying site-scope must use FindByCode(..., ScopeSite, tenantID, code).
	// Course and section scopes use the natural foreign id.
	ScopeType GamificationScopeType `json:"scope_type" gorm:"not null;type:text"`
	ScopeID   uint                  `json:"scope_id" gorm:"not null"`
	Code                string               `json:"code" gorm:"not null"`
	DisplayLabel        string               `json:"display_label" gorm:"not null"`
	DisplayLabelPlural  string               `json:"display_label_plural"`
	Icon                string               `json:"icon"`
	Color               string               `json:"color"`
	DisplayOrder        int                  `json:"display_order" gorm:"not null;default:0"`
	Spendable           bool                 `json:"spendable" gorm:"not null;default:false"`
	Monotonic           bool                 `json:"monotonic" gorm:"not null;default:true"`
	FerpaClassification FerpaClassification  `json:"ferpa_classification" gorm:"not null;type:text;default:'non_PII'"`
	MaxBalance          *int64               `json:"max_balance,omitempty"`
	DecayPolicy         datatypes.JSON       `json:"decay_policy,omitempty" gorm:"type:jsonb"`
	VisibleToStudent    bool                 `json:"visible_to_student" gorm:"not null;default:true"`
	VisibleInTopbar     bool                 `json:"visible_in_topbar" gorm:"not null;default:true"`
	SystemOwned         bool                 `json:"system_owned" gorm:"not null;default:false"`
	Description         string               `json:"description"`
	CreatedAt           time.Time            `json:"created_at" gorm:"not null;default:now()"`
	UpdatedAt           time.Time            `json:"updated_at" gorm:"not null;default:now()"`
}

func (GamificationCurrencyType) TableName() string { return "gamification_currency_types" }
