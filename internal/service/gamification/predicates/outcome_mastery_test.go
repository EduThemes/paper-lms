package predicates_test

import (
	"context"
	"testing"

	"github.com/EduThemes/paper-lms/internal/service/gamification/mastery"
	"github.com/EduThemes/paper-lms/internal/service/gamification/predicates"
)

func TestOutcomeMastery_NoCachedState(t *testing.T) {
	p := predicates.OutcomeMastery{OutcomeID: 3, MinLevel: mastery.LevelProficient}
	got, trace := p.Evaluate(context.Background(), predicates.ActorSnapshot{})
	if got {
		t.Fatalf("expected false when no cached mastery")
	}
	if trace.Reason == "" {
		t.Fatalf("expected trace.Reason")
	}
}

func TestOutcomeMastery_InvalidMinLevel(t *testing.T) {
	p := predicates.OutcomeMastery{OutcomeID: 3, MinLevel: "expert"}
	got, trace := p.Evaluate(context.Background(), predicates.ActorSnapshot{})
	if got {
		t.Fatalf("expected false for unknown MinLevel")
	}
	if trace.Reason == "" {
		t.Fatalf("expected trace.Reason")
	}
}

func TestOutcomeMastery_AtLevel(t *testing.T) {
	actor := predicates.ActorSnapshot{
		OutcomeMastery: map[uint]predicates.MasteryState{
			3: {OutcomeID: 3, Level: mastery.LevelProficient, Value: 0.72},
		},
	}
	p := predicates.OutcomeMastery{OutcomeID: 3, MinLevel: mastery.LevelProficient}
	got, _ := p.Evaluate(context.Background(), actor)
	if !got {
		t.Fatalf("expected true at proficient when MinLevel=proficient")
	}
}

func TestOutcomeMastery_AboveLevel(t *testing.T) {
	actor := predicates.ActorSnapshot{
		OutcomeMastery: map[uint]predicates.MasteryState{
			3: {OutcomeID: 3, Level: mastery.LevelMastered, Value: 0.95},
		},
	}
	p := predicates.OutcomeMastery{OutcomeID: 3, MinLevel: mastery.LevelProficient}
	got, _ := p.Evaluate(context.Background(), actor)
	if !got {
		t.Fatalf("expected true at mastered when MinLevel=proficient")
	}
}

func TestOutcomeMastery_BelowLevel(t *testing.T) {
	actor := predicates.ActorSnapshot{
		OutcomeMastery: map[uint]predicates.MasteryState{
			3: {OutcomeID: 3, Level: mastery.LevelFamiliar, Value: 0.50},
		},
	}
	p := predicates.OutcomeMastery{OutcomeID: 3, MinLevel: mastery.LevelProficient}
	got, trace := p.Evaluate(context.Background(), actor)
	if got {
		t.Fatalf("expected false at familiar when MinLevel=proficient")
	}
	if trace.Reason == "" {
		t.Fatalf("expected trace.Reason explaining the gap")
	}
}

func TestOutcomeMastery_CalcMethodRecordedInTrace(t *testing.T) {
	actor := predicates.ActorSnapshot{
		OutcomeMastery: map[uint]predicates.MasteryState{
			3: {OutcomeID: 3, Level: mastery.LevelProficient, Value: 0.70},
		},
	}
	p := predicates.OutcomeMastery{
		OutcomeID:  3,
		MinLevel:   mastery.LevelProficient,
		CalcMethod: string(mastery.MethodKhanSpacedRetrieval),
	}
	_, trace := p.Evaluate(context.Background(), actor)
	if got, ok := trace.Params["calc_method"].(string); !ok || got != string(mastery.MethodKhanSpacedRetrieval) {
		t.Fatalf("expected calc_method in trace.Params, got %v", trace.Params["calc_method"])
	}
}
