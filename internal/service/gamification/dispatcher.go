package gamification

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/repository"
	"github.com/EduThemes/paper-lms/internal/service/gamification/effects"
	"github.com/EduThemes/paper-lms/internal/service/gamification/predicates"
)

// DispatchDeps bundles every repository + factory the dispatch loop needs.
// The Emitter assembles this once per process; Dispatch takes a RuleIndex
// per call so the caller decides how stale the rule cache may be.
type DispatchDeps struct {
	Snapshot SnapshotDeps
	Rules    repository.GamificationRuleRepository
	Effects  effects.EffectDeps
}

// EvaluationOutcome captures everything that happened — or didn't — for
// one rule against one event. The slice of outcomes returned by Dispatch
// is the audit trail; each outcome also becomes a row in
// gamification_rule_evaluations.
type EvaluationOutcome struct {
	RuleID       uint
	Fired        bool             // true iff predicates evaluated true AND all effects succeeded
	BlockedBy    string           // "cooldown" | "max_per_window" | "" (not blocked)
	Reason       string           // human-readable explanation when Fired=false
	Trace        predicates.Trace // predicate evaluation trace
	Effects      []effects.EffectResult
	EffectErrors []string // effect-index → error string for effects that failed
}

// DispatchResult is the aggregate response — useful for callers wanting
// a summary without iterating outcomes.
type DispatchResult struct {
	RulesConsidered int
	RulesFired      int
	RulesBlocked    int // by cooldown or max_per_window
	RulesFalse      int // condition_set evaluated false
	RulesSkipped    int // malformed rule data, decoder failure
	Outcomes        []EvaluationOutcome
}

// Dispatcher routes one event through every rule that matches its
// (verb, object_type). The dispatcher does not own the rule cache — the
// caller passes a RuleIndex each call so the same dispatcher can serve
// rules at any scope.
type Dispatcher struct {
	deps DispatchDeps
	// now is overridable for tests so cooldown math is deterministic.
	now func() time.Time
}

// NewDispatcher constructs a dispatcher bound to a fixed set of repos
// and effect deps. Each Dispatch call must still be given the rule
// index applicable to the event's tenant + scope chain.
func NewDispatcher(deps DispatchDeps) *Dispatcher {
	return &Dispatcher{deps: deps, now: time.Now}
}

// Dispatch evaluates every OnEvent rule matching the event's
// (verb, object_type) in the supplied RuleIndex. For each matching
// rule:
//
//  1. Decode condition_set + effects JSONB into typed values.
//  2. Check cooldown + max_per_window — if blocked, record the outcome
//     and skip to the next rule (no effects fire, no rule_evaluation
//     row inserted because nothing was evaluated; this matches D2L's
//     semantics where a denied evaluation doesn't count as a firing
//     for cap purposes).
//  3. Hydrate the actor snapshot against the union of predicate Needs.
//  4. Evaluate the condition_set tree. If false, record the trace +
//     write a rule_evaluation row with result=false; no effects fire.
//  5. Run effects in declaration order with stop-on-first-error
//     semantics. Effects 1..N-1 stay durable when effect N fails;
//     effects N+1..end are recorded as skipped.
//  6. Write the rule_evaluation row capturing predicate_state +
//     effects_fired for the full audit trail.
//
// Dispatch never short-circuits across rules: a failure on rule 3 does
// not prevent rule 4 from evaluating. Errors that would prevent
// dispatch from continuing (e.g. context cancellation) bubble up; per-
// rule failures land in the outcome.
func (d *Dispatcher) Dispatch(ctx context.Context, index *RuleIndex, event *models.GamificationEvent) (DispatchResult, error) {
	if index == nil {
		return DispatchResult{}, errors.New("dispatcher: nil RuleIndex")
	}
	if event == nil {
		return DispatchResult{}, errors.New("dispatcher: nil event")
	}

	matching := index.LookupOnEvent(event.Verb, event.ObjectType)
	res := DispatchResult{RulesConsidered: len(matching)}

	for _, rule := range matching {
		outcome, fired, blocked, evalFalse, skipped, err := d.dispatchOne(ctx, &rule, event)
		if err != nil {
			return res, fmt.Errorf("rule %d: %w", rule.ID, err)
		}
		res.Outcomes = append(res.Outcomes, outcome)
		switch {
		case fired:
			res.RulesFired++
		case blocked:
			res.RulesBlocked++
		case evalFalse:
			res.RulesFalse++
		case skipped:
			res.RulesSkipped++
		}
	}
	return res, nil
}

// dispatchOne runs the per-rule pipeline and returns the outcome plus
// boolean flags so Dispatch's aggregate counters stay accurate without
// re-inspecting the outcome struct.
func (d *Dispatcher) dispatchOne(ctx context.Context, rule *models.GamificationRule, event *models.GamificationEvent) (outcome EvaluationOutcome, fired, blocked, evalFalse, skipped bool, err error) {
	outcome = EvaluationOutcome{RuleID: rule.ID}

	predicate, perr := predicates.DecodePredicate(json.RawMessage(rule.ConditionSet))
	if perr != nil {
		outcome.Reason = "decode condition_set: " + perr.Error()
		return outcome, false, false, false, true, nil
	}
	effectList, eerr := effects.DecodeEffects(json.RawMessage(rule.Effects))
	if eerr != nil {
		outcome.Reason = "decode effects: " + eerr.Error()
		return outcome, false, false, false, true, nil
	}

	now := d.now()
	gate, gerr := CheckCooldown(ctx, d.deps.Rules, rule, event.ActorID, now)
	if gerr != nil {
		return outcome, false, false, false, false, fmt.Errorf("cooldown check: %w", gerr)
	}
	if !gate.Allowed {
		outcome.BlockedBy = classifyGate(gate.Reason)
		outcome.Reason = gate.Reason
		return outcome, false, true, false, false, nil
	}

	needs := predicate.Needs()
	snapshot, serr := LoadSnapshot(ctx, d.deps.Snapshot, event.ActorID, event.TenantID, needs, "")
	if serr != nil {
		return outcome, false, false, false, false, fmt.Errorf("load snapshot: %w", serr)
	}

	ok, trace := predicate.Evaluate(ctx, snapshot)
	outcome.Trace = trace
	if !ok {
		outcome.Reason = trace.Reason
		if err := d.recordEvaluation(ctx, rule, event, false, trace, nil, nil); err != nil {
			return outcome, false, false, false, false, fmt.Errorf("record evaluation: %w", err)
		}
		return outcome, false, false, true, false, nil
	}

	// Predicates fired — run effects with stop-on-first-error semantics.
	results, errs, allOK := d.runEffects(ctx, effectList, effects.TriggeringContext{
		ActorID:   event.ActorID,
		TenantID:  event.TenantID,
		ScopeType: rule.ScopeType,
		ScopeID:   rule.ScopeID,
		EventID:   &event.ID,
		RuleID:    rule.ID,
	})
	outcome.Effects = results
	outcome.EffectErrors = errs
	outcome.Fired = allOK
	if !allOK {
		outcome.Reason = "one or more effects failed"
	}

	if err := d.recordEvaluation(ctx, rule, event, true, trace, results, errs); err != nil {
		return outcome, allOK, false, false, false, fmt.Errorf("record evaluation: %w", err)
	}
	return outcome, allOK, false, false, false, nil
}

// runEffects applies each effect in order. The moment one fails, the
// rest are recorded as skipped — prior successes stay durable in the
// wallet/event ledger. Returns parallel slices (one entry per effect)
// plus a bool indicating whether every effect succeeded.
func (d *Dispatcher) runEffects(ctx context.Context, list []effects.Effect, trig effects.TriggeringContext) ([]effects.EffectResult, []string, bool) {
	results := make([]effects.EffectResult, len(list))
	errs := make([]string, len(list))
	allOK := true
	for i, eff := range list {
		if !allOK {
			results[i] = effects.EffectResult{Kind: eff.Kind(), Summary: "skipped: prior effect failed"}
			errs[i] = "skipped"
			continue
		}
		r, err := eff.Apply(ctx, d.deps.Effects, trig)
		if err != nil {
			results[i] = effects.EffectResult{Kind: eff.Kind(), Summary: "failed: " + err.Error()}
			errs[i] = err.Error()
			allOK = false
			continue
		}
		results[i] = r
	}
	return results, errs, allOK
}

// recordEvaluation appends the audit row. predicate_state captures the
// trace; effects_fired carries the per-effect result + error. Both are
// stored as JSONB so debuggers can inspect arbitrary depth weeks later.
func (d *Dispatcher) recordEvaluation(ctx context.Context, rule *models.GamificationRule, event *models.GamificationEvent, result bool, trace predicates.Trace, effectResults []effects.EffectResult, effectErrs []string) error {
	traceJSON, err := json.Marshal(trace)
	if err != nil {
		return fmt.Errorf("marshal trace: %w", err)
	}
	var effectsJSON []byte
	if len(effectResults) > 0 {
		// Pair results with errors so a single audit blob carries both.
		paired := make([]map[string]any, len(effectResults))
		for i, r := range effectResults {
			entry := map[string]any{
				"kind":    r.Kind,
				"summary": r.Summary,
			}
			if r.Detail != nil {
				entry["detail"] = r.Detail
			}
			if i < len(effectErrs) && effectErrs[i] != "" {
				entry["error"] = effectErrs[i]
			}
			paired[i] = entry
		}
		effectsJSON, err = json.Marshal(paired)
		if err != nil {
			return fmt.Errorf("marshal effects: %w", err)
		}
	}
	eval := &models.GamificationRuleEvaluation{
		RuleID:            rule.ID,
		UserID:            event.ActorID,
		EvaluatedAt:       d.now(),
		PredicateState:    traceJSON,
		Result:            result,
		EffectsFired:      effectsJSON,
		TriggeringEventID: &event.ID,
	}
	return d.deps.Rules.RecordEvaluation(ctx, eval)
}

// classifyGate maps a CooldownCheckResult.Reason string to a short
// machine-readable tag so callers don't have to substring-match.
func classifyGate(reason string) string {
	switch {
	case len(reason) >= 8 && reason[:8] == "cooldown":
		return "cooldown"
	case len(reason) >= 14 && reason[:14] == "max_per_window":
		return "max_per_window"
	default:
		return "blocked"
	}
}
