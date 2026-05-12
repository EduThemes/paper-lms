// Package gamification holds top-level gamification services that don't fit
// inside predicates/, mastery/, or effects/. Wave 1 contributes the
// system-currency seeder; later sprints add the rule dispatcher and the
// snapshot loader.
package gamification

import (
	"context"
	"fmt"

	"github.com/EduThemes/paper-lms/internal/domain/models"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// systemCurrencySeed is the shape of one of the four hard-coded rows the
// seeder inserts per tenant. Kept private to this package — extending the
// list is a deliberate decision that should land here, not be configured
// at runtime.
type systemCurrencySeed struct {
	Code, Label, LabelPlural string
	Icon, Color, Description string
	Order                    int
	Spendable, Monotonic     bool
	Ferpa                    string
	VisibleInTopbar          bool
}

// systemCurrencies is the canonical four-currency seed list, locked by the
// gamification design (SYNTHESIS.md §2). Each row is `system_owned=true`
// — tenant admins can rename labels/icons/colors but cannot delete these
// because rules and capability unlocks reference them by Code.
var systemCurrencies = []systemCurrencySeed{
	{
		Code: "xp", Label: "XP", LabelPlural: "XP",
		Icon: "zap", Color: "#F59E0B", Order: 1,
		Spendable: false, Monotonic: true,
		Ferpa: "non_PII", VisibleInTopbar: true,
		Description: "Experience points. Earned through any productive activity. Never decreases.",
	},
	{
		Code: "gems", Label: "Gem", LabelPlural: "Gems",
		Icon: "gem", Color: "#A855F7", Order: 2,
		Spendable: true, Monotonic: false,
		Ferpa: "non_PII", VisibleInTopbar: true,
		Description: "Rare currency for the shop. Earned through quests, perfect scores, and surprises.",
	},
	{
		Code: "mastery_points", Label: "Mastery Point", LabelPlural: "Mastery Points",
		Icon: "target", Color: "#0EA5E9", Order: 3,
		Spendable: false, Monotonic: true,
		Ferpa: "education_record", VisibleInTopbar: false,
		Description: "Skill mastery. Tied to learning outcomes. Visible to student and teacher only.",
	},
	{
		Code: "reputation", Label: "Rep", LabelPlural: "Rep",
		Icon: "shield-check", Color: "#10B981", Order: 4,
		Spendable: false, Monotonic: true,
		Ferpa: "non_PII", VisibleInTopbar: true,
		Description: "Community reputation. Earned through helpful contributions. Unlocks capabilities.",
	},
}

// SeedSystemCurrenciesForTenant inserts the four system-owned currencies
// (xp, gems, mastery_points, reputation) at site scope for the given
// tenant. Idempotent: relies on the uniq_gam_currency_scope_code unique
// index (migration 000034) and ON CONFLICT DO NOTHING, so re-running on a
// populated tenant is a no-op.
//
// Wave 1 always seeds at site scope with scope_id = tenantID, since the
// `accounts` table is the only tenant-like root in the schema today.
// District/school scopes will land when those models are introduced.
func SeedSystemCurrenciesForTenant(ctx context.Context, db *gorm.DB, tenantID uint) error {
	if tenantID == 0 {
		return fmt.Errorf("SeedSystemCurrenciesForTenant: tenantID must be > 0")
	}
	rows := make([]models.GamificationCurrencyType, 0, len(systemCurrencies))
	for _, s := range systemCurrencies {
		rows = append(rows, models.GamificationCurrencyType{
			TenantID:            tenantID,
			ScopeType:           models.ScopeSite,
			ScopeID:             tenantID,
			Code:                s.Code,
			DisplayLabel:        s.Label,
			DisplayLabelPlural:  s.LabelPlural,
			Icon:                s.Icon,
			Color:               s.Color,
			DisplayOrder:        s.Order,
			Spendable:           s.Spendable,
			Monotonic:           s.Monotonic,
			FerpaClassification: s.Ferpa,
			VisibleToStudent:    true,
			VisibleInTopbar:     s.VisibleInTopbar,
			SystemOwned:         true,
			Description:         s.Description,
		})
	}
	return db.WithContext(ctx).
		Clauses(clause.OnConflict{DoNothing: true}).
		Create(&rows).Error
}
