package wiring_test

// DB-integration test for the ViewedContentEmitCallback adapter. The
// callback is the bridge between ContentViewService.RecordView and the
// gamification.Emitter — it must walk page → course → account to resolve
// tenant_id, build the right event shape, and never propagate errors.
//
// Tests rely on the same freshDB / pgvector pattern the gamification
// e2e tests use; they skip cleanly when PARITY_DB_URL isn't set.

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
	"github.com/EduThemes/paper-lms/internal/repository/postgres"
	"github.com/EduThemes/paper-lms/internal/service/gamification"
	"github.com/EduThemes/paper-lms/internal/service/gamification/effects"
	"github.com/EduThemes/paper-lms/internal/service/gamification/wiring"

	_ "github.com/lib/pq"
	gormpg "gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// freshDB spins up a one-shot database for a single test, mirroring the
// pattern used in internal/service/gamification/seed_test.go. We
// duplicate it here rather than export it to keep the gamification
// package's test surface unchanged.
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

	g, err := gorm.Open(gormpg.Open(dbURL), &gorm.Config{
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

// viewFixture bundles the rows + emitter the three callback tests share.
type viewFixture struct {
	db       *gorm.DB
	tenantID uint
	courseID uint
	pageID   uint
	userID   uint
	emitter  *gamification.Emitter
	pageRepo repository.PageRepository
	crsRepo  repository.CourseRepository
}

func setupViewFixture(t *testing.T) viewFixture {
	t.Helper()
	g, cleanup := freshDB(t)
	t.Cleanup(cleanup)

	// Account = tenant.
	account := models.Account{Name: "View Tenant", WorkflowState: "active"}
	if err := g.Create(&account).Error; err != nil {
		t.Fatalf("create account: %v", err)
	}

	// Course inside the tenant.
	course := models.Course{
		AccountID:     account.ID,
		Name:          "Test Course",
		CourseCode:    "VIEW101",
		WorkflowState: "available",
	}
	if err := g.Create(&course).Error; err != nil {
		t.Fatalf("create course: %v", err)
	}

	// Wiki page inside the course.
	page := models.WikiPage{
		CourseID:      course.ID,
		Title:         "Intro",
		URL:           "intro",
		Body:          "<p>hi</p>",
		WorkflowState: "active",
	}
	if err := g.Create(&page).Error; err != nil {
		t.Fatalf("create page: %v", err)
	}

	// Wire emitter with the smallest possible dep set — the rules engine
	// won't fire any rule (none seeded) but the events row must land.
	subRepo := postgres.NewSubmissionRepository(g)
	quizSubRepo := postgres.NewQuizSubmissionRepository(g)
	outcomeRepo := postgres.NewLearningOutcomeResultRepository(g)
	contentViewRepo := postgres.NewContentViewRepository(g)
	walletRepo := postgres.NewGamificationWalletRepository(g)
	currencyRepo := postgres.NewGamificationCurrencyTypeRepository(g)
	ruleRepo := postgres.NewGamificationRuleRepository(g)
	eventRepo := postgres.NewGamificationEventRepository(g)
	ferpaRepo := postgres.NewGamificationFerpaFieldTagRepository(g)

	emitter := gamification.NewEmitter(gamification.EmitterDeps{
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
	})

	return viewFixture{
		db:       g,
		tenantID: account.ID,
		courseID: course.ID,
		pageID:   page.ID,
		userID:   77,
		emitter:  emitter,
		pageRepo: postgres.NewPageRepository(g),
		crsRepo:  postgres.NewCourseRepository(g),
	}
}

// TestViewedContentEmitCallback_HappyPath asserts the canonical path:
// invoke the callback with a real page ID, see exactly one
// gamification_events row with verb=viewed and the right tenant/actor.
func TestViewedContentEmitCallback_HappyPath(t *testing.T) {
	fx := setupViewFixture(t)

	cb := wiring.ViewedContentEmitCallback(fx.emitter, fx.pageRepo, fx.crsRepo)
	cb(context.Background(), fx.userID, gamification.ObjectPage, fx.pageID, 42)

	var events []models.GamificationEvent
	if err := fx.db.Where("verb = ?", gamification.VerbViewed).Find(&events).Error; err != nil {
		t.Fatalf("list events: %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("expected 1 viewed event, got %d", len(events))
	}
	ev := events[0]
	if ev.ObjectType != gamification.ObjectPage {
		t.Errorf("ObjectType = %q, want %q", ev.ObjectType, gamification.ObjectPage)
	}
	if ev.ObjectID == nil || *ev.ObjectID != fx.pageID {
		t.Errorf("ObjectID = %v, want %d", ev.ObjectID, fx.pageID)
	}
	if ev.ActorID != fx.userID {
		t.Errorf("ActorID = %d, want %d", ev.ActorID, fx.userID)
	}
	if ev.TenantID != fx.tenantID {
		t.Errorf("TenantID = %d, want %d", ev.TenantID, fx.tenantID)
	}
	if ev.Source != gamification.EmitterSource {
		t.Errorf("Source = %q, want %q", ev.Source, gamification.EmitterSource)
	}

	// Result should carry duration_seconds=42.
	var resultPayload map[string]any
	if err := json.Unmarshal(ev.Result, &resultPayload); err != nil {
		t.Fatalf("unmarshal result: %v", err)
	}
	if got, _ := resultPayload["duration_seconds"].(float64); int64(got) != 42 {
		t.Errorf("result.duration_seconds = %v, want 42", resultPayload["duration_seconds"])
	}

	// Context should carry course_id.
	var ctxPayload map[string]any
	if err := json.Unmarshal(ev.Context, &ctxPayload); err != nil {
		t.Fatalf("unmarshal context: %v", err)
	}
	if got, _ := ctxPayload["course_id"].(float64); uint(got) != fx.courseID {
		t.Errorf("context.course_id = %v, want %d", ctxPayload["course_id"], fx.courseID)
	}
}

// TestViewedContentEmitCallback_WrongObjectType confirms an unsupported
// object_type is logged-and-skipped: no event row, no panic.
func TestViewedContentEmitCallback_WrongObjectType(t *testing.T) {
	fx := setupViewFixture(t)

	cb := wiring.ViewedContentEmitCallback(fx.emitter, fx.pageRepo, fx.crsRepo)
	cb(context.Background(), fx.userID, "Quiz", fx.pageID, 0)

	var count int64
	if err := fx.db.Model(&models.GamificationEvent{}).Count(&count).Error; err != nil {
		t.Fatalf("count events: %v", err)
	}
	if count != 0 {
		t.Fatalf("expected 0 events for unsupported object type, got %d", count)
	}
}

// TestViewedContentEmitCallback_MissingPage confirms a non-existent page
// ID is logged-and-skipped: no event row, no panic.
func TestViewedContentEmitCallback_MissingPage(t *testing.T) {
	fx := setupViewFixture(t)

	cb := wiring.ViewedContentEmitCallback(fx.emitter, fx.pageRepo, fx.crsRepo)
	cb(context.Background(), fx.userID, gamification.ObjectPage, 999999, 0)

	var count int64
	if err := fx.db.Model(&models.GamificationEvent{}).Count(&count).Error; err != nil {
		t.Fatalf("count events: %v", err)
	}
	if count != 0 {
		t.Fatalf("expected 0 events for missing page, got %d", count)
	}
}
