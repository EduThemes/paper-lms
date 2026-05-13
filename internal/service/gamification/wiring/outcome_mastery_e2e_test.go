package wiring_test

// Sprint D-2 end-to-end test: prove that the full
// LearningOutcomeService → OnMasteryCrossed → wiring emit → rule fire →
// wallet ledger path works against real services.
//
// Critically, this is the test that pins the per-row transition
// semantics: a SECOND CreateResult call against the same
// (user, outcome, asset) composite — when prior mastery was already
// true — must NOT re-emit. The assertion is exactly one wallet
// transaction at the end, not two.

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/EduThemes/paper-lms/internal/domain/models"
	pgrepo "github.com/EduThemes/paper-lms/internal/repository/postgres"
	"github.com/EduThemes/paper-lms/internal/service"
	"github.com/EduThemes/paper-lms/internal/service/gamification"
	"github.com/EduThemes/paper-lms/internal/service/gamification/wiring"
	"gorm.io/datatypes"
)

func TestCreateResult_TriggersMasteryRuleOnFirstTransitionOnly(t *testing.T) {
	g, cleanup := freshDB(t)
	defer cleanup()
	ctx := context.Background()

	// Tenant + course.
	account := models.Account{Name: "E2E Mastery Tenant", WorkflowState: "active"}
	if err := g.Create(&account).Error; err != nil {
		t.Fatalf("create account: %v", err)
	}
	course := models.Course{
		AccountID:     account.ID,
		Name:          "E2E Mastery Course",
		CourseCode:    "E2EM101",
		WorkflowState: "available",
	}
	if err := g.Create(&course).Error; err != nil {
		t.Fatalf("create course: %v", err)
	}

	// Outcome group + outcome (Course-scoped). MasteryPoints=3.0.
	group := models.LearningOutcomeGroup{
		ContextType:   "Course",
		ContextID:     course.ID,
		Title:         "Root",
		WorkflowState: "active",
	}
	if err := g.Create(&group).Error; err != nil {
		t.Fatalf("create group: %v", err)
	}
	outcome := models.LearningOutcome{
		ContextType:       "Course",
		ContextID:         course.ID,
		OutcomeGroupID:    group.ID,
		Title:             "Add fractions",
		MasteryPoints:     3.0,
		PointsPossible:    5.0,
		CalculationMethod: "decaying_average",
		CalculationInt:    65,
		RatingsData:       "[]",
		WorkflowState:     "active",
	}
	if err := g.Create(&outcome).Error; err != nil {
		t.Fatalf("create outcome: %v", err)
	}

	const learnerID uint = 7777

	// Seed system currencies so the AwardCurrency effect can resolve xp.
	if err := gamification.SeedSystemCurrenciesForTenant(ctx, g, account.ID); err != nil {
		t.Fatalf("seed currencies: %v", err)
	}
	var xp models.GamificationCurrencyType
	if err := g.Where("tenant_id = ? AND code = ?", account.ID, "xp").First(&xp).Error; err != nil {
		t.Fatalf("look up xp: %v", err)
	}

	// Rule: on verb=mastered, object_type=Outcome → award 100 xp.
	// Predicate is the unconditional AlwaysTrue shape (an empty AND
	// ConditionSet evaluates true), so this fires for every mastered
	// event — the transition guard lives in LearningOutcomeService, not
	// in the rule.
	trigger, _ := json.Marshal(map[string]any{
		"kind":        "OnEvent",
		"verb":        "mastered",
		"object_type": "Outcome",
	})
	conditionSet, _ := json.Marshal(map[string]any{
		"kind":     "ConditionSet",
		"op":       "AND",
		"children": []any{},
	})
	effectsJSON, _ := json.Marshal([]map[string]any{
		{"kind": "AwardCurrency", "code": "xp", "amount": 100},
	})
	rule := models.GamificationRule{
		TenantID:      account.ID,
		ScopeType:     models.ScopeSite,
		ScopeID:       account.ID,
		AudienceLevel: models.AudienceHigherEd,
		Name:          "Award XP on outcome mastery",
		Enabled:       true,
		TriggerEvent:  datatypes.JSON(trigger),
		ConditionSet:  datatypes.JSON(conditionSet),
		Effects:       datatypes.JSON(effectsJSON),
	}
	if err := g.Create(&rule).Error; err != nil {
		t.Fatalf("create rule: %v", err)
	}

	// Build LearningOutcomeService with the real callback wired.
	outcomeGroupRepo := pgrepo.NewLearningOutcomeGroupRepository(g)
	outcomeRepo := pgrepo.NewLearningOutcomeRepository(g)
	outcomeResultRepo := pgrepo.NewLearningOutcomeResultRepository(g)
	courseRepo := pgrepo.NewCourseRepository(g)

	outcomeSvc := service.NewLearningOutcomeService(outcomeGroupRepo, outcomeRepo, outcomeResultRepo)
	emitter := buildEmitter(t, g)
	outcomeSvc.OnMasteryCrossed(wiring.OutcomeMasteryCrossedEmitCallback(
		emitter, outcomeResultRepo, outcomeRepo, courseRepo,
	))

	// First CreateResult — score ≥ mastery_points, no prior row. Mastery
	// flips nil → true. Callback fires.
	score := 5.0
	possible := 5.0
	assessedAt := time.Now()
	firstResult := &models.LearningOutcomeResult{
		UserID:              learnerID,
		LearningOutcomeID:   outcome.ID,
		ContextType:         "Course",
		ContextID:           course.ID,
		AssociatedAssetType: "Assignment",
		AssociatedAssetID:   42,
		Score:               &score,
		Possible:            &possible,
		AssessedAt:          &assessedAt,
		Attempt:             1,
		Title:               outcome.Title,
	}
	if err := outcomeSvc.CreateResult(ctx, firstResult); err != nil {
		t.Fatalf("first CreateResult: %v", err)
	}

	// Poll for the wallet transaction (callback fires in a goroutine).
	deadline := time.Now().Add(5 * time.Second)
	var tx models.GamificationWalletTransaction
	for {
		err := g.Where("user_id = ? AND currency_type_id = ?", learnerID, xp.ID).First(&tx).Error
		if err == nil {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("wallet transaction never landed after first transition; last err: %v", err)
		}
		time.Sleep(50 * time.Millisecond)
	}
	if tx.Delta != 100 {
		t.Fatalf("expected delta=100 on first transition, got %d", tx.Delta)
	}

	// Second CreateResult on the same (user, outcome, asset) composite —
	// still mastered. No transition. Callback MUST NOT fire again. Give
	// the goroutine fan-out time to settle, then assert single tx + single
	// rule_evaluation row.
	scoreAgain := 4.5
	assessedAgain := time.Now()
	secondResult := &models.LearningOutcomeResult{
		UserID:              learnerID,
		LearningOutcomeID:   outcome.ID,
		ContextType:         "Course",
		ContextID:           course.ID,
		AssociatedAssetType: "Assignment",
		AssociatedAssetID:   42,
		Score:               &scoreAgain,
		Possible:            &possible,
		AssessedAt:          &assessedAgain,
		Attempt:             2,
		Title:               outcome.Title,
	}
	if err := outcomeSvc.CreateResult(ctx, secondResult); err != nil {
		t.Fatalf("second CreateResult: %v", err)
	}
	// 500ms is generous — the goroutine fires immediately or not at all.
	time.Sleep(500 * time.Millisecond)

	var txCount int64
	if err := g.Model(&models.GamificationWalletTransaction{}).
		Where("user_id = ? AND currency_type_id = ?", learnerID, xp.ID).
		Count(&txCount).Error; err != nil {
		t.Fatalf("count txs: %v", err)
	}
	if txCount != 1 {
		t.Fatalf("expected exactly 1 wallet transaction (no re-emit on second already-mastered write), got %d", txCount)
	}

	var balance models.GamificationWalletBalance
	if err := g.Where("user_id = ? AND currency_type_id = ?", learnerID, xp.ID).First(&balance).Error; err != nil {
		t.Fatalf("get balance: %v", err)
	}
	if balance.Balance != 100 {
		t.Fatalf("expected balance=100, got %+v", balance)
	}

	// Exactly one mastered event in the event log too.
	var eventCount int64
	if err := g.Model(&models.GamificationEvent{}).
		Where("verb = ? AND object_type = ? AND actor_id = ?",
			gamification.VerbMastered, gamification.ObjectOutcome, learnerID).
		Count(&eventCount).Error; err != nil {
		t.Fatalf("count events: %v", err)
	}
	if eventCount != 1 {
		t.Fatalf("expected exactly 1 mastered event, got %d", eventCount)
	}
}
