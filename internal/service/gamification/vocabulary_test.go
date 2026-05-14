package gamification_test

import (
	"encoding/json"
	"testing"

	"github.com/EduThemes/paper-lms/internal/service/gamification"
	"github.com/EduThemes/paper-lms/internal/service/gamification/effects"
	"github.com/EduThemes/paper-lms/internal/service/gamification/predicates"
)

// TestPredicateCatalog_NoDriftFromFactory asserts that every kind in
// PredicateCatalog is accepted by the runtime decoder
// (predicates.DecodePredicate) when fed a minimum-valid JSON instance
// synthesised from the catalog's ParamSpecs. CI fails if a kind is
// added to the catalog but the decoder doesn't know it, or vice versa.
//
// The synthesised instances use placeholder values (1 for refs, "novice"
// for mastery levels, etc.) — the assertion is only that the decoder
// accepts the shape, not that the rule would evaluate true at runtime.
func TestPredicateCatalog_NoDriftFromFactory(t *testing.T) {
	for _, spec := range gamification.PredicateCatalog {
		t.Run(spec.Kind, func(t *testing.T) {
			payload := minViable(spec)
			raw, err := json.Marshal(payload)
			if err != nil {
				t.Fatalf("marshal: %v", err)
			}
			if _, err := predicates.DecodePredicate(raw); err != nil {
				t.Fatalf("DecodePredicate(%s) failed for catalog entry: %v\npayload: %s",
					spec.Kind, err, raw)
			}
		})
	}
}

// TestPredicateFactory_AllKindsInCatalog is the reverse drift check —
// the kinds DecodePredicate handles must each appear in the catalog so
// the recipe-builder UI surfaces them. Hard-coded list mirrors the
// switch in predicates/factory.go; new kinds added there must be added
// here AND to PredicateCatalog at the same time.
func TestPredicateFactory_AllKindsInCatalog(t *testing.T) {
	factoryKinds := []string{
		"SubmittedAssignment",
		"SubmittedQuiz",
		"ViewedContent",
		"OutcomeMastery",
		"CurrencyThreshold",
		"EarnedBadge",
		"ReputationThreshold",
		// ConditionSet is the recursive wrapper — surfaced separately in
		// the vocabulary response via SetOps, not as a catalog entry.
	}
	have := map[string]bool{}
	for _, s := range gamification.PredicateCatalog {
		have[s.Kind] = true
	}
	for _, k := range factoryKinds {
		if !have[k] {
			t.Errorf("predicate kind %q decoded by factory but missing from PredicateCatalog", k)
		}
	}
}

// TestEffectCatalog_NoDriftFromFactory mirrors the predicate drift test
// against effects.DecodeEffects.
func TestEffectCatalog_NoDriftFromFactory(t *testing.T) {
	for _, spec := range gamification.EffectCatalog {
		t.Run(spec.Kind, func(t *testing.T) {
			payload := minViable(spec)
			arr := []map[string]any{payload}
			raw, err := json.Marshal(arr)
			if err != nil {
				t.Fatalf("marshal: %v", err)
			}
			if _, err := effects.DecodeEffects(raw); err != nil {
				t.Fatalf("DecodeEffects(%s) failed for catalog entry: %v\npayload: %s",
					spec.Kind, err, raw)
			}
		})
	}
}

func TestEffectFactory_AllKindsInCatalog(t *testing.T) {
	factoryKinds := []string{"AwardCurrency", "AwardBadge"}
	have := map[string]bool{}
	for _, s := range gamification.EffectCatalog {
		have[s.Kind] = true
	}
	for _, k := range factoryKinds {
		if !have[k] {
			t.Errorf("effect kind %q decoded by factory but missing from EffectCatalog", k)
		}
	}
}

// minViable synthesises the smallest payload that satisfies the
// runtime decoder's required-field checks for a given catalog entry.
// Pick placeholder values that the decoder will accept without
// touching the database — refs become id=1, currency codes become
// "xp", mastery_level becomes "novice", etc.
func minViable(spec gamification.KindSpec) map[string]any {
	out := map[string]any{"kind": spec.Kind}
	for _, p := range spec.Params {
		if !p.Required {
			continue
		}
		switch p.Type {
		case gamification.ParamTypeInt:
			// AwardCurrency.amount has Min=1, so use 1 not 0.
			if p.Min != nil && *p.Min > 0 {
				out[p.Name] = int(*p.Min)
			} else {
				out[p.Name] = 1
			}
		case gamification.ParamTypeFloat:
			out[p.Name] = 1.0
		case gamification.ParamTypeBool:
			out[p.Name] = true
		case gamification.ParamTypeString:
			out[p.Name] = "x"
		case gamification.ParamTypeEnum:
			if len(p.Enum) > 0 {
				out[p.Name] = p.Enum[0]
			} else {
				out[p.Name] = "x"
			}
		case gamification.ParamTypeRef:
			switch p.Ref {
			case "currency_code":
				out[p.Name] = "xp"
			case "badge_code":
				out[p.Name] = "first_quiz"
			default:
				// assignment | quiz | content | outcome | badge — all uint ids
				out[p.Name] = 1
			}
		}
	}
	return out
}
