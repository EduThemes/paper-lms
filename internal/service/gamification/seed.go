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
	// Raw INSERT (not gorm.Create) because GORM's `default:` tags cause
	// zero-valued bools to be elided in favor of the column DEFAULT —
	// which silently flips `mastery_points.visible_in_topbar` and
	// `gems.monotonic` to TRUE in violation of SYNTHESIS §2's FERPA
	// contract and the four-currency design. Every column is written
	// explicitly here; ON CONFLICT DO NOTHING keeps this idempotent
	// against the uniq_gam_currency_scope_code unique index.
	const insertSQL = `
		INSERT INTO gamification_currency_types
			(tenant_id, scope_type, scope_id, code, display_label,
			 display_label_plural, icon, color, display_order, spendable,
			 monotonic, ferpa_classification, visible_to_student,
			 visible_in_topbar, system_owned, description, created_at, updated_at)
		VALUES
			(?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, now(), now())
		ON CONFLICT ON CONSTRAINT uniq_gam_currency_scope_code DO NOTHING`
	tx := db.WithContext(ctx)
	for _, s := range systemCurrencies {
		if err := tx.Exec(insertSQL,
			tenantID, models.ScopeSite, tenantID, s.Code, s.Label,
			s.LabelPlural, s.Icon, s.Color, s.Order, s.Spendable,
			s.Monotonic, s.Ferpa, true,
			s.VisibleInTopbar, true, s.Description,
		).Error; err != nil {
			return fmt.Errorf("seed %s: %w", s.Code, err)
		}
	}
	return nil
}
