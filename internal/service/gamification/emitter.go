package gamification

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"gorm.io/datatypes"

	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/repository"
	"github.com/EduThemes/paper-lms/internal/service/gamification/effects"
)

// EmitterDeps wraps DispatchDeps with the two extra repositories the Emit
// path needs that the dispatcher itself doesn't: the FERPA-tag repo (for
// pre-flight policy enforcement) and the event repo (for persisting the
// xAPI row before dispatching).
type EmitterDeps struct {
	Dispatch  DispatchDeps
	Events    repository.GamificationEventRepository
	FerpaTags repository.GamificationFerpaFieldTagRepository
}

// EmitResult is the Emitter's return shape: the persisted event plus the
// dispatcher's outcome. Callers that just want the rule outcome can use
// .Dispatch directly; the EventID field is exposed so async retry paths
// can reference the row without re-querying.
type EmitResult struct {
	EventID  uint
	Dispatch DispatchResult
}

// Emitter is the single public entry point gamification-relevant code
// uses to surface activity to the rules engine. Internal services
// (assignment, quiz, lesson, course completion) will call Emit in Sprint
// D once the call-site wiring lands.
//
// Wave 1 dispatch is synchronous: Emit blocks until every matching rule
// has been evaluated and its effects applied. A later wave introduces an
// outbox queue when load profiles demand it; the API shape doesn't
// change.
type Emitter struct {
	deps EmitterDeps

	// now overridable so tests pin the EmittedAt timestamp and the
	// dispatcher's cooldown math runs deterministically.
	now func() time.Time
}

// NewEmitter constructs an Emitter bound to a fixed set of repositories.
func NewEmitter(deps EmitterDeps) *Emitter {
	return &Emitter{deps: deps, now: time.Now}
}

// SetBadgeEmitter wires the chained-emit sink that the AwardBadge effect
// uses to fire `badge.earned` events on first-time awards (W2-E.1).
// Solves the construction-order chicken-and-egg: *Emitter itself
// satisfies the effects.BadgeEarnedEmitter interface, but it can't be
// referenced inside the EmitterDeps literal that builds it.
//
// Calling pattern: `e := NewEmitter(...); e.SetBadgeEmitter(e)`.
// Subsequent Emit calls build a fresh dispatcher off `e.deps.Dispatch`,
// so the BadgeEmit field is picked up by every dispatch from then on.
//
// Passing a nil emitter is allowed and yields the W2-D behavior
// (issue badges but don't chain).
func (e *Emitter) SetBadgeEmitter(b effects.BadgeEarnedEmitter) {
	e.deps.Dispatch.Effects.BadgeEmit = b
}

// Emit runs the full ingest pipeline:
//
//  1. Policy-flag derivation (DerivePolicyFlags) — appends
//     ferpa_protected + education_record to PolicyFlags whenever the
//     event carries an education_record-tagged field. Internal emit
//     call-sites never set these manually; the derivation is the single
//     source of truth.
//  2. FERPA guard (CheckFerpa) — backstop that rejects events whose
//     result/context fields are tagged education_record but whose
//     PolicyFlags STILL don't carry both required flags. After
//     derivation this only fires on hand-built events (e.g. a future
//     POST /events endpoint, webhook bridge, etc.).
//  3. EmittedAt = Emitter.now() if zero (callers that backdate must set
//     OccurredAt explicitly; EmittedAt is always "when the engine saw
//     it").
//  4. Persist the gamification_events row.
//  5. Build a fresh RuleIndex from every enabled rule at site scope
//     for the event's tenant. Course/section/school/district rollup
//     lands in Sprint D; Wave 1 only fires site-scoped rules.
//  6. Dispatch through the rule loop.
//
// Returns (EmitResult, error). The event is persisted before dispatch,
// so a dispatch failure does not erase the event — callers can re-run
// dispatch later by re-fetching the event row.
func (e *Emitter) Emit(ctx context.Context, event *models.GamificationEvent) (EmitResult, error) {
	if event == nil {
		return EmitResult{}, errors.New("emitter: nil event")
	}

	// 1. Derive policy_flags from the FERPA tag lookup.
	if err := DerivePolicyFlags(ctx, e.deps.FerpaTags, event); err != nil {
		return EmitResult{}, fmt.Errorf("derive policy flags: %w", err)
	}

	// 2. FERPA guard. Education-record fields require ferpa_protected +
	// education_record on PolicyFlags. After derivation this only fires
	// for events the caller hand-built without going through Emit's
	// derivation step.
	violations, err := CheckFerpa(ctx, e.deps.FerpaTags, event)
	if err != nil {
		return EmitResult{}, fmt.Errorf("ferpa check: %w", err)
	}
	if len(violations) > 0 {
		return EmitResult{}, fmt.Errorf("ferpa policy violation(s): %s", summarizeViolations(violations))
	}

	// 2. Stamp emitted_at if the caller left it zero.
	if event.EmittedAt.IsZero() {
		event.EmittedAt = e.now()
	}
	if event.OccurredAt.IsZero() {
		// xAPI semantics: occurred_at is required. Default to emitted_at
		// when the caller doesn't have a better signal.
		event.OccurredAt = event.EmittedAt
	}

	// 3. Persist the event.
	if err := e.deps.Events.Create(ctx, event); err != nil {
		return EmitResult{}, fmt.Errorf("persist event: %w", err)
	}

	// 4. Build the rule index for this tenant at site scope.
	rules, err := e.deps.Dispatch.Rules.ListEnabledByScope(ctx, models.ScopeSite, event.TenantID)
	if err != nil {
		return EmitResult{EventID: event.ID}, fmt.Errorf("list rules: %w", err)
	}
	index := BuildRuleIndex(rules)

	// 5. Dispatch through every matching rule.
	disp := NewDispatcher(e.deps.Dispatch)
	disp.now = e.now // share the clock for deterministic tests
	result, err := disp.Dispatch(ctx, index, event)
	if err != nil {
		return EmitResult{EventID: event.ID, Dispatch: result}, fmt.Errorf("dispatch: %w", err)
	}
	return EmitResult{EventID: event.ID, Dispatch: result}, nil
}

// EmitBadgeEarned satisfies the effects.BadgeEarnedEmitter interface
// (defined in the effects package to avoid an effects→gamification
// import cycle). The AwardBadge effect calls this on first-time awards
// so rule authors can chain reactions to the `badge.earned` event.
//
// Recursion bound: the BadgeAward repo's INSERT … ON CONFLICT DO
// NOTHING returns created=false on the second hop for the same (user,
// badge); AwardBadge skips this method on dedup'd fires. A chain that
// grants a *different* badge therefore terminates after at most one
// hop per distinct badge in the catalog. Maintainers extending this
// path must preserve that invariant — silently no-opping the recursion
// guard would risk an infinite loop.
func (e *Emitter) EmitBadgeEarned(
	ctx context.Context,
	tenantID, actorID, badgeID uint,
	scopeType models.GamificationScopeType,
	scopeID uint,
	evidenceEventID *uint,
) error {
	badgeIDCopy := badgeID
	ctxJSON, err := badgeEarnedContextJSON(scopeType, scopeID, evidenceEventID)
	if err != nil {
		return fmt.Errorf("EmitBadgeEarned: encode context: %w", err)
	}
	event := &models.GamificationEvent{
		TenantID:   tenantID,
		ActorID:    actorID,
		Verb:       VerbEarned,
		ObjectType: ObjectBadge,
		ObjectID:   &badgeIDCopy,
		Source:     EmitterSource,
		Context:    ctxJSON,
	}
	_, err = e.Emit(ctx, event)
	return err
}

// badgeEarnedContextJSON encodes the originating scope + evidence event id
// into the event's Context JSON so downstream listeners (rule-trace
// audits, the W3 awards-page surface) can see why the badge fired without
// re-walking the rule_evaluation chain. Was previously dropped on the floor
// via `_ = scopeType / _ = scopeID / _ = evidenceEventID` — the audit's
// F2.4 finding.
func badgeEarnedContextJSON(scopeType models.GamificationScopeType, scopeID uint, evidenceEventID *uint) (datatypes.JSON, error) {
	ctx := map[string]any{
		"scope_type": string(scopeType),
		"scope_id":   scopeID,
	}
	if evidenceEventID != nil {
		ctx["evidence_event_id"] = *evidenceEventID
	}
	b, err := json.Marshal(ctx)
	if err != nil {
		return nil, err
	}
	return datatypes.JSON(b), nil
}

func summarizeViolations(vs []FerpaViolation) string {
	var b strings.Builder
	for i, v := range vs {
		if i > 0 {
			b.WriteString("; ")
		}
		fmt.Fprintf(&b, "%s.%s requires %v", v.ObjectType, v.FieldPath, v.Missing)
	}
	return b.String()
}
