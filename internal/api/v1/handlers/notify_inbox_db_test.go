package handlers_test

// DB-backed end-to-end tests for the notify-inbox lane: the inbox
// (G092: context recipients, individual sends, attachments, restricted
// replies, star/archive/delete, forward), the address book (G093),
// announcement replies (G090) and the notification policy grid (G091).
// Runs against a scratch, fully migrated database (coreScratchDB); skipped
// without PARITY_DB_URL / DATABASE_URL. All data is made up.

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/textproto"
	"testing"

	"github.com/gofiber/fiber/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"github.com/EduThemes/paper-lms/internal/api/v1/handlers"
	"github.com/EduThemes/paper-lms/internal/api/v1/middleware"
	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/repository/postgres"
	"github.com/EduThemes/paper-lms/internal/service"
	"github.com/EduThemes/paper-lms/internal/storage"
	"github.com/EduThemes/paper-lms/internal/testutil"
)

const (
	niTeacher  uint = 20
	niStudent1 uint = 21
	niStudent2 uint = 22
	niStranger uint = 23 // same tenant, shares nothing
	niForeign  uint = 24 // other tenant
)

type niFixture struct {
	g        *gorm.DB
	courseID uint
	app      func(callerID uint) *fiber.App
	recorder *niNotifier
}

type niNotifier struct{ calls []string }

func (n *niNotifier) QueueNotification(_ context.Context, userID uint, kind, _, _, _ string, _ uint) error {
	n.calls = append(n.calls, fmt.Sprintf("%d:%s", userID, kind))
	return nil
}

func newNIFixture(t *testing.T) *niFixture {
	t.Helper()
	g := coreScratchDB(t)
	ctx := context.Background()
	users := postgres.NewUserRepository(g)
	for _, u := range []struct {
		id, acct uint
		name     string
	}{{niTeacher, 1, "Ada Teacher"}, {niStudent1, 1, "Ben Student"}, {niStudent2, 1, "Cy Student"}, {niStranger, 1, "Dee Stranger"}, {niForeign, 2, "Eve Elsewhere"}} {
		require.NoError(t, users.Create(ctx, &models.User{ID: u.id, AccountID: u.acct, Name: u.name, SortableName: u.name,
			LoginID: fmt.Sprintf("u%d@example.test", u.id), Email: fmt.Sprintf("u%d@example.test", u.id), PasswordHash: "x", Role: "user"}))
	}
	course := &models.Course{AccountID: 1, Name: "Biology 101", CourseCode: "BIO-101", WorkflowState: models.CourseAvailable}
	require.NoError(t, postgres.NewCourseRepository(g).Create(ctx, course))
	for _, e := range []struct {
		uid uint
		typ string
	}{{niTeacher, "TeacherEnrollment"}, {niStudent1, "StudentEnrollment"}, {niStudent2, "StudentEnrollment"}} {
		require.NoError(t, g.Create(&models.Enrollment{UserID: e.uid, CourseID: course.ID, Type: e.typ, Role: e.typ, WorkflowState: "active"}).Error)
	}

	userRepo := postgres.NewUserRepository(g)
	enrollRepo := postgres.NewEnrollmentRepository(g)
	courseRepo := postgres.NewCourseRepository(g)
	convSvc := service.NewConversationService(postgres.NewConversationRepository(g), postgres.NewConversationParticipantRepository(g), postgres.NewConversationMessageRepository(g))
	userSvc := service.NewUserService(userRepo)
	fileSvc := service.NewFileServiceWithBackend(postgres.NewFolderRepository(g), postgres.NewAttachmentRepository(g), storage.NewLocalBackend(t.TempDir()))

	recorder := &niNotifier{}
	conv := handlers.NewConversationHandler(convSvc, userSvc, postgres.NewAccountRepository(g), enrollRepo, nil)
	ab := postgres.NewRecipientSearchRepository(g)
	conv.SetRecipientSearch(ab)
	conv.SetAddressBook(ab)
	conv.SetInbox(postgres.NewInboxRepository(g))
	conv.SetFileService(fileSvc)
	conv.SetNotifier(recorder)

	annSvc := service.NewAnnouncementService(postgres.NewAnnouncementRepository(g), postgres.NewAnnouncementReadReceiptRepository(g), enrollRepo)
	annSvc.SetExtras(postgres.NewAnnouncementExtrasRepository(g))
	annSvc.SetNotifier(recorder)
	authz := handlers.NewResourceAuthorizer(enrollRepo, userRepo, courseRepo)
	ann := handlers.NewAnnouncementHandler(annSvc, authz, nil)

	delivery := service.NewNotificationDeliveryService(postgres.NewNotificationDeliveryRepository(g), postgres.NewCommunicationChannelRepository(g),
		postgres.NewNotificationPreferenceRepository(g), postgres.NewNotificationRepository(g), userRepo, nil)
	delivery.SetPolicyStore(postgres.NewNotificationPolicyRepository(g))
	nd := handlers.NewNotificationDeliveryHandler(delivery, postgres.NewCommunicationChannelRepository(g))

	return &niFixture{g: g, courseID: course.ID, recorder: recorder, app: func(callerID uint) *fiber.App {
		acct := uint(1)
		if callerID == niForeign {
			acct = 2
		}
		app := testutil.SetupTestApp()
		app.Use(callerStub(callerID, acct), middleware.PaginationParams())
		// Same order as production: static paths before /:id.
		app.Get("/conversations/unread_count", conv.UnreadCount)
		app.Get("/conversations/contexts", conv.ListInboxContexts)
		app.Put("/conversations", conv.BatchUpdateConversations)
		app.Post("/conversations/attachments", conv.UploadAttachment)
		app.Delete("/conversations/:id", conv.DeleteConversation)
		app.Post("/conversations/:id/forward", conv.ForwardMessages)
		app.Get("/conversations/:id/attachments/:attachment_id", conv.DownloadAttachment)
		app.Get("/conversations", conv.ListConversations)
		app.Post("/conversations", conv.CreateConversation)
		app.Get("/conversations/:id", conv.GetConversation)
		app.Put("/conversations/:id", conv.UpdateConversation)
		app.Get("/conversations/:id/messages", conv.ListMessages)
		app.Post("/conversations/:id/messages", conv.CreateMessage)
		app.Get("/search/recipients", conv.SearchRecipients)
		app.Post("/courses/:course_id/announcements", ann.CreateCourseAnnouncement)
		app.Get("/announcements/:id/replies", ann.ListReplies)
		app.Post("/announcements/:id/replies", ann.CreateReply)
		app.Put("/announcements/:id/replies/:reply_id", ann.UpdateReply)
		app.Delete("/announcements/:id/replies/:reply_id", ann.DeleteReply)
		app.Get("/users/self/notification_policies", nd.GetNotificationPolicies)
		app.Put("/users/self/notification_policies", nd.UpdateNotificationPolicies)
		app.Get("/users/self/communication_channel_types", nd.ListChannelTypes)
		return app
	}}
}

func niCreate(t *testing.T, app *fiber.App, conv map[string]any) (*http.Response, map[string]any) {
	t.Helper()
	resp := testutil.MakeRequest(app, http.MethodPost, "/conversations", testutil.JSONBody(map[string]any{"conversation": conv}))
	body, _ := testutil.ParseJSONMap(resp)
	return resp, body
}

func niList(t *testing.T, app *fiber.App, query string) []map[string]any {
	t.Helper()
	resp := testutil.MakeRequest(app, http.MethodGet, "/conversations"+query, nil)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	items, err := testutil.ParseJSONArray(resp)
	require.NoError(t, err)
	return items
}

func niUpload(t *testing.T, app *fiber.App, name, content string) *http.Response {
	t.Helper()
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	h := make(textproto.MIMEHeader)
	h.Set("Content-Disposition", fmt.Sprintf(`form-data; name="file"; filename="%s"`, name))
	h.Set("Content-Type", "text/plain")
	part, err := w.CreatePart(h)
	require.NoError(t, err)
	_, _ = part.Write([]byte(content))
	require.NoError(t, w.Close())
	req := httptest.NewRequest(http.MethodPost, "/conversations/attachments", &buf)
	req.Header.Set("Content-Type", w.FormDataContentType())
	resp, err := app.Test(req, -1)
	require.NoError(t, err)
	return resp
}

func idOf(m map[string]any) uint { return uint(m["id"].(float64)) }

// G093: recipients must share a course or group with the sender.
func TestInboxDB_AddressBookScopesRecipients(t *testing.T) {
	f := newNIFixture(t)
	s1 := f.app(niStudent1)

	resp, _ := niCreate(t, s1, map[string]any{"subject": "hi", "recipients": []uint{niStranger}, "body": "hello"})
	assert.Equal(t, http.StatusNotFound, resp.StatusCode, "a same-tenant user who shares no course is not addressable")
	resp, _ = niCreate(t, s1, map[string]any{"subject": "hi", "recipients": []uint{niForeign}, "body": "hello"})
	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
	resp, _ = niCreate(t, s1, map[string]any{"subject": "hi", "recipients": []any{fmt.Sprintf("course_%d", f.courseID)}, "body": "hello"})
	assert.Equal(t, http.StatusForbidden, resp.StatusCode, "students cannot message the whole course")
	resp, _ = niCreate(t, f.app(niStranger), map[string]any{"subject": "hi", "recipients": []any{fmt.Sprintf("course_%d", f.courseID)}, "body": "x"})
	assert.Equal(t, http.StatusNotFound, resp.StatusCode, "a non-member gets no hint the course exists")
	resp, _ = niCreate(t, s1, map[string]any{"subject": "hi", "recipients": []uint{niStudent2}, "body": "hello"})
	assert.Equal(t, http.StatusCreated, resp.StatusCode, "classmates are addressable")

	// Browse the course in the address book.
	resp = testutil.MakeRequest(s1, http.MethodGet, fmt.Sprintf("/search/recipients?context=course_%d", f.courseID), nil)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	items, _ := testutil.ParseJSONArray(resp)
	var names []string
	for _, it := range items {
		names = append(names, it["name"].(string))
	}
	assert.ElementsMatch(t, []string{"Ada Teacher", "Cy Student"}, names)
	resp = testutil.MakeRequest(f.app(niStranger), http.MethodGet, fmt.Sprintf("/search/recipients?context=course_%d", f.courseID), nil)
	assert.Equal(t, http.StatusNotFound, resp.StatusCode)

	resp = testutil.MakeRequest(f.app(niTeacher), http.MethodGet, "/conversations/contexts", nil)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	ctxs, _ := testutil.ParseJSONArray(resp)
	require.Len(t, ctxs, 1)
	assert.Equal(t, fmt.Sprintf("course_%d", f.courseID), ctxs[0]["context_code"])
	assert.Equal(t, true, ctxs[0]["permissions"].(map[string]any)["send_messages_all"])
}

// G092: course-role recipients, individual sends, unread/star/archive/
// delete, context filter.
func TestInboxDB_IndividualSendsAndParticipantState(t *testing.T) {
	f := newNIFixture(t)
	teacher := f.app(niTeacher)

	resp, body := niCreate(t, teacher, map[string]any{
		"subject": "Lab safety", "recipients": []any{fmt.Sprintf("course_%d_students", f.courseID)},
		"body": "Read the lab rules", "group_conversation": false,
	})
	require.Equal(t, http.StatusCreated, resp.StatusCode)
	assert.Len(t, body["conversation_ids"], 2, "one private conversation per student")
	assert.ElementsMatch(t, []string{fmt.Sprintf("%d:new_message", niStudent1), fmt.Sprintf("%d:new_message", niStudent2)}, f.recorder.calls)

	s1 := f.app(niStudent1)
	resp = testutil.MakeRequest(s1, http.MethodGet, "/conversations/unread_count", nil)
	m, _ := testutil.ParseJSONMap(resp)
	assert.EqualValues(t, 1, m["unread_count"])

	items := niList(t, s1, "?scope=unread")
	require.Len(t, items, 1)
	convID := idOf(items[0])
	assert.Equal(t, "unread", items[0]["workflow_state"])
	assert.Equal(t, "Read the lab rules", items[0]["last_message"])
	assert.Equal(t, fmt.Sprintf("course_%d", f.courseID), items[0]["context_code"])
	assert.Equal(t, "Biology 101", items[0]["context_name"])
	assert.Len(t, niList(t, s1, fmt.Sprintf("?filter=course_%d", f.courseID)), 1)
	assert.Len(t, niList(t, s1, "?filter=course_999999"), 0)
	resp = testutil.MakeRequest(s1, http.MethodGet, "/conversations?filter=bogus", nil)
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)

	// Per-participant state via PUT /conversations/:id and the batch PUT.
	resp = testutil.MakeRequest(s1, http.MethodPut, fmt.Sprintf("/conversations/%d", convID), testutil.JSONBody(map[string]any{"conversation": map[string]any{"workflow_state": "read", "starred": true}}))
	require.Equal(t, http.StatusOK, resp.StatusCode)
	m, _ = testutil.ParseJSONMap(resp)
	assert.Equal(t, "read", m["workflow_state"])
	assert.Equal(t, true, m["starred"])
	assert.Len(t, niList(t, s1, "?scope=starred"), 1)
	assert.Len(t, niList(t, teacher, "?scope=starred"), 0, "starring is the caller's own flag")

	resp = testutil.MakeRequest(s1, http.MethodPut, "/conversations", testutil.JSONBody(map[string]any{"conversation_ids": []uint{convID}, "event": "archive"}))
	require.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Len(t, niList(t, s1, ""), 0)
	assert.Len(t, niList(t, s1, "?scope=archived"), 1)
	resp = testutil.MakeRequest(s1, http.MethodPut, "/conversations", testutil.JSONBody(map[string]any{"conversation_ids": []uint{convID}, "event": "explode"}))
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)

	// A new message brings an archived conversation back as unread.
	resp = testutil.MakeRequest(teacher, http.MethodPost, fmt.Sprintf("/conversations/%d/messages", convID), testutil.JSONBody(map[string]any{"message": "Reminder: goggles"}))
	require.Equal(t, http.StatusCreated, resp.StatusCode)
	items = niList(t, s1, "")
	require.Len(t, items, 1)
	assert.Equal(t, "unread", items[0]["workflow_state"])

	// Delete for me only.
	resp = testutil.MakeRequest(s1, http.MethodDelete, fmt.Sprintf("/conversations/%d", convID), nil)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Len(t, niList(t, s1, ""), 0)
	assert.Len(t, niList(t, teacher, ""), 2, "the other participant keeps it")
	resp = testutil.MakeRequest(f.app(niStudent2), http.MethodDelete, fmt.Sprintf("/conversations/%d", convID), nil)
	assert.Equal(t, http.StatusNotFound, resp.StatusCode, "not a participant of that private conversation")
}

// G092: attachments, Reply (restricted) vs Reply all, forward.
func TestInboxDB_AttachmentsRepliesAndForward(t *testing.T) {
	f := newNIFixture(t)
	teacher := f.app(niTeacher)

	resp := niUpload(t, teacher, "syllabus.txt", "Week 1: cells")
	require.Equal(t, http.StatusCreated, resp.StatusCode)
	up, _ := testutil.ParseJSONMap(resp)
	attID := idOf(up)

	// Someone else's upload can't be attached.
	resp = niUpload(t, f.app(niStudent1), "mine.txt", "x")
	require.Equal(t, http.StatusCreated, resp.StatusCode)
	other, _ := testutil.ParseJSONMap(resp)
	resp, _ = niCreate(t, teacher, map[string]any{"subject": "x", "recipients": []uint{niStudent1}, "body": "x", "attachment_ids": []uint{idOf(other)}})
	assert.Equal(t, http.StatusNotFound, resp.StatusCode)

	resp, body := niCreate(t, teacher, map[string]any{"subject": "Group project", "recipients": []uint{niStudent1, niStudent2}, "body": "See attached", "attachment_ids": []uint{attID}})
	require.Equal(t, http.StatusCreated, resp.StatusCode)
	convID := idOf(body)

	s2 := f.app(niStudent2)
	resp = testutil.MakeRequest(s2, http.MethodGet, fmt.Sprintf("/conversations/%d/messages", convID), nil)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	msgs, _ := testutil.ParseJSONArray(resp)
	require.Len(t, msgs, 1)
	atts := msgs[0]["attachments"].([]any)
	require.Len(t, atts, 1)
	url := atts[0].(map[string]any)["url"].(string)
	assert.Equal(t, fmt.Sprintf("/api/v1/conversations/%d/attachments/%d", convID, attID), url)

	resp = testutil.MakeRequest(s2, http.MethodGet, fmt.Sprintf("/conversations/%d/attachments/%d", convID, attID), nil)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	data, _ := io.ReadAll(resp.Body)
	assert.Equal(t, "Week 1: cells", string(data))
	assert.Contains(t, resp.Header.Get("Content-Disposition"), "attachment;")
	resp = testutil.MakeRequest(f.app(niStranger), http.MethodGet, fmt.Sprintf("/conversations/%d/attachments/%d", convID, attID), nil)
	assert.Equal(t, http.StatusNotFound, resp.StatusCode)

	// Ben replies to the teacher only.
	s1 := f.app(niStudent1)
	f.recorder.calls = nil
	resp = testutil.MakeRequest(s1, http.MethodPost, fmt.Sprintf("/conversations/%d/messages", convID), testutil.JSONBody(map[string]any{"message": "Can I work alone?", "recipients": []uint{niTeacher}}))
	require.Equal(t, http.StatusCreated, resp.StatusCode)
	assert.Equal(t, []string{fmt.Sprintf("%d:new_message", niTeacher)}, f.recorder.calls, "only the addressed participant is notified")
	resp = testutil.MakeRequest(s2, http.MethodGet, fmt.Sprintf("/conversations/%d/messages", convID), nil)
	msgs, _ = testutil.ParseJSONArray(resp)
	assert.Len(t, msgs, 1, "Cy does not see a reply addressed to the teacher")
	resp = testutil.MakeRequest(teacher, http.MethodGet, fmt.Sprintf("/conversations/%d/messages", convID), nil)
	msgs, _ = testutil.ParseJSONArray(resp)
	require.Len(t, msgs, 2)
	assert.Equal(t, []any{float64(niTeacher)}, msgs[1]["recipient_ids"])
	resp = testutil.MakeRequest(s1, http.MethodPost, fmt.Sprintf("/conversations/%d/messages", convID), testutil.JSONBody(map[string]any{"message": "x", "recipients": []uint{niStranger}}))
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode, "a reply can only go to participants")

	// Cy forwards the teacher's message to Ben in a new conversation.
	resp = testutil.MakeRequest(s2, http.MethodPost, fmt.Sprintf("/conversations/%d/forward", convID), testutil.JSONBody(map[string]any{"recipients": []uint{niStudent1}, "body": "FYI"}))
	require.Equal(t, http.StatusCreated, resp.StatusCode)
	fwd, _ := testutil.ParseJSONMap(resp)
	assert.Equal(t, "Fwd: Group project", fwd["subject"])
	resp = testutil.MakeRequest(s1, http.MethodGet, fmt.Sprintf("/conversations/%d/messages", idOf(fwd)), nil)
	msgs, _ = testutil.ParseJSONArray(resp)
	require.Len(t, msgs, 1)
	assert.Contains(t, msgs[0]["body"], "FYI")
	assert.Contains(t, msgs[0]["body"], "See attached")
	assert.NotContains(t, msgs[0]["body"], "Can I work alone?", "only messages the forwarder could see are forwarded")
	assert.Len(t, msgs[0]["attachments"], 1, "forwarded attachments come along")
	resp = testutil.MakeRequest(f.app(niStranger), http.MethodPost, fmt.Sprintf("/conversations/%d/forward", convID), testutil.JSONBody(map[string]any{"recipients": []uint{niStudent1}}))
	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
}

// G090: replies on a course announcement.
func TestAnnouncementRepliesDB(t *testing.T) {
	f := newNIFixture(t)
	teacher := f.app(niTeacher)
	resp := testutil.MakeRequest(teacher, http.MethodPost, fmt.Sprintf("/courses/%d/announcements", f.courseID), testutil.JSONBody(map[string]any{
		"title": "Field trip", "message": "<p>Bring lunch</p>", "allow_comments": true,
	}))
	require.Equal(t, http.StatusCreated, resp.StatusCode)
	ann, _ := testutil.ParseJSONMap(resp)
	annID := idOf(ann)

	s1 := f.app(niStudent1)
	f.recorder.calls = nil
	resp = testutil.MakeRequest(s1, http.MethodPost, fmt.Sprintf("/announcements/%d/replies", annID), testutil.JSONBody(map[string]any{"message": "<p>Is it paid?</p><script>alert(1)</script>"}))
	require.Equal(t, http.StatusCreated, resp.StatusCode)
	reply, _ := testutil.ParseJSONMap(resp)
	assert.NotContains(t, reply["message"], "<script>", "rich replies are sanitized")
	assert.Equal(t, "Ben Student", reply["user_name"])
	assert.Contains(t, f.recorder.calls, fmt.Sprintf("%d:announcement_reply", niTeacher))

	resp = testutil.MakeRequest(teacher, http.MethodPost, fmt.Sprintf("/announcements/%d/replies", annID), testutil.JSONBody(map[string]any{"message": "<p>Free</p>", "parent_id": idOf(reply)}))
	require.Equal(t, http.StatusCreated, resp.StatusCode)
	teacherReply, _ := testutil.ParseJSONMap(resp)

	resp = testutil.MakeRequest(f.app(niStudent2), http.MethodGet, fmt.Sprintf("/announcements/%d/replies", annID), nil)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	list, _ := testutil.ParseJSONArray(resp)
	assert.Len(t, list, 2)
	resp = testutil.MakeRequest(f.app(niStranger), http.MethodGet, fmt.Sprintf("/announcements/%d/replies", annID), nil)
	assert.Equal(t, http.StatusForbidden, resp.StatusCode, "non-members can't read replies (same rule as GET /announcements/:id)")

	resp = testutil.MakeRequest(f.app(niStudent2), http.MethodPut, fmt.Sprintf("/announcements/%d/replies/%d", annID, idOf(reply)), testutil.JSONBody(map[string]any{"message": "hijack"}))
	assert.Equal(t, http.StatusForbidden, resp.StatusCode)
	resp = testutil.MakeRequest(s1, http.MethodDelete, fmt.Sprintf("/announcements/%d/replies/%d", annID, idOf(teacherReply)), nil)
	assert.Equal(t, http.StatusForbidden, resp.StatusCode, "students can't delete others' replies")
	resp = testutil.MakeRequest(teacher, http.MethodDelete, fmt.Sprintf("/announcements/%d/replies/%d", annID, idOf(reply)), nil)
	assert.Equal(t, http.StatusOK, resp.StatusCode, "instructors moderate replies")

	// Comments off: students can't reply.
	resp = testutil.MakeRequest(teacher, http.MethodPost, fmt.Sprintf("/courses/%d/announcements", f.courseID), testutil.JSONBody(map[string]any{"title": "Closed", "message": "<p>x</p>"}))
	closed, _ := testutil.ParseJSONMap(resp)
	resp = testutil.MakeRequest(s1, http.MethodPost, fmt.Sprintf("/announcements/%d/replies", idOf(closed)), testutil.JSONBody(map[string]any{"message": "hi"}))
	assert.Equal(t, http.StatusForbidden, resp.StatusCode)
}

// G091: the preference grid endpoint.
func TestNotificationPoliciesDB(t *testing.T) {
	f := newNIFixture(t)
	s1 := f.app(niStudent1)
	resp := testutil.MakeRequest(s1, http.MethodGet, "/users/self/notification_policies", nil)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	grid, _ := testutil.ParseJSONMap(resp)
	require.Len(t, grid["channels"], 1, "the account email column")
	assert.NotEmpty(t, grid["categories"])

	resp = testutil.MakeRequest(s1, http.MethodPut, "/users/self/notification_policies", testutil.JSONBody(map[string]any{"policies": []map[string]any{{"channel_id": 0, "category": "grading", "frequency": "immediately"}}}))
	require.Equal(t, http.StatusOK, resp.StatusCode)
	grid, _ = testutil.ParseJSONMap(resp)
	found := false
	for _, p := range grid["policies"].([]any) {
		cell := p.(map[string]any)
		if cell["category"] == "grading" {
			assert.Equal(t, "immediately", cell["frequency"])
			assert.Equal(t, true, cell["explicit"])
			found = true
		}
	}
	assert.True(t, found)

	resp = testutil.MakeRequest(s1, http.MethodPut, "/users/self/notification_policies", testutil.JSONBody(map[string]any{"policies": []map[string]any{{"channel_id": 4242, "category": "grading", "frequency": "never"}}}))
	assert.Equal(t, http.StatusNotFound, resp.StatusCode, "another user's / unknown channel")
	resp = testutil.MakeRequest(s1, http.MethodPut, "/users/self/notification_policies", testutil.JSONBody(map[string]any{"policies": []map[string]any{{"channel_id": 0, "category": "grading", "frequency": "hourly"}}}))
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)

	resp = testutil.MakeRequest(s1, http.MethodGet, "/users/self/communication_channel_types", nil)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	types, _ := testutil.ParseJSONArray(resp)
	byType := map[string]bool{}
	for _, ct := range types {
		byType[ct["type"].(string)] = ct["available"].(bool)
	}
	assert.Equal(t, map[string]bool{"email": true, "webhook": true, "sms": false, "push": false}, byType)
}

// G091: a comment by someone else on a visible submission notifies its
// owner (in-app row + deliveries); the owner's own comment and a comment on
// an unposted manual-policy submission do not.
func TestSubmissionCommentNotificationDB(t *testing.T) {
	f := newNIFixture(t)
	ctx := context.Background()
	delivery := service.NewNotificationDeliveryService(postgres.NewNotificationDeliveryRepository(f.g), postgres.NewCommunicationChannelRepository(f.g),
		postgres.NewNotificationPreferenceRepository(f.g), postgres.NewNotificationRepository(f.g), postgres.NewUserRepository(f.g), nil)
	h := handlers.NewSubmissionHandler(nil, nil, nil, nil, nil, delivery, nil, nil, nil, nil)

	count := func() int64 {
		var n int64
		require.NoError(t, f.g.Model(&models.Notification{}).Where("user_id = ? AND notification_type = ?", niStudent1, "submission_comment").Count(&n).Error)
		return n
	}
	auto := &models.Assignment{ID: 900, CourseID: f.courseID, Name: "Lab report"}
	sub := &models.Submission{ID: 901, AssignmentID: 900, UserID: niStudent1}

	h.NotifySubmissionCommentForTest(ctx, auto, sub, niStudent1, "Ben Student", "my own note")
	assert.Equal(t, int64(0), count(), "the owner's own comment is not a notification")

	manual := &models.Assignment{ID: 902, CourseID: f.courseID, Name: "Essay", PostPolicy: "manual"}
	h.NotifySubmissionCommentForTest(ctx, manual, sub, niTeacher, "Ada Teacher", "hidden until posted")
	assert.Equal(t, int64(0), count(), "an unposted manual-policy comment stays quiet")

	h.NotifySubmissionCommentForTest(ctx, auto, sub, niTeacher, "Ada Teacher", "Nice <work>")
	assert.Equal(t, int64(1), count())
	var d models.NotificationDelivery
	require.NoError(t, f.g.Where("user_id = ?", niStudent1).Order("id DESC").First(&d).Error)
	assert.Contains(t, d.Body, "Nice &lt;work&gt;", "the comment is escaped in the email body")
}
