package handlers_test

// Calendar authorization seams: events and appointment groups a caller
// can't see are 404 on every verb, an assignment visible only to its
// overrides is off the calendar (and ICS) of students outside them, and
// enrollments outside their date window are not calendar contexts. All
// data is made up.

import (
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/testutil"
)

func TestCalendarAuthz_InvisibleEventsAre404OnEveryVerb(t *testing.T) {
	f := newCalFixture(t)
	start := time.Now().UTC().Add(24 * time.Hour)
	mine := &models.CalendarEvent{ContextType: "User", ContextID: calStudent, Title: "Dentist", StartAt: start, CreatedByUserID: calStudent, WorkflowState: "active"}
	require.NoError(t, f.g.Create(mine).Error)
	other := &models.CalendarEvent{ContextType: "Course", ContextID: calCourseB, Title: "Other tenant", StartAt: start, CreatedByUserID: calOutsider, WorkflowState: "active"}
	require.NoError(t, f.g.Create(other).Error)
	course := &models.CalendarEvent{ContextType: "Course", ContextID: calCourse, Title: "Star party", StartAt: start, CreatedByUserID: calTeacher, WorkflowState: "active"}
	require.NoError(t, f.g.Create(course).Error)

	classmate := f.app(calStudent2, 1)
	body := testutil.JSONBody(map[string]interface{}{"calendar_event": map[string]interface{}{"title": "x"}})
	for _, id := range []uint{mine.ID, other.ID} {
		p := fmt.Sprintf("/calendar_events/%d", id)
		assert.Equal(t, http.StatusNotFound, testutil.MakeRequest(classmate, http.MethodGet, p, nil).StatusCode, "GET %d", id)
		assert.Equal(t, http.StatusNotFound, testutil.MakeRequest(classmate, http.MethodPut, p, body).StatusCode, "PUT %d", id)
		assert.Equal(t, http.StatusNotFound, testutil.MakeRequest(classmate, http.MethodDelete, p, nil).StatusCode, "DELETE %d", id)
	}
	// A visible course event the student can't change stays a 403.
	p := fmt.Sprintf("/calendar_events/%d", course.ID)
	assert.Equal(t, http.StatusForbidden, testutil.MakeRequest(classmate, http.MethodPut, p, testutil.JSONBody(map[string]interface{}{"calendar_event": map[string]interface{}{"title": "x"}})).StatusCode)
	// Another tenant's teacher can't see the course event.
	assert.Equal(t, http.StatusNotFound, testutil.MakeRequest(f.app(calOutsider, 2), http.MethodDelete, p, nil).StatusCode)
	// The owner still manages their own event.
	assert.Equal(t, http.StatusOK, testutil.MakeRequest(f.app(calStudent, 1), http.MethodPut, fmt.Sprintf("/calendar_events/%d", mine.ID), testutil.JSONBody(map[string]interface{}{"calendar_event": map[string]interface{}{"title": "Dentist (moved)"}})).StatusCode)
}

func TestCalendarAuthz_OverrideOnlyAssignmentsAndCurrentEnrollments(t *testing.T) {
	f := newCalFixture(t)
	base := time.Now().UTC().Add(48 * time.Hour).Truncate(time.Hour)
	due := base.Add(2 * time.Hour)
	ovDue := base.Add(5 * time.Hour)
	a := &models.Assignment{CourseID: calCourse, Name: "Section A only lab", DueAt: &due, WorkflowState: models.AssignmentPublished, Published: true, OnlyVisibleToOverrides: true}
	require.NoError(t, f.g.Create(a).Error)
	secA := calSectionA
	require.NoError(t, f.g.Create(&models.AssignmentOverride{AssignmentID: a.ID, DueAt: &ovDue, CourseSectionID: &secA, WorkflowState: "active"}).Error)

	window := calRange(base, base.Add(24*time.Hour))
	assert.Contains(t, titles(calItems(t, f.app(calStudent, 1), window)), "Section A only lab", "assigned through the section override")
	assert.NotContains(t, titles(calItems(t, f.app(calStudent2, 1), window)), "Section A only lab", "not assigned to section B")

	// The ICS feed follows the calendar.
	app := f.app(calStudent2, 1)
	resp := testutil.MakeRequest(app, http.MethodPost, "/calendar/feed", nil)
	require.Equal(t, http.StatusCreated, resp.StatusCode)
	var feed map[string]interface{}
	calJSON(t, resp, &feed)
	feedPath := strings.TrimPrefix(feed["feed_path"].(string), "/api/v1")
	resp = testutil.MakeRequest(app, http.MethodGet, feedPath, nil)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	ics, _ := io.ReadAll(resp.Body)
	assert.NotContains(t, string(ics), "Section A only lab")

	// An enrollment whose end date has passed is not a calendar context,
	// and its course's items leave the calendar.
	past := time.Now().Add(-24 * time.Hour)
	longAgo := time.Now().Add(-90 * 24 * time.Hour)
	require.NoError(t, f.g.Model(&models.Enrollment{}).Where("user_id = ? AND course_id = ?", calStudent, calCourse).
		Updates(map[string]interface{}{"start_at": longAgo, "end_at": past}).Error)
	var ctxs []map[string]interface{}
	resp = testutil.MakeRequest(f.app(calStudent, 1), http.MethodGet, "/calendar/contexts", nil)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	calJSON(t, resp, &ctxs)
	for _, c := range ctxs {
		assert.NotEqual(t, fmt.Sprintf("course_%d", calCourse), c["asset_string"], "concluded enrollment")
	}
	assert.NotContains(t, titles(calItems(t, f.app(calStudent, 1), window)), "Section A only lab")
	// A current enrollment is still listed.
	resp = testutil.MakeRequest(f.app(calStudent2, 1), http.MethodGet, "/calendar/contexts", nil)
	calJSON(t, resp, &ctxs)
	found := false
	for _, c := range ctxs {
		found = found || c["asset_string"] == fmt.Sprintf("course_%d", calCourse)
	}
	assert.True(t, found)
}
