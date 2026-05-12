package gamification_test

// End-to-end integration test for the gamification engine. Builds a fresh
// Postgres, seeds a tenant + currencies + assignment + submission + rule,
// emits a matching event, and asserts every downstream artifact: the
// rule_evaluation audit row, the wallet transaction, the resulting
// balance. A second test verifies cooldown enforcement on the same rule.

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/repository/postgres"
	"github.com/EduThemes/paper-lms/internal/service/gamification"
	"github.com/EduThemes/paper-lms/internal/service/gamification/effects"
	"github.com/EduThemes/paper-lms/internal/service/gamification/predicates"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

// fixture bundles everything one e2e test needs. Built once per test so
// each test is hermetic.
type emitterFixture struct {
	db           *gorm.DB
	tenantID     uint
	userID       uint
	assignmentID uint
	ruleID       uint
	xpCurrencyID uint
	emitter      *gamification.Emitter
}

func setupEmitterFixture(t *testing.T, cooldownSeconds *int) emitterFixture {
	t.Helper()
	g, cleanup := freshDB(t)
	t.Cleanup(cleanup)
	ctx := context.Background()

	// Tenant — accounts.id becomes tenant_id for downstream rows.
	account := models.Account{Name: "Test Tenant", WorkflowState: "active"}
	if err := g.Create(&account).Error; err != nil {
		t.Fatalf("create account: %v", err)
	}
	tenantID := account.ID

	// Seed system currencies so xp exists at site scope for the rule's
	// AwardCurrency effect to resolve.
	if err := gamification.SeedSystemCurrenciesForTenant(ctx, g, tenantID); err != nil {
		t.Fatalf("seed currencies: %v", err)
	}
	var xp models.GamificationCurrencyType
	if err := g.Where("tenant_id = ? AND code = ?", tenantID, "xp").First(&xp).Error; err != nil {
		t.Fatalf("look up xp currency: %v", err)
	}

	// Assignment row — only the submission predicate reads it (indirectly,
	// by id), but ListByUserAndAssignmentIDs walks the submissions table.
	// We still create a real Assignment so the FK is satisfied if any
	// later check needs it.
	pointsPossible := 100.0
	assignment := models.Assignment{Name: "Reading 1", CourseID: 1, WorkflowState: "published", PointsPossible: &pointsPossible}
	if err := g.Create(&assignment).Error; err != nil {
		t.Fatalf("create assignment: %v", err)
	}

	// User submitted with score 95.
	score := 95.0
	submittedAt := time.Now()
	sub := models.Submission{
		AssignmentID:  assignment.ID,
		UserID:        42,
		SubmittedAt:   &submittedAt,
		Score:         &score,
		WorkflowState: "graded",
	}
	if err := g.Create(&sub).Error; err != nil {
		t.Fatalf("create submission: %v", err)
	}

	// Rule: when verb=completed and object_type=Assignment, IF
	// SubmittedAssignment(id, min_score=90), THEN AwardCurrency(xp, 50).
	conditionSet, err := json.Marshal(map[string]any{
		"kind":          "SubmittedAssignment",
		"assignment_id": assignment.ID,
		"min_score":     90.0,
	})
	if err != nil {
		t.Fatalf("marshal condition: %v", err)
	}
	effectsJSON, err := json.Marshal([]map[string]any{
		{"kind": "AwardCurrency", "code": "xp", "amount": 50},
	})
	if err != nil {
		t.Fatalf("marshal effects: %v", err)
	}
	trigger, err := json.Marshal(map[string]any{
		"kind":        "OnEvent",
		"verb":        "completed",
		"object_type": "Assignment",
	})
	if err != nil {
		t.Fatalf("marshal trigger: %v", err)
	}
	rule := models.GamificationRule{
		TenantID:        tenantID,
		ScopeType:       models.ScopeSite,
		ScopeID:         tenantID,
		AudienceLevel:   models.AudienceHigherEd,
		Name:            "Award XP on assignment score ≥ 90",
		Enabled:         true,
		TriggerEvent:    datatypes.JSON(trigger),
		ConditionSet:    datatypes.JSON(conditionSet),
		Effects:         datatypes.JSON(effectsJSON),
		CooldownSeconds: cooldownSeconds,
	}
	if err := g.Create(&rule).Error; err != nil {
		t.Fatalf("create rule: %v", err)
	}

	// Wire repos exactly the way cmd/server will in Sprint D.
	subRepo := postgres.NewSubmissionRepository(g)
	quizSubRepo := postgres.NewQuizSubmissionRepository(g)
	outcomeRepo := postgres.NewLearningOutcomeResultRepository(g)
	contentViewRepo := postgres.NewContentViewRepository(g)
	walletRepo := postgres.NewGamificationWalletRepository(g)
	currencyRepo := postgres.NewGamificationCurrencyTypeRepository(g)
	ruleRepo := postgres.NewGamificationRuleRepository(g)
	eventRepo := postgres.NewGamificationEventRepository(g)
	ferpaRepo := postgres.NewGamificationFerpaFieldTagRepository(g)

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
	emitter := gamification.NewEmitter(deps)

	return emitterFixture{
		db:           g,
		tenantID:     tenantID,
		userID:       42,
		assignmentID: assignment.ID,
		ruleID:       rule.ID,
		xpCurrencyID: xp.ID,
		emitter:      emitter,
	}
}

// TestEmit_AwardXP_EndToEnd is the headline proof. It walks the full
// pipeline: emit → FERPA pre-flight → persist event → build rule index
// → snapshot load → predicate evaluation → AwardCurrency effect →
// rule_evaluation row + wallet transaction + balance.
func TestEmit_AwardXP_EndToEnd(t *testing.T) {
	fx := setupEmitterFixture(t, nil)

	event := &models.GamificationEvent{
		OccurredAt: time.Now(),
		TenantID:   fx.tenantID,
		ActorID:    fx.userID,
		Verb:       "completed",
		ObjectType: "Assignment",
		ObjectID:   &fx.assignmentID,
		Source:     "internal",
	}
	result, err := fx.emitter.Emit(context.Background(), event)
	if err != nil {
		t.Fatalf("emit: %v", err)
	}

	if result.EventID == 0 {
		t.Fatalf("expected non-zero event id, got %d", result.EventID)
	}
	if result.Dispatch.RulesFired != 1 {
		t.Fatalf("expected 1 rule fired, got %+v", result.Dispatch)
	}
	if len(result.Dispatch.Outcomes) != 1 || !result.Dispatch.Outcomes[0].Fired {
		t.Fatalf("expected outcome[0].Fired=true, got %+v", result.Dispatch.Outcomes)
	}
	if len(result.Dispatch.Outcomes[0].Effects) != 1 || result.Dispatch.Outcomes[0].Effects[0].Kind != "AwardCurrency" {
		t.Fatalf("expected one AwardCurrency effect, got %+v", result.Dispatch.Outcomes[0].Effects)
	}

	// Assert side-effects in the database.
	gdb := fx.db

	var evals []models.GamificationRuleEvaluation
	if err := gdb.Where("rule_id = ? AND user_id = ?", fx.ruleID, fx.userID).Find(&evals).Error; err != nil {
		t.Fatalf("list evals: %v", err)
	}
	if len(evals) != 1 {
		t.Fatalf("expected 1 rule_evaluation row, got %d", len(evals))
	}
	if !evals[0].Result {
		t.Fatalf("expected eval.Result=true, got %+v", evals[0])
	}

	var txs []models.GamificationWalletTransaction
	if err := gdb.Where("user_id = ? AND currency_type_id = ?", fx.userID, fx.xpCurrencyID).Find(&txs).Error; err != nil {
		t.Fatalf("list txs: %v", err)
	}
	if len(txs) != 1 || txs[0].Delta != 50 {
		t.Fatalf("expected one wallet tx with delta=50, got %+v", txs)
	}
	if txs[0].TriggeringRuleID == nil || *txs[0].TriggeringRuleID != fx.ruleID {
		t.Fatalf("expected triggering_rule_id=%d, got %v", fx.ruleID, txs[0].TriggeringRuleID)
	}

	var balance models.GamificationWalletBalance
	if err := gdb.Where("user_id = ? AND currency_type_id = ?", fx.userID, fx.xpCurrencyID).First(&balance).Error; err != nil {
		t.Fatalf("get balance: %v", err)
	}
	if balance.Balance != 50 || balance.LifetimeEarned != 50 {
		t.Fatalf("expected balance=50 lifetime=50, got %+v", balance)
	}
}

// TestEmit_NotMatching_NoFire confirms that an event for an unrelated
// (verb, object_type) doesn't trip the rule.
func TestEmit_NotMatching_NoFire(t *testing.T) {
	fx := setupEmitterFixture(t, nil)

	objectID := fx.assignmentID
	event := &models.GamificationEvent{
		OccurredAt: time.Now(),
		TenantID:   fx.tenantID,
		ActorID:    fx.userID,
		Verb:       "viewed", // not "completed"
		ObjectType: "Assignment",
		ObjectID:   &objectID,
		Source:     "internal",
	}
	result, err := fx.emitter.Emit(context.Background(), event)
	if err != nil {
		t.Fatalf("emit: %v", err)
	}
	if result.Dispatch.RulesFired != 0 || result.Dispatch.RulesConsidered != 0 {
		t.Fatalf("expected zero rules considered/fired, got %+v", result.Dispatch)
	}

	gdb := fx.db
	var count int64
	if err := gdb.Model(&models.GamificationWalletTransaction{}).Count(&count).Error; err != nil {
		t.Fatalf("count txs: %v", err)
	}
	if count != 0 {
		t.Fatalf("expected zero wallet txs, got %d", count)
	}
}

// TestEmit_PredicateFalse_NoEffects: matching trigger, but the score gate
// rejects. Expected outcome: rule_evaluation row with result=false, no
// wallet transaction.
func TestEmit_PredicateFalse_NoEffects(t *testing.T) {
	fx := setupEmitterFixture(t, nil)
	gdb := fx.db

	// Re-grade the existing submission to a sub-threshold score.
	if err := gdb.Model(&models.Submission{}).Where("assignment_id = ? AND user_id = ?", fx.assignmentID, fx.userID).Update("score", 75).Error; err != nil {
		t.Fatalf("update score: %v", err)
	}

	event := &models.GamificationEvent{
		OccurredAt: time.Now(),
		TenantID:   fx.tenantID,
		ActorID:    fx.userID,
		Verb:       "completed",
		ObjectType: "Assignment",
		ObjectID:   &fx.assignmentID,
		Source:     "internal",
	}
	result, err := fx.emitter.Emit(context.Background(), event)
	if err != nil {
		t.Fatalf("emit: %v", err)
	}
	if result.Dispatch.RulesFalse != 1 {
		t.Fatalf("expected RulesFalse=1, got %+v", result.Dispatch)
	}

	var evals []models.GamificationRuleEvaluation
	if err := gdb.Where("rule_id = ?", fx.ruleID).Find(&evals).Error; err != nil {
		t.Fatalf("list evals: %v", err)
	}
	if len(evals) != 1 || evals[0].Result {
		t.Fatalf("expected one eval with result=false, got %+v", evals)
	}

	var txCount int64
	if err := gdb.Model(&models.GamificationWalletTransaction{}).Count(&txCount).Error; err != nil {
		t.Fatalf("count txs: %v", err)
	}
	if txCount != 0 {
		t.Fatalf("expected zero wallet txs when predicate is false, got %d", txCount)
	}
}

// TestEmit_Cooldown blocks a same-rule re-fire within the configured
// cooldown window. The first emit lands the award; the second is
// blocked with BlockedBy=cooldown.
func TestEmit_Cooldown(t *testing.T) {
	sixty := 60
	fx := setupEmitterFixture(t, &sixty)

	// First emit fires the rule normally.
	event := &models.GamificationEvent{
		OccurredAt: time.Now(),
		TenantID:   fx.tenantID,
		ActorID:    fx.userID,
		Verb:       "completed",
		ObjectType: "Assignment",
		ObjectID:   &fx.assignmentID,
		Source:     "internal",
	}
	first, err := fx.emitter.Emit(context.Background(), event)
	if err != nil {
		t.Fatalf("first emit: %v", err)
	}
	if first.Dispatch.RulesFired != 1 {
		t.Fatalf("expected first emit to fire 1 rule, got %+v", first.Dispatch)
	}

	// Second emit, immediately after — should be blocked by cooldown.
	event2 := &models.GamificationEvent{
		OccurredAt: time.Now(),
		TenantID:   fx.tenantID,
		ActorID:    fx.userID,
		Verb:       "completed",
		ObjectType: "Assignment",
		ObjectID:   &fx.assignmentID,
		Source:     "internal",
	}
	second, err := fx.emitter.Emit(context.Background(), event2)
	if err != nil {
		t.Fatalf("second emit: %v", err)
	}
	if second.Dispatch.RulesBlocked != 1 || second.Dispatch.RulesFired != 0 {
		t.Fatalf("expected second emit blocked, got %+v", second.Dispatch)
	}
	if got := second.Dispatch.Outcomes[0].BlockedBy; got != "cooldown" {
		t.Fatalf("expected BlockedBy=cooldown, got %q", got)
	}

	// Wallet still shows exactly one transaction.
	gdb := fx.db
	var txCount int64
	if err := gdb.Model(&models.GamificationWalletTransaction{}).Count(&txCount).Error; err != nil {
		t.Fatalf("count txs: %v", err)
	}
	if txCount != 1 {
		t.Fatalf("expected exactly 1 wallet tx after cooldown-blocked second emit, got %d", txCount)
	}
}

// TestEmit_PredicateExposesEnumValues sanity-checks that the
// reputation predicate compiles against the gamification engine end to
// end. Sprint D adds the actual capability-unlock path; this just guards
// against the ReputationCode constant drifting out of sync with the
// seeded currency table.
func TestEmit_ReputationCurrencySeeded(t *testing.T) {
	fx := setupEmitterFixture(t, nil)
	gdb := fx.db
	var rep models.GamificationCurrencyType
	if err := gdb.Where("tenant_id = ? AND code = ?", fx.tenantID, predicates.ReputationCode).First(&rep).Error; err != nil {
		t.Fatalf("reputation currency not seeded: %v", err)
	}
	if !rep.SystemOwned {
		t.Fatalf("reputation should be system_owned=true")
	}
}

