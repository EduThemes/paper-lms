package gamification

import (
	"testing"
	"time"
)

// MostRecentClosedWeekly should walk back to the most recent Sunday
// 00:00 UTC strictly before `now`. Edge cases: now=Sunday-at-noon
// (most recent close is today's 00:00); now=exactly-Sunday-00:00
// (most recent close is the PRIOR Sunday).
func TestMostRecentClosedWeekly(t *testing.T) {
	cases := []struct {
		name string
		now  time.Time
		want time.Time
	}{
		{
			name: "thursday afternoon → previous sunday 00:00",
			now:  time.Date(2026, 5, 14, 12, 0, 0, 0, time.UTC),
			want: time.Date(2026, 5, 10, 0, 0, 0, 0, time.UTC),
		},
		{
			name: "sunday at noon → today 00:00 (same Sunday)",
			now:  time.Date(2026, 5, 17, 12, 0, 0, 0, time.UTC),
			want: time.Date(2026, 5, 17, 0, 0, 0, 0, time.UTC),
		},
		{
			name: "sunday at exactly 00:00 → previous sunday 00:00 (current window just opened)",
			now:  time.Date(2026, 5, 17, 0, 0, 0, 0, time.UTC),
			want: time.Date(2026, 5, 10, 0, 0, 0, 0, time.UTC),
		},
		{
			name: "saturday 23:59 → this week's sunday (start of week)",
			now:  time.Date(2026, 5, 16, 23, 59, 0, 0, time.UTC),
			want: time.Date(2026, 5, 10, 0, 0, 0, 0, time.UTC),
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := MostRecentClosedWeekly(tc.now)
			if !got.Equal(tc.want) {
				t.Errorf("MostRecentClosedWeekly(%s) = %s, want %s",
					tc.now.Format(time.RFC3339), got.Format(time.RFC3339), tc.want.Format(time.RFC3339))
			}
		})
	}
}

func TestWeeklyWindowForOffset(t *testing.T) {
	// Anchor: Thursday 2026-05-14 12:00 UTC.
	// Most recent close: 2026-05-10 (Sunday).
	// Next close (current open window end): 2026-05-17 (Sunday).
	now := time.Date(2026, 5, 14, 12, 0, 0, 0, time.UTC)

	cases := []struct {
		offset      int
		wantStart   time.Time
		wantEnd     time.Time
		description string
	}{
		{
			offset:      0,
			wantStart:   time.Date(2026, 5, 10, 0, 0, 0, 0, time.UTC),
			wantEnd:     time.Date(2026, 5, 17, 0, 0, 0, 0, time.UTC),
			description: "current open window (Sun→Sun)",
		},
		{
			offset:      1,
			wantStart:   time.Date(2026, 5, 3, 0, 0, 0, 0, time.UTC),
			wantEnd:     time.Date(2026, 5, 10, 0, 0, 0, 0, time.UTC),
			description: "previous closed window (the most-recent snapshotted one)",
		},
		{
			offset:      2,
			wantStart:   time.Date(2026, 4, 26, 0, 0, 0, 0, time.UTC),
			wantEnd:     time.Date(2026, 5, 3, 0, 0, 0, 0, time.UTC),
			description: "two weeks ago",
		},
	}
	for _, tc := range cases {
		t.Run(tc.description, func(t *testing.T) {
			gotStart, gotEnd := WeeklyWindowForOffset(now, tc.offset)
			if !gotStart.Equal(tc.wantStart) {
				t.Errorf("offset=%d start: got %s want %s", tc.offset, gotStart.Format(time.RFC3339), tc.wantStart.Format(time.RFC3339))
			}
			if !gotEnd.Equal(tc.wantEnd) {
				t.Errorf("offset=%d end: got %s want %s", tc.offset, gotEnd.Format(time.RFC3339), tc.wantEnd.Format(time.RFC3339))
			}
		})
	}
}
