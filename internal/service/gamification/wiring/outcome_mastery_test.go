package wiring_test

// DB-integration test for OutcomeMasteryCrossedEmitCallback. Mirrors the
// freshDB / buildEmitter helpers in testhelpers_test.go (do not redefine
// them here). Seeds the canonical account → course → outcome group →
// outcome → mastered result chain, invokes the adapter callback
// directly, and asserts the persisted gamification_events row has the
// right shape.

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/EduThemes/paper-lms/internal/domain/models"
	pgrepo "github.com/EduThemes/paper-lms/internal/repository/postgres"
	"github.com/EduThemes/paper-lms/internal/service/gamification"
	"github.com/EduThemes/paper-lms/internal/service/gamification/wiring"
)

func TestOutcomeMasteryCrossedEmitCallback_EmitsEvent(t *testing.T) {
	g, cleanup := freshDB(t)
	defer cleanup()
	ctx := context.Background()

	// Tenant chain: account → course. course.AccountID is what the
	// callback should resolve as tenant_id for ContextType="Course".
	account := models.Account{Name: "Outcome Mastery Tenant", WorkflowState: "active"}
	if err := g.Create(&account).Error; err != nil {
		t.Fatalf("create account: %v", err)
	}
	course := models.Course{
		AccountID:     account.ID,
		Name:          "Mastery Course",
		CourseCode:    "MAST-101",
		WorkflowState: "available",
	}
	if err := g.Create(&course).Error; err != nil {
		t.Fatalf("create course: %v", err)
	}

	// Seed system currencies so the FERPA preflight + dispatcher walk
	// inside Emit has somewhere to land. No rule is registered, so the
	// dispatcher is a no-op walk.
	if err := gamification.SeedSystemCurrenciesForTenant(ctx, g, account.ID); err != nil {
		t.Fatalf("seed currencies: %v", err)
	}

	// Outcome group + outcome scoped to the course.
	group := models.LearningOutcomeGroup{
		ContextType:   "Course",
		ContextID:     course.ID,
		Title:         "Standards",
		WorkflowState: "active",
	}
	if err := g.Create(&group).Error; err != nil {
		t.Fatalf("create outcome group: %v", err)
	}
	outcome := models.LearningOutcome{
		ContextType:       "Course",
		ContextID:         course.ID,
		OutcomeGroupID:    group.ID,
		Title:             "Solve linear equations",
		CalculationMethod: "decaying_average",
		CalculationInt:    65,
		MasteryPoints:     3.0,
		PointsPossible:    5.0,
		RatingsData:       "[]",
		WorkflowState:     "active",
	}
	if err := g.Create(&outcome).Error; err != nil {
		t.Fatalf("create outcome: %v", err)
	}

	// Mastered result row. Mirrors what LearningOutcomeService.CreateResult
	// would Upsert after the false/nil → true transition the service
	// already detected. We seed it pre-mastered because this adapter
	// only handles the emit side — the transition is upstream.
	score := 4.5
	possible := 5.0
	percent := 0.9
	mastery := true
	assessedAt := time.Now().Add(-2 * time.Minute).UTC().Truncate(time.Second)
	result := models.LearningOutcomeResult{
		UserID:              4242,
		LearningOutcomeID:   outcome.ID,
		ContextType:         "Course",
		ContextID:           course.ID,
		AssociatedAssetType: "Assignment",
		AssociatedAssetID:   777,
		Score:               &score,
		Possible:            &possible,
		Percent:             &percent,
		Mastery:             &mastery,
		AssessedAt:          &assessedAt,
		Attempt:             1,
		Title:               outcome.Title,
	}
	if err := g.Create(&result).Error; err != nil {
		t.Fatalf("create outcome result: %v", err)
	}

	emitter := buildEmitter(t, g)
	cb := wiring.OutcomeMasteryCrossedEmitCallback(
		emitter,
		pgrepo.NewLearningOutcomeResultRepository(g),
		pgrepo.NewLearningOutcomeRepository(g),
		pgrepo.NewCourseRepository(g),
	)

	// Direct invocation — synchronous; no goroutine wrapper, no polling.
	cb(ctx, result.UserID, outcome.ID, result.ID)

	// Exactly one event with verb=mastered, object_type=Outcome.
	var events []models.GamificationEvent
	if err := g.Where("verb = ? AND object_type = ?",
		gamification.VerbMastered, gamification.ObjectOutcome).Find(&events).Error; err != nil {
		t.Fatalf("list events: %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("expected 1 mastered/Outcome event, got %d", len(events))
	}
	ev := events[0]
	if ev.TenantID != account.ID {
		t.Errorf("TenantID = %d, want %d", ev.TenantID, account.ID)
	}
	if ev.ActorID != result.UserID {
		t.Errorf("ActorID = %d, want %d", ev.ActorID, result.UserID)
	}
	if ev.ObjectID == nil || *ev.ObjectID != outcome.ID {
		t.Errorf("ObjectID = %v, want %d (outcome ID, not result ID)", ev.ObjectID, outcome.ID)
	}
	if ev.Source != gamification.EmitterSource {
		t.Errorf("Source = %q, want %q", ev.Source, gamification.EmitterSource)
	}

	// Result JSONB carries mastery:true and result_id.
	var resultDecoded map[string]any
	if err := json.Unmarshal(ev.Result, &resultDecoded); err != nil {
		t.Fatalf("decode result blob: %v (raw=%s)", err, string(ev.Result))
	}
	if got, ok := resultDecoded["mastery"].(bool); !ok || !got {
		t.Errorf("result.mastery = %v, want true", resultDecoded["mastery"])
	}
	gotResultID, ok := resultDecoded["result_id"].(float64)
	if !ok {
		t.Fatalf("result.result_id not a number: %v (raw=%s)", resultDecoded["result_id"], string(ev.Result))
	}
	if uint(gotResultID) != result.ID {
		t.Errorf("result.result_id = %v, want %d", gotResultID, result.ID)
	}

	// Context JSONB carries context_type, context_id, calculation_method.
	var contextDecoded map[string]any
	if err := json.Unmarshal(ev.Context, &contextDecoded); err != nil {
		t.Fatalf("decode context blob: %v (raw=%s)", err, string(ev.Context))
	}
	if got := contextDecoded["context_type"]; got != "Course" {
		t.Errorf("context.context_type = %v, want %q", got, "Course")
	}
	gotContextID, ok := contextDecoded["context_id"].(float64)
	if !ok {
		t.Fatalf("context.context_id not a number: %v (raw=%s)", contextDecoded["context_id"], string(ev.Context))
	}
	if uint(gotContextID) != course.ID {
		t.Errorf("context.context_id = %v, want %d", gotContextID, course.ID)
	}
	if got := contextDecoded["calculation_method"]; got != "decaying_average" {
		t.Errorf("context.calculation_method = %v, want %q", got, "decaying_average")
	}
}
