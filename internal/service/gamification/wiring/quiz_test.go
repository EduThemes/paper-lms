package wiring_test

// Integration test for CompletedQuizEmitCallback. Boots a fresh Postgres
// database (gated on PARITY_DB_URL / DATABASE_URL — mirrors the pattern
// in internal/service/gamification/seed_test.go) and walks the callback
// end-to-end: insert a real account + course + quiz + quiz_submission,
// build the callback against real postgres repos + a real Emitter, fire
// it, then assert exactly one gamification_events row exists with the
// expected verb/object_type/tenant/actor/result/context.

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/repository/postgres"
	"github.com/EduThemes/paper-lms/internal/service/gamification"
	"github.com/EduThemes/paper-lms/internal/service/gamification/wiring"
)

// TestCompletedQuizEmitCallback_HappyPath inserts a complete fixture
// (account + course + quiz + quiz_submission with FinishedAt + Score),
// invokes the callback, and asserts exactly one gamification_events row
// with the right shape.
func TestCompletedQuizEmitCallback_HappyPath(t *testing.T) {
	g, cleanup := freshDB(t)
	defer cleanup()

	ctx := context.Background()

	// Account = tenant.
	account := models.Account{Name: "Test Tenant", WorkflowState: "active"}
	if err := g.Create(&account).Error; err != nil {
		t.Fatalf("create account: %v", err)
	}

	// Course owned by that tenant.
	course := models.Course{
		AccountID:     account.ID,
		Name:          "Algebra 1",
		CourseCode:    "ALG-1",
		WorkflowState: "available",
	}
	if err := g.Create(&course).Error; err != nil {
		t.Fatalf("create course: %v", err)
	}

	// Quiz in that course.
	quiz := models.Quiz{
		CourseID:      course.ID,
		Title:         "Chapter 1 Quiz",
		QuizType:      "assignment",
		WorkflowState: "available",
		Published:     true,
	}
	if err := g.Create(&quiz).Error; err != nil {
		t.Fatalf("create quiz: %v", err)
	}

	// QuizSubmission: complete, scored 80, finished one hour ago.
	score := 80.0
	finishedAt := time.Now().Add(-time.Hour).UTC().Truncate(time.Second)
	startedAt := finishedAt.Add(-15 * time.Minute)
	qs := models.QuizSubmission{
		QuizID:          quiz.ID,
		UserID:          42,
		Attempt:         1,
		Score:           &score,
		StartedAt:       &startedAt,
		FinishedAt:      &finishedAt,
		TimeSpent:       900,
		ValidationToken: "tok-deadbeef",
		WorkflowState:   "complete",
	}
	if err := g.Create(&qs).Error; err != nil {
		t.Fatalf("create quiz submission: %v", err)
	}

	emitter := buildEmitter(t, g)
	cb := wiring.CompletedQuizEmitCallback(
		emitter,
		postgres.NewQuizSubmissionRepository(g),
		postgres.NewQuizRepository(g),
		postgres.NewCourseRepository(g),
	)

	cb(ctx, qs.ID)

	// Exactly one event row should exist.
	var events []models.GamificationEvent
	if err := g.Where("tenant_id = ?", account.ID).Find(&events).Error; err != nil {
		t.Fatalf("list events: %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("expected 1 gamification_events row, got %d", len(events))
	}
	got := events[0]

	if got.Verb != gamification.VerbCompleted {
		t.Errorf("Verb = %q, want %q", got.Verb, gamification.VerbCompleted)
	}
	if got.ObjectType != gamification.ObjectQuiz {
		t.Errorf("ObjectType = %q, want %q", got.ObjectType, gamification.ObjectQuiz)
	}
	if got.ObjectID == nil || *got.ObjectID != quiz.ID {
		t.Errorf("ObjectID = %v, want %d", got.ObjectID, quiz.ID)
	}
	if got.TenantID != account.ID {
		t.Errorf("TenantID = %d, want %d", got.TenantID, account.ID)
	}
	if got.ActorID != qs.UserID {
		t.Errorf("ActorID = %d, want %d", got.ActorID, qs.UserID)
	}
	if got.Source != gamification.EmitterSource {
		t.Errorf("Source = %q, want %q", got.Source, gamification.EmitterSource)
	}
	// OccurredAt should match FinishedAt (truncated to second to absorb
	// Postgres microsecond rounding).
	if got.OccurredAt.UTC().Truncate(time.Second) != finishedAt {
		t.Errorf("OccurredAt = %v, want %v", got.OccurredAt.UTC().Truncate(time.Second), finishedAt)
	}

	// Result JSON: {score, workflow_state, time_spent_seconds, attempt}.
	var result map[string]any
	if err := json.Unmarshal(got.Result, &result); err != nil {
		t.Fatalf("unmarshal result: %v", err)
	}
	if got, want := result["score"], 80.0; got != want {
		t.Errorf("result.score = %v, want %v", got, want)
	}
	if got, want := result["workflow_state"], "complete"; got != want {
		t.Errorf("result.workflow_state = %v, want %v", got, want)
	}
	if got, want := result["time_spent_seconds"], 900.0; got != want {
		t.Errorf("result.time_spent_seconds = %v, want %v", got, want)
	}
	if got, want := result["attempt"], 1.0; got != want {
		t.Errorf("result.attempt = %v, want %v", got, want)
	}

	// Context JSON: {course_id, quiz_submission_id}.
	var contextMap map[string]any
	if err := json.Unmarshal(got.Context, &contextMap); err != nil {
		t.Fatalf("unmarshal context: %v", err)
	}
	if got, want := contextMap["course_id"], float64(course.ID); got != want {
		t.Errorf("context.course_id = %v, want %v", got, want)
	}
	if got, want := contextMap["quiz_submission_id"], float64(qs.ID); got != want {
		t.Errorf("context.quiz_submission_id = %v, want %v", got, want)
	}
}

// TestCompletedQuizEmitCallback_MissingSubmission feeds a non-existent
// submission ID. The callback should log + return cleanly with no event
// row written and no panic.
func TestCompletedQuizEmitCallback_MissingSubmission(t *testing.T) {
	g, cleanup := freshDB(t)
	defer cleanup()

	emitter := buildEmitter(t, g)
	cb := wiring.CompletedQuizEmitCallback(
		emitter,
		postgres.NewQuizSubmissionRepository(g),
		postgres.NewQuizRepository(g),
		postgres.NewCourseRepository(g),
	)

	// Should not panic; should not produce events.
	cb(context.Background(), 99999)

	var count int64
	if err := g.Model(&models.GamificationEvent{}).Count(&count).Error; err != nil {
		t.Fatalf("count events: %v", err)
	}
	if count != 0 {
		t.Fatalf("expected zero events after missing-submission callback, got %d", count)
	}
}

// buildEmitter + DB plumbing (freshDB + swapDatabase) live in testhelpers_test.go and are
// shared across every wiring_test integration test.
