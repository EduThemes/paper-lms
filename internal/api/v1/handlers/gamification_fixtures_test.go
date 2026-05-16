// Shared test fixtures for the gamification handler suite. Extracted
// from gamification_test.go (the original 1100-line god-file) to make
// the fixture set discoverable and re-usable across the leaderboard,
// pseudonym, and recipe handler tests.
//
// Closes F2.5 from docs/audits/2026-05-15-gamification-audit.md. The
// audit verdict was PARTIAL — these helpers move here so future test
// additions don't have to scroll past 700 lines of mocks to find them.
//
// The mocks (mockGamWalletRepo et al.) and the setupGamificationHandler
// builder remain in gamification_test.go for now because they carry
// testify mock.Mock state that's tightly coupled to that file's
// individual test bodies. Migrating them is a separate, larger refactor.
package handlers_test

import (
	"github.com/EduThemes/paper-lms/internal/domain/models"
)

// fixtureXP returns the canonical XP currency the seed installs for
// every tenant. ID = 11, site-scoped, system-owned, non-spendable.
func fixtureXP() models.GamificationCurrencyType {
	return models.GamificationCurrencyType{
		ID:                 11,
		TenantID:           1,
		ScopeType:          models.ScopeSite,
		ScopeID:            1,
		Code:               "xp",
		DisplayLabel:       "XP",
		DisplayLabelPlural: "XP",
		Icon:               "zap",
		Color:              "#F59E0B",
		DisplayOrder:       1,
		Spendable:          false,
		Monotonic:          true,
		VisibleToStudent:   true,
		VisibleInTopbar:    true,
		SystemOwned:        true,
		Description:        "Experience points",
	}
}

// fixtureGems returns the spendable Gems currency. ID = 12.
func fixtureGems() models.GamificationCurrencyType {
	return models.GamificationCurrencyType{
		ID:                 12,
		TenantID:           1,
		ScopeType:          models.ScopeSite,
		ScopeID:            1,
		Code:               "gems",
		DisplayLabel:       "Gem",
		DisplayLabelPlural: "Gems",
		Icon:               "gem",
		Color:              "#10B981",
		DisplayOrder:       2,
		Spendable:          true,
		Monotonic:          false,
		VisibleToStudent:   true,
		VisibleInTopbar:    true,
		SystemOwned:        true,
	}
}

// fixtureHidden returns Mastery Points — visible to students but NOT
// in the topbar. Used by tests that exercise the "internal-only"
// surface path (e.g. wallet drawer that shows everything; topbar pill
// row that shows only VisibleInTopbar=true).
func fixtureHidden() models.GamificationCurrencyType {
	return models.GamificationCurrencyType{
		ID:               13,
		TenantID:         1,
		Code:             "mastery_points",
		DisplayLabel:     "Mastery",
		DisplayOrder:     3,
		VisibleToStudent: true,
		VisibleInTopbar:  false,
		SystemOwned:      true,
	}
}

// fixtureBadge is the canonical "First Quiz" site-scope badge used in
// badge CRUD tests. ID = 50, InternalOnly default-on.
func fixtureBadge() models.GamificationBadge {
	return models.GamificationBadge{
		ID:           50,
		TenantID:     1,
		ScopeType:    models.ScopeSite,
		ScopeID:      1,
		Code:         "first_quiz",
		Name:         "First Quiz",
		Description:  "Pass your first quiz.",
		Icon:         "trophy",
		Color:        "#F59E0B",
		InternalOnly: true,
	}
}
