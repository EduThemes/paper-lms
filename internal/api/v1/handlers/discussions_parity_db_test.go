package handlers_test

// DB-backed tests for the discussions parity lane (G083-G089): the real
// discussion handlers, services and Postgres repos behind a stub auth
// middleware that takes the caller from request headers. Gated on
// PARITY_DB_URL / DATABASE_URL (contentLaneDB); made-up data only.

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/textproto"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

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

type capturedNotification struct {
	UserID uint
	Type   string
	Title  string
}

type captureNotifier struct {
	mu    sync.Mutex
	calls []capturedNotification
}

func (n *captureNotifier) QueueNotification(_ context.Context, userID uint, notificationType, subject, _ string, _ string, _ uint) error {
	n.mu.Lock()
	defer n.mu.Unlock()
	n.calls = append(n.calls, capturedNotification{UserID: userID, Type: notificationType, Title: subject})
	return nil
}

func (n *captureNotifier) of(kind string) map[uint]int {
	n.mu.Lock()
	defer n.mu.Unlock()
	out := map[uint]int{}
	for _, c := range n.calls {
		if c.Type == kind {
			out[c.UserID]++
		}
	}
	return out
}

func (n *captureNotifier) reset() {
	n.mu.Lock()
	n.calls = nil
	n.mu.Unlock()
}

type discParityEnv struct {
	g        *gorm.DB
	app      *fiber.App
	notifier *captureNotifier
	parity   *service.DiscussionParityService
	course   *models.Course
	other    *models.Course // other tenant
	teacher  *models.User
	alice    *models.User // section A, group 1
	bob      *models.User // section B, group 2
	carol    *models.User // section A, group 1
	secA     *models.CourseSection
	secB     *models.CourseSection
	category *models.GroupCategory
	group1   *models.Group
	group2   *models.Group
	roles    map[uint]string
}

func newDiscParityEnv(t *testing.T) *discParityEnv {
	t.Helper()
	g := contentLaneDB(t)
	seedContentAccounts(t, g, 1, 2)
	ctx := context.Background()
	env := &discParityEnv{g: g, notifier: &captureNotifier{}, roles: map[uint]string{}}

	env.course = seedContentCourse(t, g, 1, "BIO-101")
	env.other = seedContentCourse(t, g, 2, "CHEM-1")

	users := postgres.NewUserRepository(g)
	mk := func(name, login string) *models.User {
		u := &models.User{Name: name, LoginID: login, Email: login, PasswordHash: "x", Role: "user"}
		require.NoError(t, users.Create(ctx, u))
		return u
	}
	env.teacher = mk("Terry Teacher", "terry@example.test")
	env.alice = mk("Alice Doe", "alice@example.test")
	env.bob = mk("Bob Roe", "bob@example.test")
	env.carol = mk("Carol Poe", "carol@example.test")

	env.secA = &models.CourseSection{CourseID: env.course.ID, Name: "Section A", WorkflowState: "active"}
	env.secB = &models.CourseSection{CourseID: env.course.ID, Name: "Section B", WorkflowState: "active"}
	require.NoError(t, g.Create(env.secA).Error)
	require.NoError(t, g.Create(env.secB).Error)

	enroll := func(u *models.User, typ string, sec *models.CourseSection) {
		e := &models.Enrollment{UserID: u.ID, CourseID: env.course.ID, Type: typ, Role: typ, WorkflowState: models.EnrollmentActive}
		if sec != nil {
			id := sec.ID
			e.CourseSectionID = &id
		}
		require.NoError(t, g.Create(e).Error)
		env.roles[u.ID] = typ
	}
	enroll(env.teacher, "TeacherEnrollment", env.secA)
	enroll(env.alice, "StudentEnrollment", env.secA)
	enroll(env.bob, "StudentEnrollment", env.secB)
	enroll(env.carol, "StudentEnrollment", env.secA)

	cid := env.course.ID
	env.category = &models.GroupCategory{CourseID: &cid, Name: "Project groups", WorkflowState: "active"}
	require.NoError(t, g.Create(env.category).Error)
	env.group1 = &models.Group{GroupCategoryID: env.category.ID, Name: "Group One", ContextType: "Course", ContextID: cid, WorkflowState: "available"}
	env.group2 = &models.Group{GroupCategoryID: env.category.ID, Name: "Group Two", ContextType: "Course", ContextID: cid, WorkflowState: "available"}
	require.NoError(t, g.Create(env.group1).Error)
	require.NoError(t, g.Create(env.group2).Error)
	for _, m := range []struct {
		g *models.Group
		u *models.User
	}{{env.group1, env.alice}, {env.group1, env.carol}, {env.group2, env.bob}} {
		require.NoError(t, g.Create(&models.GroupMembership{GroupID: m.g.ID, UserID: m.u.ID, WorkflowState: "accepted"}).Error)
	}

	topicRepo := postgres.NewDiscussionTopicRepository(g)
	entryRepo := postgres.NewDiscussionEntryRepository(g)
	ratingRepo := postgres.NewDiscussionEntryRatingRepository(g)
	topicPartRepo := postgres.NewDiscussionTopicParticipantRepository(g)
	attachmentRepo := postgres.NewAttachmentRepository(g)

	disc := service.NewDiscussionService(topicRepo, entryRepo, ratingRepo)
	v2 := service.NewDiscussionV2Service(topicRepo, entryRepo, ratingRepo,
		postgres.NewDiscussionEntryParticipantRepository(g), topicPartRepo,
		postgres.NewDiscussionEntryVersionRepository(g), users)
	parity := service.NewDiscussionParityService(topicRepo, entryRepo, postgres.NewDiscussionParityRepository(g),
		topicPartRepo, postgres.NewAssignmentRepository(g), postgres.NewSubmissionRepository(g), env.notifier)
	parity.SetAttachmentRepo(attachmentRepo)
	disc.SetAudience(parity)
	v2.SetAudience(parity)
	disc.OnEntryCreated(parity.EntryNotificationCallback())
	disc.OnEntryCreated(parity.SubmissionCallback())
	env.parity = parity

	files := service.NewFileServiceWithBackend(postgres.NewFolderRepository(g), attachmentRepo, storage.NewLocalBackend(t.TempDir()))

	th := handlers.NewDiscussionHandler(disc).SetParity(parity)
	eh := handlers.NewDiscussionEntryHandler(disc).SetVersionHistory(v2).SetParity(parity)
	vh := handlers.NewDiscussionV2Handler(v2).SetParity(parity)
	ph := handlers.NewDiscussionParityHandler(disc, parity, files)

	app := testutil.SetupTestApp()
	app.Use(middleware.PaginationParams())
	stub := func(c *fiber.Ctx) error {
		var uid uint
		_, _ = fmt.Sscanf(c.Get("X-Test-User"), "%d", &uid)
		c.Locals("user_id", uid)
		c.Locals("account_id", uint(1))
		c.Locals("enrollment_type", env.roles[uid])
		c.Locals("is_admin", false)
		return c.Next()
	}
	base := "/courses/:course_id/discussion_topics"
	app.Get(base, stub, th.ListTopics)
	app.Post(base, stub, th.CreateTopic)
	app.Get(base+"/:topic_id", stub, th.GetTopic)
	app.Put(base+"/:topic_id", stub, th.UpdateTopic)
	app.Delete(base+"/:topic_id", stub, th.DeleteTopic)
	app.Get(base+"/:topic_id/view", stub, th.GetFullView)
	app.Get(base+"/:topic_id/view_v2", stub, vh.GetFullViewV2)
	app.Get(base+"/:topic_id/entries", stub, eh.ListEntries)
	app.Post(base+"/:topic_id/entries", stub, eh.CreateEntry)
	app.Put(base+"/:topic_id/entries/:id", stub, eh.UpdateEntry)
	app.Delete(base+"/:topic_id/entries/:id", stub, eh.DeleteEntry)
	app.Post(base+"/:topic_id/entries/:entry_id/replies", stub, eh.CreateReply)
	app.Put(base+"/:topic_id/subscription", stub, vh.ToggleSubscription)
	app.Post(base+"/:topic_id/attachments", stub, ph.UploadAttachment)
	app.Get(base+"/:topic_id/entries/:entry_id/attachment", stub, ph.DownloadEntryAttachment)
	app.Get(base+"/:topic_id/mentionable_users", stub, ph.MentionableUsers)
	app.Get("/courses/:course_id/assignments/:assignment_id/discussion_topic", stub, ph.AssignmentDiscussionTopic)
	app.Get("/courses/:course_id/assignments/:assignment_id/discussion_entries", stub, ph.StudentEntries)
	env.app = app
	return env
}

func (env *discParityEnv) req(t *testing.T, user *models.User, method, path string, body any) (int, []byte) {
	t.Helper()
	var rdr io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		require.NoError(t, err)
		rdr = bytes.NewReader(b)
	}
	r := httptest.NewRequest(method, path, rdr)
	r.Header.Set("Content-Type", "application/json")
	r.Header.Set("X-Test-User", fmt.Sprint(user.ID))
	resp, err := env.app.Test(r, -1)
	require.NoError(t, err)
	out, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, out
}

func (env *discParityEnv) topicsPath(rest string) string {
	return fmt.Sprintf("/courses/%d/discussion_topics%s", env.course.ID, rest)
}

func (env *discParityEnv) createTopic(t *testing.T, body map[string]any) map[string]any {
	t.Helper()
	status, out := env.req(t, env.teacher, http.MethodPost, env.topicsPath(""), map[string]any{"discussion_topic": body})
	require.Equal(t, http.StatusCreated, status, string(out))
	var m map[string]any
	require.NoError(t, json.Unmarshal(out, &m))
	return m
}

func (env *discParityEnv) post(t *testing.T, user *models.User, topicID uint, body map[string]any) (int, map[string]any) {
	t.Helper()
	status, out := env.req(t, user, http.MethodPost, env.topicsPath(fmt.Sprintf("/%d/entries", topicID)), body)
	var m map[string]any
	_ = json.Unmarshal(out, &m)
	return status, m
}

func idOf(m map[string]any) uint { return uint(m["id"].(float64)) }

func (env *discParityEnv) viewV2(t *testing.T, user *models.User, topicID uint) (int, map[string]any) {
	t.Helper()
	status, out := env.req(t, user, http.MethodGet, env.topicsPath(fmt.Sprintf("/%d/view_v2", topicID)), nil)
	var m map[string]any
	_ = json.Unmarshal(out, &m)
	return status, m
}

func (env *discParityEnv) index(t *testing.T, user *models.User, query string) []map[string]any {
	t.Helper()
	status, out := env.req(t, user, http.MethodGet, env.topicsPath(query), nil)
	require.Equal(t, http.StatusOK, status, string(out))
	var list []map[string]any
	require.NoError(t, json.Unmarshal(out, &list))
	return list
}

func eventually(t *testing.T, cond func() bool, msg string) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatal(msg)
}

// G083: delayed post, lock_at / closed for comments, require_initial_post.
func TestDiscussionParity_AvailabilityRules(t *testing.T) {
	env := newDiscParityEnv(t)

	future := time.Now().Add(72 * time.Hour).UTC().Format(time.RFC3339)
	delayed := env.createTopic(t, map[string]any{"title": "Later", "message": "<p>soon</p>", "delayed_post_at": future})
	assert.Equal(t, "post_delayed", delayed["workflow_state"])
	status, _ := env.viewV2(t, env.alice, idOf(delayed))
	assert.Equal(t, http.StatusNotFound, status, "a delayed topic is hidden from students")
	for _, tp := range env.index(t, env.alice, "") {
		assert.NotEqual(t, idOf(delayed), idOf(tp), "the index must not list a delayed topic")
	}
	status, _ = env.viewV2(t, env.teacher, idOf(delayed))
	assert.Equal(t, http.StatusOK, status)

	// Clearing the date releases it.
	status, out := env.req(t, env.teacher, http.MethodPut, env.topicsPath(fmt.Sprintf("/%d", idOf(delayed))),
		map[string]any{"discussion_topic": map[string]any{"delayed_post_at": nil}})
	require.Equal(t, http.StatusOK, status, string(out))
	status, _ = env.viewV2(t, env.alice, idOf(delayed))
	assert.Equal(t, http.StatusOK, status)

	// lock_at in the past closes the topic for students only.
	past := time.Now().Add(-time.Hour).UTC().Format(time.RFC3339)
	locked := env.createTopic(t, map[string]any{"title": "Closed", "lock_at": past})
	status, _ = env.post(t, env.alice, idOf(locked), map[string]any{"message": "<p>hi</p>"})
	assert.Equal(t, http.StatusForbidden, status)
	status, _ = env.post(t, env.teacher, idOf(locked), map[string]any{"message": "<p>staff note</p>"})
	assert.Equal(t, http.StatusCreated, status)

	// Closed for comments.
	closed := env.createTopic(t, map[string]any{"title": "Comments closed", "locked": true})
	status, _ = env.post(t, env.bob, idOf(closed), map[string]any{"message": "<p>hi</p>"})
	assert.Equal(t, http.StatusForbidden, status)

	// Post before seeing replies.
	gated := env.createTopic(t, map[string]any{"title": "Intro yourself", "require_initial_post": true})
	status, _ = env.post(t, env.bob, idOf(gated), map[string]any{"message": "<p>I am Bob</p>"})
	require.Equal(t, http.StatusCreated, status)

	status, view := env.viewV2(t, env.alice, idOf(gated))
	require.Equal(t, http.StatusOK, status)
	assert.Equal(t, true, view["topic"].(map[string]any)["initial_post_required"])
	assert.Empty(t, view["entries"], "replies are hidden until the student posts")
	status, _ = env.req(t, env.alice, http.MethodGet, env.topicsPath(fmt.Sprintf("/%d/entries", idOf(gated))), nil)
	assert.Equal(t, http.StatusForbidden, status)

	status, _ = env.post(t, env.alice, idOf(gated), map[string]any{"message": "<p>I am Alice</p>"})
	require.Equal(t, http.StatusCreated, status)
	_, view = env.viewV2(t, env.alice, idOf(gated))
	assert.Equal(t, false, view["topic"].(map[string]any)["initial_post_required"])
	assert.Len(t, view["entries"], 2)

	_, view = env.viewV2(t, env.teacher, idOf(gated))
	assert.Len(t, view["entries"], 2, "staff are never gated")
}

// G083: a delayed topic is released (and announced) by the minute job.
func TestDiscussionParity_DelayedTopicRelease(t *testing.T) {
	env := newDiscParityEnv(t)

	now := env.createTopic(t, map[string]any{"title": "Open now"})
	eventually(t, func() bool { return env.notifier.of("new_discussion_topic")[env.alice.ID] == 1 }, "students hear about a new topic")
	assert.Zero(t, env.notifier.of("new_discussion_topic")[env.teacher.ID], "the author isn't notified")
	_ = now

	env.notifier.reset()
	at := time.Now().Add(2 * time.Hour)
	delayed := env.createTopic(t, map[string]any{"title": "Tomorrow", "delayed_post_at": at.UTC().Format(time.RFC3339)})
	time.Sleep(100 * time.Millisecond)
	assert.Empty(t, env.notifier.of("new_discussion_topic"), "a delayed topic isn't announced early")

	n, err := env.parity.ReleaseDelayedTopics(context.Background())
	require.NoError(t, err)
	assert.Zero(t, n)

	env.parity.SetClock(func() time.Time { return at.Add(time.Minute) })
	n, err = env.parity.ReleaseDelayedTopics(context.Background())
	require.NoError(t, err)
	assert.Equal(t, 1, n)
	var stored models.DiscussionTopic
	require.NoError(t, env.g.First(&stored, idOf(delayed)).Error)
	assert.Equal(t, models.DiscussionTopicActive, stored.WorkflowState)
	assert.Equal(t, 1, env.notifier.of("new_discussion_topic")[env.bob.ID])
}

// G084: graded discussions create submissions; SpeedGrader lists posts.
func TestDiscussionParity_GradedDiscussion(t *testing.T) {
	env := newDiscParityEnv(t)
	due := time.Now().Add(48 * time.Hour).UTC().Format(time.RFC3339)
	topic := env.createTopic(t, map[string]any{
		"title": "Graded debate", "message": "<p>Argue</p>",
		"assignment": map[string]any{"points_possible": 10, "due_at": due},
	})
	a, ok := topic["assignment"].(map[string]any)
	require.True(t, ok, "a graded topic reports its assignment: %v", topic)
	assignmentID := uint(a["id"].(float64))

	var assignment models.Assignment
	require.NoError(t, env.g.First(&assignment, assignmentID).Error)
	assert.Equal(t, "discussion_topic", assignment.SubmissionTypes)
	assert.Equal(t, models.AssignmentPublished, assignment.WorkflowState)
	assert.Equal(t, "Graded debate", assignment.Name)

	status, entry := env.post(t, env.alice, idOf(topic), map[string]any{"message": "<p>My argument</p>"})
	require.Equal(t, http.StatusCreated, status)

	subFor := func(uid uint) *models.Submission {
		var s models.Submission
		if err := env.g.Where("assignment_id = ? AND user_id = ?", assignmentID, uid).First(&s).Error; err != nil {
			return nil
		}
		return &s
	}
	eventually(t, func() bool {
		s := subFor(env.alice.ID)
		return s != nil && s.WorkflowState == models.SubmissionSubmitted
	}, "posting in a graded discussion submits the assignment")
	s := subFor(env.alice.ID)
	require.NotNil(t, s.SubmissionType)
	assert.Equal(t, "discussion_topic", *s.SubmissionType)

	// The teacher's own post makes no submission.
	status, _ = env.post(t, env.teacher, idOf(topic), map[string]any{"message": "<p>Prompt</p>"})
	require.Equal(t, http.StatusCreated, status)

	// SpeedGrader view of Alice's posts; the assignment page lookup.
	status, out := env.req(t, env.teacher, http.MethodGet,
		fmt.Sprintf("/courses/%d/assignments/%d/discussion_entries?user_id=%d", env.course.ID, assignmentID, env.alice.ID), nil)
	require.Equal(t, http.StatusOK, status, string(out))
	var sg struct {
		Entries []map[string]any `json:"entries"`
	}
	require.NoError(t, json.Unmarshal(out, &sg))
	require.Len(t, sg.Entries, 1)
	assert.Equal(t, "<p>My argument</p>", sg.Entries[0]["message"])

	status, out = env.req(t, env.bob, http.MethodGet,
		fmt.Sprintf("/courses/%d/assignments/%d/discussion_topic", env.course.ID, assignmentID), nil)
	require.Equal(t, http.StatusOK, status, string(out))
	assert.Contains(t, string(out), fmt.Sprintf(`"id":%d`, idOf(topic)))

	// Can't drop grading once students have submitted.
	status, _ = env.req(t, env.teacher, http.MethodPut, env.topicsPath(fmt.Sprintf("/%d", idOf(topic))),
		map[string]any{"discussion_topic": map[string]any{"assignment": nil}})
	assert.Equal(t, http.StatusBadRequest, status)

	// Deleting the only post unsubmits.
	status, _ = env.req(t, env.alice, http.MethodDelete, env.topicsPath(fmt.Sprintf("/%d/entries/%d", idOf(topic), idOf(entry))), nil)
	require.Equal(t, http.StatusOK, status)
	s = subFor(env.alice.ID)
	require.NotNil(t, s)
	assert.Equal(t, models.SubmissionUnsubmitted, s.WorkflowState)

	// An assignment of another tenant can't be linked.
	foreign := seedContentAssignment(t, env.g, env.other.ID, "Foreign")
	status, _ = env.req(t, env.teacher, http.MethodPost, env.topicsPath(""),
		map[string]any{"discussion_topic": map[string]any{"title": "Steal", "assignment_id": foreign.ID}})
	assert.Equal(t, http.StatusBadRequest, status)
}

// G085: group discussions split per group.
func TestDiscussionParity_GroupDiscussion(t *testing.T) {
	env := newDiscParityEnv(t)
	root := env.createTopic(t, map[string]any{"title": "Group plan", "group_category_id": env.category.ID})
	rootID := idOf(root)

	status, view := env.viewV2(t, env.teacher, rootID)
	require.Equal(t, http.StatusOK, status)
	groupTopics, _ := view["topic"].(map[string]any)["group_topics"].([]any)
	require.Len(t, groupTopics, 2, "one topic per group")

	status, view = env.viewV2(t, env.alice, rootID)
	require.Equal(t, http.StatusOK, status)
	childA := uint(view["topic"].(map[string]any)["group_topic_id"].(float64))
	_, view = env.viewV2(t, env.bob, rootID)
	childB := uint(view["topic"].(map[string]any)["group_topic_id"].(float64))
	assert.NotEqual(t, childA, childB)

	status, _ = env.post(t, env.alice, rootID, map[string]any{"message": "<p>on the root</p>"})
	assert.Equal(t, http.StatusBadRequest, status, "posts go to the group's topic")
	status, _ = env.post(t, env.alice, childA, map[string]any{"message": "<p>group one idea</p>"})
	assert.Equal(t, http.StatusCreated, status)

	status, _ = env.viewV2(t, env.bob, childA)
	assert.Equal(t, http.StatusNotFound, status, "another group's topic is hidden")
	status, _ = env.post(t, env.bob, childA, map[string]any{"message": "<p>sneak</p>"})
	assert.Equal(t, http.StatusNotFound, status)
	status, view = env.viewV2(t, env.carol, childA)
	require.Equal(t, http.StatusOK, status)
	assert.Len(t, view["entries"], 1)
	assert.Equal(t, "Group One", view["topic"].(map[string]any)["group_name"])

	// The index lists the root once, with the viewer's group counts.
	var listed map[string]any
	for _, tp := range env.index(t, env.carol, "") {
		assert.NotEqual(t, childA, idOf(tp), "child topics are not listed")
		if idOf(tp) == rootID {
			listed = tp
		}
	}
	require.NotNil(t, listed)
	assert.EqualValues(t, 1, listed["discussion_subentry_count"])
	assert.EqualValues(t, 1, listed["unread_count"])
	for _, tp := range env.index(t, env.bob, "") {
		if idOf(tp) == rootID {
			assert.EqualValues(t, 0, tp["discussion_subentry_count"], "Bob's group has no posts")
		}
	}

	// The group set can't change once someone has posted.
	status, _ = env.req(t, env.teacher, http.MethodPut, env.topicsPath(fmt.Sprintf("/%d", rootID)),
		map[string]any{"discussion_topic": map[string]any{"group_category_id": nil}})
	assert.Equal(t, http.StatusBadRequest, status)

	// Edits to the root reach the group topics.
	status, _ = env.req(t, env.teacher, http.MethodPut, env.topicsPath(fmt.Sprintf("/%d", rootID)),
		map[string]any{"discussion_topic": map[string]any{"title": "Group plan v2"}})
	require.Equal(t, http.StatusOK, status)
	var child models.DiscussionTopic
	require.NoError(t, env.g.First(&child, childA).Error)
	assert.Equal(t, "Group plan v2", child.Title)
}

// G087: anonymous and section-specific discussions.
func TestDiscussionParity_AnonymousAndSections(t *testing.T) {
	env := newDiscParityEnv(t)

	anon := env.createTopic(t, map[string]any{"title": "Confessions", "anonymous_state": "full_anonymity"})
	status, mine := env.post(t, env.alice, idOf(anon), map[string]any{"message": "<p>secret</p>"})
	require.Equal(t, http.StatusCreated, status)
	assert.EqualValues(t, env.alice.ID, mine["user_id"], "authors still recognise their own post")

	for _, viewer := range []*models.User{env.bob, env.teacher} {
		_, view := env.viewV2(t, viewer, idOf(anon))
		entries := view["entries"].([]any)
		require.Len(t, entries, 1)
		e := entries[0].(map[string]any)
		assert.Nil(t, e["user_id"], "the author is hidden from %s", viewer.Name)
		assert.Contains(t, e["user_name"], "Anonymous ")
		assert.NotContains(t, fmt.Sprint(e), "Alice")
	}
	status, out := env.req(t, env.bob, http.MethodGet, env.topicsPath(fmt.Sprintf("/%d/entries", idOf(anon))), nil)
	require.Equal(t, http.StatusOK, status)
	assert.NotContains(t, string(out), fmt.Sprintf(`"user_id":%d`, env.alice.ID))

	// Anonymous + graded is refused (Canvas).
	status, _ = env.req(t, env.teacher, http.MethodPost, env.topicsPath(""), map[string]any{"discussion_topic": map[string]any{
		"title": "Bad", "anonymous_state": "full_anonymity", "assignment": map[string]any{"points_possible": 5},
	}})
	assert.Equal(t, http.StatusBadRequest, status)

	// Partial anonymity: each author decides.
	partial := env.createTopic(t, map[string]any{"title": "Choose", "anonymous_state": "partial_anonymity"})
	_, _ = env.post(t, env.alice, idOf(partial), map[string]any{"message": "<p>named</p>"})
	_, _ = env.post(t, env.carol, idOf(partial), map[string]any{"message": "<p>hidden</p>", "anonymous": true})
	_, view := env.viewV2(t, env.bob, idOf(partial))
	named, hidden := 0, 0
	for _, raw := range view["entries"].([]any) {
		e := raw.(map[string]any)
		if e["user_id"] == nil {
			hidden++
		} else {
			named++
		}
	}
	assert.Equal(t, 1, named)
	assert.Equal(t, 1, hidden)

	// Section-specific: Bob (section B) can't see a section A topic.
	secTopic := env.createTopic(t, map[string]any{"title": "Section A only", "is_section_specific": true, "section_ids": []uint{env.secA.ID}})
	status, _ = env.viewV2(t, env.alice, idOf(secTopic))
	assert.Equal(t, http.StatusOK, status)
	status, _ = env.viewV2(t, env.bob, idOf(secTopic))
	assert.Equal(t, http.StatusNotFound, status)
	for _, tp := range env.index(t, env.bob, "") {
		assert.NotEqual(t, idOf(secTopic), idOf(tp))
	}
	var staffRow map[string]any
	for _, tp := range env.index(t, env.teacher, "") {
		if idOf(tp) == idOf(secTopic) {
			staffRow = tp
		}
	}
	require.NotNil(t, staffRow)
	assert.Len(t, staffRow["section_ids"], 1)

	// Sections must belong to the course; one is required.
	status, _ = env.req(t, env.teacher, http.MethodPost, env.topicsPath(""), map[string]any{"discussion_topic": map[string]any{
		"title": "No sections", "is_section_specific": true,
	}})
	assert.Equal(t, http.StatusBadRequest, status)
}

// G088: subscriptions and @mentions notify.
func TestDiscussionParity_SubscriptionsAndMentions(t *testing.T) {
	env := newDiscParityEnv(t)
	topic := env.createTopic(t, map[string]any{"title": "Lab safety"})
	tid := idOf(topic)

	status, first := env.post(t, env.alice, tid, map[string]any{"message": "<p>Goggles?</p>"})
	require.Equal(t, http.StatusCreated, status)
	// The teacher (author) is subscribed by default and hears about it.
	eventually(t, func() bool { return env.notifier.of("discussion_reply")[env.teacher.ID] > 0 }, "topic author notified")

	// Carol unsubscribes; Bob posts; Alice (a poster) is notified, Carol is not.
	status, _ = env.req(t, env.carol, http.MethodPut, env.topicsPath(fmt.Sprintf("/%d/subscription", tid)), map[string]any{"subscribed": false})
	require.Equal(t, http.StatusOK, status)
	env.notifier.reset()
	status, _ = env.req(t, env.bob, http.MethodPost, env.topicsPath(fmt.Sprintf("/%d/entries/%d/replies", tid, idOf(first))), map[string]any{"message": "<p>Yes, always.</p>"})
	require.Equal(t, http.StatusCreated, status)
	eventually(t, func() bool { return env.notifier.of("discussion_reply")[env.alice.ID] > 0 }, "a subscribed poster is notified")
	time.Sleep(100 * time.Millisecond)
	got := env.notifier.of("discussion_reply")
	assert.Zero(t, got[env.carol.ID], "an unsubscribed user is not notified")
	assert.Zero(t, got[env.bob.ID], "nobody is notified of their own post")

	// @mention: Carol is named and picked; Bob is picked but not named.
	env.notifier.reset()
	status, _ = env.post(t, env.alice, tid, map[string]any{
		"message":            "<p>Thanks @Carol Poe and everyone</p>",
		"mentioned_user_ids": []uint{env.carol.ID, env.bob.ID},
	})
	require.Equal(t, http.StatusCreated, status)
	mentions := env.notifier.of("discussion_mention")
	assert.Equal(t, 1, mentions[env.carol.ID])
	assert.Zero(t, mentions[env.bob.ID], "picking without naming doesn't notify")

	// Mentions are limited to people who can see the topic.
	secTopic := env.createTopic(t, map[string]any{"title": "Section A only", "is_section_specific": true, "section_ids": []uint{env.secA.ID}})
	env.notifier.reset()
	status, _ = env.post(t, env.alice, idOf(secTopic), map[string]any{
		"message": "<p>@Bob Roe look</p>", "mentioned_user_ids": []uint{env.bob.ID},
	})
	require.Equal(t, http.StatusCreated, status)
	assert.Zero(t, env.notifier.of("discussion_mention")[env.bob.ID])

	status, out := env.req(t, env.alice, http.MethodGet, env.topicsPath(fmt.Sprintf("/%d/mentionable_users?search=poe", tid)), nil)
	require.Equal(t, http.StatusOK, status)
	assert.Contains(t, string(out), "Carol Poe")
	assert.NotContains(t, string(out), "Bob Roe")
}

// G086: file attachments on posts.
func TestDiscussionParity_Attachments(t *testing.T) {
	env := newDiscParityEnv(t)
	topic := env.createTopic(t, map[string]any{"title": "Share files"})
	tid := idOf(topic)

	upload := func(user *models.User) (int, map[string]any) {
		var buf bytes.Buffer
		w := multipart.NewWriter(&buf)
		hdr := textproto.MIMEHeader{}
		hdr.Set("Content-Disposition", `form-data; name="file"; filename="notes.txt"`)
		hdr.Set("Content-Type", "text/plain")
		fw, err := w.CreatePart(hdr)
		require.NoError(t, err)
		_, _ = fw.Write([]byte("made-up lab notes"))
		require.NoError(t, w.Close())
		r := httptest.NewRequest(http.MethodPost, env.topicsPath(fmt.Sprintf("/%d/attachments", tid)), &buf)
		r.Header.Set("Content-Type", w.FormDataContentType())
		r.Header.Set("X-Test-User", fmt.Sprint(user.ID))
		resp, err := env.app.Test(r, -1)
		require.NoError(t, err)
		body, _ := io.ReadAll(resp.Body)
		var m map[string]any
		_ = json.Unmarshal(body, &m)
		return resp.StatusCode, m
	}

	status, file := upload(env.alice)
	require.Equal(t, http.StatusCreated, status, "%v", file)
	fileID := idOf(file)

	// Bob can't attach Alice's file.
	status, _ = env.post(t, env.bob, tid, map[string]any{"message": "<p>mine now</p>", "attachment_id": fileID})
	assert.Equal(t, http.StatusBadRequest, status)

	status, entry := env.post(t, env.alice, tid, map[string]any{"message": "<p>see attached</p>", "attachment_id": fileID})
	require.Equal(t, http.StatusCreated, status)
	att, ok := entry["attachment"].(map[string]any)
	require.True(t, ok, "%v", entry)
	assert.Equal(t, "notes.txt", att["display_name"])

	status, body := env.req(t, env.bob, http.MethodGet, env.topicsPath(fmt.Sprintf("/%d/entries/%d/attachment", tid, idOf(entry))), nil)
	require.Equal(t, http.StatusOK, status)
	assert.Equal(t, "made-up lab notes", string(body))

	_, view := env.viewV2(t, env.bob, tid)
	e := view["entries"].([]any)[0].(map[string]any)
	assert.NotNil(t, e["attachment"])
}

// G089: index counts, unread filter, search and pagination.
func TestDiscussionParity_Index(t *testing.T) {
	env := newDiscParityEnv(t)
	t1 := env.createTopic(t, map[string]any{"title": "Photosynthesis questions"})
	t2 := env.createTopic(t, map[string]any{"title": "Cell division", "message": "<p>mitosis and meiosis</p>"})
	env.createTopic(t, map[string]any{"title": "Field trip"})

	status, _ := env.post(t, env.bob, idOf(t1), map[string]any{"message": "<p>why green?</p>"})
	require.Equal(t, http.StatusCreated, status)
	status, _ = env.post(t, env.bob, idOf(t1), map[string]any{"message": "<p>and light?</p>"})
	require.Equal(t, http.StatusCreated, status)

	rows := env.index(t, env.alice, "")
	require.Len(t, rows, 3)
	for _, r := range rows {
		if idOf(r) == idOf(t1) {
			assert.EqualValues(t, 2, r["discussion_subentry_count"])
			assert.EqualValues(t, 2, r["unread_count"])
			assert.Equal(t, "unread", r["read_state"])
		}
	}

	unread := env.index(t, env.alice, "?filter_by=unread")
	require.Len(t, unread, 1)
	assert.Equal(t, idOf(t1), idOf(unread[0]))
	assert.Empty(t, env.index(t, env.bob, "?filter_by=unread"), "own posts are never unread")

	found := env.index(t, env.alice, "?search_term=meiosis")
	require.Len(t, found, 1)
	assert.Equal(t, idOf(t2), idOf(found[0]))
	assert.Empty(t, env.index(t, env.alice, "?search_term=%25"), "LIKE wildcards are literal")

	page := env.index(t, env.alice, "?per_page=2&page=2")
	assert.Len(t, page, 1)
}
