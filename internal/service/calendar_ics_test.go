package service

import (
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func icsLines(b []byte) []string {
	return strings.Split(strings.TrimSpace(string(b)), "\r\n")
}

func icsValue(lines []string, prefix string) string {
	for _, l := range lines {
		if strings.HasPrefix(l, prefix) {
			return strings.TrimPrefix(l, prefix)
		}
	}
	return ""
}

// G096: an all-day event is stored as the instant of its local midnight.
// The feed must date it in the owner's zone (east of UTC that instant is
// the previous UTC day) and give it an exclusive DTEND after its start.
func TestRenderICSIn_AllDayDatesInOwnerZone(t *testing.T) {
	berlin, err := time.LoadLocation("Europe/Berlin")
	require.NoError(t, err)
	now := time.Date(2026, 10, 1, 12, 0, 0, 0, time.UTC)

	cases := []struct {
		name      string
		start     time.Time
		end       *time.Time
		wantStart string
		wantEnd   string
	}{
		{"no end", time.Date(2026, 10, 10, 0, 0, 0, 0, berlin), nil, "20261010", "20261011"},
		{"ends the same day", time.Date(2026, 10, 10, 0, 0, 0, 0, berlin), ptrTime(time.Date(2026, 10, 10, 23, 59, 0, 0, berlin)), "20261010", "20261011"},
		{"ends at next midnight", time.Date(2026, 10, 10, 0, 0, 0, 0, berlin), ptrTime(time.Date(2026, 10, 11, 0, 0, 0, 0, berlin)), "20261010", "20261011"},
		{"three days", time.Date(2026, 10, 10, 0, 0, 0, 0, berlin), ptrTime(time.Date(2026, 10, 12, 12, 0, 0, 0, berlin)), "20261010", "20261013"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			out := RenderICSIn([]CalendarItem{{ID: "event_1", Type: CalendarItemEvent, Title: "Field trip", StartAt: tc.start, EndAt: tc.end, AllDay: true}},
				"Cal", "", now, berlin)
			lines := icsLines(out)
			assert.Equal(t, tc.wantStart, icsValue(lines, "DTSTART;VALUE=DATE:"))
			assert.Equal(t, tc.wantEnd, icsValue(lines, "DTEND;VALUE=DATE:"))
		})
	}

	// Timed events stay in UTC whatever the zone.
	start := time.Date(2026, 10, 10, 9, 30, 0, 0, berlin)
	out := RenderICSIn([]CalendarItem{{ID: "event_2", Type: CalendarItemEvent, Title: "Lecture", StartAt: start}}, "Cal", "", now, berlin)
	assert.Equal(t, "20261010T073000Z", icsValue(icsLines(out), "DTSTART:"))

	// RenderICS keeps the UTC reading.
	out = RenderICS([]CalendarItem{{ID: "event_3", Type: CalendarItemEvent, Title: "Day", StartAt: time.Date(2026, 10, 10, 0, 0, 0, 0, time.UTC), AllDay: true}}, "Cal", "", now)
	assert.Equal(t, "20261010", icsValue(icsLines(out), "DTSTART;VALUE=DATE:"))
	assert.Equal(t, "20261011", icsValue(icsLines(out), "DTEND;VALUE=DATE:"))
}

func ptrTime(t time.Time) *time.Time { return &t }
