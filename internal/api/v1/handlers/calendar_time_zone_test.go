package handlers

// G157: bare YYYY-MM-DD dates on the calendar and planner endpoints are
// days in the caller's profile time zone, not UTC days.

import (
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/EduThemes/paper-lms/internal/service"
)

func TestParseCalendarQuery_BareDatesAreProfileZoneDays(t *testing.T) {
	ny, err := time.LoadLocation("America/New_York")
	if err != nil {
		t.Skip("tzdata unavailable")
	}
	run := func(target string, loc *time.Location) service.CalendarQuery {
		var q service.CalendarQuery
		app := fiber.New()
		app.Get("/q", func(c *fiber.Ctx) error {
			var perr error
			q, perr = parseCalendarQuery(c, 1, 1, false, loc)
			require.NoError(t, perr)
			return nil
		})
		_, err := app.Test(httptest.NewRequest("GET", target, nil), -1)
		require.NoError(t, err)
		return q
	}

	q := run("/q?start_date=2030-01-15&end_date=2030-01-16", ny)
	assert.True(t, q.Start.Equal(time.Date(2030, 1, 15, 5, 0, 0, 0, time.UTC)), "start is NY midnight, got %s", q.Start.UTC())
	assert.True(t, q.End.Equal(time.Date(2030, 1, 17, 4, 59, 59, 0, time.UTC)), "end is the last second of Jan 16 in NY, got %s", q.End.UTC())

	// A lone start date covers that whole (profile-zone) day.
	q = run("/q?start_date=2030-01-15", ny)
	assert.True(t, q.End.Equal(time.Date(2030, 1, 16, 4, 59, 59, 0, time.UTC)), "got %s", q.End.UTC())

	// The default is today in the profile zone.
	q = run("/q", ny)
	assert.Equal(t, ny.String(), q.Start.Location().String())
	assert.Equal(t, 0, q.Start.Hour())
	nowNY := time.Now().In(ny)
	assert.Equal(t, nowNY.Day(), q.Start.Day())

	// RFC 3339 instants pass through; nil loc keeps the UTC reading.
	q = run("/q?start_date=2030-01-15T10:00:00Z&end_date=2030-01-15T12:00:00Z", nil)
	assert.True(t, q.Start.Equal(time.Date(2030, 1, 15, 10, 0, 0, 0, time.UTC)))
	assert.True(t, q.End.Equal(time.Date(2030, 1, 15, 12, 0, 0, 0, time.UTC)))
	q = run("/q?start_date=2030-01-15", nil)
	assert.True(t, q.Start.Equal(time.Date(2030, 1, 15, 0, 0, 0, 0, time.UTC)))
}

func TestParsePlannerTime_BareDatesAreProfileZoneDays(t *testing.T) {
	tokyo, err := time.LoadLocation("Asia/Tokyo")
	if err != nil {
		t.Skip("tzdata unavailable")
	}
	start, err := parsePlannerTime("2030-03-01", false, tokyo)
	require.NoError(t, err)
	assert.True(t, start.Equal(time.Date(2030, 2, 28, 15, 0, 0, 0, time.UTC)), "got %s", start.UTC())

	end, err := parsePlannerTime("2030-03-01", true, tokyo)
	require.NoError(t, err)
	assert.True(t, end.Equal(time.Date(2030, 3, 1, 15, 0, 0, 0, time.UTC).Add(-time.Nanosecond)), "got %s", end.UTC())

	inst, err := parsePlannerTime("2030-03-01T09:00:00Z", true, tokyo)
	require.NoError(t, err)
	assert.True(t, inst.Equal(time.Date(2030, 3, 1, 9, 0, 0, 0, time.UTC)))

	utc, err := parsePlannerTime("2030-03-01", false, nil)
	require.NoError(t, err)
	assert.True(t, utc.Equal(time.Date(2030, 3, 1, 0, 0, 0, 0, time.UTC)))

	_, err = parsePlannerTime("03/01/2030", false, tokyo)
	assert.Error(t, err)
}
