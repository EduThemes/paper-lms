package effects

import (
	"context"

	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/repository"
)

// ResolveCurrencyByCode walks the org tree from the trigger's scope upward
// (section → course → school → district → site) and returns the first
// match for (tenant_id, code). Returns (nil, nil) when no match is found
// at any scope level — callers treat that as "this currency isn't defined
// here."
//
// Wave 1 walk: try the given scope, then fall back to site. School and
// district are skipped because Paper LMS doesn't model those relationships
// yet. Section→course rollup is also not modeled — sections don't carry
// course_id in a reachable form for this resolver. Once those edges land,
// extend this function to fetch the parent scope id at each step.
//
// Future scopes added here must preserve the section→course→school→
// district→site ordering: the first match wins, so a course-scoped
// override of a site-scoped currency correctly takes precedence.
func ResolveCurrencyByCode(
	ctx context.Context,
	repo repository.GamificationCurrencyTypeRepository,
	tenantID uint,
	scopeType models.GamificationScopeType,
	scopeID uint,
	code string,
) (*models.GamificationCurrencyType, error) {
	if c, err := repo.FindByCode(ctx, tenantID, scopeType, scopeID, code); err != nil || c != nil {
		return c, err
	}
	// Fall back to site-scoped only if we weren't already looking at site.
	if scopeType != models.ScopeSite {
		if c, err := repo.FindByCode(ctx, tenantID, models.ScopeSite, tenantID, code); err != nil || c != nil {
			return c, err
		}
	}
	return nil, nil
}

// ResolveBadgeByCode is the badge equivalent of ResolveCurrencyByCode —
// same section → course → school → district → site walk, same first-
// match-wins semantics, same (nil, nil) on no match. Powers W2-D's
// AwardBadge effect.
func ResolveBadgeByCode(
	ctx context.Context,
	repo repository.GamificationBadgeRepository,
	tenantID uint,
	scopeType models.GamificationScopeType,
	scopeID uint,
	code string,
) (*models.GamificationBadge, error) {
	if b, err := repo.FindByCode(ctx, tenantID, scopeType, scopeID, code); err != nil || b != nil {
		return b, err
	}
	if scopeType != models.ScopeSite {
		if b, err := repo.FindByCode(ctx, tenantID, models.ScopeSite, tenantID, code); err != nil || b != nil {
			return b, err
		}
	}
	return nil, nil
}
