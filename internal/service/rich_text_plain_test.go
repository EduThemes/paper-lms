package service

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/testutil/mocks"
)

func TestRichTextToPlain(t *testing.T) {
	cases := map[string]string{
		"":                            "",
		"Plain text typed before RCE": "Plain text typed before RCE",
		"<p>Bring goggles</p><p>Don&#39;t be late &amp; sign in</p>": "Bring goggles\nDon't be late & sign in",
		"<ul><li>One</li><li>Two</li></ul>":                          "One\nTwo",
		"line one<br>line two":                                       "line one\nline two",
	}
	for in, want := range cases {
		assert.Equal(t, want, RichTextToPlain(in), in)
	}
}

// G010: the iCalendar feed carries the description as plain text, not the
// stored HTML.
func TestExportAsICalendar_DescriptionIsPlainText(t *testing.T) {
	repo := new(mocks.MockCalendarEventRepository)
	start := time.Date(2026, 3, 2, 15, 0, 0, 0, time.UTC)
	repo.On("ListByContextAndDateRange", mock.Anything, "Course", uint(5), mock.Anything, mock.Anything).Return([]models.CalendarEvent{
		{ID: 1, Title: "Lab day", Description: "<p>Bring <strong>goggles</strong></p><p>Room 101</p>", StartAt: start},
	}, nil)
	out, err := NewCalendarService(repo).ExportAsICalendar(context.Background(), "Course", 5, start.Add(-time.Hour), start.Add(time.Hour))
	require.NoError(t, err)
	ics := string(out)
	assert.Contains(t, ics, `DESCRIPTION:Bring goggles\nRoom 101`)
	assert.False(t, strings.Contains(ics, "<p>") || strings.Contains(ics, "<strong>"))
}
