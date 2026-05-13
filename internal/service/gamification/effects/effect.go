// Package effects implements the effect side of the Phase 6 rules engine.
// Where predicates decide whether a rule fires, effects describe what
// happens when it does.
//
// Wave 1 ships only AwardCurrency; the surface area below is shaped to
// match every effect named in SYNTHESIS.md §1 (AwardBadge, ReleaseContent,
// BranchPath, UnlockCapability, Notify, AdvanceRankOrLevel, EnrollInGroup)
// without refactor. Each future effect implements the same `Effect`
// interface, accepts the same `EffectDeps` (extended with whatever new
// repository it needs), and is invoked against the same
// `TriggeringContext` produced by the rule dispatcher.
package effects

import (
	"context"

	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/repository"
)

// Effect is the unit of "thing that happens when a rule fires." Effects are
// pure descriptions; their dependencies arrive via EffectDeps at Apply time
// so each rule_evaluation can be replayed deterministically.
type Effect interface {
	// Kind identifies the effect's JSONB discriminator (e.g.
	// "AwardCurrency"). Used by the dispatcher's factory and the
	// rule_evaluation row's effects_fired audit trail.
	Kind() string

	// Apply runs the effect against the given dependencies and triggering
	// context. The returned EffectResult is appended to the rule_evaluation
	// row's effects_fired JSONB so debuggers and audits can see exactly
	// what happened, in order.
	Apply(ctx context.Context, deps EffectDeps, trig TriggeringContext) (EffectResult, error)
}

// EffectDeps is the bag of repositories effects pull from. Future effects
// extend this struct with their own dependencies (NotificationDispatcher,
// ContentReleaseService, …); old effects ignore the new fields. Adding
// a field is non-breaking — calling code only sets the deps it provides.
type EffectDeps struct {
	Wallet       repository.GamificationWalletRepository
	CurrencyType repository.GamificationCurrencyTypeRepository
	// W2-D: badge deps for AwardBadge. Nil-safe — effects that don't
	// touch badges (AwardCurrency, etc.) never read these fields, and
	// dispatcher wiring that doesn't ship badges yet can leave them nil.
	Badge      repository.GamificationBadgeRepository
	BadgeAward repository.GamificationBadgeAwardRepository
}

// TriggeringContext is everything an effect needs to know about the event
// + rule that brought it here. Populated by the rule dispatcher (Sprint C)
// from the gamification_events row + the gamification_rules row.
type TriggeringContext struct {
	ActorID   uint
	TenantID  uint
	ScopeType models.GamificationScopeType
	ScopeID   uint
	EventID   *uint // gamification_events.id; nil for replay/backfill
	RuleID    uint  // gamification_rules.id
}

// EffectResult is the audit-trail row written into
// gamification_rule_evaluations.effects_fired. Kind mirrors Effect.Kind();
// Summary is a human-readable one-liner for teacher-facing debuggers;
// Detail is the machine-readable record of exactly what changed.
type EffectResult struct {
	Kind    string         `json:"kind"`
	Summary string         `json:"summary"`
	Detail  map[string]any `json:"detail,omitempty"`
}
