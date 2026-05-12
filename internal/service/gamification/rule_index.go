package gamification

import (
	"encoding/json"
	"fmt"

	"github.com/EduThemes/paper-lms/internal/domain/models"
)

// Trigger kinds recognized in a rule's trigger_event JSON. The dispatcher
// indexes OnEvent rules by (verb, object_type), keeps OnSchedule rules in
// a flat list for the cron evaluator, and indexes OnManualTrigger rules
// by handle for admin/API-driven firing.
const (
	triggerKindOnEvent         = "OnEvent"
	triggerKindOnSchedule      = "OnSchedule"
	triggerKindOnManualTrigger = "OnManualTrigger"
)

// TriggerKey is the (verb, object_type) tuple an OnEvent rule fires on.
// Built once at index time so dispatch is an O(1) map read per event,
// rather than an O(N) scan over every rule per emit.
type TriggerKey struct {
	Verb       string
	ObjectType string
}

// RuleIndex is a read-only lookup built once per rule-set load. The
// dispatcher rebuilds it whenever rules change (cheap; rules table is
// small). All Lookup methods are safe for concurrent reads — the maps
// are never mutated after BuildRuleIndex returns.
type RuleIndex struct {
	onEvent  map[TriggerKey][]models.GamificationRule
	onSched  []models.GamificationRule
	onManual map[string][]models.GamificationRule
	skipped  []SkippedRule
}

// SkippedRule captures rules the index couldn't parse — bad trigger_event
// JSON, unknown trigger kind, missing required fields. The dispatcher
// surfaces these in logs without blocking other rules from firing.
type SkippedRule struct {
	RuleID uint
	Reason string
}

// triggerEnvelope is the shape we Unmarshal trigger_event into. All
// variant-specific fields are optional at the JSON layer; we validate
// per-kind below so an OnEvent without a Verb lands in Skipped rather
// than silently matching the zero-value TriggerKey.
type triggerEnvelope struct {
	Kind       string `json:"kind"`
	Verb       string `json:"verb"`
	ObjectType string `json:"object_type"`
	Cron       string `json:"cron"`
	Handle     string `json:"handle"`
}

// BuildRuleIndex parses each rule's trigger_event JSON exactly once and
// places the rule into the matching lookup bucket. Disabled rules
// (`enabled = false`) are silently skipped — they should never reach
// here if callers prefilter via ListEnabledByScope, but the guard is
// cheap and keeps the dispatcher honest.
func BuildRuleIndex(rules []models.GamificationRule) *RuleIndex {
	idx := &RuleIndex{
		onEvent:  make(map[TriggerKey][]models.GamificationRule),
		onManual: make(map[string][]models.GamificationRule),
	}

	for _, rule := range rules {
		if !rule.Enabled {
			continue
		}

		raw := rule.TriggerEvent
		if len(raw) == 0 {
			idx.skipped = append(idx.skipped, SkippedRule{
				RuleID: rule.ID,
				Reason: "trigger_event is empty",
			})
			continue
		}

		var env triggerEnvelope
		if err := json.Unmarshal(raw, &env); err != nil {
			idx.skipped = append(idx.skipped, SkippedRule{
				RuleID: rule.ID,
				Reason: fmt.Sprintf("malformed trigger_event JSON: %v", err),
			})
			continue
		}

		switch env.Kind {
		case triggerKindOnEvent:
			if env.Verb == "" {
				idx.skipped = append(idx.skipped, SkippedRule{
					RuleID: rule.ID,
					Reason: "OnEvent trigger missing required field: verb",
				})
				continue
			}
			if env.ObjectType == "" {
				idx.skipped = append(idx.skipped, SkippedRule{
					RuleID: rule.ID,
					Reason: "OnEvent trigger missing required field: object_type",
				})
				continue
			}
			key := TriggerKey{Verb: env.Verb, ObjectType: env.ObjectType}
			idx.onEvent[key] = append(idx.onEvent[key], rule)

		case triggerKindOnSchedule:
			if env.Cron == "" {
				idx.skipped = append(idx.skipped, SkippedRule{
					RuleID: rule.ID,
					Reason: "OnSchedule trigger missing required field: cron",
				})
				continue
			}
			idx.onSched = append(idx.onSched, rule)

		case triggerKindOnManualTrigger:
			if env.Handle == "" {
				idx.skipped = append(idx.skipped, SkippedRule{
					RuleID: rule.ID,
					Reason: "OnManualTrigger trigger missing required field: handle",
				})
				continue
			}
			idx.onManual[env.Handle] = append(idx.onManual[env.Handle], rule)

		case "":
			idx.skipped = append(idx.skipped, SkippedRule{
				RuleID: rule.ID,
				Reason: "trigger_event missing required field: kind",
			})

		default:
			idx.skipped = append(idx.skipped, SkippedRule{
				RuleID: rule.ID,
				Reason: fmt.Sprintf("unknown trigger kind: %q", env.Kind),
			})
		}
	}

	return idx
}

// LookupOnEvent returns every OnEvent rule matching the (verb, object_type)
// tuple. Returns nil (len == 0) when no rules match — Go convention, and
// cheaper than a freshly allocated empty slice for the common no-match path.
func (idx *RuleIndex) LookupOnEvent(verb, objectType string) []models.GamificationRule {
	if idx == nil {
		return nil
	}
	return idx.onEvent[TriggerKey{Verb: verb, ObjectType: objectType}]
}

// ListOnSchedule returns every OnSchedule rule for cron-driven evaluation.
// Order matches input order so a deterministic cron loop can rely on it.
func (idx *RuleIndex) ListOnSchedule() []models.GamificationRule {
	if idx == nil {
		return nil
	}
	return idx.onSched
}

// LookupOnManual returns every OnManualTrigger rule matching the handle.
// Returns nil when no rules match.
func (idx *RuleIndex) LookupOnManual(handle string) []models.GamificationRule {
	if idx == nil {
		return nil
	}
	return idx.onManual[handle]
}

// Skipped returns every rule the indexer couldn't parse + the reason.
// Callers log these once per index build for observability — a noisy
// Skipped list usually means a migration dropped a column the rules
// still reference.
func (idx *RuleIndex) Skipped() []SkippedRule {
	if idx == nil {
		return nil
	}
	return idx.skipped
}
