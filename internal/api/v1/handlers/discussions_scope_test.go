package handlers_test

// Regression tests for the comms-discussions parity lane (B048-B052,
// B156-B160, B169). They run the real Discussion / DiscussionV2 /
// DiscussionCheckpoint handlers + services over an in-memory store that
// applies the same tenant scoping the Postgres repos do (a row resolves
// only when accountID is 0 or matches the owning course's account), so a
// handler that forgets the tenant or course tie shows up as a leak here.

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"sort"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/stretchr/testify/mock"

	"github.com/EduThemes/paper-lms/internal/api/v1/handlers"
	"github.com/EduThemes/paper-lms/internal/api/v1/middleware"
	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/repository"
	"github.com/EduThemes/paper-lms/internal/service"
	"github.com/EduThemes/paper-lms/internal/testutil"
	"github.com/EduThemes/paper-lms/internal/testutil/mocks"
)

var errDiscNotFound = errors.New("record not found")

// ---------------------------------------------------------------------------
// In-memory store
// ---------------------------------------------------------------------------

type discStore struct {
	courseAccount map[uint]uint
	topics        map[uint]*models.DiscussionTopic
	entries       map[uint]*models.DiscussionEntry
	nextEntryID   uint
	ratings       map[[2]uint]int // (entry, user) -> rating
	versions      map[uint][]models.DiscussionEntryVersion
	topicParts    map[[2]uint]*models.DiscussionTopicParticipant
	readEntries   map[[2]uint]bool // (entry, user)
	checkpoints   map[uint]*models.DiscussionCheckpoint
}

func (s *discStore) topicAccount(topicID uint) uint {
	t, ok := s.topics[topicID]
	if !ok {
		return 0
	}
	return s.courseAccount[t.CourseID]
}

func (s *discStore) entryVisible(e *models.DiscussionEntry, accountID uint) bool {
	return accountID == 0 || s.topicAccount(e.DiscussionTopicID) == accountID
}

type fakeTopicRepo struct{ s *discStore }

func (r fakeTopicRepo) Create(_ context.Context, t *models.DiscussionTopic) error {
	r.s.topics[t.ID] = t
	return nil
}
func (r fakeTopicRepo) FindByID(_ context.Context, id, accountID uint) (*models.DiscussionTopic, error) {
	t, ok := r.s.topics[id]
	if !ok || (accountID != 0 && r.s.courseAccount[t.CourseID] != accountID) {
		return nil, errDiscNotFound
	}
	cp := *t
	return &cp, nil
}
func (r fakeTopicRepo) Update(_ context.Context, t *models.DiscussionTopic) error {
	cp := *t
	r.s.topics[t.ID] = &cp
	return nil
}
func (r fakeTopicRepo) Delete(_ context.Context, id uint) error {
	if t, ok := r.s.topics[id]; ok {
		t.WorkflowState = models.DiscussionTopicDeleted
	}
	return nil
}
func (r fakeTopicRepo) ListByCourseID(_ context.Context, courseID, _ uint, p repository.PaginationParams) (*repository.PaginatedResult[models.DiscussionTopic], error) {
	var out []models.DiscussionTopic
	for _, t := range r.s.topics {
		if t.CourseID == courseID {
			out = append(out, *t)
		}
	}
	return &repository.PaginatedResult[models.DiscussionTopic]{Items: out, TotalCount: int64(len(out)), Page: p.Page, PerPage: p.PerPage}, nil
}

type fakeEntryRepo struct{ s *discStore }

func (r fakeEntryRepo) Create(_ context.Context, e *models.DiscussionEntry) error {
	r.s.nextEntryID++
	e.ID = r.s.nextEntryID
	e.CreatedAt = time.Now()
	cp := *e
	r.s.entries[e.ID] = &cp
	return nil
}
func (r fakeEntryRepo) FindByID(_ context.Context, id, accountID uint) (*models.DiscussionEntry, error) {
	e, ok := r.s.entries[id]
	if !ok || !r.s.entryVisible(e, accountID) {
		return nil, errDiscNotFound
	}
	cp := *e
	return &cp, nil
}
func (r fakeEntryRepo) Update(_ context.Context, e *models.DiscussionEntry) error {
	cp := *e
	r.s.entries[e.ID] = &cp
	return nil
}
func (r fakeEntryRepo) Delete(_ context.Context, id uint) error {
	if e, ok := r.s.entries[id]; ok {
		e.WorkflowState = "deleted"
	}
	return nil
}
func (r fakeEntryRepo) sorted(filter func(*models.DiscussionEntry) bool) []models.DiscussionEntry {
	var out []models.DiscussionEntry
	for _, e := range r.s.entries {
		if filter(e) {
			out = append(out, *e)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}
func (r fakeEntryRepo) ListByTopicID(_ context.Context, topicID, accountID uint, p repository.PaginationParams) (*repository.PaginatedResult[models.DiscussionEntry], error) {
	items := r.sorted(func(e *models.DiscussionEntry) bool {
		return e.DiscussionTopicID == topicID && e.WorkflowState != "deleted" && r.s.entryVisible(e, accountID)
	})
	return &repository.PaginatedResult[models.DiscussionEntry]{Items: items, TotalCount: int64(len(items)), Page: p.Page, PerPage: p.PerPage}, nil
}
func (r fakeEntryRepo) ListReplies(_ context.Context, entryID, accountID uint, p repository.PaginationParams) (*repository.PaginatedResult[models.DiscussionEntry], error) {
	items := r.sorted(func(e *models.DiscussionEntry) bool {
		return e.ParentID != nil && *e.ParentID == entryID && e.WorkflowState != "deleted" && r.s.entryVisible(e, accountID)
	})
	return &repository.PaginatedResult[models.DiscussionEntry]{Items: items, TotalCount: int64(len(items)), Page: p.Page, PerPage: p.PerPage}, nil
}
func (r fakeEntryRepo) ListAllByTopicID(_ context.Context, topicID, accountID uint) ([]models.DiscussionEntry, error) {
	return r.sorted(func(e *models.DiscussionEntry) bool {
		return e.DiscussionTopicID == topicID && e.WorkflowState != "deleted" && r.s.entryVisible(e, accountID)
	}), nil
}
func (r fakeEntryRepo) ListAllByTopicIDIncludingDeleted(_ context.Context, topicID, accountID uint) ([]models.DiscussionEntry, error) {
	return r.sorted(func(e *models.DiscussionEntry) bool {
		return e.DiscussionTopicID == topicID && r.s.entryVisible(e, accountID)
	}), nil
}

type fakeDiscRatingRepo struct{ s *discStore }

func (r fakeDiscRatingRepo) Upsert(_ context.Context, rt *models.DiscussionEntryRating) error {
	r.s.ratings[[2]uint{rt.DiscussionEntryID, rt.UserID}] = rt.Rating
	return nil
}
func (r fakeDiscRatingRepo) Delete(_ context.Context, entryID, userID uint) error {
	delete(r.s.ratings, [2]uint{entryID, userID})
	return nil
}
func (r fakeDiscRatingRepo) SumByEntryID(_ context.Context, entryID uint) (int64, int64, error) {
	var count, sum int64
	for k, v := range r.s.ratings {
		if k[0] == entryID {
			count++
			sum += int64(v)
		}
	}
	return count, sum, nil
}
func (r fakeDiscRatingRepo) ListUserRatingsByTopic(_ context.Context, topicID, userID uint) (map[uint]int, error) {
	out := map[uint]int{}
	for k, v := range r.s.ratings {
		if e, ok := r.s.entries[k[0]]; ok && e.DiscussionTopicID == topicID && k[1] == userID {
			out[k[0]] = v
		}
	}
	return out, nil
}

type fakeEntryPartRepo struct{ s *discStore }

func (r fakeEntryPartRepo) Create(_ context.Context, p *models.DiscussionEntryParticipant) error {
	r.s.readEntries[[2]uint{p.DiscussionEntryID, p.UserID}] = p.ReadAt != nil
	return nil
}
func (r fakeEntryPartRepo) FindByEntryAndUser(_ context.Context, entryID, userID, _ uint) (*models.DiscussionEntryParticipant, error) {
	return nil, errDiscNotFound
}
func (r fakeEntryPartRepo) MarkAsRead(_ context.Context, entryID, userID uint) error {
	r.s.readEntries[[2]uint{entryID, userID}] = true
	return nil
}
func (r fakeEntryPartRepo) MarkTopicAsRead(_ context.Context, topicID, userID uint) error {
	for _, e := range r.s.entries {
		if e.DiscussionTopicID == topicID {
			r.s.readEntries[[2]uint{e.ID, userID}] = true
		}
	}
	return nil
}
func (r fakeEntryPartRepo) unread(topicID, userID uint) []uint {
	var ids []uint
	for _, e := range r.s.entries {
		if e.DiscussionTopicID == topicID && e.WorkflowState != "deleted" && e.UserID != userID && !r.s.readEntries[[2]uint{e.ID, userID}] {
			ids = append(ids, e.ID)
		}
	}
	return ids
}
func (r fakeEntryPartRepo) CountUnreadByTopic(_ context.Context, topicID, userID uint) (int64, error) {
	return int64(len(r.unread(topicID, userID))), nil
}
func (r fakeEntryPartRepo) ListUnreadByTopic(_ context.Context, topicID, userID uint) ([]uint, error) {
	return r.unread(topicID, userID), nil
}

type fakeTopicPartRepo struct{ s *discStore }

func (r fakeTopicPartRepo) Upsert(_ context.Context, p *models.DiscussionTopicParticipant) error {
	cp := *p
	r.s.topicParts[[2]uint{p.DiscussionTopicID, p.UserID}] = &cp
	return nil
}
func (r fakeTopicPartRepo) FindByTopicAndUser(_ context.Context, topicID, userID uint) (*models.DiscussionTopicParticipant, error) {
	p, ok := r.s.topicParts[[2]uint{topicID, userID}]
	if !ok {
		return nil, errDiscNotFound
	}
	cp := *p
	return &cp, nil
}
func (r fakeTopicPartRepo) ListByTopicID(_ context.Context, _ uint) ([]models.DiscussionTopicParticipant, error) {
	return nil, nil
}
func (r fakeTopicPartRepo) UpdateSubscription(_ context.Context, topicID, userID uint, subscribed bool) error {
	key := [2]uint{topicID, userID}
	if p, ok := r.s.topicParts[key]; ok {
		p.Subscribed = subscribed
		return nil
	}
	r.s.topicParts[key] = &models.DiscussionTopicParticipant{DiscussionTopicID: topicID, UserID: userID, Subscribed: subscribed}
	return nil
}

type fakeVersionRepo struct{ s *discStore }

func (r fakeVersionRepo) Create(_ context.Context, v *models.DiscussionEntryVersion) error {
	r.s.versions[v.DiscussionEntryID] = append(r.s.versions[v.DiscussionEntryID], *v)
	return nil
}
func (r fakeVersionRepo) ListByEntryID(_ context.Context, entryID uint) ([]models.DiscussionEntryVersion, error) {
	return append([]models.DiscussionEntryVersion(nil), r.s.versions[entryID]...), nil
}
func (r fakeVersionRepo) CountByEntryID(_ context.Context, entryID uint) (int64, error) {
	return int64(len(r.s.versions[entryID])), nil
}

type fakeCheckpointRepo struct{ s *discStore }

func (r fakeCheckpointRepo) Create(_ context.Context, cp *models.DiscussionCheckpoint) error {
	r.s.checkpoints[cp.ID] = cp
	return nil
}
func (r fakeCheckpointRepo) FindByID(_ context.Context, id, accountID uint) (*models.DiscussionCheckpoint, error) {
	cp, ok := r.s.checkpoints[id]
	if !ok || (accountID != 0 && r.s.topicAccount(cp.DiscussionTopicID) != accountID) {
		return nil, errDiscNotFound
	}
	c := *cp
	return &c, nil
}
func (r fakeCheckpointRepo) Update(_ context.Context, cp *models.DiscussionCheckpoint) error {
	c := *cp
	r.s.checkpoints[cp.ID] = &c
	return nil
}
func (r fakeCheckpointRepo) Delete(_ context.Context, id uint) error {
	if cp, ok := r.s.checkpoints[id]; ok {
		cp.WorkflowState = "deleted"
	}
	return nil
}
func (r fakeCheckpointRepo) ListByTopicID(_ context.Context, topicID, _ uint) ([]models.DiscussionCheckpoint, error) {
	var out []models.DiscussionCheckpoint
	for _, cp := range r.s.checkpoints {
		if cp.DiscussionTopicID == topicID && cp.WorkflowState != "deleted" {
			out = append(out, *cp)
		}
	}
	return out, nil
}
func (r fakeCheckpointRepo) DeleteByTopicID(_ context.Context, _ uint) error { return nil }

type fakeCheckpointSubRepo struct{}

func (fakeCheckpointSubRepo) UpsertSubmission(_ context.Context, _ *models.DiscussionCheckpointSubmission) error {
	return nil
}
func (fakeCheckpointSubRepo) FindByCheckpointAndUser(_ context.Context, _, _, _ uint) (*models.DiscussionCheckpointSubmission, error) {
	return nil, errDiscNotFound
}
func (fakeCheckpointSubRepo) ListByCheckpoint(_ context.Context, _, _ uint) ([]models.DiscussionCheckpointSubmission, error) {
	return nil, nil
}
func (fakeCheckpointSubRepo) ListByUserAndTopic(_ context.Context, _, _, _ uint) ([]models.DiscussionCheckpointSubmission, error) {
	return nil, nil
}

// ---------------------------------------------------------------------------
// Fixture
// ---------------------------------------------------------------------------

const (
	dAcctA uint = 1
	dAcctB uint = 2

	dCourseA1 uint = 1 // tenant A
	dCourseA2 uint = 2 // tenant A, a different course
	dCourseB  uint = 3 // tenant B

	dTeacher  uint = 100 // teaches course 1
	dStudent1 uint = 200 // student in course 1, author of entry 1
	dStudent2 uint = 201 // student in course 1

	dTopicOpen      uint = 10 // course 1
	dTopicLocked    uint = 11 // course 1, locked
	dTopicDelayed   uint = 12 // course 1, delayed_post_at in the future
	dTopicRated     uint = 13 // course 1, allow_rating
	dTopicGraderOnl uint = 14 // course 1, allow_rating + only_graders_can_rate
	dTopicDeleted   uint = 15 // course 1, soft-deleted
	dTopicDraft     uint = 16 // course 1, unpublished (e.g. from a course copy)
	dTopicReleased  uint = 17 // course 1, post_delayed with delayed_post_at in the past
	dTopicOtherCrs  uint = 20 // course 2 (same tenant)
	dTopicOtherTen  uint = 30 // course 3 (tenant B)

	dEntryRoot       uint = 1 // topic 10, by student 1
	dEntryReply      uint = 2 // topic 10, reply to 1, by student 2
	dEntryOtherCrs   uint = 3 // topic 20
	dEntryOtherTen   uint = 4 // topic 30
	dEntryRated      uint = 5 // topic 13, by student 1
	dEntryGraderOnly uint = 6 // topic 14, by student 1
)

type discEnv struct {
	store *discStore
	app   *fiber.App
}

func newDiscEnv(t *testing.T, callerID uint, enrollmentType string) *discEnv {
	t.Helper()
	future := time.Now().Add(48 * time.Hour)
	past := time.Now().Add(-48 * time.Hour)
	s := &discStore{
		courseAccount: map[uint]uint{dCourseA1: dAcctA, dCourseA2: dAcctA, dCourseB: dAcctB},
		topics:        map[uint]*models.DiscussionTopic{},
		entries:       map[uint]*models.DiscussionEntry{},
		ratings:       map[[2]uint]int{},
		versions:      map[uint][]models.DiscussionEntryVersion{},
		topicParts:    map[[2]uint]*models.DiscussionTopicParticipant{},
		readEntries:   map[[2]uint]bool{},
		checkpoints:   map[uint]*models.DiscussionCheckpoint{},
	}
	addTopic := func(id, course uint, mut func(*models.DiscussionTopic)) {
		tp := &models.DiscussionTopic{ID: id, CourseID: course, UserID: dTeacher, Title: fmt.Sprintf("Topic %d", id), WorkflowState: models.DiscussionTopicActive}
		if mut != nil {
			mut(tp)
		}
		s.topics[id] = tp
	}
	addTopic(dTopicOpen, dCourseA1, nil)
	addTopic(dTopicLocked, dCourseA1, func(tp *models.DiscussionTopic) { tp.Locked = true })
	addTopic(dTopicDelayed, dCourseA1, func(tp *models.DiscussionTopic) { tp.DelayedPostAt = &future })
	addTopic(dTopicRated, dCourseA1, func(tp *models.DiscussionTopic) { tp.AllowRating = true })
	addTopic(dTopicGraderOnl, dCourseA1, func(tp *models.DiscussionTopic) { tp.AllowRating = true; tp.OnlyGradersCanRate = true })
	addTopic(dTopicDeleted, dCourseA1, func(tp *models.DiscussionTopic) { tp.WorkflowState = models.DiscussionTopicDeleted })
	addTopic(dTopicDraft, dCourseA1, func(tp *models.DiscussionTopic) { tp.WorkflowState = models.DiscussionTopicUnpublished })
	addTopic(dTopicReleased, dCourseA1, func(tp *models.DiscussionTopic) {
		tp.WorkflowState = models.DiscussionTopicPostDelayed
		tp.DelayedPostAt = &past
	})
	addTopic(dTopicOtherCrs, dCourseA2, nil)
	addTopic(dTopicOtherTen, dCourseB, nil)

	root := dEntryRoot
	addEntry := func(id, topic, user uint, parent *uint, msg string) {
		s.entries[id] = &models.DiscussionEntry{ID: id, DiscussionTopicID: topic, UserID: user, ParentID: parent, Message: msg, WorkflowState: "active"}
	}
	addEntry(dEntryRoot, dTopicOpen, dStudent1, nil, "<p>original</p>")
	addEntry(dEntryReply, dTopicOpen, dStudent2, &root, "<p>reply</p>")
	addEntry(dEntryOtherCrs, dTopicOtherCrs, 300, nil, "<p>other course</p>")
	addEntry(dEntryOtherTen, dTopicOtherTen, 900, nil, "<p>other tenant</p>")
	addEntry(dEntryRated, dTopicRated, dStudent1, nil, "<p>rate me</p>")
	addEntry(dEntryGraderOnly, dTopicGraderOnl, dStudent1, nil, "<p>graders only</p>")
	s.nextEntryID = 100

	s.checkpoints[1] = &models.DiscussionCheckpoint{ID: 1, DiscussionTopicID: dTopicOpen, CheckpointType: models.CheckpointTypeReplyToTopic, WorkflowState: "active"}
	s.checkpoints[2] = &models.DiscussionCheckpoint{ID: 2, DiscussionTopicID: dTopicOtherTen, CheckpointType: models.CheckpointTypeReplyToTopic, WorkflowState: "active"}

	topicRepo := fakeTopicRepo{s}
	entryRepo := fakeEntryRepo{s}
	ratingRepo := fakeDiscRatingRepo{s}

	userRepo := new(mocks.MockUserRepository)
	userRepo.On("FindByID", mock.Anything, mock.Anything, mock.Anything).
		Return(&models.User{ID: 1, Name: "Jane Doe"}, nil).Maybe()

	v1 := service.NewDiscussionService(topicRepo, entryRepo, ratingRepo)
	v2 := service.NewDiscussionV2Service(topicRepo, entryRepo, ratingRepo, fakeEntryPartRepo{s}, fakeTopicPartRepo{s}, fakeVersionRepo{s}, userRepo)
	cps := service.NewDiscussionCheckpointService(fakeCheckpointRepo{s}, fakeCheckpointSubRepo{}, topicRepo, entryRepo, new(mocks.MockAssignmentRepository))

	th := handlers.NewDiscussionHandler(v1)
	eh := handlers.NewDiscussionEntryHandler(v1).SetVersionHistory(v2)
	vh := handlers.NewDiscussionV2Handler(v2)
	ch := handlers.NewDiscussionCheckpointHandler(cps)

	app := testutil.SetupTestApp()
	app.Use(func(c *fiber.Ctx) error {
		c.Locals("user_id", callerID)
		c.Locals("account_id", dAcctA)
		if enrollmentType != "" {
			c.Locals("enrollment_type", enrollmentType)
		}
		return c.Next()
	}, middleware.PaginationParams())

	base := "/courses/:course_id/discussion_topics"
	app.Get(base, th.ListTopics)
	app.Get(base+"/:topic_id", th.GetTopic)
	app.Put(base+"/:topic_id", th.UpdateTopic)
	app.Delete(base+"/:topic_id", th.DeleteTopic)
	app.Get(base+"/:topic_id/view", th.GetFullView)
	app.Get(base+"/:topic_id/entries", eh.ListEntries)
	app.Post(base+"/:topic_id/entries", eh.CreateEntry)
	app.Put(base+"/:topic_id/entries/:id", eh.UpdateEntry)
	app.Delete(base+"/:topic_id/entries/:id", eh.DeleteEntry)
	app.Get(base+"/:topic_id/entries/:entry_id/replies", eh.ListReplies)
	app.Post(base+"/:topic_id/entries/:entry_id/replies", eh.CreateReply)
	app.Post(base+"/:topic_id/entries/:entry_id/rating", eh.RateEntry)
	app.Get(base+"/:topic_id/view_v2", vh.GetFullViewV2)
	app.Post(base+"/:topic_id/entries/:entry_id/read", vh.MarkEntryRead)
	app.Post(base+"/:topic_id/mark_all_read", vh.MarkTopicRead)
	app.Get(base+"/:topic_id/unread_count", vh.GetUnreadCount)
	app.Put(base+"/:topic_id/subscription", vh.ToggleSubscription)
	app.Get(base+"/:topic_id/entries/:entry_id/versions", vh.GetEntryVersions)
	app.Put(base+"/:topic_id/entries/:entry_id/v2", vh.UpdateEntryV2)
	app.Get(base+"/:topic_id/checkpoints", ch.ListCheckpoints)
	app.Get(base+"/:topic_id/checkpoints/progress", ch.GetUserProgress)
	app.Put(base+"/:topic_id/checkpoints/:id", ch.UpdateCheckpoint)
	app.Delete(base+"/:topic_id/checkpoints/:id", ch.DeleteCheckpoint)

	return &discEnv{store: s, app: app}
}

func (e *discEnv) do(t *testing.T, method, path string, body any) (int, []byte) {
	t.Helper()
	var r io.Reader
	if body != nil {
		b, _ := json.Marshal(body)
		r = bytes.NewReader(b)
	}
	resp := testutil.MakeRequest(e.app, method, path, r)
	defer resp.Body.Close()
	out, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, out
}

func topicPath(course, topic uint, rest string) string {
	return fmt.Sprintf("/courses/%d/discussion_topics/%d%s", course, topic, rest)
}

// ---------------------------------------------------------------------------
// B048 — topic delete/update tenant + course scoping
// ---------------------------------------------------------------------------

func TestDiscussionTopic_DeleteAndUpdate_ScopedToCourseAndTenant(t *testing.T) {
	env := newDiscEnv(t, dTeacher, "TeacherEnrollment")

	for _, tc := range []struct {
		name  string
		topic uint
	}{
		{"other tenant", dTopicOtherTen},
		{"other course same tenant", dTopicOtherCrs},
	} {
		t.Run("delete "+tc.name, func(t *testing.T) {
			status, _ := env.do(t, http.MethodDelete, topicPath(dCourseA1, tc.topic, ""), nil)
			if status != http.StatusNotFound {
				t.Fatalf("status = %d, want 404", status)
			}
			if env.store.topics[tc.topic].WorkflowState == models.DiscussionTopicDeleted {
				t.Fatalf("topic %d was deleted through another course's route", tc.topic)
			}
		})
		t.Run("update "+tc.name, func(t *testing.T) {
			status, _ := env.do(t, http.MethodPut, topicPath(dCourseA1, tc.topic, ""), map[string]any{
				"discussion_topic": map[string]any{"title": "hijacked"},
			})
			if status != http.StatusNotFound {
				t.Fatalf("status = %d, want 404", status)
			}
			if env.store.topics[tc.topic].Title == "hijacked" {
				t.Fatalf("topic %d was rewritten through another course's route", tc.topic)
			}
		})
	}

	t.Run("own course topic deletes", func(t *testing.T) {
		status, _ := env.do(t, http.MethodDelete, topicPath(dCourseA1, dTopicOpen, ""), nil)
		if status != http.StatusOK {
			t.Fatalf("status = %d, want 200", status)
		}
		if env.store.topics[dTopicOpen].WorkflowState != models.DiscussionTopicDeleted {
			t.Fatalf("own topic was not deleted")
		}
	})
}

// ---------------------------------------------------------------------------
// B049 — V2 edit owner/tenant checks + version attribution
// ---------------------------------------------------------------------------

func TestDiscussionEntryV2Edit_OwnerAndTenantEnforced(t *testing.T) {
	t.Run("another student cannot edit", func(t *testing.T) {
		env := newDiscEnv(t, dStudent2, "StudentEnrollment")
		status, _ := env.do(t, http.MethodPut, topicPath(dCourseA1, dTopicOpen, fmt.Sprintf("/entries/%d/v2", dEntryRoot)), map[string]string{"message": "defaced"})
		if status != http.StatusForbidden {
			t.Fatalf("status = %d, want 403", status)
		}
		if env.store.entries[dEntryRoot].Message != "<p>original</p>" {
			t.Fatalf("entry was overwritten by a non-author")
		}
	})

	t.Run("cross-tenant entry is 404", func(t *testing.T) {
		env := newDiscEnv(t, dStudent1, "StudentEnrollment")
		status, _ := env.do(t, http.MethodPut, topicPath(dCourseA1, dTopicOpen, fmt.Sprintf("/entries/%d/v2", dEntryOtherTen)), map[string]string{"message": "x"})
		if status != http.StatusNotFound {
			t.Fatalf("status = %d, want 404", status)
		}
		if env.store.entries[dEntryOtherTen].Message != "<p>other tenant</p>" {
			t.Fatalf("cross-tenant entry was overwritten")
		}
	})

	t.Run("entry on a different topic is 404", func(t *testing.T) {
		env := newDiscEnv(t, dTeacher, "TeacherEnrollment")
		status, _ := env.do(t, http.MethodPut, topicPath(dCourseA1, dTopicOpen, fmt.Sprintf("/entries/%d/v2", dEntryOtherCrs)), map[string]string{"message": "x"})
		if status != http.StatusNotFound {
			t.Fatalf("status = %d, want 404", status)
		}
	})

	t.Run("author edit records original and new versions", func(t *testing.T) {
		env := newDiscEnv(t, dStudent1, "StudentEnrollment")
		status, body := env.do(t, http.MethodPut, topicPath(dCourseA1, dTopicOpen, fmt.Sprintf("/entries/%d/v2", dEntryRoot)), map[string]string{"message": "<p>edited</p>"})
		if status != http.StatusOK {
			t.Fatalf("status = %d, want 200 (%s)", status, body)
		}
		vs := env.store.versions[dEntryRoot]
		if len(vs) != 2 {
			t.Fatalf("versions = %d, want 2 (original + edit)", len(vs))
		}
		if vs[0].Version != 1 || vs[0].Message != "<p>original</p>" || vs[0].UserID != dStudent1 {
			t.Errorf("version 1 = %+v, want original message by the author", vs[0])
		}
		if vs[1].Version != 2 || vs[1].Message != "<p>edited</p>" || vs[1].UserID != dStudent1 {
			t.Errorf("version 2 = %+v, want edited message by the editor", vs[1])
		}
	})

	t.Run("teacher edit is attributed to the teacher", func(t *testing.T) {
		env := newDiscEnv(t, dTeacher, "TeacherEnrollment")
		status, _ := env.do(t, http.MethodPut, topicPath(dCourseA1, dTopicOpen, fmt.Sprintf("/entries/%d/v2", dEntryRoot)), map[string]string{"message": "<p>moderated</p>"})
		if status != http.StatusOK {
			t.Fatalf("status = %d, want 200", status)
		}
		vs := env.store.versions[dEntryRoot]
		if len(vs) != 2 || vs[0].UserID != dStudent1 || vs[1].UserID != dTeacher {
			t.Fatalf("versions = %+v, want original by author then edit by teacher", vs)
		}
	})
}

// ---------------------------------------------------------------------------
// B050 — reads are tenant + course scoped
// ---------------------------------------------------------------------------

func TestDiscussionReads_RefuseForeignTopicsAndEntries(t *testing.T) {
	env := newDiscEnv(t, dStudent1, "StudentEnrollment")

	foreign := []string{
		topicPath(dCourseA1, dTopicOtherTen, ""),
		topicPath(dCourseA1, dTopicOtherTen, "/view"),
		topicPath(dCourseA1, dTopicOtherTen, "/view_v2"),
		topicPath(dCourseA1, dTopicOtherTen, "/entries"),
		topicPath(dCourseA1, dTopicOtherTen, "/unread_count"),
		topicPath(dCourseA1, dTopicOtherTen, "/checkpoints"),
		topicPath(dCourseA1, dTopicOtherCrs, "/view_v2"),
		topicPath(dCourseA1, dTopicOtherCrs, "/entries"),
		topicPath(dCourseA1, dTopicDeleted, "/view_v2"),
		topicPath(dCourseA1, dTopicDelayed, "/view_v2"), // not yet posted: hidden from students
		topicPath(dCourseA1, dTopicOpen, fmt.Sprintf("/entries/%d/versions", dEntryOtherTen)),
		topicPath(dCourseA1, dTopicOpen, fmt.Sprintf("/entries/%d/replies", dEntryOtherCrs)),
	}
	for _, p := range foreign {
		t.Run(p, func(t *testing.T) {
			status, body := env.do(t, http.MethodGet, p, nil)
			if status != http.StatusNotFound {
				t.Fatalf("status = %d, want 404 (%s)", status, body)
			}
		})
	}

	t.Run("own course topic still loads", func(t *testing.T) {
		status, _ := env.do(t, http.MethodGet, topicPath(dCourseA1, dTopicOpen, "/view_v2"), nil)
		if status != http.StatusOK {
			t.Fatalf("status = %d, want 200", status)
		}
	})

	t.Run("teacher can see a delayed topic", func(t *testing.T) {
		tenv := newDiscEnv(t, dTeacher, "TeacherEnrollment")
		status, _ := tenv.do(t, http.MethodGet, topicPath(dCourseA1, dTopicDelayed, "/view_v2"), nil)
		if status != http.StatusOK {
			t.Fatalf("status = %d, want 200", status)
		}
	})

	t.Run("marking a foreign entry read is refused", func(t *testing.T) {
		status, _ := env.do(t, http.MethodPost, topicPath(dCourseA1, dTopicOpen, fmt.Sprintf("/entries/%d/read", dEntryOtherTen)), nil)
		if status != http.StatusNotFound {
			t.Fatalf("status = %d, want 404", status)
		}
	})
}

// ---------------------------------------------------------------------------
// B051 — posting checks topic ownership, lock, delay, deletion, parent
// ---------------------------------------------------------------------------

func TestDiscussionPosting_EnforcesTopicState(t *testing.T) {
	msg := map[string]string{"message": "<p>hello</p>"}

	cases := []struct {
		name   string
		caller uint
		role   string
		path   string
		want   int
	}{
		{"foreign tenant topic", dStudent1, "StudentEnrollment", topicPath(dCourseA1, dTopicOtherTen, "/entries"), http.StatusNotFound},
		{"other course topic", dStudent1, "StudentEnrollment", topicPath(dCourseA1, dTopicOtherCrs, "/entries"), http.StatusNotFound},
		{"deleted topic", dStudent1, "StudentEnrollment", topicPath(dCourseA1, dTopicDeleted, "/entries"), http.StatusNotFound},
		{"delayed topic", dStudent1, "StudentEnrollment", topicPath(dCourseA1, dTopicDelayed, "/entries"), http.StatusNotFound},
		{"locked topic (student)", dStudent1, "StudentEnrollment", topicPath(dCourseA1, dTopicLocked, "/entries"), http.StatusForbidden},
		{"locked topic (teacher)", dTeacher, "TeacherEnrollment", topicPath(dCourseA1, dTopicLocked, "/entries"), http.StatusCreated},
		{"reply to parent on another topic", dStudent1, "StudentEnrollment", topicPath(dCourseA1, dTopicOpen, fmt.Sprintf("/entries/%d/replies", dEntryOtherCrs)), http.StatusNotFound},
		{"reply on locked topic", dStudent1, "StudentEnrollment", topicPath(dCourseA1, dTopicLocked, fmt.Sprintf("/entries/%d/replies", dEntryRoot)), http.StatusForbidden},
		{"open topic", dStudent1, "StudentEnrollment", topicPath(dCourseA1, dTopicOpen, "/entries"), http.StatusCreated},
		{"reply on open topic", dStudent2, "StudentEnrollment", topicPath(dCourseA1, dTopicOpen, fmt.Sprintf("/entries/%d/replies", dEntryRoot)), http.StatusCreated},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			env := newDiscEnv(t, tc.caller, tc.role)
			before := len(env.store.entries)
			status, body := env.do(t, http.MethodPost, tc.path, msg)
			if status != tc.want {
				t.Fatalf("status = %d, want %d (%s)", status, tc.want, body)
			}
			created := len(env.store.entries) - before
			if tc.want == http.StatusCreated && created != 1 {
				t.Fatalf("expected one entry created, got %d", created)
			}
			if tc.want != http.StatusCreated && created != 0 {
				t.Fatalf("refused request still created %d entries", created)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// B052 — entry update/delete read :id and let authors delete their post
// ---------------------------------------------------------------------------

func TestDiscussionEntryUpdateDelete_RouteParamAndOwnership(t *testing.T) {
	t.Run("author deletes own entry", func(t *testing.T) {
		env := newDiscEnv(t, dStudent1, "StudentEnrollment")
		status, body := env.do(t, http.MethodDelete, topicPath(dCourseA1, dTopicOpen, fmt.Sprintf("/entries/%d", dEntryRoot)), nil)
		if status != http.StatusOK {
			t.Fatalf("status = %d, want 200 (%s)", status, body)
		}
		if env.store.entries[dEntryRoot].WorkflowState != "deleted" {
			t.Fatalf("entry not deleted")
		}
	})
	t.Run("other student cannot delete", func(t *testing.T) {
		env := newDiscEnv(t, dStudent2, "StudentEnrollment")
		status, _ := env.do(t, http.MethodDelete, topicPath(dCourseA1, dTopicOpen, fmt.Sprintf("/entries/%d", dEntryRoot)), nil)
		if status != http.StatusForbidden {
			t.Fatalf("status = %d, want 403", status)
		}
		if env.store.entries[dEntryRoot].WorkflowState == "deleted" {
			t.Fatalf("entry deleted by a non-author")
		}
	})
	t.Run("teacher deletes any entry in the course", func(t *testing.T) {
		env := newDiscEnv(t, dTeacher, "TeacherEnrollment")
		status, _ := env.do(t, http.MethodDelete, topicPath(dCourseA1, dTopicOpen, fmt.Sprintf("/entries/%d", dEntryReply)), nil)
		if status != http.StatusOK {
			t.Fatalf("status = %d, want 200", status)
		}
	})
	t.Run("teacher cannot delete a cross-tenant entry", func(t *testing.T) {
		env := newDiscEnv(t, dTeacher, "TeacherEnrollment")
		status, _ := env.do(t, http.MethodDelete, topicPath(dCourseA1, dTopicOpen, fmt.Sprintf("/entries/%d", dEntryOtherTen)), nil)
		if status != http.StatusNotFound {
			t.Fatalf("status = %d, want 404", status)
		}
		if env.store.entries[dEntryOtherTen].WorkflowState == "deleted" {
			t.Fatalf("cross-tenant entry deleted")
		}
	})
	t.Run("author updates own entry via :id", func(t *testing.T) {
		env := newDiscEnv(t, dStudent1, "StudentEnrollment")
		status, body := env.do(t, http.MethodPut, topicPath(dCourseA1, dTopicOpen, fmt.Sprintf("/entries/%d", dEntryRoot)), map[string]string{"message": "<p>changed</p>"})
		if status != http.StatusOK {
			t.Fatalf("status = %d, want 200 (%s)", status, body)
		}
		if env.store.entries[dEntryRoot].Message != "<p>changed</p>" {
			t.Fatalf("message = %q", env.store.entries[dEntryRoot].Message)
		}
		// V1 edits keep the same history as /v2 edits: the original and
		// the edited text.
		if got := len(env.store.versions[dEntryRoot]); got != 2 {
			t.Fatalf("versions = %d, want 2 (original + edit)", got)
		}
	})
}

// ---------------------------------------------------------------------------
// B157 — deleting a parent keeps its replies nested under a placeholder
// ---------------------------------------------------------------------------

type viewV2Body struct {
	Topic   map[string]any `json:"topic"`
	Entries []viewV2Entry  `json:"entries"`
}

type viewV2Entry struct {
	ID            uint          `json:"id"`
	Message       string        `json:"message"`
	WorkflowState string        `json:"workflow_state"`
	ReadState     bool          `json:"read_state"`
	Rating        int           `json:"rating"`
	VersionCount  int           `json:"version_count"`
	Replies       []viewV2Entry `json:"replies"`
}

func TestDiscussionView_DeletedParentKeepsRepliesNested(t *testing.T) {
	env := newDiscEnv(t, dStudent1, "StudentEnrollment")
	// A third-level reply, to check deep nesting survives too.
	replyID := dEntryReply
	env.store.entries[50] = &models.DiscussionEntry{ID: 50, DiscussionTopicID: dTopicOpen, UserID: dStudent1, ParentID: &replyID, Message: "<p>deep</p>", WorkflowState: "active"}

	if status, _ := env.do(t, http.MethodDelete, topicPath(dCourseA1, dTopicOpen, fmt.Sprintf("/entries/%d", dEntryRoot)), nil); status != http.StatusOK {
		t.Fatalf("delete status = %d", status)
	}

	status, raw := env.do(t, http.MethodGet, topicPath(dCourseA1, dTopicOpen, "/view_v2"), nil)
	if status != http.StatusOK {
		t.Fatalf("view status = %d", status)
	}
	var view viewV2Body
	if err := json.Unmarshal(raw, &view); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(view.Entries) != 1 {
		t.Fatalf("root entries = %d, want 1 (the deleted placeholder) — replies were promoted: %s", len(view.Entries), raw)
	}
	root := view.Entries[0]
	if root.ID != dEntryRoot || root.WorkflowState != "deleted" || root.Message != "" {
		t.Fatalf("root = %+v, want deleted placeholder with blank message", root)
	}
	if len(root.Replies) != 1 || root.Replies[0].ID != dEntryReply {
		t.Fatalf("reply not nested under placeholder: %+v", root.Replies)
	}
	if len(root.Replies[0].Replies) != 1 || root.Replies[0].Replies[0].ID != 50 {
		t.Fatalf("third-level reply lost: %+v", root.Replies[0].Replies)
	}
}

// ---------------------------------------------------------------------------
// B158 — likes: 0/1 only, allow_rating / only_graders honored, unlike
// ---------------------------------------------------------------------------

func TestDiscussionRating_ValidatedAndToggleable(t *testing.T) {
	ratePath := func(topic, entry uint) string {
		return topicPath(dCourseA1, topic, fmt.Sprintf("/entries/%d/rating", entry))
	}

	t.Run("arbitrary rating rejected", func(t *testing.T) {
		env := newDiscEnv(t, dStudent2, "StudentEnrollment")
		status, _ := env.do(t, http.MethodPost, ratePath(dTopicRated, dEntryRated), map[string]int{"rating": 100000})
		if status != http.StatusBadRequest {
			t.Fatalf("status = %d, want 400", status)
		}
		if env.store.entries[dEntryRated].RatingSum != 0 {
			t.Fatalf("rating_sum inflated to %d", env.store.entries[dEntryRated].RatingSum)
		}
	})
	t.Run("allow_rating=false refuses", func(t *testing.T) {
		env := newDiscEnv(t, dStudent2, "StudentEnrollment")
		status, _ := env.do(t, http.MethodPost, ratePath(dTopicOpen, dEntryRoot), map[string]int{"rating": 1})
		if status != http.StatusForbidden {
			t.Fatalf("status = %d, want 403", status)
		}
	})
	t.Run("only graders: student refused, teacher allowed", func(t *testing.T) {
		env := newDiscEnv(t, dStudent2, "StudentEnrollment")
		if status, _ := env.do(t, http.MethodPost, ratePath(dTopicGraderOnl, dEntryGraderOnly), map[string]int{"rating": 1}); status != http.StatusForbidden {
			t.Fatalf("student status = %d, want 403", status)
		}
		tenv := newDiscEnv(t, dTeacher, "TeacherEnrollment")
		if status, _ := tenv.do(t, http.MethodPost, ratePath(dTopicGraderOnl, dEntryGraderOnly), map[string]int{"rating": 1}); status != http.StatusOK {
			t.Fatalf("teacher status = %d, want 200", status)
		}
	})
	t.Run("like then unlike", func(t *testing.T) {
		env := newDiscEnv(t, dStudent2, "StudentEnrollment")
		if status, _ := env.do(t, http.MethodPost, ratePath(dTopicRated, dEntryRated), map[string]int{"rating": 1}); status != http.StatusOK {
			t.Fatalf("like status = %d", status)
		}
		if got := env.store.entries[dEntryRated].RatingSum; got != 1 {
			t.Fatalf("rating_sum after like = %d, want 1", got)
		}
		_, raw := env.do(t, http.MethodGet, topicPath(dCourseA1, dTopicRated, "/view_v2"), nil)
		var view viewV2Body
		_ = json.Unmarshal(raw, &view)
		if len(view.Entries) != 1 || view.Entries[0].Rating != 1 {
			t.Fatalf("view does not report the viewer's like: %s", raw)
		}
		if status, _ := env.do(t, http.MethodPost, ratePath(dTopicRated, dEntryRated), map[string]int{"rating": 0}); status != http.StatusOK {
			t.Fatalf("unlike status = %d", status)
		}
		if got := env.store.entries[dEntryRated].RatingSum; got != 0 {
			t.Fatalf("rating_sum after unlike = %d, want 0", got)
		}
	})
}

// ---------------------------------------------------------------------------
// B156 / B159 / B160 — read state, subscription, version count
// ---------------------------------------------------------------------------

func TestDiscussionViewV2_ReadStateSubscriptionAndVersions(t *testing.T) {
	t.Run("own entries are read; others unread until marked", func(t *testing.T) {
		env := newDiscEnv(t, dStudent1, "StudentEnrollment")
		_, raw := env.do(t, http.MethodGet, topicPath(dCourseA1, dTopicOpen, "/view_v2"), nil)
		var view viewV2Body
		if err := json.Unmarshal(raw, &view); err != nil {
			t.Fatalf("decode: %v", err)
		}
		root := view.Entries[0]
		if !root.ReadState {
			t.Errorf("author's own entry should be read")
		}
		if root.Replies[0].ReadState {
			t.Errorf("classmate's reply should be unread (read_state=false)")
		}
		_, raw = env.do(t, http.MethodGet, topicPath(dCourseA1, dTopicOpen, "/unread_count"), nil)
		var cnt map[string]int
		_ = json.Unmarshal(raw, &cnt)
		if cnt["unread_count"] != 1 {
			t.Errorf("unread_count = %d, want 1", cnt["unread_count"])
		}
	})

	t.Run("subscription is reported and survives mark-all-read", func(t *testing.T) {
		env := newDiscEnv(t, dStudent1, "StudentEnrollment")
		// Student 1 has posted, so the Canvas default is subscribed.
		_, raw := env.do(t, http.MethodGet, topicPath(dCourseA1, dTopicOpen, "/view_v2"), nil)
		var view viewV2Body
		_ = json.Unmarshal(raw, &view)
		if view.Topic["subscribed"] != true {
			t.Fatalf("subscribed = %v, want true for a poster", view.Topic["subscribed"])
		}

		if status, _ := env.do(t, http.MethodPut, topicPath(dCourseA1, dTopicOpen, "/subscription"), map[string]bool{"subscribed": false}); status != http.StatusOK {
			t.Fatalf("unsubscribe status = %d", status)
		}
		if status, _ := env.do(t, http.MethodPost, topicPath(dCourseA1, dTopicOpen, "/mark_all_read"), nil); status != http.StatusOK {
			t.Fatalf("mark_all_read status = %d", status)
		}
		_, raw = env.do(t, http.MethodGet, topicPath(dCourseA1, dTopicOpen, "/view_v2"), nil)
		view = viewV2Body{}
		_ = json.Unmarshal(raw, &view)
		if view.Topic["subscribed"] != false {
			t.Fatalf("mark_all_read re-subscribed the user: subscribed = %v", view.Topic["subscribed"])
		}
		p := env.store.topicParts[[2]uint{dTopicOpen, dStudent1}]
		if p == nil || p.Subscribed || p.LastReadAt == nil {
			t.Fatalf("participant = %+v, want unsubscribed with last_read_at set", p)
		}
	})

	t.Run("non-poster defaults to unsubscribed", func(t *testing.T) {
		env := newDiscEnv(t, dStudent2, "StudentEnrollment")
		_, raw := env.do(t, http.MethodGet, topicPath(dCourseA1, dTopicRated, "/view_v2"), nil)
		var view viewV2Body
		_ = json.Unmarshal(raw, &view)
		if view.Topic["subscribed"] != false {
			t.Fatalf("subscribed = %v, want false", view.Topic["subscribed"])
		}
	})

	t.Run("a single edit is visible as edited", func(t *testing.T) {
		env := newDiscEnv(t, dStudent1, "StudentEnrollment")
		env.do(t, http.MethodPut, topicPath(dCourseA1, dTopicOpen, fmt.Sprintf("/entries/%d/v2", dEntryRoot)), map[string]string{"message": "<p>once</p>"})
		_, raw := env.do(t, http.MethodGet, topicPath(dCourseA1, dTopicOpen, "/view_v2"), nil)
		var view viewV2Body
		_ = json.Unmarshal(raw, &view)
		if view.Entries[0].VersionCount < 2 {
			t.Fatalf("version_count after one edit = %d, want >= 2", view.Entries[0].VersionCount)
		}
	})
}

// ---------------------------------------------------------------------------
// B169 — checkpoint progress is self-or-staff
// ---------------------------------------------------------------------------

func TestDiscussionCheckpointProgress_SelfOrStaffOnly(t *testing.T) {
	progress := func(user string) string {
		return topicPath(dCourseA1, dTopicOpen, "/checkpoints/progress?user_id="+user)
	}

	student := newDiscEnv(t, dStudent2, "StudentEnrollment")
	if status, _ := student.do(t, http.MethodGet, progress(fmt.Sprint(dStudent1)), nil); status != http.StatusForbidden {
		t.Fatalf("student reading classmate progress: status = %d, want 403", status)
	}
	if status, _ := student.do(t, http.MethodGet, progress(fmt.Sprint(dStudent2)), nil); status != http.StatusOK {
		t.Fatalf("student reading own progress: status = %d, want 200", status)
	}
	if status, _ := student.do(t, http.MethodGet, topicPath(dCourseA1, dTopicOpen, "/checkpoints/progress"), nil); status != http.StatusOK {
		t.Fatalf("student reading own progress (implicit): status = %d, want 200", status)
	}

	teacher := newDiscEnv(t, dTeacher, "TeacherEnrollment")
	if status, _ := teacher.do(t, http.MethodGet, progress(fmt.Sprint(dStudent1)), nil); status != http.StatusOK {
		t.Fatalf("teacher reading student progress: status = %d, want 200", status)
	}

	t.Run("checkpoint on a foreign topic cannot be modified", func(t *testing.T) {
		status, _ := teacher.do(t, http.MethodDelete, topicPath(dCourseA1, dTopicOpen, "/checkpoints/2"), nil)
		if status != http.StatusNotFound {
			t.Fatalf("status = %d, want 404", status)
		}
		if teacher.store.checkpoints[2].WorkflowState == "deleted" {
			t.Fatalf("foreign checkpoint deleted")
		}
	})
}
