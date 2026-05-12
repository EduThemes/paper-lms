package mastery_test

import (
	"math"
	"testing"
	"time"

	"github.com/EduThemes/paper-lms/internal/service/gamification/mastery"
)

func TestNTimes(t *testing.T) {
	base := time.Date(2026, 5, 12, 9, 0, 0, 0, time.UTC)
	cases := []struct {
		name      string
		events    []mastery.Event
		params    mastery.Params
		wantValue float64
		wantZero  bool
	}{
		{
			name:     "fewer than N events",
			events:   []mastery.Event{{OccurredAt: base, Score: 0.9}, {OccurredAt: base, Score: 0.9}},
			wantZero: true,
		},
		{
			name: "last three all above default threshold",
			events: []mastery.Event{
				{OccurredAt: base, Score: 0.5},
				{OccurredAt: base.Add(24 * time.Hour), Score: 0.9},
				{OccurredAt: base.Add(48 * time.Hour), Score: 0.85},
				{OccurredAt: base.Add(72 * time.Hour), Score: 0.95},
			},
			wantValue: 0.9, // mean(0.9, 0.85, 0.95)
		},
		{
			name: "one of the last three below threshold",
			events: []mastery.Event{
				{OccurredAt: base, Score: 0.95},
				{OccurredAt: base.Add(24 * time.Hour), Score: 0.7}, // breaks the streak
				{OccurredAt: base.Add(48 * time.Hour), Score: 0.85},
				{OccurredAt: base.Add(72 * time.Hour), Score: 0.95},
			},
			wantZero: true,
		},
		{
			name: "custom N and threshold",
			events: []mastery.Event{
				{OccurredAt: base, Score: 0.6},
				{OccurredAt: base.Add(24 * time.Hour), Score: 0.6},
			},
			params:    mastery.Params{NTimesRequired: 2, NTimesThreshold: 0.5},
			wantValue: 0.6,
		},
		{
			name: "unsorted input still works",
			events: []mastery.Event{
				{OccurredAt: base.Add(72 * time.Hour), Score: 0.9},
				{OccurredAt: base, Score: 0.5},
				{OccurredAt: base.Add(24 * time.Hour), Score: 0.9},
				{OccurredAt: base.Add(48 * time.Hour), Score: 0.85},
			},
			wantValue: 0.883333333333,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := mastery.NTimes{}.Calculate(tc.events, tc.params)
			if tc.wantZero {
				if got.Value != 0 {
					t.Fatalf("expected zero State, got %+v", got)
				}
				return
			}
			if math.Abs(got.Value-tc.wantValue) > 1e-9 {
				t.Errorf("Value = %v, want %v", got.Value, tc.wantValue)
			}
		})
	}
}
