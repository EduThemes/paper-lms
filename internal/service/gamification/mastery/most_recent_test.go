package mastery_test

import (
	"testing"
	"time"

	"github.com/EduThemes/paper-lms/internal/service/gamification/mastery"
)

func TestMostRecent(t *testing.T) {
	base := time.Date(2026, 5, 12, 9, 0, 0, 0, time.UTC)
	cases := []struct {
		name      string
		events    []mastery.Event
		wantValue float64
		wantLevel string
	}{
		{
			name:   "empty events",
			events: nil,
		},
		{
			name: "single event",
			events: []mastery.Event{
				{OccurredAt: base, Score: 0.7},
			},
			wantValue: 0.7,
			wantLevel: mastery.LevelProficient,
		},
		{
			name: "latest wins regardless of slice order",
			events: []mastery.Event{
				{OccurredAt: base.Add(2 * 24 * time.Hour), Score: 0.4},
				{OccurredAt: base, Score: 0.9},
			},
			wantValue: 0.4,
			wantLevel: mastery.LevelFamiliar,
		},
		{
			name: "clamps above 1",
			events: []mastery.Event{
				{OccurredAt: base, Score: 1.5},
			},
			wantValue: 1.0,
			wantLevel: mastery.LevelMastered,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := mastery.MostRecent{}.Calculate(tc.events, mastery.Params{})
			if got.Value != tc.wantValue {
				t.Errorf("Value = %v, want %v", got.Value, tc.wantValue)
			}
			if tc.events != nil && got.Level != tc.wantLevel {
				t.Errorf("Level = %q, want %q", got.Level, tc.wantLevel)
			}
		})
	}
}
