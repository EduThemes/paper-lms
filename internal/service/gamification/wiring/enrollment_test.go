package wiring_test

// DB-integration tests for the EnrolledCourseEmitCallback wiring. Gated
// on PARITY_DB_URL / DATABASE_URL — mirrors the freshDB pattern used by
// internal/service/gamification/seed_test.go so `go test ./...` stays
// green on laptops without a dev container.

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/repository"
	pgrepo "github.com/EduThemes/paper-lms/internal/repository/postgres"
	"github.com/EduThemes/paper-lms/internal/service/gamification"
	"github.com/EduThemes/paper-lms/internal/service/gamification/wiring"
	"gorm.io/gorm"
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
// freshDB. Uses the shared buildEmitter helper in testhelpers_test.go and
// adds just the two repos the enrollment callback closes over.
func buildEmitterFixture(t *testing.T, g *gorm.DB) fixture {
	t.Helper()
	return fixture{
		emitter:        buildEmitter(t, g),
		enrollmentRepo: pgrepo.NewEnrollmentRepository(g),
		courseRepo:     pgrepo.NewCourseRepository(g),
	}
}

// DB plumbing (freshDB + swapDatabase) lives in testhelpers_test.go and is
// shared across every wiring_test integration test.
