package mastery_test

import (
	"math"
	"testing"
	"time"

	"github.com/EduThemes/paper-lms/internal/service/gamification/mastery"
)

func TestKhanSpacedRetrieval(t *testing.T) {
	base := time.Date(2026, 5, 12, 9, 0, 0, 0, time.UTC)
	halfLife := 7.0

	t.Run("empty events", func(t *testing.T) {
		got := mastery.KhanSpacedRetrieval{}.Calculate(nil, mastery.Params{})
		if got.Value != 0 || got.Level != "" {
			t.Fatalf("expected zero State, got %+v", got)
		}
	})

	t.Run("single passing event with Now=event time gives 1.0", func(t *testing.T) {
		got := mastery.KhanSpacedRetrieval{}.Calculate(
			[]mastery.Event{{OccurredAt: base, Score: 0.9}},
			mastery.Params{Now: base, HalfLifeDays: halfLife},
		)
		if math.Abs(got.Value-1.0) > 1e-9 {
			t.Errorf("Value = %v, want 1.0", got.Value)
		}
		if got.Level != mastery.LevelMastered {
			t.Errorf("Level = %q, want mastered", got.Level)
		}
	})

	t.Run("decay over one half-life", func(t *testing.T) {
		// One pass at t=0, evaluated at t=7d → expect ~0.5.
		got := mastery.KhanSpacedRetrieval{}.Calculate(
			[]mastery.Event{{OccurredAt: base, Score: 0.9}},
			mastery.Params{Now: base.Add(7 * 24 * time.Hour), HalfLifeDays: halfLife},
		)
		want := 0.5
		if math.Abs(got.Value-want) > 1e-6 {
			t.Errorf("Value = %v, want ~%v", got.Value, want)
		}
		if got.Level != mastery.LevelFamiliar {
			t.Errorf("Level = %q, want familiar", got.Level)
		}
	})

	t.Run("reattempt-pass re-clamps to 1.0 and re-decays from there", func(t *testing.T) {
		// Pass at t=0, decays for 7 days, reattempt-pass at t=7d resets to 1.0,
		// then decay another 7 days to Now → ~0.5.
		got := mastery.KhanSpacedRetrieval{}.Calculate(
			[]mastery.Event{
				{OccurredAt: base, Score: 0.9},
				{OccurredAt: base.Add(7 * 24 * time.Hour), Score: 0.9},
			},
			mastery.Params{Now: base.Add(14 * 24 * time.Hour), HalfLifeDays: halfLife},
		)
		want := 0.5
		if math.Abs(got.Value-want) > 1e-6 {
			t.Errorf("Value = %v, want ~%v", got.Value, want)
		}
	})

	t.Run("reattempt-below-threshold replaces with the new lower score", func(t *testing.T) {
		// Pass at t=0 (→ 1.0), reattempt at t=7d scoring 0.5 (below default
		// threshold 0.8) replaces score with 0.5; no further decay (Now=event).
		got := mastery.KhanSpacedRetrieval{}.Calculate(
			[]mastery.Event{
				{OccurredAt: base, Score: 0.9},
				{OccurredAt: base.Add(7 * 24 * time.Hour), Score: 0.5},
			},
			mastery.Params{
				Now:          base.Add(7 * 24 * time.Hour),
				HalfLifeDays: halfLife,
			},
		)
		if math.Abs(got.Value-0.5) > 1e-9 {
			t.Errorf("Value = %v, want 0.5", got.Value)
		}
	})

	t.Run("Now zero means evaluate at latest event (no extra decay)", func(t *testing.T) {
		got := mastery.KhanSpacedRetrieval{}.Calculate(
			[]mastery.Event{{OccurredAt: base, Score: 0.9}},
			mastery.Params{HalfLifeDays: halfLife},
		)
		if math.Abs(got.Value-1.0) > 1e-9 {
			t.Errorf("Value = %v, want 1.0 (no decay when Now=latest event)", got.Value)
		}
	})

	t.Run("custom threshold lets a lower score count as pass", func(t *testing.T) {
		got := mastery.KhanSpacedRetrieval{}.Calculate(
			[]mastery.Event{{OccurredAt: base, Score: 0.6}},
			mastery.Params{Now: base, HalfLifeDays: halfLife, ReattemptThreshold: 0.5},
		)
		if math.Abs(got.Value-1.0) > 1e-9 {
			t.Errorf("Value = %v, want 1.0 with custom threshold 0.5", got.Value)
		}
	})

	t.Run("Now before latest event is treated as zero elapsed", func(t *testing.T) {
		got := mastery.KhanSpacedRetrieval{}.Calculate(
			[]mastery.Event{{OccurredAt: base.Add(24 * time.Hour), Score: 0.9}},
			mastery.Params{Now: base, HalfLifeDays: halfLife},
		)
		if math.Abs(got.Value-1.0) > 1e-9 {
			t.Errorf("Value = %v, want 1.0 (clamped to zero elapsed)", got.Value)
		}
	})
}
