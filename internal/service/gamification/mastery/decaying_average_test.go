package mastery_test

import (
	"math"
	"testing"
	"time"

	"github.com/EduThemes/paper-lms/internal/service/gamification/mastery"
)

func TestDecayingAverage(t *testing.T) {
	base := time.Date(2026, 5, 12, 9, 0, 0, 0, time.UTC)
	cases := []struct {
		name      string
		events    []mastery.Event
		params    mastery.Params
		wantValue float64
	}{
		{
			name:   "empty",
			events: nil,
		},
		{
			name: "single event returns its own score",
			events: []mastery.Event{
				{OccurredAt: base, Score: 0.7},
			},
			wantValue: 0.7,
		},
		{
			name: "default 0.65/0.35 blend",
			events: []mastery.Event{
				{OccurredAt: base, Score: 0.4},
				{OccurredAt: base.Add(24 * time.Hour), Score: 0.6},
				{OccurredAt: base.Add(48 * time.Hour), Score: 1.0},
			},
			// priors mean = (0.4 + 0.6) / 2 = 0.5
			// recent = 1.0
			// blend = 1.0·0.65 + 0.5·0.35 = 0.825
			wantValue: 0.825,
		},
		{
			name: "explicit weight override",
			events: []mastery.Event{
				{OccurredAt: base, Score: 0.4},
				{OccurredAt: base.Add(24 * time.Hour), Score: 1.0},
			},
			params: mastery.Params{DecayingAverageRecentWeight: 0.5},
			// priors mean = 0.4; recent = 1.0; blend = 0.5·1.0 + 0.5·0.4 = 0.7
			wantValue: 0.7,
		},
		{
			name: "unsorted input identifies recent by OccurredAt",
			events: []mastery.Event{
				{OccurredAt: base.Add(48 * time.Hour), Score: 1.0},
				{OccurredAt: base, Score: 0.4},
				{OccurredAt: base.Add(24 * time.Hour), Score: 0.6},
			},
			wantValue: 0.825,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := mastery.DecayingAverage{}.Calculate(tc.events, tc.params)
			if math.Abs(got.Value-tc.wantValue) > 1e-9 {
				t.Errorf("Value = %v, want %v", got.Value, tc.wantValue)
			}
		})
	}
}
