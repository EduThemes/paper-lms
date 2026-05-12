package mastery_test

import (
	"testing"
	"time"

	"github.com/EduThemes/paper-lms/internal/service/gamification/mastery"
)

func TestHighest(t *testing.T) {
	base := time.Date(2026, 5, 12, 9, 0, 0, 0, time.UTC)
	cases := []struct {
		name      string
		events    []mastery.Event
		wantValue float64
		wantAsOf  time.Time
	}{
		{
			name:   "empty",
			events: nil,
		},
		{
			name: "max wins",
			events: []mastery.Event{
				{OccurredAt: base, Score: 0.4},
				{OccurredAt: base.Add(24 * time.Hour), Score: 0.9},
				{OccurredAt: base.Add(48 * time.Hour), Score: 0.6},
			},
			wantValue: 0.9,
			wantAsOf:  base.Add(24 * time.Hour),
		},
		{
			name: "tie broken by latest",
			events: []mastery.Event{
				{OccurredAt: base, Score: 0.9},
				{OccurredAt: base.Add(24 * time.Hour), Score: 0.9},
			},
			wantValue: 0.9,
			wantAsOf:  base.Add(24 * time.Hour),
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := mastery.Highest{}.Calculate(tc.events, mastery.Params{})
			if got.Value != tc.wantValue {
				t.Errorf("Value = %v, want %v", got.Value, tc.wantValue)
			}
			if tc.events != nil && !got.AsOf.Equal(tc.wantAsOf) {
				t.Errorf("AsOf = %v, want %v", got.AsOf, tc.wantAsOf)
			}
		})
	}
}
