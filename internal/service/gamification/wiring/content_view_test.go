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
	"encoding/json"
	"testing"

	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/repository"
	"github.com/EduThemes/paper-lms/internal/repository/postgres"
	"github.com/EduThemes/paper-lms/internal/service/gamification"
	"github.com/EduThemes/paper-lms/internal/service/gamification/wiring"
	"gorm.io/gorm"
)

// DB plumbing (freshDB + swapDatabase) lives in testhelpers_test.go and is
// shared across every wiring_test integration test.

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

	// Shared buildEmitter from testhelpers_test.go wires every repository
	// the gamification.Emitter needs against this scratch GORM connection.
	emitter := buildEmitter(t, g)

	learner := seedTestUser(t, g, account.ID, "viewer@example.test")

	return viewFixture{
		db:       g,
		tenantID: account.ID,
		courseID: course.ID,
		pageID:   page.ID,
		userID:   learner.ID,
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
