package handlers_test

// DB-backed tests for the calendar lane: the aggregated calendar
// (G094/G146), section-specific and recurring events (G095), the
// subscribable ICS feed (G096) and the scheduler additions (G097).
// Skipped without PARITY_DB_URL / DATABASE_URL. All data is made up.

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"github.com/EduThemes/paper-lms/internal/api/v1/handlers"
	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/repository/postgres"
	"github.com/EduThemes/paper-lms/internal/service"
	"github.com/EduThemes/paper-lms/internal/testutil"
)

const (
	calTeacher   uint = 901
	calStudent   uint = 902 // section A
	calStudent2  uint = 903 // section B
	calOutsider  uint = 904 // other tenant
	calCourse    uint = 950
	calCourseB   uint = 951 // tenant 2
	calSectionA  uint = 960
	calSectionB  uint = 961
	calGroupCat  uint = 970
	calGroup     uint = 971
	calGroup2    uint = 972
	calClassmate uint = 905 // section A, same student group as calStudent
)

type calFixture struct {
	g     *gorm.DB
	authz *handlers.ResourceAuthorizer
	agg   *service.CalendarAggregateService
	cal   *service.CalendarService
	feeds *service.CalendarFeedService
	appt  *service.AppointmentGroupService
	conv  *service.ConversationService
}

func newCalFixture(t *testing.T) *calFixture {
	t.Helper()
	g := coreScratchDB(t)
	users := postgres.NewUserRepository(g)
	for _, u := range []*models.User{
		{ID: calTeacher, AccountID: 1, Name: "Terry Teacher", LoginID: "terry.cal@example.test", Email: "terry.cal@example.test", PasswordHash: "x", Role: "user", TimeZone: "America/New_York"},
		{ID: calStudent, AccountID: 1, Name: "Sam Student", LoginID: "sam.cal@example.test", Email: "sam.cal@example.test", PasswordHash: "x", Role: "user"},
		{ID: calStudent2, AccountID: 1, Name: "Bea Student", LoginID: "bea.cal@example.test", Email: "bea.cal@example.test", PasswordHash: "x", Role: "user"},
		{ID: calClassmate, AccountID: 1, Name: "Cal Classmate", LoginID: "cal.cal@example.test", Email: "cal.cal@example.test", PasswordHash: "x", Role: "user"},
		{ID: calOutsider, AccountID: 2, Name: "Olly Outsider", LoginID: "olly.cal@example.test", Email: "olly.cal@example.test", PasswordHash: "x", Role: "user"},
	} {
		require.NoError(t, users.Create(t.Context(), u))
	}
	for _, c := range []*models.Course{
		{ID: calCourse, AccountID: 1, Name: "Astronomy", CourseCode: "AST", WorkflowState: models.CourseAvailable},
		{ID: calCourseB, AccountID: 2, Name: "Geology", CourseCode: "GEO", WorkflowState: models.CourseAvailable},
	} {
		require.NoError(t, g.Create(c).Error)
	}
	for _, s := range []*models.CourseSection{
		{ID: calSectionA, CourseID: calCourse, Name: "Section A", WorkflowState: "active"},
		{ID: calSectionB, CourseID: calCourse, Name: "Section B", WorkflowState: "active"},
	} {
		require.NoError(t, g.Create(s).Error)
	}
	secA, secB := calSectionA, calSectionB
	for _, e := range []*models.Enrollment{
		{UserID: calTeacher, CourseID: calCourse, Type: "TeacherEnrollment", Role: "TeacherEnrollment", WorkflowState: models.EnrollmentActive},
		{UserID: calStudent, CourseID: calCourse, CourseSectionID: &secA, Type: "StudentEnrollment", Role: "StudentEnrollment", WorkflowState: models.EnrollmentActive},
		{UserID: calClassmate, CourseID: calCourse, CourseSectionID: &secA, Type: "StudentEnrollment", Role: "StudentEnrollment", WorkflowState: models.EnrollmentActive},
		{UserID: calStudent2, CourseID: calCourse, CourseSectionID: &secB, Type: "StudentEnrollment", Role: "StudentEnrollment", WorkflowState: models.EnrollmentActive},
		{UserID: calOutsider, CourseID: calCourseB, Type: "TeacherEnrollment", Role: "TeacherEnrollment", WorkflowState: models.EnrollmentActive},
	} {
		require.NoError(t, g.Create(e).Error)
	}
	cid := calCourse
	require.NoError(t, g.Create(&models.GroupCategory{ID: calGroupCat, CourseID: &cid, Name: "Lab teams"}).Error)
	require.NoError(t, g.Create(&models.Group{ID: calGroup, GroupCategoryID: calGroupCat, Name: "Team Orion", ContextType: "Course", ContextID: calCourse}).Error)
	require.NoError(t, g.Create(&models.Group{ID: calGroup2, GroupCategoryID: calGroupCat, Name: "Team Lyra", ContextType: "Course", ContextID: calCourse}).Error)
	for _, m := range []*models.GroupMembership{
		{GroupID: calGroup, UserID: calStudent, WorkflowState: "accepted"},
		{GroupID: calGroup, UserID: calClassmate, WorkflowState: "accepted"},
		{GroupID: calGroup2, UserID: calStudent2, WorkflowState: "accepted"},
	} {
		require.NoError(t, g.Create(m).Error)
	}

	return &calFixture{
		g:     g,
		authz: handlers.NewResourceAuthorizer(postgres.NewEnrollmentRepository(g), users, postgres.NewCourseRepository(g)),
		agg:   service.NewCalendarAggregateService(g),
		cal:   service.NewCalendarService(postgres.NewCalendarEventRepository(g)),
		feeds: service.NewCalendarFeedService(g, nil, nil),
		appt: service.NewAppointmentGroupService(postgres.NewAppointmentGroupRepository(g), postgres.NewAppointmentSlotRepository(g),
			postgres.NewAppointmentReservationRepository(g), g),
		conv: service.NewConversationService(postgres.NewConversationRepository(g), postgres.NewConversationParticipantRepository(g), postgres.NewConversationMessageRepository(g)),
	}
}

func (f *calFixture) app(userID, accountID uint) *fiber.App {
	ev := handlers.NewCalendarEventHandler(f.cal, f.authz)
	ev.SetAggregator(f.agg)
	ch := handlers.NewCalendarHandler(f.agg, f.feeds, f.authz)
	ag := handlers.NewAppointmentGroupHandler(f.appt, f.authz, nil)
	ag.SetMessenger(f.conv)

	app := testutil.SetupTestApp()
	app.Get("/feeds/calendars/:token", ch.PublicFeed)
	app.Use(callerStub(userID, accountID))
	app.Get("/calendar_events", ev.ListEvents)
	app.Post("/calendar_events", ev.CreateEvent)
	app.Get("/calendar_events/:id", ev.GetEvent)
	app.Put("/calendar_events/:id", ev.UpdateEvent)
	app.Delete("/calendar_events/:id", ev.DeleteEvent)
	app.Get("/calendar/contexts", ch.ListContexts)
	app.Get("/calendar/items", ch.ListItems)
	app.Get("/calendar/feed", ch.GetFeed)
	app.Post("/calendar/feed", ch.RotateFeed)
	app.Get("/appointment_groups", ag.List)
	app.Post("/courses/:course_id/appointment_groups", ag.Create)
	app.Get("/appointment_groups/:id", ag.Get)
	app.Put("/appointment_groups/:id", ag.Update)
	app.Get("/appointment_groups/:id/appointments", ag.ListSlots)
	app.Delete("/appointment_groups/:id/appointments/:slot_id", ag.DeleteSlot)
	app.Post("/appointment_groups/:id/appointments/:slot_id/reservations", ag.Reserve)
	app.Delete("/appointment_groups/:id/appointments/:slot_id/reservations/:reservation_id", ag.CancelReservation)
	app.Get("/appointment_groups/:id/users", ag.ListParticipants)
	app.Get("/appointment_groups/:id/groups", ag.ListParticipants)
	app.Post("/appointment_groups/:id/messages", ag.MessageParticipants)
	app.Get("/users/self/appointments", ag.ListMyReservations)
	return app
}

func calJSON(t *testing.T, resp *http.Response, out interface{}) {
	t.Helper()
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.NoErrorf(t, json.Unmarshal(body, out), "body: %s", body)
}

func calItems(t *testing.T, app *fiber.App, query string) []map[string]interface{} {
	t.Helper()
	resp := testutil.MakeRequest(app, http.MethodGet, "/calendar/items?"+query, nil)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	var items []map[string]interface{}
	calJSON(t, resp, &items)
	return items
}

func titles(items []map[string]interface{}) []string {
	out := make([]string, 0, len(items))
	for _, it := range items {
		out = append(out, it["title"].(string))
	}
	return out
}

func calRange(from, to time.Time) string {
	return fmt.Sprintf("start_date=%s&end_date=%s", from.UTC().Format(time.RFC3339), to.UTC().Format(time.RFC3339))
}

// G094/G146: one request returns personal, course, section, group events,
// assignment and quiz due dates (with the student's own override dates)
// and the student's appointments; teachers see every section and every
// dated override.
func TestCalendarLane_AggregatesEverythingTheUserCanSee(t *testing.T) {
	f := newCalFixture(t)
	g := f.g
	base := time.Now().UTC().Add(48 * time.Hour).Truncate(time.Hour)
	at := func(h int) time.Time { return base.Add(time.Duration(h) * time.Hour) }
	end := func(h int) *time.Time { e := at(h); return &e }

	require.NoError(t, g.Create(&models.CalendarEvent{ContextType: "User", ContextID: calStudent, Title: "Dentist", StartAt: at(1), CreatedByUserID: calStudent, WorkflowState: "active"}).Error)
	require.NoError(t, g.Create(&models.CalendarEvent{ContextType: "User", ContextID: calStudent2, Title: "Not mine", StartAt: at(1), CreatedByUserID: calStudent2, WorkflowState: "active"}).Error)
	require.NoError(t, g.Create(&models.CalendarEvent{ContextType: "Course", ContextID: calCourse, Title: "Star party", StartAt: at(2), EndAt: end(3), CreatedByUserID: calTeacher, WorkflowState: "active"}).Error)
	require.NoError(t, g.Create(&models.CalendarEvent{ContextType: "Course", ContextID: calCourseB, Title: "Other tenant", StartAt: at(2), CreatedByUserID: calOutsider, WorkflowState: "active"}).Error)
	require.NoError(t, g.Create(&models.CalendarEvent{ContextType: "Group", ContextID: calGroup, Title: "Team sync", StartAt: at(4), CreatedByUserID: calStudent, WorkflowState: "active"}).Error)

	// Section-specific lab: the parent stays hidden.
	parent := &models.CalendarEvent{ContextType: "Course", ContextID: calCourse, Title: "Lab", StartAt: at(5), CreatedByUserID: calTeacher}
	_, err := f.cal.CreateWithOptions(t.Context(), parent, service.CreateOptions{Children: []service.ChildEventInput{
		{SectionID: calSectionA, StartAt: at(5), EndAt: end(6)},
		{SectionID: calSectionB, StartAt: at(7), EndAt: end(8)},
	}})
	require.NoError(t, err)

	pts := 10.0
	plain := &models.Assignment{CourseID: calCourse, Name: "Moon log", DueAt: end(9), PointsPossible: &pts, WorkflowState: models.AssignmentPublished, Published: true}
	draft := &models.Assignment{CourseID: calCourse, Name: "Draft essay", DueAt: end(9), WorkflowState: models.AssignmentUnpublished}
	sectioned := &models.Assignment{CourseID: calCourse, Name: "Telescope report", DueAt: end(10), WorkflowState: models.AssignmentPublished, Published: true}
	adhoc := &models.Assignment{CourseID: calCourse, Name: "Comet sketch", DueAt: end(11), WorkflowState: models.AssignmentPublished, Published: true}
	for _, a := range []*models.Assignment{plain, draft, sectioned, adhoc} {
		require.NoError(t, g.Create(a).Error)
	}
	secA := calSectionA
	require.NoError(t, g.Create(&models.AssignmentOverride{AssignmentID: sectioned.ID, DueAt: end(30), CourseSectionID: &secA, WorkflowState: "active"}).Error)
	ov := &models.AssignmentOverride{AssignmentID: adhoc.ID, Title: "Extension", DueAt: end(40), WorkflowState: "active"}
	require.NoError(t, g.Create(ov).Error)
	require.NoError(t, g.Create(&models.AssignmentOverrideStudent{AssignmentOverrideID: ov.ID, UserID: calStudent, AssignmentID: adhoc.ID}).Error)
	require.NoError(t, g.Create(&models.Quiz{CourseID: calCourse, Title: "Constellation quiz", DueAt: end(12), Published: true, WorkflowState: "available"}).Error)

	studentApp := f.app(calStudent, 1)
	items := calItems(t, studentApp, calRange(base, base.Add(72*time.Hour)))
	got := titles(items)
	assert.ElementsMatch(t, []string{"Dentist", "Star party", "Team sync", "Lab", "Moon log", "Telescope report", "Comet sketch", "Constellation quiz"}, got)
	for _, it := range items {
		switch it["title"] {
		case "Lab":
			assert.Equal(t, "Section A", it["section_name"])
			assert.Equal(t, at(5).Format(time.RFC3339), it["start_at"])
		case "Telescope report":
			assert.Equal(t, at(30).Format(time.RFC3339), it["start_at"], "section override date replaces the base date")
		case "Comet sketch":
			assert.Equal(t, at(40).Format(time.RFC3339), it["start_at"], "the student's own override applies")
		case "Moon log":
			assert.Equal(t, "assignment", it["type"])
			assert.Equal(t, fmt.Sprintf("course_%d", calCourse), it["context_code"])
		}
	}

	// Only the personal calendar.
	only := calItems(t, studentApp, calRange(base, base.Add(72*time.Hour))+fmt.Sprintf("&context_codes[]=user_%d", calStudent))
	assert.Equal(t, []string{"Dentist"}, titles(only))
	// A context the student can't see is ignored, not leaked.
	none := calItems(t, studentApp, calRange(base, base.Add(72*time.Hour))+fmt.Sprintf("&context_codes[]=course_%d", calCourseB))
	assert.Empty(t, none)

	// The teacher sees both section times, the draft and each dated override.
	teacher := titles(calItems(t, f.app(calTeacher, 1), calRange(base, base.Add(72*time.Hour))))
	assert.Contains(t, teacher, "Draft essay")
	assert.Contains(t, teacher, "Telescope report (Section A)")
	assert.Contains(t, teacher, "Comet sketch (Extension)")
	labs := 0
	for _, tt := range teacher {
		if tt == "Lab" {
			labs++
		}
	}
	assert.Equal(t, 2, labs, "teacher sees every section's time")
	assert.NotContains(t, teacher, "Dentist")

	// Canvas shape: /calendar_events?type=assignment&context_codes[]=course_X
	resp := testutil.MakeRequest(studentApp, http.MethodGet, "/calendar_events?type=assignment&"+calRange(base, base.Add(72*time.Hour))+fmt.Sprintf("&context_codes[]=course_%d", calCourse), nil)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	var asg []map[string]interface{}
	calJSON(t, resp, &asg)
	assert.ElementsMatch(t, []string{"Moon log", "Telescope report", "Comet sketch", "Constellation quiz"}, titles(asg))

	// Contexts list the personal calendar, the course (with sections for
	// the teacher) and the student's group.
	resp = testutil.MakeRequest(f.app(calTeacher, 1), http.MethodGet, "/calendar/contexts", nil)
	var ctxs []map[string]interface{}
	calJSON(t, resp, &ctxs)
	require.GreaterOrEqual(t, len(ctxs), 2)
	assert.Equal(t, fmt.Sprintf("user_%d", calTeacher), ctxs[0]["asset_string"])
	assert.Equal(t, fmt.Sprintf("course_%d", calCourse), ctxs[1]["asset_string"])
	assert.Len(t, ctxs[1]["sections"], 2)
}

// G095: section times through the API, parent-tie on sections, and a
// recurring series edited and deleted with which=one|all|following.
func TestCalendarLane_SectionAndRecurringEvents(t *testing.T) {
	f := newCalFixture(t)
	teacher := f.app(calTeacher, 1)
	base := time.Now().UTC().Add(72 * time.Hour).Truncate(time.Hour)

	// A section that isn't in the course is refused.
	resp := testutil.MakeRequest(teacher, http.MethodPost, "/calendar_events", testutil.JSONBody(map[string]interface{}{
		"calendar_event": map[string]interface{}{
			"context_code": fmt.Sprintf("course_%d", calCourse), "title": "Lab",
			"child_event_data": []map[string]interface{}{{"start_at": base, "context_code": "course_section_99999"}},
		},
	}))
	assert.Equal(t, http.StatusNotFound, resp.StatusCode)

	resp = testutil.MakeRequest(teacher, http.MethodPost, "/calendar_events", testutil.JSONBody(map[string]interface{}{
		"calendar_event": map[string]interface{}{
			"context_code": fmt.Sprintf("course_%d", calCourse), "title": "Lab",
			"child_event_data": map[string]interface{}{
				"0": map[string]interface{}{"start_at": base, "end_at": base.Add(time.Hour), "context_code": fmt.Sprintf("course_section_%d", calSectionA)},
				"1": map[string]interface{}{"start_at": base.Add(24 * time.Hour), "end_at": base.Add(25 * time.Hour), "context_code": fmt.Sprintf("course_section_%d", calSectionB)},
			},
		},
	}))
	require.Equal(t, http.StatusCreated, resp.StatusCode)
	var parent map[string]interface{}
	calJSON(t, resp, &parent)
	assert.EqualValues(t, 2, parent["child_events_count"])
	assert.Equal(t, base.Add(25*time.Hour).Format(time.RFC3339), parent["end_at"], "parent spans its sections")
	parentID := uint(parent["id"].(float64))

	// Students in section B only see section B's time.
	b := calItems(t, f.app(calStudent2, 1), calRange(base.Add(-time.Hour), base.Add(48*time.Hour)))
	require.Len(t, b, 1)
	assert.Equal(t, "Section B", b[0]["section_name"])

	// Dropping section B deletes its child; a child can't be edited directly.
	resp = testutil.MakeRequest(teacher, http.MethodPut, fmt.Sprintf("/calendar_events/%d", parentID), testutil.JSONBody(map[string]interface{}{
		"calendar_event": map[string]interface{}{
			"title":            "Lab (rescheduled)",
			"child_event_data": []map[string]interface{}{{"start_at": base.Add(2 * time.Hour), "context_code": fmt.Sprintf("course_section_%d", calSectionA)}},
		},
	}))
	require.Equal(t, http.StatusOK, resp.StatusCode)
	kids, err := f.cal.ListChildren(t.Context(), parentID)
	require.NoError(t, err)
	require.Len(t, kids, 1)
	assert.Equal(t, "Lab (rescheduled)", kids[0].Title)
	resp = testutil.MakeRequest(teacher, http.MethodPut, fmt.Sprintf("/calendar_events/%d", kids[0].ID), testutil.JSONBody(map[string]interface{}{"calendar_event": map[string]interface{}{"title": "x"}}))
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	assert.Empty(t, calItems(t, f.app(calStudent2, 1), calRange(base.Add(-time.Hour), base.Add(48*time.Hour))))

	// Students can't create course events.
	resp = testutil.MakeRequest(f.app(calStudent, 1), http.MethodPost, "/calendar_events", testutil.JSONBody(map[string]interface{}{
		"calendar_event": map[string]interface{}{"context_code": fmt.Sprintf("course_%d", calCourse), "title": "Nope", "start_at": base},
	}))
	assert.Equal(t, http.StatusForbidden, resp.StatusCode)

	// Group members can add group events; outsiders get a 404.
	resp = testutil.MakeRequest(f.app(calStudent, 1), http.MethodPost, "/calendar_events", testutil.JSONBody(map[string]interface{}{
		"calendar_event": map[string]interface{}{"context_code": fmt.Sprintf("group_%d", calGroup), "title": "Team sync", "start_at": base},
	}))
	assert.Equal(t, http.StatusCreated, resp.StatusCode)
	resp = testutil.MakeRequest(f.app(calStudent2, 1), http.MethodPost, "/calendar_events", testutil.JSONBody(map[string]interface{}{
		"calendar_event": map[string]interface{}{"context_code": fmt.Sprintf("group_%d", calGroup), "title": "Gatecrash", "start_at": base},
	}))
	assert.Equal(t, http.StatusNotFound, resp.StatusCode)

	// Weekly series of 4 in the teacher's zone.
	student := f.app(calStudent, 1)
	resp = testutil.MakeRequest(student, http.MethodPost, "/calendar_events", testutil.JSONBody(map[string]interface{}{
		"calendar_event": map[string]interface{}{
			"title": "Study group", "start_at": base, "end_at": base.Add(time.Hour),
			"rrule": "FREQ=WEEKLY;COUNT=4", "time_zone": "UTC",
		},
	}))
	require.Equal(t, http.StatusCreated, resp.StatusCode)
	var head map[string]interface{}
	calJSON(t, resp, &head)
	assert.EqualValues(t, 4, head["series_count"])
	assert.Equal(t, true, head["series_head"])
	series, err := f.cal.ListSeries(t.Context(), head["series_uuid"].(string))
	require.NoError(t, err)
	require.Len(t, series, 4)

	// which=following from the third: retitle and move an hour later.
	third := series[2]
	resp = testutil.MakeRequest(student, http.MethodPut, fmt.Sprintf("/calendar_events/%d", third.ID), testutil.JSONBody(map[string]interface{}{
		"which":          "following",
		"calendar_event": map[string]interface{}{"title": "Study group (late)", "start_at": third.StartAt.Add(time.Hour)},
	}))
	require.Equal(t, http.StatusOK, resp.StatusCode)
	series, _ = f.cal.ListSeries(t.Context(), head["series_uuid"].(string))
	assert.Equal(t, "Study group", series[1].Title)
	assert.Equal(t, "Study group (late)", series[2].Title)
	assert.Equal(t, "Study group (late)", series[3].Title)
	assert.Equal(t, base.Add(14*24*time.Hour+time.Hour), series[2].StartAt.UTC())
	assert.Equal(t, time.Hour, series[3].EndAt.Sub(series[3].StartAt), "duration is kept")

	// which=one on the head promotes the next occurrence on delete.
	resp = testutil.MakeRequest(student, http.MethodDelete, fmt.Sprintf("/calendar_events/%d?which=one", series[0].ID), nil)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	series, _ = f.cal.ListSeries(t.Context(), head["series_uuid"].(string))
	require.Len(t, series, 3)
	assert.True(t, series[0].SeriesHead)

	// which=all with a new rule regenerates the series.
	resp = testutil.MakeRequest(student, http.MethodPut, fmt.Sprintf("/calendar_events/%d", series[0].ID), testutil.JSONBody(map[string]interface{}{
		"which":          "all",
		"calendar_event": map[string]interface{}{"rrule": "FREQ=DAILY;COUNT=2", "time_zone": "UTC"},
	}))
	require.Equal(t, http.StatusOK, resp.StatusCode)
	series, _ = f.cal.ListSeries(t.Context(), head["series_uuid"].(string))
	assert.Len(t, series, 2)

	// which=all delete.
	resp = testutil.MakeRequest(student, http.MethodDelete, fmt.Sprintf("/calendar_events/%d?which=all", series[1].ID), nil)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	series, _ = f.cal.ListSeries(t.Context(), head["series_uuid"].(string))
	assert.Empty(t, series)

	// A malformed rule is a 400.
	resp = testutil.MakeRequest(student, http.MethodPost, "/calendar_events", testutil.JSONBody(map[string]interface{}{
		"calendar_event": map[string]interface{}{"title": "Bad", "start_at": base, "rrule": "FREQ=WEEKLY"},
	}))
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

// G096: the feed URL works without a session, carries the aggregated
// calendar, and stops working after rotation.
func TestCalendarLane_PublicFeed(t *testing.T) {
	f := newCalFixture(t)
	base := time.Now().UTC().Add(24 * time.Hour).Truncate(time.Hour)
	require.NoError(t, f.g.Create(&models.CalendarEvent{ContextType: "Course", ContextID: calCourse, Title: "Eclipse viewing; bring glasses", StartAt: base, CreatedByUserID: calTeacher, WorkflowState: "active"}).Error)
	due := base.Add(2 * time.Hour)
	require.NoError(t, f.g.Create(&models.Assignment{CourseID: calCourse, Name: "Moon log", DueAt: &due, WorkflowState: models.AssignmentPublished, Published: true}).Error)

	app := f.app(calStudent, 1)
	resp := testutil.MakeRequest(app, http.MethodGet, "/calendar/feed", nil)
	var st map[string]interface{}
	calJSON(t, resp, &st)
	assert.Equal(t, false, st["exists"])

	resp = testutil.MakeRequest(app, http.MethodPost, "/calendar/feed", nil)
	require.Equal(t, http.StatusCreated, resp.StatusCode)
	calJSON(t, resp, &st)
	path := st["feed_path"].(string)
	require.True(t, strings.HasPrefix(path, "/api/v1/feeds/calendars/pcal_"))
	feedURL := strings.TrimPrefix(path, "/api/v1")

	// No caller stub runs before the feed route: the token is the credential.
	resp = testutil.MakeRequest(app, http.MethodGet, feedURL, nil)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Contains(t, resp.Header.Get("Content-Type"), "text/calendar")
	body, _ := io.ReadAll(resp.Body)
	ics := string(body)
	assert.Contains(t, ics, "BEGIN:VCALENDAR")
	assert.Contains(t, ics, `SUMMARY:Eclipse viewing\; bring glasses`)
	assert.Contains(t, ics, "SUMMARY:Moon log [Astronomy]")

	// Without a secretbox key the URL can't be shown again, only remade.
	resp = testutil.MakeRequest(app, http.MethodGet, "/calendar/feed", nil)
	calJSON(t, resp, &st)
	assert.Equal(t, true, st["exists"])
	assert.Equal(t, false, st["url_available"])

	// Rotating revokes the old URL; junk tokens are a 404 too.
	resp = testutil.MakeRequest(app, http.MethodPost, "/calendar/feed", nil)
	require.Equal(t, http.StatusCreated, resp.StatusCode)
	resp = testutil.MakeRequest(app, http.MethodGet, feedURL, nil)
	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
	resp = testutil.MakeRequest(app, http.MethodGet, "/feeds/calendars/pcal_nope.ics", nil)
	assert.Equal(t, http.StatusNotFound, resp.StatusCode)

	// A sealed copy lets the owner see the URL again.
	keyed := service.NewCalendarFeedService(f.g, func(b []byte) ([]byte, error) { return append([]byte("k:"), b...), nil },
		func(b []byte) ([]byte, error) { return b[2:], nil })
	made, err := keyed.Rotate(t.Context(), calTeacher)
	require.NoError(t, err)
	again, err := keyed.Get(t.Context(), calTeacher, false)
	require.NoError(t, err)
	assert.Equal(t, made.Token, again.Token)
}

// G097: section-limited and group signup, adding/removing slots after
// creation, the student's own appointments, the roster and messaging.
func TestCalendarLane_Scheduler(t *testing.T) {
	f := newCalFixture(t)
	teacher := f.app(calTeacher, 1)
	slotAt := time.Now().UTC().Add(96 * time.Hour).Truncate(time.Hour)

	// Section A only.
	resp := testutil.MakeRequest(teacher, http.MethodPost, fmt.Sprintf("/courses/%d/appointment_groups", calCourse), testutil.JSONBody(map[string]interface{}{
		"appointment_group": map[string]interface{}{
			"title": "Office hours", "workflow_state": "active",
			"sub_context_codes": []string{fmt.Sprintf("course_section_%d", calSectionA)},
			"new_appointments":  []map[string]interface{}{{"start_at": slotAt, "end_at": slotAt.Add(15 * time.Minute)}},
		},
	}))
	require.Equal(t, http.StatusCreated, resp.StatusCode)
	var og map[string]interface{}
	calJSON(t, resp, &og)
	ogID := uint(og["id"].(float64))
	assert.Equal(t, []interface{}{fmt.Sprintf("course_section_%d", calSectionA)}, og["sub_context_codes"])

	// Section B can't see or book it.
	resp = testutil.MakeRequest(f.app(calStudent2, 1), http.MethodGet, fmt.Sprintf("/appointment_groups/%d", ogID), nil)
	assert.Equal(t, http.StatusForbidden, resp.StatusCode)
	var list []map[string]interface{}
	resp = testutil.MakeRequest(f.app(calStudent2, 1), http.MethodGet, "/appointment_groups?scope=reservable", nil)
	calJSON(t, resp, &list)
	assert.Empty(t, list)
	resp = testutil.MakeRequest(f.app(calStudent, 1), http.MethodGet, "/appointment_groups?scope=reservable", nil)
	calJSON(t, resp, &list)
	require.Len(t, list, 1)

	// Editing adds a slot; the teacher can remove one.
	resp = testutil.MakeRequest(teacher, http.MethodPut, fmt.Sprintf("/appointment_groups/%d", ogID), testutil.JSONBody(map[string]interface{}{
		"appointment_group": map[string]interface{}{"new_appointments": []map[string]interface{}{{"start_at": slotAt.Add(time.Hour), "end_at": slotAt.Add(75 * time.Minute)}}},
	}))
	require.Equal(t, http.StatusOK, resp.StatusCode)
	var slots []map[string]interface{}
	resp = testutil.MakeRequest(teacher, http.MethodGet, fmt.Sprintf("/appointment_groups/%d/appointments", ogID), nil)
	calJSON(t, resp, &slots)
	require.Len(t, slots, 2)
	firstSlot := uint(slots[0]["id"].(float64))
	resp = testutil.MakeRequest(teacher, http.MethodDelete, fmt.Sprintf("/appointment_groups/%d/appointments/%d", ogID, uint(slots[1]["id"].(float64))), nil)
	assert.Equal(t, http.StatusNoContent, resp.StatusCode)

	// Sam books with a note; it shows in "my appointments" and on the
	// calendar; the roster splits registered / unregistered.
	resp = testutil.MakeRequest(f.app(calStudent, 1), http.MethodPost, fmt.Sprintf("/appointment_groups/%d/appointments/%d/reservations", ogID, firstSlot), testutil.JSONBody(map[string]interface{}{"comments": "Question about parallax"}))
	require.Equal(t, http.StatusCreated, resp.StatusCode)
	var mine []map[string]interface{}
	resp = testutil.MakeRequest(f.app(calStudent, 1), http.MethodGet, "/users/self/appointments", nil)
	calJSON(t, resp, &mine)
	require.Len(t, mine, 1)
	assert.Equal(t, "Office hours", mine[0]["title"])
	assert.Equal(t, "Astronomy", mine[0]["course_name"])
	assert.Equal(t, "Question about parallax", mine[0]["comments"])
	cal := calItems(t, f.app(calStudent, 1), calRange(slotAt.Add(-time.Hour), slotAt.Add(time.Hour)))
	require.Len(t, cal, 1)
	assert.Equal(t, "appointment", cal[0]["type"])
	tcal := calItems(t, teacher, calRange(slotAt.Add(-time.Hour), slotAt.Add(time.Hour)))
	require.Len(t, tcal, 1)
	assert.Equal(t, "appointment_slot", tcal[0]["type"])
	assert.Equal(t, []interface{}{"Sam Student"}, tcal[0]["reserved_by"])

	var roster []map[string]interface{}
	resp = testutil.MakeRequest(teacher, http.MethodGet, fmt.Sprintf("/appointment_groups/%d/users?registration_status=unregistered", ogID), nil)
	calJSON(t, resp, &roster)
	require.Len(t, roster, 1, "only section A students are participants")
	assert.Equal(t, "Cal Classmate", roster[0]["name"])
	resp = testutil.MakeRequest(f.app(calStudent, 1), http.MethodGet, fmt.Sprintf("/appointment_groups/%d/users", ogID), nil)
	assert.Equal(t, http.StatusForbidden, resp.StatusCode)

	// Message the unregistered: one private conversation each.
	resp = testutil.MakeRequest(teacher, http.MethodPost, fmt.Sprintf("/appointment_groups/%d/messages", ogID), testutil.JSONBody(map[string]interface{}{
		"body": "Please sign up for a slot", "registration_status": "unregistered",
	}))
	require.Equal(t, http.StatusCreated, resp.StatusCode)
	var sent map[string]interface{}
	calJSON(t, resp, &sent)
	assert.EqualValues(t, 1, sent["sent"])
	var n int64
	f.g.Model(&models.ConversationParticipant{}).Where("user_id = ?", calClassmate).Count(&n)
	assert.EqualValues(t, 1, n)

	// Group signup: one booking per student group, visible to the whole team.
	resp = testutil.MakeRequest(teacher, http.MethodPost, fmt.Sprintf("/courses/%d/appointment_groups", calCourse), testutil.JSONBody(map[string]interface{}{
		"appointment_group": map[string]interface{}{
			"title": "Project check-in", "workflow_state": "active", "participants_per_appointment": 2,
			"sub_context_codes": []string{fmt.Sprintf("group_category_%d", calGroupCat)},
			"new_appointments":  []map[string]interface{}{{"start_at": slotAt.Add(2 * time.Hour), "end_at": slotAt.Add(150 * time.Minute)}},
		},
	}))
	require.Equal(t, http.StatusCreated, resp.StatusCode)
	var gg map[string]interface{}
	calJSON(t, resp, &gg)
	ggID := uint(gg["id"].(float64))
	assert.Equal(t, "Group", gg["participant_type"])
	slot := uint(gg["appointments"].([]interface{})[0].(map[string]interface{})["id"].(float64))

	resp = testutil.MakeRequest(f.app(calStudent, 1), http.MethodPost, fmt.Sprintf("/appointment_groups/%d/appointments/%d/reservations", ggID, slot), nil)
	require.Equal(t, http.StatusCreated, resp.StatusCode)
	var res map[string]interface{}
	calJSON(t, resp, &res)
	assert.EqualValues(t, calGroup, res["participant_group_id"])
	resp = testutil.MakeRequest(f.app(calClassmate, 1), http.MethodPost, fmt.Sprintf("/appointment_groups/%d/appointments/%d/reservations", ggID, slot), nil)
	assert.Equal(t, http.StatusConflict, resp.StatusCode, "the team already holds this slot")
	resp = testutil.MakeRequest(f.app(calClassmate, 1), http.MethodGet, "/users/self/appointments", nil)
	calJSON(t, resp, &mine)
	require.Len(t, mine, 2-1, "the classmate sees the team booking")
	assert.Equal(t, "Team Orion", mine[0]["participant_name"])

	resp = testutil.MakeRequest(teacher, http.MethodGet, fmt.Sprintf("/appointment_groups/%d/groups?registration_status=registered", ggID), nil)
	calJSON(t, resp, &roster)
	require.Len(t, roster, 1)
	assert.Equal(t, "Team Orion", roster[0]["name"])

	// A groupmate may cancel the team's booking.
	resp = testutil.MakeRequest(f.app(calClassmate, 1), http.MethodDelete, fmt.Sprintf("/appointment_groups/%d/appointments/%d/reservations/%d", ggID, slot, uint(res["id"].(float64))), nil)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	// Another tenant can't reach the group at all.
	resp = testutil.MakeRequest(f.app(calOutsider, 2), http.MethodGet, fmt.Sprintf("/appointment_groups/%d", ggID), nil)
	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
}

// Review fixes (G097): only students sign up (an observer who can read
// the group can't take a slot), a group can't be created in an unknown
// state, and a group-set appointment group can't also gain section
// limits.
func TestCalendarLane_SchedulerSignupRules(t *testing.T) {
	f := newCalFixture(t)
	const observer uint = 906
	require.NoError(t, postgres.NewUserRepository(f.g).Create(t.Context(), &models.User{
		ID: observer, AccountID: 1, Name: "Oz Observer", LoginID: "oz.cal@example.test", Email: "oz.cal@example.test", PasswordHash: "x", Role: "user",
	}))
	require.NoError(t, f.g.Create(&models.Enrollment{UserID: observer, CourseID: calCourse, Type: "ObserverEnrollment", Role: "ObserverEnrollment", WorkflowState: models.EnrollmentActive}).Error)

	teacher := f.app(calTeacher, 1)
	slotAt := time.Now().UTC().Add(96 * time.Hour).Truncate(time.Hour)
	resp := testutil.MakeRequest(teacher, http.MethodPost, fmt.Sprintf("/courses/%d/appointment_groups", calCourse), testutil.JSONBody(map[string]interface{}{
		"appointment_group": map[string]interface{}{
			"title": "Office hours", "workflow_state": "active",
			"new_appointments": []map[string]interface{}{{"start_at": slotAt, "end_at": slotAt.Add(15 * time.Minute)}},
		},
	}))
	require.Equal(t, http.StatusCreated, resp.StatusCode)
	var og map[string]interface{}
	calJSON(t, resp, &og)
	ogID := uint(og["id"].(float64))
	slotID := uint(og["appointments"].([]interface{})[0].(map[string]interface{})["id"].(float64))

	resp = testutil.MakeRequest(f.app(observer, 1), http.MethodPost, fmt.Sprintf("/appointment_groups/%d/appointments/%d/reservations", ogID, slotID), nil)
	assert.Equal(t, http.StatusForbidden, resp.StatusCode, "an observer can't book a student's slot")
	resp = testutil.MakeRequest(f.app(calStudent, 1), http.MethodPost, fmt.Sprintf("/appointment_groups/%d/appointments/%d/reservations", ogID, slotID), nil)
	assert.Equal(t, http.StatusCreated, resp.StatusCode)

	resp = testutil.MakeRequest(teacher, http.MethodPost, fmt.Sprintf("/courses/%d/appointment_groups", calCourse), testutil.JSONBody(map[string]interface{}{
		"appointment_group": map[string]interface{}{"title": "Ghost", "workflow_state": "deleted"},
	}))
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)

	resp = testutil.MakeRequest(teacher, http.MethodPost, fmt.Sprintf("/courses/%d/appointment_groups", calCourse), testutil.JSONBody(map[string]interface{}{
		"appointment_group": map[string]interface{}{
			"title": "Project check-in", "workflow_state": "active",
			"sub_context_codes": []string{fmt.Sprintf("group_category_%d", calGroupCat)},
		},
	}))
	require.Equal(t, http.StatusCreated, resp.StatusCode)
	var gg map[string]interface{}
	calJSON(t, resp, &gg)
	resp = testutil.MakeRequest(teacher, http.MethodPut, fmt.Sprintf("/appointment_groups/%d", uint(gg["id"].(float64))), testutil.JSONBody(map[string]interface{}{
		"appointment_group": map[string]interface{}{"sub_context_codes": []string{fmt.Sprintf("course_section_%d", calSectionA)}},
	}))
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

// Review fix (G094): a quiz published through its workflow_state (the
// quiz pages' test) shows on the student calendar; an unpublished one
// doesn't.
func TestCalendarLane_QuizPublishedByWorkflowState(t *testing.T) {
	f := newCalFixture(t)
	due := time.Now().UTC().Add(50 * time.Hour).Truncate(time.Hour)
	require.NoError(t, f.g.Create(&models.Quiz{CourseID: calCourse, Title: "Nebula quiz", DueAt: &due, WorkflowState: "published"}).Error)
	require.NoError(t, f.g.Create(&models.Quiz{CourseID: calCourse, Title: "Hidden quiz", DueAt: &due, WorkflowState: "unpublished"}).Error)
	got := titles(calItems(t, f.app(calStudent, 1), calRange(due.Add(-time.Hour), due.Add(time.Hour))))
	assert.Equal(t, []string{"Nebula quiz"}, got)
}

// Review fix (G096): the feed dates all-day events in the owner's zone.
func TestCalendarLane_FeedAllDayInOwnerZone(t *testing.T) {
	f := newCalFixture(t)
	require.NoError(t, f.g.Model(&models.User{}).Where("id = ?", calStudent).Update("time_zone", "Asia/Tokyo").Error)
	tokyo, err := time.LoadLocation("Asia/Tokyo")
	require.NoError(t, err)
	day := time.Now().In(tokyo).AddDate(0, 0, 3)
	midnight := time.Date(day.Year(), day.Month(), day.Day(), 0, 0, 0, 0, tokyo)
	require.NoError(t, f.g.Create(&models.CalendarEvent{ContextType: "User", ContextID: calStudent, Title: "Sports day", StartAt: midnight, AllDay: true, CreatedByUserID: calStudent, WorkflowState: "active"}).Error)

	app := f.app(calStudent, 1)
	resp := testutil.MakeRequest(app, http.MethodPost, "/calendar/feed", nil)
	require.Equal(t, http.StatusCreated, resp.StatusCode)
	var st map[string]interface{}
	calJSON(t, resp, &st)
	resp = testutil.MakeRequest(app, http.MethodGet, strings.TrimPrefix(st["feed_path"].(string), "/api/v1"), nil)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	body, _ := io.ReadAll(resp.Body)
	ics := string(body)
	assert.Contains(t, ics, "DTSTART;VALUE=DATE:"+midnight.Format("20060102"), "Tokyo midnight is the previous UTC day")
	assert.Contains(t, ics, "DTEND;VALUE=DATE:"+midnight.AddDate(0, 0, 1).Format("20060102"))
}
