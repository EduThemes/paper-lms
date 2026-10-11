package service_test

import (
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/EduThemes/paper-lms/internal/service"
)

func occ(t *testing.T, rule string, start time.Time, loc *time.Location) []time.Time {
	t.Helper()
	r, err := service.ParseRRule(rule)
	require.NoError(t, err)
	out, err := r.Occurrences(start, loc)
	require.NoError(t, err)
	return out
}

func TestRRule_WeeklyByDayCount(t *testing.T) {
	// Monday 2026-09-07 09:00 UTC, every Mon/Wed, 5 times.
	start := time.Date(2026, 9, 7, 9, 0, 0, 0, time.UTC)
	got := occ(t, "FREQ=WEEKLY;BYDAY=MO,WE;COUNT=5", start, time.UTC)
	want := []string{"2026-09-07", "2026-09-09", "2026-09-14", "2026-09-16", "2026-09-21"}
	require.Len(t, got, len(want))
	for i, w := range want {
		assert.Equal(t, w, got[i].Format("2006-01-02"))
		assert.Equal(t, 9, got[i].Hour())
	}
}

func TestRRule_WeeklyStartsOnFirstMatchingDay(t *testing.T) {
	// Starting on a Tuesday with BYDAY=MO,WE: the first event is Wednesday.
	start := time.Date(2026, 9, 8, 14, 0, 0, 0, time.UTC)
	got := occ(t, "RRULE:FREQ=WEEKLY;BYDAY=MO,WE;COUNT=3", start, time.UTC)
	assert.Equal(t, "2026-09-09", got[0].Format("2006-01-02"))
	assert.Equal(t, "2026-09-14", got[1].Format("2006-01-02"))
}

func TestRRule_IntervalAndUntilDate(t *testing.T) {
	start := time.Date(2026, 1, 1, 10, 0, 0, 0, time.UTC)
	got := occ(t, "FREQ=DAILY;INTERVAL=2;UNTIL=20260107", start, time.UTC)
	// Jan 1, 3, 5, 7 — the UNTIL date is inclusive.
	require.Len(t, got, 4)
	assert.Equal(t, "2026-01-07", got[3].Format("2006-01-02"))
}

func TestRRule_MonthlyOrdinalWeekday(t *testing.T) {
	// Second Tuesday of each month.
	start := time.Date(2026, 1, 1, 18, 0, 0, 0, time.UTC)
	got := occ(t, "FREQ=MONTHLY;BYDAY=2TU;COUNT=3", start, time.UTC)
	assert.Equal(t, "2026-01-13", got[0].Format("2006-01-02"))
	assert.Equal(t, "2026-02-10", got[1].Format("2006-01-02"))
	assert.Equal(t, "2026-03-10", got[2].Format("2006-01-02"))

	// Last Friday.
	got = occ(t, "FREQ=MONTHLY;BYDAY=-1FR;COUNT=2", start, time.UTC)
	assert.Equal(t, "2026-01-30", got[0].Format("2006-01-02"))
	assert.Equal(t, "2026-02-27", got[1].Format("2006-01-02"))
}

func TestRRule_MonthlySkipsShortMonths(t *testing.T) {
	start := time.Date(2026, 1, 31, 9, 0, 0, 0, time.UTC)
	got := occ(t, "FREQ=MONTHLY;COUNT=3", start, time.UTC)
	assert.Equal(t, "2026-01-31", got[0].Format("2006-01-02"))
	assert.Equal(t, "2026-03-31", got[1].Format("2006-01-02"))
	assert.Equal(t, "2026-05-31", got[2].Format("2006-01-02"))
}

func TestRRule_KeepsWallClockAcrossDST(t *testing.T) {
	ny, err := time.LoadLocation("America/New_York")
	require.NoError(t, err)
	// 9:00 New York on Tuesday 2026-10-27; DST ends 2026-11-01.
	start := time.Date(2026, 10, 27, 9, 0, 0, 0, ny)
	got := occ(t, "FREQ=WEEKLY;COUNT=2", start, ny)
	assert.Equal(t, 9, got[0].In(ny).Hour())
	assert.Equal(t, 9, got[1].In(ny).Hour())
	assert.Equal(t, 13, got[0].Hour(), "EDT is UTC-4")
	assert.Equal(t, 14, got[1].Hour(), "EST is UTC-5")
}

func TestRRule_Rejects(t *testing.T) {
	for _, rule := range []string{
		"",
		"FREQ=WEEKLY",                   // unbounded
		"FREQ=HOURLY;COUNT=3",           // unsupported
		"FREQ=WEEKLY;COUNT=500",         // over the cap
		"FREQ=WEEKLY;BYDAY=2MO;COUNT=2", // ordinal on weekly
		"FREQ=DAILY;COUNT=2;BYSETPOS=1",
		"FREQ=DAILY;UNTIL=tomorrow",
	} {
		_, err := service.ParseRRule(rule)
		assert.Truef(t, errors.Is(err, service.ErrInvalidRRule), "rule %q should be rejected, got %v", rule, err)
	}
}

func TestRRule_UntilCapsAtLimit(t *testing.T) {
	r, err := service.ParseRRule("FREQ=DAILY;UNTIL=20400101")
	require.NoError(t, err)
	_, err = r.Occurrences(time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC), time.UTC)
	assert.ErrorIs(t, err, service.ErrInvalidRRule)
}

func TestRRule_RoundTripAndWithUntil(t *testing.T) {
	r, err := service.ParseRRule("FREQ=WEEKLY;INTERVAL=2;BYDAY=TU,TH;COUNT=6")
	require.NoError(t, err)
	assert.Equal(t, "FREQ=WEEKLY;INTERVAL=2;BYDAY=TU,TH;COUNT=6", r.String())
	cut := r.WithUntil(time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC))
	assert.Equal(t, "FREQ=WEEKLY;INTERVAL=2;BYDAY=TU,TH;UNTIL=20260301T000000Z", cut.String())
}
