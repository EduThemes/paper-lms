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

// An all-day event with its own time_zone is dated in that zone, whoever
// reads the feed; one without (rows from before migration 000134) keeps
// the reader's zone.
func TestRenderICSIn_AllDayUsesEventZone(t *testing.T) {
	ny, err := time.LoadLocation("America/New_York")
	require.NoError(t, err)
	tokyo, err := time.LoadLocation("Asia/Tokyo")
	require.NoError(t, err)
	now := time.Date(2026, 10, 1, 12, 0, 0, 0, time.UTC)
	// Midnight of Oct 10 in New York is already Oct 10 13:00 in Tokyo;
	// Tokyo midnight of Oct 10 is Oct 9 in New York.
	nyMidnight := time.Date(2026, 10, 10, 0, 0, 0, 0, ny)
	tokyoMidnight := time.Date(2026, 10, 10, 0, 0, 0, 0, tokyo)

	out := RenderICSIn([]CalendarItem{{ID: "event_1", Type: CalendarItemEvent, Title: "Fair", StartAt: tokyoMidnight, AllDay: true, TimeZone: "Asia/Tokyo"}},
		"Cal", "", now, ny)
	assert.Equal(t, "20261010", icsValue(icsLines(out), "DTSTART;VALUE=DATE:"), "dated in the event's zone, not the reader's")
	assert.Equal(t, "20261011", icsValue(icsLines(out), "DTEND;VALUE=DATE:"))

	out = RenderICSIn([]CalendarItem{{ID: "event_2", Type: CalendarItemEvent, Title: "Fair", StartAt: tokyoMidnight, AllDay: true}},
		"Cal", "", now, ny)
	assert.Equal(t, "20261009", icsValue(icsLines(out), "DTSTART;VALUE=DATE:"), "no event zone: the reader's zone, as before")

	// An unknown zone name falls back to the reader's zone.
	out = RenderICSIn([]CalendarItem{{ID: "event_3", Type: CalendarItemEvent, Title: "Fair", StartAt: nyMidnight, AllDay: true, TimeZone: "Not/AZone"}},
		"Cal", "", now, ny)
	assert.Equal(t, "20261010", icsValue(icsLines(out), "DTSTART;VALUE=DATE:"))
}

func TestEventLocationAndValidZone(t *testing.T) {
	assert.True(t, ValidCalendarTimeZone("America/Chicago"))
	assert.True(t, ValidCalendarTimeZone("UTC"))
	for _, bad := range []string{"", "Local", "Mars/Olympus", "../../etc/passwd"} {
		assert.False(t, ValidCalendarTimeZone(bad), bad)
	}
	tz := "Asia/Tokyo"
	assert.Equal(t, "Asia/Tokyo", EventLocation(&tz, time.UTC).String())
	assert.Equal(t, time.UTC, EventLocation(nil, nil))
	bad := "Local"
	ny, err := time.LoadLocation("America/New_York")
	require.NoError(t, err)
	assert.Equal(t, ny, EventLocation(&bad, ny))
}

func ptrTime(t time.Time) *time.Time { return &t }
