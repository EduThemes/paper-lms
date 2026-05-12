package wiring_test

// DB-integration tests for the graded-submission emit adapter. Mirrors
// the PARITY_DB_URL / DATABASE_URL skip pattern in
// internal/service/gamification/seed_test.go and the emitter fixture
// in emitter_test.go — we need a real Postgres so the full Emit
// pipeline (FERPA pre-flight + event persistence + dispatcher walk)
// runs end-to-end. No rule is registered; we only need to confirm the
// callback shapes an event correctly and persists it via Emit.

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"testing"
	"time"

	"github.com/EduThemes/paper-lms/internal/db"
	"github.com/EduThemes/paper-lms/internal/domain/models"
	pgrepo "github.com/EduThemes/paper-lms/internal/repository/postgres"
	"github.com/EduThemes/paper-lms/internal/service/gamification"
	"github.com/EduThemes/paper-lms/internal/service/gamification/effects"
	"github.com/EduThemes/paper-lms/internal/service/gamification/wiring"
	_ "github.com/lib/pq"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func TestGradedSubmissionEmitCallback_HappyPath(t *testing.T) {
	g, cleanup := freshDB(t)
	defer cleanup()
	ctx := context.Background()

	// Seed account → course → assignment → graded submission. account.ID
	// is the tenant_id every downstream gamification row carries.
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
	pointsPossible := 100.0
	assignment := models.Assignment{
		CourseID:       course.ID,
		Name:           "Reading 1",
		WorkflowState:  "published",
		PointsPossible: &pointsPossible,
	}
	if err := g.Create(&assignment).Error; err != nil {
		t.Fatalf("create assignment: %v", err)
	}
	score := 87.5
	gradedAt := time.Now().Add(-1 * time.Minute).UTC().Truncate(time.Second)
	submission := models.Submission{
		AssignmentID:  assignment.ID,
		UserID:        4242,
		Score:         &score,
		GradedAt:      &gradedAt,
		WorkflowState: "graded",
	}
	if err := g.Create(&submission).Error; err != nil {
		t.Fatalf("create submission: %v", err)
	}

	// Seed system currencies so the FERPA preflight + event persistence
	// path inside Emit has somewhere to land. The Emit pipeline still
	// runs the dispatcher; without any rule rows it's a no-op walk.
	if err := gamification.SeedSystemCurrenciesForTenant(ctx, g, account.ID); err != nil {
		t.Fatalf("seed currencies: %v", err)
	}

	emitter := buildEmitter(t, g)
	cb := wiring.GradedSubmissionEmitCallback(
		emitter,
		pgrepo.NewSubmissionRepository(g),
		pgrepo.NewAssignmentRepository(g),
		pgrepo.NewCourseRepository(g),
	)

	// Invoke synchronously. (The go-routine wrapper lives in the service,
	// not in our callback.)
	cb(ctx, submission.ID)

	// Exactly one gamification_events row should exist.
	var events []models.GamificationEvent
	if err := g.Find(&events).Error; err != nil {
		t.Fatalf("list events: %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("expected 1 gamification_events row, got %d", len(events))
	}
	ev := events[0]
	if ev.Verb != gamification.VerbGraded {
		t.Errorf("Verb = %q, want %q", ev.Verb, gamification.VerbGraded)
	}
	if ev.ObjectType != gamification.ObjectSubmission {
		t.Errorf("ObjectType = %q, want %q", ev.ObjectType, gamification.ObjectSubmission)
	}
	if ev.ActorID != submission.UserID {
		t.Errorf("ActorID = %d, want %d", ev.ActorID, submission.UserID)
	}
	if ev.TenantID != account.ID {
		t.Errorf("TenantID = %d, want %d", ev.TenantID, account.ID)
	}
	if ev.ObjectID == nil || *ev.ObjectID != submission.ID {
		t.Errorf("ObjectID = %v, want %d", ev.ObjectID, submission.ID)
	}
	if ev.Source != gamification.EmitterSource {
		t.Errorf("Source = %q, want %q", ev.Source, gamification.EmitterSource)
	}

	// Result JSONB carries the score (and workflow_state).
	var resultDecoded map[string]any
	if err := json.Unmarshal(ev.Result, &resultDecoded); err != nil {
		t.Fatalf("decode result: %v (raw=%s)", err, string(ev.Result))
	}
	gotScore, ok := resultDecoded["score"].(float64)
	if !ok {
		t.Fatalf("result.score not a number: %v (raw=%s)", resultDecoded["score"], string(ev.Result))
	}
	if gotScore != score {
		t.Errorf("result.score = %v, want %v", gotScore, score)
	}
	if got := resultDecoded["workflow_state"]; got != "graded" {
		t.Errorf("result.workflow_state = %v, want %q", got, "graded")
	}

	// Context JSONB carries course_id + assignment_id.
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
	gotAssignmentID, ok := contextDecoded["assignment_id"].(float64)
	if !ok {
		t.Fatalf("context.assignment_id not a number: %v (raw=%s)", contextDecoded["assignment_id"], string(ev.Context))
	}
	if uint(gotAssignmentID) != assignment.ID {
		t.Errorf("context.assignment_id = %v, want %d", gotAssignmentID, assignment.ID)
	}
}

func TestGradedSubmissionEmitCallback_MissingSubmission_NoEventNoPanic(t *testing.T) {
	g, cleanup := freshDB(t)
	defer cleanup()
	ctx := context.Background()

	// Build emitter against a fresh DB; no submission row exists, so the
	// callback should log + return without persisting an event.
	emitter := buildEmitter(t, g)
	cb := wiring.GradedSubmissionEmitCallback(
		emitter,
		pgrepo.NewSubmissionRepository(g),
		pgrepo.NewAssignmentRepository(g),
		pgrepo.NewCourseRepository(g),
	)

	// Wrap in a defer so a panic surfaces as a test failure (the
	// callback contract says it must never panic).
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("callback panicked on missing submission: %v", r)
		}
	}()
	cb(ctx, 99999999)

	var count int64
	if err := g.Model(&models.GamificationEvent{}).Count(&count).Error; err != nil {
		t.Fatalf("count events: %v", err)
	}
	if count != 0 {
		t.Fatalf("expected zero events on missing submission, got %d", count)
	}
}

// buildEmitter assembles the full Emitter the same way Sprint D's
// cmd/server wiring will. Copies the structure of
// setupEmitterFixture in emitter_test.go.
func buildEmitter(t *testing.T, g *gorm.DB) *gamification.Emitter {
	t.Helper()
	subRepo := pgrepo.NewSubmissionRepository(g)
	quizSubRepo := pgrepo.NewQuizSubmissionRepository(g)
	outcomeRepo := pgrepo.NewLearningOutcomeResultRepository(g)
	contentViewRepo := pgrepo.NewContentViewRepository(g)
	walletRepo := pgrepo.NewGamificationWalletRepository(g)
	currencyRepo := pgrepo.NewGamificationCurrencyTypeRepository(g)
	ruleRepo := pgrepo.NewGamificationRuleRepository(g)
	eventRepo := pgrepo.NewGamificationEventRepository(g)
	ferpaRepo := pgrepo.NewGamificationFerpaFieldTagRepository(g)

	deps := gamification.EmitterDeps{
		Dispatch: gamification.DispatchDeps{
			Snapshot: gamification.SnapshotDeps{
				Submissions:     subRepo,
				QuizSubmissions: quizSubRepo,
				OutcomeResults:  outcomeRepo,
				ContentViews:    contentViewRepo,
				Wallet:          walletRepo,
				CurrencyType:    currencyRepo,
			},
			Rules: ruleRepo,
			Effects: effects.EffectDeps{
				Wallet:       walletRepo,
				CurrencyType: currencyRepo,
			},
		},
		Events:    eventRepo,
		FerpaTags: ferpaRepo,
	}
	return gamification.NewEmitter(deps)
}

// --- DB plumbing — duplicated from internal/service/gamification/seed_test.go ---
// We can't import that helper because it lives in package gamification_test.

func freshDB(t *testing.T) (*gorm.DB, func()) {
	t.Helper()
	parityURL := os.Getenv("PARITY_DB_URL")
	if parityURL == "" {
		parityURL = os.Getenv("DATABASE_URL")
	}
	if parityURL == "" {
		t.Skip("set PARITY_DB_URL (or DATABASE_URL) to run wiring integration tests")
	}

	adminURL := swapDatabase(t, parityURL, "postgres")
	admin, err := sql.Open("postgres", adminURL)
	if err != nil {
		t.Fatalf("open admin: %v", err)
	}

	name := fmt.Sprintf("paper_lms_wiring_%d", time.Now().UnixNano())
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if _, err := admin.ExecContext(ctx, fmt.Sprintf(`CREATE DATABASE %q`, name)); err != nil {
		_ = admin.Close()
		t.Fatalf("create db %s: %v", name, err)
	}

	dbURL := swapDatabase(t, parityURL, name)
	bs, err := sql.Open("postgres", dbURL)
	if err != nil {
		t.Fatalf("open %s: %v", dbURL, err)
	}
	if _, err := bs.Exec(`CREATE EXTENSION IF NOT EXISTS vector`); err != nil {
		_ = bs.Close()
		t.Fatalf("create extension: %v", err)
	}
	_ = bs.Close()

	g, err := gorm.Open(postgres.Open(dbURL), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		t.Fatalf("gorm open: %v", err)
	}
	if err := db.MigrateUp(g); err != nil {
		t.Fatalf("migrate up: %v", err)
	}

	cleanup := func() {
		if raw, err := g.DB(); err == nil {
			_ = raw.Close()
		}
		dropCtx, dropCancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer dropCancel()
		_, _ = admin.ExecContext(dropCtx, fmt.Sprintf(`DROP DATABASE IF EXISTS %q WITH (FORCE)`, name))
		_ = admin.Close()
	}
	return g, cleanup
}

func swapDatabase(t *testing.T, rawURL, dbName string) string {
	t.Helper()
	u, err := url.Parse(rawURL)
	if err != nil {
		t.Fatalf("parse URL: %v", err)
	}
	u.Path = "/" + dbName
	return u.String()
}
