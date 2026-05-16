package wiring_test

// DB-integration test for the discussion-entry-posted emit adapter.
// Mirrors the PARITY_DB_URL / DATABASE_URL skip pattern in
// internal/service/gamification/seed_test.go via freshDB. No rule is
// registered; we only confirm the callback shapes a gamification event
// correctly and persists it via Emit.

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/EduThemes/paper-lms/internal/domain/models"
	pgrepo "github.com/EduThemes/paper-lms/internal/repository/postgres"
	"github.com/EduThemes/paper-lms/internal/service/gamification"
	"github.com/EduThemes/paper-lms/internal/service/gamification/wiring"
)

func TestDiscussionEntryPostedEmitCallback_EmitsEvent(t *testing.T) {
	g, cleanup := freshDB(t)
	defer cleanup()
	ctx := context.Background()

	// Seed account → course → topic → entry. account.ID is the tenant_id
	// every downstream gamification row carries.
	account := models.Account{Name: "Wiring Tenant", WorkflowState: "active"}
	if err := g.Create(&account).Error; err != nil {
		t.Fatalf("create account: %v", err)
	}
	course := models.Course{
		AccountID:     account.ID,
		Name:          "Course 1",
		CourseCode:    "C-1",
		WorkflowState: "available",
	}
	if err := g.Create(&course).Error; err != nil {
		t.Fatalf("create course: %v", err)
	}
	author := seedTestUser(t, g, account.ID, "topic-author@example.test")
	replier := seedTestUser(t, g, account.ID, "topic-replier@example.test")
	topic := models.DiscussionTopic{
		CourseID:      course.ID,
		UserID:        author.ID,
		Title:         "Week 1 Discussion",
		Message:       "Introduce yourself.",
		WorkflowState: "active",
	}
	if err := g.Create(&topic).Error; err != nil {
		t.Fatalf("create topic: %v", err)
	}
	parentID := uint(5150)
	entry := models.DiscussionEntry{
		DiscussionTopicID: topic.ID,
		UserID:            replier.ID,
		ParentID:          &parentID,
		Message:           "Replying to the prompt.",
		WorkflowState:     "active",
	}
	if err := g.Create(&entry).Error; err != nil {
		t.Fatalf("create entry: %v", err)
	}

	// Seed system currencies so the FERPA preflight + event persistence
	// path inside Emit has somewhere to land. No rule rows means the
	// dispatcher walk is a no-op, but Emit still persists the event.
	if err := gamification.SeedSystemCurrenciesForTenant(ctx, g, account.ID); err != nil {
		t.Fatalf("seed currencies: %v", err)
	}

	emitter := buildEmitter(t, g)
	cb := wiring.DiscussionEntryPostedEmitCallback(
		emitter,
		pgrepo.NewDiscussionEntryRepository(g),
		pgrepo.NewDiscussionTopicRepository(g),
		pgrepo.NewCourseRepository(g),
	)

	// Invoke synchronously. (The goroutine wrapper lives in the service,
	// not in our callback.)
	cb(ctx, entry.ID)

	// Exactly one gamification_events row should exist.
	var events []models.GamificationEvent
	if err := g.Find(&events).Error; err != nil {
		t.Fatalf("list events: %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("expected 1 gamification_events row, got %d", len(events))
	}
	ev := events[0]
	if ev.Verb != gamification.VerbPosted {
		t.Errorf("Verb = %q, want %q", ev.Verb, gamification.VerbPosted)
	}
	if ev.ObjectType != gamification.ObjectDiscussionEntry {
		t.Errorf("ObjectType = %q, want %q", ev.ObjectType, gamification.ObjectDiscussionEntry)
	}
	if ev.ActorID != entry.UserID {
		t.Errorf("ActorID = %d, want %d", ev.ActorID, entry.UserID)
	}
	if ev.TenantID != account.ID {
		t.Errorf("TenantID = %d, want %d", ev.TenantID, account.ID)
	}
	if ev.ObjectID == nil || *ev.ObjectID != entry.ID {
		t.Errorf("ObjectID = %v, want %d", ev.ObjectID, entry.ID)
	}
	if ev.Source != gamification.EmitterSource {
		t.Errorf("Source = %q, want %q", ev.Source, gamification.EmitterSource)
	}

	// Result JSONB carries parent_id (snake_case key).
	var resultDecoded map[string]any
	if err := json.Unmarshal(ev.Result, &resultDecoded); err != nil {
		t.Fatalf("decode result: %v (raw=%s)", err, string(ev.Result))
	}
	rawParent, present := resultDecoded["parent_id"]
	if !present {
		t.Fatalf("result.parent_id missing (raw=%s)", string(ev.Result))
	}
	gotParent, ok := rawParent.(float64)
	if !ok {
		t.Fatalf("result.parent_id not a number: %v (raw=%s)", rawParent, string(ev.Result))
	}
	if uint(gotParent) != parentID {
		t.Errorf("result.parent_id = %v, want %d", gotParent, parentID)
	}

	// Context JSONB carries course_id + discussion_topic_id (snake_case).
	var contextDecoded map[string]any
	if err := json.Unmarshal(ev.Context, &contextDecoded); err != nil {
		t.Fatalf("decode context: %v (raw=%s)", err, string(ev.Context))
	}
	gotCourseID, ok := contextDecoded["course_id"].(float64)
	if !ok {
		t.Fatalf("context.course_id not a number: %v (raw=%s)", contextDecoded["course_id"], string(ev.Context))
	}
	if uint(gotCourseID) != course.ID {
		t.Errorf("context.course_id = %v, want %d", gotCourseID, course.ID)
	}
	gotTopicID, ok := contextDecoded["discussion_topic_id"].(float64)
	if !ok {
		t.Fatalf("context.discussion_topic_id not a number: %v (raw=%s)", contextDecoded["discussion_topic_id"], string(ev.Context))
	}
	if uint(gotTopicID) != topic.ID {
		t.Errorf("context.discussion_topic_id = %v, want %d", gotTopicID, topic.ID)
	}
}
