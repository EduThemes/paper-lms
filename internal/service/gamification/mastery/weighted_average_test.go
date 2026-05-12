package mastery_test

import (
	"math"
	"testing"
	"time"

	"github.com/EduThemes/paper-lms/internal/service/gamification/mastery"
)

func TestWeightedAverage(t *testing.T) {
	base := time.Date(2026, 5, 12, 9, 0, 0, 0, time.UTC)
	cases := []struct {
		name      string
		events    []mastery.Event
		wantValue float64
	}{
		{
			name:   "empty",
			events: nil,
		},
		{
			name: "unweighted defaults to plain mean",
			events: []mastery.Event{
				{OccurredAt: base, Score: 0.4},
				{OccurredAt: base, Score: 0.8},
			},
			wantValue: 0.6,
		},
		{
			name: "weights bias toward heavier event",
			events: []mastery.Event{
				{OccurredAt: base, Score: 0.4, Weight: 1.0},
				{OccurredAt: base, Score: 1.0, Weight: 3.0},
			},
			// (0.4·1 + 1.0·3) / 4 = 0.85
			wantValue: 0.85,
		},
		{
			name: "zero weight gets promoted to 1.0",
			events: []mastery.Event{
				{OccurredAt: base, Score: 0.5, Weight: 0},
				{OccurredAt: base, Score: 0.7, Weight: 0},
			},
			wantValue: 0.6,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := mastery.WeightedAverage{}.Calculate(tc.events, mastery.Params{})
			if math.Abs(got.Value-tc.wantValue) > 1e-9 {
				t.Errorf("Value = %v, want %v", got.Value, tc.wantValue)
			}
		})
	}
}
