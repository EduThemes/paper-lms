package handlers_test

// G010: calendar event descriptions are rich content now (edited with the
// rich content editor), so they keep safe formatting and lose script.

import (
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/testutil"
)

func TestCreateCalendarEvent_DescriptionIsSanitizedRichContent(t *testing.T) {
	app, calRepo := calendarFixture(commsTeacherID)
	var saved *models.CalendarEvent
	calRepo.On("Create", mock.Anything, mock.AnythingOfType("*models.CalendarEvent")).
		Run(func(args mock.Arguments) { saved = args.Get(1).(*models.CalendarEvent) }).
		Return(nil)

	body := map[string]any{"calendar_event": map[string]any{
		"context_type": "Course", "context_id": commsCourseID, "title": "Lab day",
		"start_at":    time.Now().Add(24 * time.Hour).Format(time.RFC3339),
		"description": `<p>Bring <strong>goggles</strong></p><img src=x onerror="alert(1)"><script>alert(2)</script>`,
	}}
	resp := testutil.MakeRequest(app, http.MethodPost, "/calendar_events", testutil.JSONBody(body))
	assert.Equal(t, http.StatusCreated, resp.StatusCode)
	if assert.NotNil(t, saved) {
		assert.Contains(t, saved.Description, "<strong>goggles</strong>")
		assert.NotContains(t, saved.Description, "onerror")
		assert.NotContains(t, saved.Description, "<script")
	}
}
