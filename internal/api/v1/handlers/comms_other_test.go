package handlers_test

// Regression tests for the comms-other lane: announcements, calendar,
// collaborations, conferences, groups, appointment groups, inbox and
// notification channels. Example data only.

import (
	"context"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"github.com/EduThemes/paper-lms/internal/api/v1/handlers"
	"github.com/EduThemes/paper-lms/internal/api/v1/middleware"
	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/repository"
	"github.com/EduThemes/paper-lms/internal/service"
	"github.com/EduThemes/paper-lms/internal/testutil"
	"github.com/EduThemes/paper-lms/internal/testutil/mocks"
)

const (
	commsTeacherID uint = 40
	commsStudentID uint = 50
	commsOtherID   uint = 60
	commsCourseID  uint = 5
)

// commsApp mounts the auth Locals a Protected route would set.
func commsApp(callerID, accountID uint) *fiber.App {
	app := testutil.SetupTestApp()
	app.Use(func(c *fiber.Ctx) error {
		c.Locals("user_id", callerID)
		c.Locals("account_id", accountID)
		return c.Next()
	}, middleware.PaginationParams())
	return app
}

// commsRoles wires the user + enrollment mocks: teacher 40 teaches course
// 5, student 50 is a student in it, user 60 is a student elsewhere. None
// of them is an admin.
func commsRoles() (*mocks.MockUserRepository, *mocks.MockEnrollmentRepository) {
	userRepo := new(mocks.MockUserRepository)
	enrollRepo := new(mocks.MockEnrollmentRepository)
	for _, id := range []uint{commsTeacherID, commsStudentID, commsOtherID} {
		userRepo.On("FindByID", mock.Anything, id, mock.Anything).Return(&models.User{ID: id, Role: "user", Name: "Example User"}, nil)
	}
	teacher := models.Enrollment{UserID: commsTeacherID, CourseID: commsCourseID, Type: "TeacherEnrollment", WorkflowState: "active"}
	student := models.Enrollment{UserID: commsStudentID, CourseID: commsCourseID, Type: "StudentEnrollment", WorkflowState: "active"}
	enrollRepo.On("FindByUserAndCourse", mock.Anything, commsTeacherID, commsCourseID, mock.Anything).Return(&teacher, nil)
	enrollRepo.On("FindByUserAndCourse", mock.Anything, commsStudentID, commsCourseID, mock.Anything).Return(&student, nil)
	enrollRepo.On("FindByUserAndCourse", mock.Anything, commsOtherID, commsCourseID, mock.Anything).Return(nil, errFakeNotFound)
	enrollRepo.On("ListByUserID", mock.Anything, commsTeacherID, mock.Anything).Return([]models.Enrollment{teacher}, nil)
	enrollRepo.On("ListByUserID", mock.Anything, commsStudentID, mock.Anything).Return([]models.Enrollment{student}, nil)
	enrollRepo.On("ListByUserID", mock.Anything, commsOtherID, mock.Anything).Return([]models.Enrollment{}, nil)
	enrollRepo.On("ListByCourseID", mock.Anything, commsCourseID, mock.Anything, mock.Anything).Return(&repository.PaginatedResult[models.Enrollment]{Items: []models.Enrollment{teacher, student}, TotalCount: 2}, nil)
	return userRepo, enrollRepo
}

// ===========================================================================
// Announcements (B054, B150, B152)
// ===========================================================================

func announcementFixture(callerID, accountID uint, a *models.Announcement) (*fiber.App, *fakeAnnouncementRepo) {
	annRepo := newFakeAnnouncementRepo()
	annRepo.byID[a.ID] = a
	userRepo, enrollRepo := commsRoles()
	svc := service.NewAnnouncementService(annRepo, newFakeReceiptRepo(), enrollRepo)
	h := handlers.NewAnnouncementHandler(svc, handlers.NewResourceAuthorizer(enrollRepo, userRepo), nil)
	app := commsApp(callerID, accountID)
	app.Get("/announcements/:id", h.GetAnnouncement)
	app.Put("/announcements/:id", h.UpdateAnnouncement)
	app.Delete("/announcements/:id", h.DeleteAnnouncement)
	app.Get("/announcements/:id/read_receipts", h.GetReadReceipts)
	app.Get("/accounts/:account_id/announcements", h.ListAccountAnnouncements)
	return app, annRepo
}

func accountAnnouncement(accountID uint) *models.Announcement {
	return &models.Announcement{ID: 10, AccountID: &accountID, Title: "Campus closed", Message: "m", WorkflowState: "published"}
}

func TestAccountAnnouncement_StudentCannotUpdateOrDelete(t *testing.T) {
	app, repo := announcementFixture(commsStudentID, 1, accountAnnouncement(1))

	resp := testutil.MakeRequest(app, http.MethodPut, "/announcements/10", testutil.JSONBody(map[string]any{"title": "hijacked"}))
	assert.Equal(t, http.StatusForbidden, resp.StatusCode)
	assert.Equal(t, "Campus closed", repo.byID[10].Title, "a student must not rewrite an account announcement")

	resp = testutil.MakeRequest(app, http.MethodDelete, "/announcements/10", nil)
	assert.Equal(t, http.StatusForbidden, resp.StatusCode)
	assert.Contains(t, repo.byID, uint(10), "a student must not delete an account announcement")
}

func TestAccountAnnouncement_CrossTenantIs404(t *testing.T) {
	// Announcement belongs to tenant 2; caller is in tenant 1.
	app, repo := announcementFixture(commsStudentID, 1, accountAnnouncement(2))

	for _, method := range []string{http.MethodGet, http.MethodPut, http.MethodDelete} {
		resp := testutil.MakeRequest(app, method, "/announcements/10", testutil.JSONBody(map[string]any{"title": "x"}))
		assert.Equal(t, http.StatusNotFound, resp.StatusCode, method)
	}
	assert.Equal(t, "Campus closed", repo.byID[10].Title)
}

func TestListAccountAnnouncements_OtherTenantIs404(t *testing.T) {
	app, _ := announcementFixture(commsStudentID, 1, accountAnnouncement(1))
	resp := testutil.MakeRequest(app, http.MethodGet, "/accounts/2/announcements", nil)
	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
	resp = testutil.MakeRequest(app, http.MethodGet, "/accounts/1/announcements", nil)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
}

func courseAnnouncement(state string) *models.Announcement {
	cid := commsCourseID
	a := &models.Announcement{ID: 10, CourseID: &cid, UserID: commsTeacherID, Title: "Quiz moved", Message: "m", TargetAudience: "all", WorkflowState: state}
	if state == "scheduled" {
		future := time.Now().Add(48 * time.Hour)
		a.DelayedPostAt = &future
	}
	if state == "published" {
		past := time.Now().Add(-time.Hour)
		a.PostedAt = &past
	}
	return a
}

func TestGetAnnouncement_StudentCannotReadDraftOrScheduled(t *testing.T) {
	for _, state := range []string{"draft", "scheduled"} {
		app, _ := announcementFixture(commsStudentID, 1, courseAnnouncement(state))
		resp := testutil.MakeRequest(app, http.MethodGet, "/announcements/10", nil)
		assert.Equal(t, http.StatusNotFound, resp.StatusCode, "student reading a %s announcement", state)

		app, _ = announcementFixture(commsTeacherID, 1, courseAnnouncement(state))
		resp = testutil.MakeRequest(app, http.MethodGet, "/announcements/10", nil)
		assert.Equal(t, http.StatusOK, resp.StatusCode, "teacher reading a %s announcement", state)
	}

	app, _ := announcementFixture(commsStudentID, 1, courseAnnouncement("published"))
	resp := testutil.MakeRequest(app, http.MethodGet, "/announcements/10", nil)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
}

func TestGetAnnouncement_StudentCannotReadTeachersOnly(t *testing.T) {
	a := courseAnnouncement("published")
	a.TargetAudience = "teachers"
	app, _ := announcementFixture(commsStudentID, 1, a)
	resp := testutil.MakeRequest(app, http.MethodGet, "/announcements/10", nil)
	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
}

// B150: the receipts route has no :course_id, so the handler authorizes
// against the announcement's course: the course teacher gets 200, a
// student is refused.
func TestGetReadReceipts_CourseTeacherAllowed_StudentRefused(t *testing.T) {
	app, _ := announcementFixture(commsTeacherID, 1, courseAnnouncement("published"))
	resp := testutil.MakeRequest(app, http.MethodGet, "/announcements/10/read_receipts", nil)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	app, _ = announcementFixture(commsStudentID, 1, courseAnnouncement("published"))
	resp = testutil.MakeRequest(app, http.MethodGet, "/announcements/10/read_receipts", nil)
	assert.Equal(t, http.StatusForbidden, resp.StatusCode)
}

// ===========================================================================
// Calendar events (B060, B164)
// ===========================================================================

func calendarFixture(callerID uint) (*fiber.App, *mocks.MockCalendarEventRepository) {
	calRepo := new(mocks.MockCalendarEventRepository)
	userRepo, enrollRepo := commsRoles()
	h := handlers.NewCalendarEventHandler(service.NewCalendarService(calRepo), handlers.NewResourceAuthorizer(enrollRepo, userRepo))
	app := commsApp(callerID, 1)
	app.Post("/calendar_events", h.CreateEvent)
	app.Get("/calendar_events/:id", h.GetEvent)
	return app, calRepo
}

func calendarBody(contextType string, contextID uint) map[string]any {
	return map[string]any{"calendar_event": map[string]any{
		"context_type": contextType, "context_id": contextID,
		"title": "Review session", "start_at": time.Now().Add(24 * time.Hour).Format(time.RFC3339),
	}}
}

func TestCreateCalendarEvent_StudentCannotPostToCourseCalendar(t *testing.T) {
	app, calRepo := calendarFixture(commsStudentID)
	resp := testutil.MakeRequest(app, http.MethodPost, "/calendar_events", testutil.JSONBody(calendarBody("Course", commsCourseID)))
	assert.Equal(t, http.StatusForbidden, resp.StatusCode)
	calRepo.AssertNotCalled(t, "Create", mock.Anything, mock.Anything)
}

func TestCreateCalendarEvent_CannotPostToAnotherUsersCalendar(t *testing.T) {
	app, calRepo := calendarFixture(commsStudentID)
	resp := testutil.MakeRequest(app, http.MethodPost, "/calendar_events", testutil.JSONBody(calendarBody("User", commsTeacherID)))
	assert.Equal(t, http.StatusForbidden, resp.StatusCode)
	calRepo.AssertNotCalled(t, "Create", mock.Anything, mock.Anything)
}

func TestCreateCalendarEvent_OtherTenantAccountIs404(t *testing.T) {
	app, calRepo := calendarFixture(commsTeacherID)
	resp := testutil.MakeRequest(app, http.MethodPost, "/calendar_events", testutil.JSONBody(calendarBody("Account", 2)))
	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
	calRepo.AssertNotCalled(t, "Create", mock.Anything, mock.Anything)
}

func TestCreateCalendarEvent_TeacherAndOwnCalendarAllowed(t *testing.T) {
	app, calRepo := calendarFixture(commsTeacherID)
	calRepo.On("Create", mock.Anything, mock.AnythingOfType("*models.CalendarEvent")).Return(nil)
	resp := testutil.MakeRequest(app, http.MethodPost, "/calendar_events", testutil.JSONBody(calendarBody("Course", commsCourseID)))
	assert.Equal(t, http.StatusCreated, resp.StatusCode)

	app, calRepo = calendarFixture(commsStudentID)
	calRepo.On("Create", mock.Anything, mock.AnythingOfType("*models.CalendarEvent")).Return(nil)
	resp = testutil.MakeRequest(app, http.MethodPost, "/calendar_events", testutil.JSONBody(calendarBody("User", commsStudentID)))
	assert.Equal(t, http.StatusCreated, resp.StatusCode)
}

func TestGetCalendarEvent_PersonalEventIsOwnerOnly(t *testing.T) {
	event := &models.CalendarEvent{ID: 77, ContextType: "User", ContextID: commsTeacherID, CreatedByUserID: commsTeacherID, Title: "Dentist"}

	app, calRepo := calendarFixture(commsStudentID)
	calRepo.On("FindByID", mock.Anything, uint(77), uint(1)).Return(event, nil)
	resp := testutil.MakeRequest(app, http.MethodGet, "/calendar_events/77", nil)
	assert.Equal(t, http.StatusNotFound, resp.StatusCode, "another user's personal event must look absent")

	app, calRepo = calendarFixture(commsTeacherID)
	calRepo.On("FindByID", mock.Anything, uint(77), uint(1)).Return(event, nil)
	resp = testutil.MakeRequest(app, http.MethodGet, "/calendar_events/77", nil)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
}

// ===========================================================================
// Collaborations (B154)
// ===========================================================================

func TestCollaboration_JavascriptURLRejected(t *testing.T) {
	collabRepo := new(mocks.MockCollaborationRepository)
	userRepo, enrollRepo := commsRoles()
	h := handlers.NewCollaborationHandler(service.NewCollaborationService(collabRepo), handlers.NewResourceAuthorizer(enrollRepo, userRepo))
	app := commsApp(commsTeacherID, 1)
	app.Post("/courses/:course_id/collaborations", h.CreateCollaboration)
	app.Put("/collaborations/:id", h.UpdateCollaboration)

	resp := testutil.MakeRequest(app, http.MethodPost, "/courses/5/collaborations", testutil.JSONBody(map[string]any{
		"collaboration": map[string]any{"title": "Notes", "collaboration_type": "google_docs", "url": "javascript:fetch('/api/v1/users')"},
	}))
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	collabRepo.AssertNotCalled(t, "Create", mock.Anything, mock.Anything)

	collabRepo.On("FindByID", mock.Anything, uint(3), uint(1)).Return(&models.Collaboration{ID: 3, ContextType: "Course", ContextID: commsCourseID, Title: "Notes", URL: "https://docs.example.com/d/1"}, nil)
	resp = testutil.MakeRequest(app, http.MethodPut, "/collaborations/3", testutil.JSONBody(map[string]any{
		"collaboration": map[string]any{"url": "JavaScript:alert(1)"},
	}))
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	collabRepo.AssertNotCalled(t, "Update", mock.Anything, mock.Anything)
}

// ===========================================================================
// Conferences (B058)
// ===========================================================================

func conferenceFixture(callerID uint, conf *models.Conference) (*fiber.App, *mocks.MockConferenceRepository, *mocks.MockConferenceParticipantRepository) {
	confRepo := new(mocks.MockConferenceRepository)
	partRepo := new(mocks.MockConferenceParticipantRepository)
	confRepo.On("FindByID", mock.Anything, conf.ID, mock.Anything).Return(conf, nil)
	userRepo, enrollRepo := commsRoles()
	h := handlers.NewConferenceHandler(service.NewConferenceService(confRepo, partRepo), handlers.NewResourceAuthorizer(enrollRepo, userRepo))
	app := commsApp(callerID, 1)
	app.Post("/conferences/:id/start", h.StartConference)
	app.Post("/conferences/:id/join", h.JoinConference)
	return app, confRepo, partRepo
}

func TestStartConference_StudentRefused(t *testing.T) {
	conf := &models.Conference{ID: 9, ContextType: "Course", ContextID: commsCourseID, ConferenceType: "Zoom", Title: "Office hours", JoinURL: "https://zoom.example.com/j/123"}
	app, confRepo, _ := conferenceFixture(commsStudentID, conf)
	resp := testutil.MakeRequest(app, http.MethodPost, "/conferences/9/start", nil)
	assert.Equal(t, http.StatusForbidden, resp.StatusCode)
	confRepo.AssertNotCalled(t, "Update", mock.Anything, mock.Anything)
}

func TestStartConference_NoProviderIsConflict(t *testing.T) {
	conf := &models.Conference{ID: 9, ContextType: "Course", ContextID: commsCourseID, ConferenceType: "BigBlueButton", Title: "Office hours"}
	app, confRepo, _ := conferenceFixture(commsTeacherID, conf)
	resp := testutil.MakeRequest(app, http.MethodPost, "/conferences/9/start", nil)
	assert.Equal(t, http.StatusConflict, resp.StatusCode)
	confRepo.AssertNotCalled(t, "Update", mock.Anything, mock.Anything)
}

func TestConference_StartThenJoinWithMeetingLink(t *testing.T) {
	conf := &models.Conference{ID: 9, ContextType: "Course", ContextID: commsCourseID, ConferenceType: "Zoom", Title: "Office hours", JoinURL: "https://zoom.example.com/j/123"}

	// A student cannot join before the teacher starts it.
	app, _, _ := conferenceFixture(commsStudentID, conf)
	resp := testutil.MakeRequest(app, http.MethodPost, "/conferences/9/join", nil)
	assert.Equal(t, http.StatusConflict, resp.StatusCode)

	// The teacher starts it.
	app, confRepo, _ := conferenceFixture(commsTeacherID, conf)
	confRepo.On("Update", mock.Anything, mock.AnythingOfType("*models.Conference")).Return(nil)
	resp = testutil.MakeRequest(app, http.MethodPost, "/conferences/9/start", nil)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.NotNil(t, conf.StartedAt)

	// Now the student gets the real meeting link, not a placeholder.
	app, _, partRepo := conferenceFixture(commsStudentID, conf)
	partRepo.On("FindByConferenceAndUser", mock.Anything, uint(9), commsStudentID, mock.Anything).Return(nil, errFakeNotFound)
	partRepo.On("Create", mock.Anything, mock.AnythingOfType("*models.ConferenceParticipant")).Return(nil)
	resp = testutil.MakeRequest(app, http.MethodPost, "/conferences/9/join", nil)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	body, _ := testutil.ParseJSONMap(resp)
	assert.Equal(t, "https://zoom.example.com/j/123", body["join_url"])
	assert.NotContains(t, body["join_url"], "placeholder")
}

// ===========================================================================
// Groups (B163)
// ===========================================================================

func groupFixture(callerID uint, selfSignup string) (*fiber.App, *mocks.MockGroupMembershipRepository) {
	catRepo := new(mocks.MockGroupCategoryRepository)
	groupRepo := new(mocks.MockGroupRepository)
	memRepo := new(mocks.MockGroupMembershipRepository)
	userRepo, enrollRepo := commsRoles()
	cid := commsCourseID
	group := &models.Group{ID: 21, GroupCategoryID: 7, Name: "Team A", ContextType: "Course", ContextID: commsCourseID}
	cat := &models.GroupCategory{ID: 7, CourseID: &cid, Name: "Project teams", SelfSignup: selfSignup}
	groupRepo.On("FindByID", mock.Anything, uint(21), mock.Anything).Return(group, nil)
	catRepo.On("FindByID", mock.Anything, uint(7), mock.Anything).Return(cat, nil)
	memRepo.On("FindByGroupAndUser", mock.Anything, uint(21), mock.Anything).Return(nil, errFakeNotFound)
	memRepo.On("ListByGroupID", mock.Anything, uint(21), mock.Anything).Return(&repository.PaginatedResult[models.GroupMembership]{}, nil)
	memRepo.On("FindUserGroupInCategory", mock.Anything, mock.Anything, uint(7)).Return(nil, errFakeNotFound)
	h := handlers.NewGroupHandler(service.NewGroupService(catRepo, groupRepo, memRepo, enrollRepo), handlers.NewResourceAuthorizer(enrollRepo, userRepo), nil)
	app := commsApp(callerID, 1)
	app.Post("/groups/:group_id/memberships", h.CreateGroupMembership)
	return app, memRepo
}

func TestGroupSelfSignup_EnabledAllowsStudentToJoin(t *testing.T) {
	app, memRepo := groupFixture(commsStudentID, "enabled")
	memRepo.On("Create", mock.Anything, mock.AnythingOfType("*models.GroupMembership")).Return(nil)
	resp := testutil.MakeRequest(app, http.MethodPost, "/groups/21/memberships", testutil.JSONBody(map[string]any{"membership": map[string]any{}}))
	assert.Equal(t, http.StatusCreated, resp.StatusCode)
	memRepo.AssertCalled(t, "Create", mock.Anything, mock.MatchedBy(func(m *models.GroupMembership) bool {
		return m.UserID == commsStudentID && !m.Moderator
	}))
}

func TestGroupSelfSignup_DisabledRefused(t *testing.T) {
	app, memRepo := groupFixture(commsStudentID, "")
	resp := testutil.MakeRequest(app, http.MethodPost, "/groups/21/memberships", testutil.JSONBody(map[string]any{"membership": map[string]any{}}))
	assert.Equal(t, http.StatusForbidden, resp.StatusCode)
	memRepo.AssertNotCalled(t, "Create", mock.Anything, mock.Anything)
}

func TestGroupSelfSignup_StudentCannotAddOthers(t *testing.T) {
	app, memRepo := groupFixture(commsStudentID, "enabled")
	resp := testutil.MakeRequest(app, http.MethodPost, "/groups/21/memberships", testutil.JSONBody(map[string]any{"membership": map[string]any{"user_id": commsOtherID}}))
	assert.Equal(t, http.StatusForbidden, resp.StatusCode)
	memRepo.AssertNotCalled(t, "Create", mock.Anything, mock.Anything)
}

// ===========================================================================
// Appointment groups (B166, B167)
// ===========================================================================

func TestAppointmentGroupUpdate_PartialKeepsOtherFields(t *testing.T) {
	groupRepo := new(mocks.MockAppointmentGroupRepository)
	userRepo, enrollRepo := commsRoles()
	g := &models.AppointmentGroup{ID: 3, CourseID: commsCourseID, Title: "Office hours", Description: "<p>Bring questions</p>", LocationName: "Room 101", LocationAddress: "1000 Main Street", MinAppointmentsPerParticipant: 1, MaxAppointmentsPerParticipant: 2, ParticipantsPerAppointment: 1, WorkflowState: "active"}
	groupRepo.On("FindByID", mock.Anything, uint(3), uint(1)).Return(g, nil)
	groupRepo.On("Update", mock.Anything, mock.AnythingOfType("*models.AppointmentGroup")).Return(nil)
	svc := service.NewAppointmentGroupService(groupRepo, new(mocks.MockAppointmentSlotRepository), new(mocks.MockAppointmentReservationRepository), nil)
	h := handlers.NewAppointmentGroupHandler(svc, handlers.NewResourceAuthorizer(enrollRepo, userRepo), nil)
	app := commsApp(commsTeacherID, 1)
	app.Put("/appointment_groups/:id", h.Update)

	resp := testutil.MakeRequest(app, http.MethodPut, "/appointment_groups/3", testutil.JSONBody(map[string]any{
		"appointment_group": map[string]any{"title": "Extended office hours"},
	}))
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, "Extended office hours", g.Title)
	assert.Equal(t, "<p>Bring questions</p>", g.Description)
	assert.Equal(t, "Room 101", g.LocationName)
	assert.Equal(t, "1000 Main Street", g.LocationAddress)
	assert.Equal(t, 1, g.MinAppointmentsPerParticipant)
}

func TestListMyAppointmentReservations(t *testing.T) {
	resRepo := new(mocks.MockAppointmentReservationRepository)
	resRepo.On("ListByUser", mock.Anything, commsStudentID).Return([]models.AppointmentReservation{
		{ID: 8, SlotID: 30, GroupID: 3, UserID: commsStudentID, WorkflowState: "reserved"},
	}, nil)
	svc := service.NewAppointmentGroupService(new(mocks.MockAppointmentGroupRepository), new(mocks.MockAppointmentSlotRepository), resRepo, nil)
	userRepo, enrollRepo := commsRoles()
	h := handlers.NewAppointmentGroupHandler(svc, handlers.NewResourceAuthorizer(enrollRepo, userRepo), nil)
	app := commsApp(commsStudentID, 1)
	app.Get("/users/self/appointments", h.ListMyReservations)

	resp := testutil.MakeRequest(app, http.MethodGet, "/users/self/appointments", nil)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	items, _ := testutil.ParseJSONArray(resp)
	if assert.Len(t, items, 1) {
		assert.EqualValues(t, 30, items[0]["slot_id"])
		assert.EqualValues(t, 8, items[0]["id"])
	}
}

// ===========================================================================
// Inbox (B059) and new-message notifications (B055)
// ===========================================================================

type fakeRecipientSearch struct {
	callerID, accountID uint
	term                string
	allInAccount        bool
	users               []models.User
}

func (f *fakeRecipientSearch) SearchRecipients(_ context.Context, callerID, accountID uint, term string, allInAccount bool, _ int) ([]models.User, error) {
	f.callerID, f.accountID, f.term, f.allInAccount = callerID, accountID, term, allInAccount
	return f.users, nil
}

func TestSearchRecipients_StudentGetsCourseScopedSearch(t *testing.T) {
	userRepo, enrollRepo := commsRoles()
	h := handlers.NewConversationHandler(service.NewConversationService(nil, nil, nil), service.NewUserService(userRepo), nil, enrollRepo, nil)
	search := &fakeRecipientSearch{users: []models.User{{ID: commsTeacherID, Name: "Pat Teacher", Email: "pat@example.com"}}}
	h.SetRecipientSearch(search)
	app := commsApp(commsStudentID, 1)
	app.Get("/search/recipients", h.SearchRecipients)

	resp := testutil.MakeRequest(app, http.MethodGet, "/search/recipients?search=pat", nil)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	items, _ := testutil.ParseJSONArray(resp)
	if assert.Len(t, items, 1) {
		assert.Equal(t, "Pat Teacher", items[0]["name"])
		_, hasEmail := items[0]["email"]
		assert.False(t, hasEmail, "recipient search must not expose emails")
	}
	assert.Equal(t, commsStudentID, search.callerID)
	assert.Equal(t, uint(1), search.accountID, "search is scoped to the caller's tenant")
	assert.False(t, search.allInAccount, "a non-admin only searches people who share a course")
}

type recordingNotifier struct {
	calls []struct {
		userID uint
		kind   string
	}
}

func (r *recordingNotifier) QueueNotification(_ context.Context, userID uint, kind, _, _, _ string, _ uint) error {
	r.calls = append(r.calls, struct {
		userID uint
		kind   string
	}{userID, kind})
	return nil
}

func TestCreateMessage_NotifiesOtherParticipants(t *testing.T) {
	convRepo := new(mocks.MockConversationRepository)
	partRepo := new(mocks.MockConversationParticipantRepository)
	msgRepo := new(mocks.MockConversationMessageRepository)
	userRepo, enrollRepo := commsRoles()
	participants := []models.ConversationParticipant{{ConversationID: 4, UserID: commsStudentID}, {ConversationID: 4, UserID: commsTeacherID}}
	partRepo.On("ListByConversationID", mock.Anything, uint(4)).Return(participants, nil)
	convRepo.On("FindByID", mock.Anything, uint(4), mock.Anything).Return(&models.Conversation{ID: 4, Subject: "Question"}, nil)
	convRepo.On("Update", mock.Anything, mock.Anything).Return(nil).Maybe()
	msgRepo.On("Create", mock.Anything, mock.AnythingOfType("*models.ConversationMessage")).Return(nil)
	partRepo.On("Update", mock.Anything, mock.Anything).Return(nil).Maybe()

	h := handlers.NewConversationHandler(service.NewConversationService(convRepo, partRepo, msgRepo), service.NewUserService(userRepo), nil, enrollRepo, nil)
	notifier := &recordingNotifier{}
	h.SetNotifier(notifier)
	app := commsApp(commsStudentID, 1)
	app.Post("/conversations/:id/messages", h.CreateMessage)

	resp := testutil.MakeRequest(app, http.MethodPost, "/conversations/4/messages", strings.NewReader(`{"message":"When is the quiz?"}`))
	if !assert.Equal(t, http.StatusCreated, resp.StatusCode) {
		return
	}
	if assert.Len(t, notifier.calls, 1) {
		assert.Equal(t, commsTeacherID, notifier.calls[0].userID, "only the other participant is notified")
		assert.Equal(t, "new_message", notifier.calls[0].kind)
	}
}

// ===========================================================================
// Notification channels (B168)
// ===========================================================================

func TestCreateCommunicationChannel_PushRejected(t *testing.T) {
	h := handlers.NewNotificationDeliveryHandler(service.NewNotificationDeliveryService(nil, nil, nil, nil, nil, nil), nil)
	app := commsApp(commsStudentID, 1)
	app.Post("/users/self/communication_channels", h.CreateChannel)
	resp := testutil.MakeRequest(app, http.MethodPost, "/users/self/communication_channels", testutil.JSONBody(map[string]any{
		"communication_channel": map[string]any{"channel_type": "push", "address": "device-token-1"},
	}))
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)

	resp = testutil.MakeRequest(app, http.MethodPost, "/users/self/communication_channels", testutil.JSONBody(map[string]any{
		"communication_channel": map[string]any{"channel_type": "email", "address": "not an email"},
	}))
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}
