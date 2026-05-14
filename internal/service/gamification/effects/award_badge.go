package effects

import (
	"context"
	"fmt"

	"github.com/EduThemes/paper-lms/internal/domain/models"
)

// AwardBadge is the effect side of badge issuance. Like AwardCurrency,
// rules reference badges by `Code` for portability; this effect resolves
// (code → badge_id) via the badge repository's FindByCode walk and then
// idempotently inserts a row into gamification_badge_awards.
//
// Idempotency contract: a rule that fires twice for the same user/badge
// produces exactly one issuance row. The W2-D repo enforces this via
// `INSERT ... ON CONFLICT DO NOTHING` against `uniq_gam_badge_award`;
// the effect surfaces the deduplication in its EffectResult so the
// rule_evaluation row records "second fire deduplicated" rather than a
// silent no-op.
//
// Chain emit (W2-E.1): on first-time awards (created=true) AwardBadge
// fires a `badge.earned` event back through the rules engine via the
// optional EffectDeps.BadgeEmit sink. Dedup'd awards (created=false)
// do not re-emit. The interface (effects.BadgeEarnedEmitter) lives in
// this package — gamification.Emitter satisfies it structurally,
// avoiding an effects→gamification import cycle (gamification already
// depends on effects via the dispatcher).
//
// When BadgeEmit is nil (legacy dispatcher wiring, unit tests that
// don't supply one), the chain emit is silently skipped and the badge
// is still awarded.
type AwardBadge struct {
	Code string `json:"code"`
	// Evidence is an optional caller-provided string surfaced into the
	// EffectResult's Detail for audit. Rules typically leave this empty;
	// the manual-award admin handler can stuff a free-form reason here.
	Evidence string `json:"evidence,omitempty"`
}

func (a AwardBadge) Kind() string { return "AwardBadge" }

func (a AwardBadge) Apply(ctx context.Context, deps EffectDeps, trig TriggeringContext) (EffectResult, error) {
	if a.Code == "" {
		return EffectResult{}, fmt.Errorf("AwardBadge.Code must be non-empty")
	}
	if deps.Badge == nil || deps.BadgeAward == nil {
		return EffectResult{}, fmt.Errorf("AwardBadge requires Badge and BadgeAward deps")
	}

	badge, err := ResolveBadgeByCode(ctx, deps.Badge, trig.TenantID, trig.ScopeType, trig.ScopeID, a.Code)
	if err != nil {
		return EffectResult{}, fmt.Errorf("resolve badge %q: %w", a.Code, err)
	}
	if badge == nil {
		return EffectResult{}, fmt.Errorf("badge %q not defined in tenant %d at %s/%d or site", a.Code, trig.TenantID, trig.ScopeType, trig.ScopeID)
	}

	award := &models.GamificationBadgeAward{
		UserID:          trig.ActorID,
		BadgeID:         badge.ID,
		EvidenceEventID: trig.EventID,
		// AwardedBy left nil — this is a rule-fired award, not a manual
		// grant. The manual-award HTTP handler is the only path that
		// sets AwardedBy.
	}
	created, err := deps.BadgeAward.Award(ctx, award)
	if err != nil {
		return EffectResult{}, fmt.Errorf("issue badge: %w", err)
	}

	detail := map[string]any{
		"code":         a.Code,
		"badge_id":     badge.ID,
		"first_time":   created,
		"award_id":     award.ID,
	}
	if a.Evidence != "" {
		detail["evidence"] = a.Evidence
	}
	summary := fmt.Sprintf("badge %q awarded to user %d", a.Code, trig.ActorID)
	if !created {
		summary = fmt.Sprintf("badge %q deduplicated for user %d (already held)", a.Code, trig.ActorID)
	}

	// Chain `badge.earned` only on first-time awards. The idempotency
	// guard at the BadgeAward repo bounds the recursion: a badge.earned
	// rule that grants the same badge yields created=false on the
	// second hop and stops here.
	if created && deps.BadgeEmit != nil {
		if err := deps.BadgeEmit.EmitBadgeEarned(
			ctx,
			trig.TenantID,
			trig.ActorID,
			badge.ID,
			trig.ScopeType,
			trig.ScopeID,
			trig.EventID,
		); err != nil {
			// Emit failure surfaces in the audit detail but does NOT
			// roll back the award — the badge is already issued and
			// the user-visible outcome is correct. A future maintainer
			// reading the rule_evaluation row sees exactly why no
			// downstream rule fired.
			detail["chain_emit_error"] = err.Error()
		} else {
			detail["chain_emit"] = "badge.earned"
		}
	}

	return EffectResult{
		Kind:    a.Kind(),
		Summary: summary,
		Detail:  detail,
	}, nil
}
