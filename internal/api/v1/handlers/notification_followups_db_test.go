package handlers_test

// DB-backed tests for the notification follow-ups: scheduler reservations
// (Appointment Reserved / Canceled By User, Appointment Deleted For User),
// web conference invitations, observer alert notifications and emailed
// course invitations. The notifications run in goroutines, so the
// recorder is mutex-protected and assertions poll with eventually. All data
// is made up.

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/EduThemes/paper-lms/internal/api/v1/handlers"
	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/repository/postgres"
	"github.com/EduThemes/paper-lms/internal/service"
	"github.com/EduThemes/paper-lms/internal/testutil"
)

type followupNotification struct {
	UserID      uint
	Type        string
	Subject     string
	Body        string
	ContextType string
	ContextID   uint
}

// followupNotifier records queued notifications (written from goroutines).
type followupNotifier struct {
	mu    sync.Mutex
	calls []followupNotification
}

func (n *followupNotifier) QueueNotification(_ context.Context, userID uint, kind, subject, body, contextType string, contextID uint) error {
	n.mu.Lock()
	defer n.mu.Unlock()
	n.calls = append(n.calls, followupNotification{userID, kind, subject, body, contextType, contextID})
	return nil
}

func (n *followupNotifier) of(kind string) []followupNotification {
	n.mu.Lock()
	defer n.mu.Unlock()
	var out []followupNotification
	for _, c := range n.calls {
		if c.Type == kind {
			out = append(out, c)
		}
	}
	return out
}

func (n *followupNotifier) countFor(kind string, userID uint) int {
	c := 0
	for _, x := range n.of(kind) {
		if x.UserID == userID {
			c++
		}
	}
	return c
}

func (n *followupNotifier) total() int {
	n.mu.Lock()
	defer n.mu.Unlock()
	return len(n.calls)
}

// ---- Scheduler ----

func TestNotificationFollowups_AppointmentReservations(t *testing.T) {
	f := newCalFixture(t)
	rec := &followupNotifier{}
	f.appt.SetNotifier(rec)
	teacher := f.app(calTeacher, 1)
	student := f.app(calStudent, 1)
	slotAt := time.Now().UTC().Add(96 * time.Hour).Truncate(time.Hour)

	resp := testutil.MakeRequest(teacher, http.MethodPost, fmt.Sprintf("/courses/%d/appointment_groups", calCourse), testutil.JSONBody(map[string]any{
		"appointment_group": map[string]any{
			"title": "Office hours", "workflow_state": "active",
			"new_appointments": []map[string]any{
				{"start_at": slotAt, "end_at": slotAt.Add(15 * time.Minute)},
				{"start_at": slotAt.Add(time.Hour), "end_at": slotAt.Add(75 * time.Minute)},
			},
		},
	}))
	require.Equal(t, http.StatusCreated, resp.StatusCode)
	var og map[string]any
	calJSON(t, resp, &og)
	ogID := uint(og["id"].(float64))
	appts := og["appointments"].([]any)
	slot1 := uint(appts[0].(map[string]any)["id"].(float64))
	slot2 := uint(appts[1].(map[string]any)["id"].(float64))
	reservePath := func(slot uint) string {
		return fmt.Sprintf("/appointment_groups/%d/appointments/%d/reservations", ogID, slot)
	}
	reserve := func(app *fiber.App, slot uint) uint {
		t.Helper()
		resp := testutil.MakeRequest(app, http.MethodPost, reservePath(slot), testutil.JSONBody(map[string]any{"comments": "Q&A about the lab"}))
		require.Equal(t, http.StatusCreated, resp.StatusCode)
		var res map[string]any
		calJSON(t, resp, &res)
		return uint(res["id"].(float64))
	}

	// The student signs up: the teacher hears about it, in their own zone.
	res1 := reserve(student, slot1)
	eventually(t, func() bool { return rec.countFor(service.NotificationAppointmentReservedByUser, calTeacher) == 1 }, "teacher notified of the signup")
	got := rec.of(service.NotificationAppointmentReservedByUser)[0]
	assert.Equal(t, "Appointment reserved: Office hours", got.Subject)
	assert.Contains(t, got.Body, "Sam Student")
	assert.Contains(t, got.Body, slotAt.In(mustLoc(t, "America/New_York")).Format("Mon, Jan 2, 2006 3:04 PM MST"))
	assert.Contains(t, got.Body, "Q&amp;A about the lab", "the student's note is escaped")
	assert.Equal(t, "Course", got.ContextType)
	assert.Equal(t, calCourse, got.ContextID)
	assert.Zero(t, rec.countFor(service.NotificationAppointmentReservedByUser, calStudent), "never the booker")

	// The student cancels: the teacher is told.
	resp = testutil.MakeRequest(student, http.MethodDelete, fmt.Sprintf("%s/%d", reservePath(slot1), res1), nil)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	eventually(t, func() bool { return rec.countFor(service.NotificationAppointmentCanceledByUser, calTeacher) == 1 }, "teacher notified of the cancelation")
	// Canceling an already canceled reservation sends nothing more.
	resp = testutil.MakeRequest(student, http.MethodDelete, fmt.Sprintf("%s/%d", reservePath(slot1), res1), nil)
	require.Equal(t, http.StatusOK, resp.StatusCode)

	// The teacher cancels the student's booking: the student is told.
	res2 := reserve(student, slot1)
	eventually(t, func() bool { return rec.countFor(service.NotificationAppointmentReservedByUser, calTeacher) == 2 }, "second signup")
	resp = testutil.MakeRequest(teacher, http.MethodDelete, fmt.Sprintf("%s/%d", reservePath(slot1), res2), nil)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	eventually(t, func() bool { return rec.countFor(service.NotificationAppointmentDeletedForUser, calStudent) == 1 }, "student notified the teacher canceled")
	del := rec.of(service.NotificationAppointmentDeletedForUser)[0]
	assert.Contains(t, del.Body, "Terry Teacher")
	assert.Equal(t, 1, rec.countFor(service.NotificationAppointmentCanceledByUser, calTeacher), "the teacher is not told of their own cancelation")

	// The teacher deletes a booked slot: its holder is told.
	reserve(f.app(calClassmate, 1), slot2)
	eventually(t, func() bool { return rec.countFor(service.NotificationAppointmentReservedByUser, calTeacher) == 3 }, "classmate signup")
	resp = testutil.MakeRequest(teacher, http.MethodDelete, fmt.Sprintf("/appointment_groups/%d/appointments/%d", ogID, slot2), nil)
	require.Equal(t, http.StatusNoContent, resp.StatusCode)
	eventually(t, func() bool { return rec.countFor(service.NotificationAppointmentDeletedForUser, calClassmate) == 1 }, "slot holder notified")
	assert.Equal(t, 1, rec.countFor(service.NotificationAppointmentDeletedForUser, calStudent))

	// Group signup canceled by the teacher: every member of the team hears.
	resp = testutil.MakeRequest(teacher, http.MethodPost, fmt.Sprintf("/courses/%d/appointment_groups", calCourse), testutil.JSONBody(map[string]any{
		"appointment_group": map[string]any{
			"title": "Project check-in", "workflow_state": "active",
			"sub_context_codes": []string{fmt.Sprintf("group_category_%d", calGroupCat)},
			"new_appointments":  []map[string]any{{"start_at": slotAt.Add(2 * time.Hour), "end_at": slotAt.Add(150 * time.Minute)}},
		},
	}))
	require.Equal(t, http.StatusCreated, resp.StatusCode)
	var gg map[string]any
	calJSON(t, resp, &gg)
	ggID := uint(gg["id"].(float64))
	gslot := uint(gg["appointments"].([]any)[0].(map[string]any)["id"].(float64))
	resp = testutil.MakeRequest(student, http.MethodPost, fmt.Sprintf("/appointment_groups/%d/appointments/%d/reservations", ggID, gslot), nil)
	require.Equal(t, http.StatusCreated, resp.StatusCode)
	var gres map[string]any
	calJSON(t, resp, &gres)
	eventually(t, func() bool {
		for _, n := range rec.of(service.NotificationAppointmentReservedByUser) {
			if strings.Contains(n.Body, "Team Orion") {
				return true
			}
		}
		return false
	}, "a group signup names the team")
	resp = testutil.MakeRequest(teacher, http.MethodDelete, fmt.Sprintf("/appointment_groups/%d/appointments/%d/reservations/%d", ggID, gslot, uint(gres["id"].(float64))), nil)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	eventually(t, func() bool {
		return rec.countFor(service.NotificationAppointmentDeletedForUser, calStudent) == 2 &&
			rec.countFor(service.NotificationAppointmentDeletedForUser, calClassmate) == 2
	}, "both team members notified")
	assert.Zero(t, rec.countFor(service.NotificationAppointmentDeletedForUser, calStudent2), "not the other team")
}

func mustLoc(t *testing.T, name string) *time.Location {
	t.Helper()
	loc, err := time.LoadLocation(name)
	require.NoError(t, err)
	return loc
}

// ---- Conferences ----

func conferenceInviteApp(f *followupFixture, svc *service.ConferenceService, userID, accountID uint) *fiber.App {
	h := handlers.NewConferenceHandler(svc, f.authz)
	app := testutil.SetupTestApp()
	app.Use(callerStub(userID, accountID))
	app.Post("/courses/:course_id/conferences", h.CreateConference)
	app.Put("/conferences/:id", h.UpdateConference)
	app.Get("/conferences/:id/participants", h.GetParticipants)
	return app
}

func TestNotificationFollowups_ConferenceInvitations(t *testing.T) {
	f := newFollowupFixture(t)
	rec := &followupNotifier{}
	svc := service.NewConferenceService(postgres.NewConferenceRepository(f.g), postgres.NewConferenceParticipantRepository(f.g)).
		SetInvitations(postgres.NewEnrollmentRepository(f.g), postgres.NewCourseRepository(f.g), rec)
	teacher := conferenceInviteApp(f, svc, fxTeacherA, 1)

	participants := func(confID uint) map[uint]string {
		var rows []models.ConferenceParticipant
		require.NoError(t, f.g.Where("conference_id = ?", confID).Find(&rows).Error)
		out := map[uint]string{}
		for _, p := range rows {
			out[p.UserID] = p.ParticipationType
		}
		return out
	}

	// Specific invitees: only course members count (another tenant's
	// teacher is ignored), and the creator is the initiator.
	resp := testutil.MakeRequest(teacher, http.MethodPost, fmt.Sprintf("/courses/%d/conferences", fxCourseA), testutil.JSONBody(map[string]any{
		"conference": map[string]any{"title": "Review <session>", "conference_type": "BigBlueButton", "description": "Bring questions",
			"join_url": "https://meet.example.test/review", "user_ids": []uint{fxStudent, fxTeacherB}},
	}))
	require.Equal(t, http.StatusCreated, resp.StatusCode)
	var conf map[string]any
	require.NoError(t, testutil.ParseJSONResponse(resp, &conf))
	confID := uint(conf["id"].(float64))
	eventually(t, func() bool { return rec.countFor(service.NotificationWebConferenceInvitation, fxStudent) == 1 }, "invitee notified")
	n := rec.of(service.NotificationWebConferenceInvitation)[0]
	assert.Equal(t, "Web conference invitation: Review <session>", n.Subject)
	assert.Contains(t, n.Body, "Review &lt;session&gt;")
	assert.Contains(t, n.Body, "Biology")
	assert.Equal(t, "Course", n.ContextType)
	assert.Equal(t, fxCourseA, n.ContextID)
	assert.Equal(t, map[uint]string{fxTeacherA: "initiator", fxStudent: "invitee"}, participants(confID))
	assert.Zero(t, rec.countFor(service.NotificationWebConferenceInvitation, fxTeacherB), "never another tenant's user")

	// Editing with invite_all adds the rest of the course; only the newly
	// invited are notified.
	resp = testutil.MakeRequest(teacher, http.MethodPut, fmt.Sprintf("/conferences/%d", confID), testutil.JSONBody(map[string]any{
		"conference": map[string]any{"invite_all": true},
	}))
	require.Equal(t, http.StatusOK, resp.StatusCode)
	eventually(t, func() bool { return rec.countFor(service.NotificationWebConferenceInvitation, fxClassmate) == 1 }, "classmate invited")
	assert.Equal(t, 1, rec.countFor(service.NotificationWebConferenceInvitation, fxStudent), "already invited: not notified again")
	assert.Zero(t, rec.countFor(service.NotificationWebConferenceInvitation, fxTeacherA), "never the inviter")
	assert.Equal(t, map[uint]string{fxTeacherA: "initiator", fxStudent: "invitee", fxClassmate: "invitee"}, participants(confID))

	// No invitees: nothing is sent.
	before := rec.total()
	resp = testutil.MakeRequest(teacher, http.MethodPost, fmt.Sprintf("/courses/%d/conferences", fxCourseA), testutil.JSONBody(map[string]any{
		"conference": map[string]any{"title": "Solo", "conference_type": "BigBlueButton", "join_url": "https://meet.example.test/solo"},
	}))
	require.Equal(t, http.StatusCreated, resp.StatusCode)
	time.Sleep(100 * time.Millisecond)
	assert.Equal(t, before, rec.total())
}

// ---- Observer alerts ----

func TestNotificationFollowups_ObserverAlertNotifiesOnGrade(t *testing.T) {
	f := newLearnerFixture(t)
	rec := &followupNotifier{}
	f.alert.SetNotifier(rec)
	resp, data := f.do("POST", "/observer_alert_thresholds", lnParent, 1, map[string]any{
		"threshold": map[string]any{"user_id": lnStudent, "alert_type": "assignment_grade_low", "threshold": 70},
	}, "")
	require.Equal(t, 201, resp.StatusCode, string(data))

	time.Sleep(10 * time.Millisecond)
	quiz := f.assignment(lnCourse, "Cell quiz", nil, 10, nil)
	sub := f.grade(quiz.ID, lnStudent, 4, true)
	// The OnGraded callback evaluates the student's grade thresholds at once.
	f.alert.GradedCallback(postgres.NewSubmissionRepository(f.g))(context.Background(), sub.ID)

	eventually(t, func() bool { return rec.countFor(service.NotificationObserverAlert, lnParent) == 1 }, "parent notified")
	n := rec.of(service.NotificationObserverAlert)[0]
	assert.Contains(t, n.Subject, "Assignment grade low")
	assert.Contains(t, n.Body, "Jane Doe")
	assert.Equal(t, "Course", n.ContextType)
	assert.Equal(t, lnCourse, n.ContextID)

	// The alert is in the parent's list, and rescans don't notify again.
	_, data = f.do("GET", "/users/self/observer_alerts", lnParent, 1, nil, "")
	require.Len(t, decodeList(t, data), 1)
	require.NoError(t, f.alert.EvaluateAll(context.Background()))
	f.alert.GradedCallback(postgres.NewSubmissionRepository(f.g))(context.Background(), sub.ID)
	time.Sleep(100 * time.Millisecond)
	assert.Equal(t, 1, rec.countFor(service.NotificationObserverAlert, lnParent))
	assert.Zero(t, rec.countFor(service.NotificationObserverAlert, lnStudent), "only the observer")

	// A hidden (unposted) grade never alerts.
	hidden := f.assignment(lnCourse, "Hidden quiz", nil, 10, nil)
	hsub := f.grade(hidden.ID, lnStudent, 1, false)
	f.alert.GradedCallback(postgres.NewSubmissionRepository(f.g))(context.Background(), hsub.ID)
	time.Sleep(100 * time.Millisecond)
	assert.Equal(t, 1, rec.countFor(service.NotificationObserverAlert, lnParent))
}

// ---- Enrollment invitations ----

func TestNotificationFollowups_EnrollmentInvitationIsEmailed(t *testing.T) {
	f := newAEFixture(t)
	users := postgres.NewUserRepository(f.g)
	delivery := service.NewNotificationDeliveryService(postgres.NewNotificationDeliveryRepository(f.g), postgres.NewCommunicationChannelRepository(f.g),
		postgres.NewNotificationPreferenceRepository(f.g), postgres.NewNotificationRepository(f.g), users, nil)
	delivery.SetPolicyStore(postgres.NewNotificationPolicyRepository(f.g))
	f.notifier = delivery
	// Bea turned invitation email off; she still gets the in-app notice.
	require.NoError(t, f.g.Create(&models.NotificationPolicy{UserID: aeStudentB, CommunicationChannelID: models.PrimaryEmailChannelID,
		Category: "invitation", Frequency: models.NotificationFrequencyNever}).Error)

	teacher := f.app(aeTeacher, 1)
	code, body := aeDo(t, teacher, http.MethodPost, fmt.Sprintf("/api/v1/courses/%d/enrollments", aeCourse), map[string]any{
		"enrollment": map[string]any{"user_id": aeOutsider, "type": "StudentEnrollment", "enrollment_state": "invited"},
	})
	require.Equal(t, http.StatusCreated, code, body)
	code, body = aeDo(t, teacher, http.MethodPost, fmt.Sprintf("/api/v1/courses/%d/enrollments", aeCourse2), map[string]any{
		"enrollment": map[string]any{"user_id": aeStudentB, "type": "StudentEnrollment", "enrollment_state": "invited"},
	})
	require.Equal(t, http.StatusCreated, code, body)

	countDeliveries := func(userID uint) int64 {
		var n int64
		f.g.Model(&models.NotificationDelivery{}).Where("user_id = ? AND subject LIKE ?", userID, "Course invitation:%").Count(&n)
		return n
	}
	countInApp := func(userID uint) int64 {
		var n int64
		f.g.Model(&models.Notification{}).Where("user_id = ? AND notification_type = ?", userID, service.NotificationEnrollmentInvitation).Count(&n)
		return n
	}
	eventually(t, func() bool { return countDeliveries(aeOutsider) == 1 && countInApp(aeStudentB) == 1 }, "invitation queued")
	var d models.NotificationDelivery
	require.NoError(t, f.g.Where("user_id = ?", aeOutsider).First(&d).Error)
	assert.Equal(t, "email", d.ChannelType)
	assert.Equal(t, "olly.ae@example.test", d.Address)
	assert.Equal(t, "Course invitation: Biology", d.Subject)
	assert.Contains(t, d.Body, "<strong>Biology</strong>")
	assert.EqualValues(t, 1, countInApp(aeOutsider))
	assert.Zero(t, countDeliveries(aeStudentB), "the invitee's preference is honored")

	// An active (not invited) enrollment sends no invitation.
	code, body = aeDo(t, teacher, http.MethodPost, fmt.Sprintf("/api/v1/courses/%d/enrollments", aeCourse2), map[string]any{
		"enrollment": map[string]any{"user_id": aeOutsider, "type": "StudentEnrollment"},
	})
	require.Equal(t, http.StatusCreated, code, body)
	time.Sleep(100 * time.Millisecond)
	assert.EqualValues(t, 1, countInApp(aeOutsider))
}
