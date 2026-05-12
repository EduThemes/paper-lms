package wiring_test

// DB-integration tests for the EnrolledCourseEmitCallback wiring. Gated
// on PARITY_DB_URL / DATABASE_URL — mirrors the freshDB pattern used by
// internal/service/gamification/seed_test.go so `go test ./...` stays
// green on laptops without a dev container.

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
	"github.com/EduThemes/paper-lms/internal/repository"
	pgrepo "github.com/EduThemes/paper-lms/internal/repository/postgres"
	"github.com/EduThemes/paper-lms/internal/service/gamification"
	gamificationEffects "github.com/EduThemes/paper-lms/internal/service/gamification/effects"
	"github.com/EduThemes/paper-lms/internal/service/gamification/wiring"
	_ "github.com/lib/pq"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func TestEnrolledCourseEmitCallback_HappyPath(t *testing.T) {
	g, cleanup := freshDB(t)
	defer cleanup()

	ctx := context.Background()
	fx := buildEmitterFixture(t, g)

	// Tenant account → its ID becomes Course.AccountID → TenantID on
	// the emitted event.
	account := models.Account{Name: "Wiring Test Tenant", WorkflowState: "active"}
	if err := g.Create(&account).Error; err != nil {
		t.Fatalf("create account: %v", err)
	}

	// Seed system currencies for the tenant so the rule dispatch path
	// has wallets / currency rows it can resolve if a rule fires. No
	// rule is created in this test so this is belt-and-braces.
	if err := gamification.SeedSystemCurrenciesForTenant(ctx, g, account.ID); err != nil {
		t.Fatalf("seed currencies: %v", err)
	}

	course := models.Course{
		AccountID:     account.ID,
		Name:          "Intro to Wiring",
		CourseCode:    "WIRE-101",
		WorkflowState: "available",
	}
	if err := g.Create(&course).Error; err != nil {
		t.Fatalf("create course: %v", err)
	}

	enrollment := models.Enrollment{
		UserID:        42,
		CourseID:      course.ID,
		Type:          "StudentEnrollment",
		Role:          "StudentEnrollment",
		WorkflowState: "active",
	}
	if err := g.Create(&enrollment).Error; err != nil {
		t.Fatalf("create enrollment: %v", err)
	}

	cb := wiring.EnrolledCourseEmitCallback(fx.emitter, fx.enrollmentRepo, fx.courseRepo)
	cb(ctx, enrollment.ID)

	// Assert: exactly one gamification_events row with the right shape.
	var events []models.GamificationEvent
	if err := g.Where("verb = ? AND object_type = ?",
		gamification.VerbEnrolled, gamification.ObjectCourse).Find(&events).Error; err != nil {
		t.Fatalf("list events: %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("expected 1 enrolled-Course event, got %d", len(events))
	}
	got := events[0]
	if got.TenantID != account.ID {
		t.Errorf("TenantID = %d, want %d", got.TenantID, account.ID)
	}
	if got.ActorID != enrollment.UserID {
		t.Errorf("ActorID = %d, want %d", got.ActorID, enrollment.UserID)
	}
	if got.Verb != gamification.VerbEnrolled {
		t.Errorf("Verb = %q, want %q", got.Verb, gamification.VerbEnrolled)
	}
	if got.ObjectType != gamification.ObjectCourse {
		t.Errorf("ObjectType = %q, want %q", got.ObjectType, gamification.ObjectCourse)
	}
	if got.ObjectID == nil || *got.ObjectID != course.ID {
		t.Errorf("ObjectID = %v, want %d", got.ObjectID, course.ID)
	}
	if got.Source != gamification.EmitterSource {
		t.Errorf("Source = %q, want %q", got.Source, gamification.EmitterSource)
	}

	// Result payload carries enrollment_type / role / workflow_state.
	var result map[string]any
	if err := json.Unmarshal(got.Result, &result); err != nil {
		t.Fatalf("unmarshal Result: %v", err)
	}
	if result["enrollment_type"] != "StudentEnrollment" {
		t.Errorf("Result.enrollment_type = %v, want StudentEnrollment", result["enrollment_type"])
	}
	if result["role"] != "StudentEnrollment" {
		t.Errorf("Result.role = %v, want StudentEnrollment", result["role"])
	}
	if result["workflow_state"] != "active" {
		t.Errorf("Result.workflow_state = %v, want active", result["workflow_state"])
	}

	// Context payload carries course_id / enrollment_id.
	var contextPayload map[string]any
	if err := json.Unmarshal(got.Context, &contextPayload); err != nil {
		t.Fatalf("unmarshal Context: %v", err)
	}
	// JSON numbers decode to float64; cast for comparison.
	if int64(contextPayload["course_id"].(float64)) != int64(course.ID) {
		t.Errorf("Context.course_id = %v, want %d", contextPayload["course_id"], course.ID)
	}
	if int64(contextPayload["enrollment_id"].(float64)) != int64(enrollment.ID) {
		t.Errorf("Context.enrollment_id = %v, want %d", contextPayload["enrollment_id"], enrollment.ID)
	}
}

func TestEnrolledCourseEmitCallback_MissingEnrollment(t *testing.T) {
	g, cleanup := freshDB(t)
	defer cleanup()

	ctx := context.Background()
	fx := buildEmitterFixture(t, g)

	// Invoke against a non-existent enrollment ID — must not panic and
	// must not emit an event.
	cb := wiring.EnrolledCourseEmitCallback(fx.emitter, fx.enrollmentRepo, fx.courseRepo)
	cb(ctx, 99999)

	var count int64
	if err := g.Model(&models.GamificationEvent{}).Count(&count).Error; err != nil {
		t.Fatalf("count events: %v", err)
	}
	if count != 0 {
		t.Fatalf("expected 0 events on missing enrollment, got %d", count)
	}
}

func TestEnrolledCourseEmitCallback_StudentAndTeacher(t *testing.T) {
	g, cleanup := freshDB(t)
	defer cleanup()

	ctx := context.Background()
	fx := buildEmitterFixture(t, g)

	account := models.Account{Name: "Multi-Role Tenant", WorkflowState: "active"}
	if err := g.Create(&account).Error; err != nil {
		t.Fatalf("create account: %v", err)
	}
	if err := gamification.SeedSystemCurrenciesForTenant(ctx, g, account.ID); err != nil {
		t.Fatalf("seed currencies: %v", err)
	}
	course := models.Course{
		AccountID:     account.ID,
		Name:          "Multi-Role",
		CourseCode:    "MR-101",
		WorkflowState: "available",
	}
	if err := g.Create(&course).Error; err != nil {
		t.Fatalf("create course: %v", err)
	}

	type want struct {
		userID uint
		typ    string
	}
	wants := []want{
		{userID: 100, typ: "StudentEnrollment"},
		{userID: 200, typ: "TeacherEnrollment"},
	}

	cb := wiring.EnrolledCourseEmitCallback(fx.emitter, fx.enrollmentRepo, fx.courseRepo)
	for _, w := range wants {
		enr := models.Enrollment{
			UserID:        w.userID,
			CourseID:      course.ID,
			Type:          w.typ,
			Role:          w.typ,
			WorkflowState: "active",
		}
		if err := g.Create(&enr).Error; err != nil {
			t.Fatalf("create %s enrollment: %v", w.typ, err)
		}
		cb(ctx, enr.ID)
	}

	var events []models.GamificationEvent
	if err := g.Where("verb = ?", gamification.VerbEnrolled).Order("id").Find(&events).Error; err != nil {
		t.Fatalf("list events: %v", err)
	}
	if len(events) != 2 {
		t.Fatalf("expected 2 events, got %d", len(events))
	}
	for i, ev := range events {
		var r map[string]any
		if err := json.Unmarshal(ev.Result, &r); err != nil {
			t.Fatalf("event %d unmarshal: %v", i, err)
		}
		if r["enrollment_type"] != wants[i].typ {
			t.Errorf("event %d enrollment_type = %v, want %s", i, r["enrollment_type"], wants[i].typ)
		}
		if ev.ActorID != wants[i].userID {
			t.Errorf("event %d ActorID = %d, want %d", i, ev.ActorID, wants[i].userID)
		}
	}
}

// --- shared fixture: emitter + the two repos the callback closes over ---

type fixture struct {
	emitter        *gamification.Emitter
	enrollmentRepo repository.EnrollmentRepository
	courseRepo     repository.CourseRepository
}

// buildEmitterFixture wires a fully real gamification.Emitter against the
// freshDB. Every Emitter dependency uses the real postgres impl so the
// callback's full path (load enrollment → load course → Emit → persist
// event → dispatch with empty rule index) exercises production code.
func buildEmitterFixture(t *testing.T, g *gorm.DB) fixture {
	t.Helper()
	submissionRepo := pgrepo.NewSubmissionRepository(g)
	quizSubRepo := pgrepo.NewQuizSubmissionRepository(g)
	outcomeRepo := pgrepo.NewLearningOutcomeResultRepository(g)
	contentViewRepo := pgrepo.NewContentViewRepository(g)
	walletRepo := pgrepo.NewGamificationWalletRepository(g)
	currencyRepo := pgrepo.NewGamificationCurrencyTypeRepository(g)
	ruleRepo := pgrepo.NewGamificationRuleRepository(g)
	eventRepo := pgrepo.NewGamificationEventRepository(g)
	ferpaRepo := pgrepo.NewGamificationFerpaFieldTagRepository(g)

	emitter := gamification.NewEmitter(gamification.EmitterDeps{
		Dispatch: gamification.DispatchDeps{
			Snapshot: gamification.SnapshotDeps{
				Submissions:     submissionRepo,
				QuizSubmissions: quizSubRepo,
				OutcomeResults:  outcomeRepo,
				ContentViews:    contentViewRepo,
				Wallet:          walletRepo,
				CurrencyType:    currencyRepo,
			},
			Rules: ruleRepo,
			Effects: gamificationEffects.EffectDeps{
				Wallet:       walletRepo,
				CurrencyType: currencyRepo,
			},
		},
		Events:    eventRepo,
		FerpaTags: ferpaRepo,
	})

	return fixture{
		emitter:        emitter,
		enrollmentRepo: pgrepo.NewEnrollmentRepository(g),
		courseRepo:     pgrepo.NewCourseRepository(g),
	}
}

// --- DB plumbing — mirrors internal/service/gamification/seed_test.go ---

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
