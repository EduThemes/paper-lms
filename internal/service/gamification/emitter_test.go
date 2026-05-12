package gamification_test

// End-to-end integration test for the gamification engine. Builds a fresh
// Postgres, seeds a tenant + currencies + assignment + submission + rule,
// emits a matching event, and asserts every downstream artifact: the
// rule_evaluation audit row, the wallet transaction, the resulting
// balance. A second test verifies cooldown enforcement on the same rule.

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/repository/postgres"
	"github.com/EduThemes/paper-lms/internal/service/gamification"
	"github.com/EduThemes/paper-lms/internal/service/gamification/effects"
	"github.com/EduThemes/paper-lms/internal/service/gamification/predicates"
	"github.com/lib/pq"
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

// TestEmit_ConditionSet_AND_EndToEnd proves that a rule with a
// ConditionSet (AND of two atomic predicates) decodes correctly via the
// predicate factory and fires end-to-end. This is the regression test for
// the JSON casing bug found in code review — the factory's ConditionSet
// shell must match the struct's snake_case tags.
func TestEmit_ConditionSet_AND_EndToEnd(t *testing.T) {
	fx := setupEmitterFixture(t, nil)
	gdb := fx.db

	// Replace the fixture's atomic rule with a ConditionSet (AND).
	conditionSet, err := json.Marshal(map[string]any{
		"kind": "ConditionSet",
		"op":   "AND",
		"children": []map[string]any{
			{"kind": "SubmittedAssignment", "assignment_id": fx.assignmentID, "min_score": 90.0},
			{"kind": "CurrencyThreshold", "code": "xp", "min_amount": 0}, // trivially true on initial balance
		},
	})
	if err != nil {
		t.Fatalf("marshal condition_set: %v", err)
	}
	if err := gdb.Model(&models.GamificationRule{}).Where("id = ?", fx.ruleID).Update("condition_set", datatypes.JSON(conditionSet)).Error; err != nil {
		t.Fatalf("update rule: %v", err)
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
	if result.Dispatch.RulesFired != 1 {
		t.Fatalf("expected 1 rule fired through ConditionSet, got %+v", result.Dispatch)
	}

	var txs []models.GamificationWalletTransaction
	if err := gdb.Find(&txs).Error; err != nil {
		t.Fatalf("list txs: %v", err)
	}
	if len(txs) != 1 || txs[0].Delta != 50 {
		t.Fatalf("expected one wallet tx of +50, got %+v", txs)
	}
}

// TestEmit_ConditionSet_OR_NoneMatch_NoFire wires up an OR of two
// predicates that both fail; the rule should not fire.
func TestEmit_ConditionSet_OR_NoneMatch_NoFire(t *testing.T) {
	fx := setupEmitterFixture(t, nil)
	gdb := fx.db

	conditionSet, err := json.Marshal(map[string]any{
		"kind": "ConditionSet",
		"op":   "OR",
		"children": []map[string]any{
			{"kind": "SubmittedAssignment", "assignment_id": fx.assignmentID, "min_score": 1000.0}, // unreachable
			{"kind": "CurrencyThreshold", "code": "xp", "min_amount": 999999},                       // unreachable
		},
	})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if err := gdb.Model(&models.GamificationRule{}).Where("id = ?", fx.ruleID).Update("condition_set", datatypes.JSON(conditionSet)).Error; err != nil {
		t.Fatalf("update rule: %v", err)
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
	if result.Dispatch.RulesFalse != 1 || result.Dispatch.RulesFired != 0 {
		t.Fatalf("expected RulesFalse=1 RulesFired=0, got %+v", result.Dispatch)
	}
}

// TestEmit_MaxPerWindow blocks a second fire within the configured
// rolling window even with no cooldown_seconds. Complements
// TestEmit_Cooldown (which exercises the cooldown_seconds gate only).
func TestEmit_MaxPerWindow(t *testing.T) {
	fx := setupEmitterFixture(t, nil)
	gdb := fx.db

	// Configure max_per_window: 1 fire per day, no cooldown.
	maxJSON, err := json.Marshal(map[string]any{"window": "day", "count": 1})
	if err != nil {
		t.Fatalf("marshal max_per_window: %v", err)
	}
	if err := gdb.Model(&models.GamificationRule{}).Where("id = ?", fx.ruleID).Update("max_per_window", datatypes.JSON(maxJSON)).Error; err != nil {
		t.Fatalf("update rule: %v", err)
	}

	event := func() *models.GamificationEvent {
		return &models.GamificationEvent{
			OccurredAt: time.Now(),
			TenantID:   fx.tenantID,
			ActorID:    fx.userID,
			Verb:       "completed",
			ObjectType: "Assignment",
			ObjectID:   &fx.assignmentID,
			Source:     "internal",
		}
	}

	first, err := fx.emitter.Emit(context.Background(), event())
	if err != nil {
		t.Fatalf("first emit: %v", err)
	}
	if first.Dispatch.RulesFired != 1 {
		t.Fatalf("expected first emit to fire, got %+v", first.Dispatch)
	}

	second, err := fx.emitter.Emit(context.Background(), event())
	if err != nil {
		t.Fatalf("second emit: %v", err)
	}
	if second.Dispatch.RulesBlocked != 1 || second.Dispatch.RulesFired != 0 {
		t.Fatalf("expected second emit blocked by max_per_window, got %+v", second.Dispatch)
	}
	if got := second.Dispatch.Outcomes[0].BlockedBy; got != string(gamification.GateMaxPerWindow) {
		t.Fatalf("expected BlockedBy=max_per_window, got %q", got)
	}

	var txCount int64
	if err := gdb.Model(&models.GamificationWalletTransaction{}).Count(&txCount).Error; err != nil {
		t.Fatalf("count txs: %v", err)
	}
	if txCount != 1 {
		t.Fatalf("expected exactly 1 wallet tx after max_per_window-blocked second emit, got %d", txCount)
	}
}

// TestEmit_FerpaViolation rejects an event whose result carries an
// education_record-classified field without the matching policy flags.
// Proves the FERPA guard is wired into Emit's pre-flight, not just unit-
// tested in isolation.
func TestEmit_FerpaViolation(t *testing.T) {
	fx := setupEmitterFixture(t, nil)
	gdb := fx.db

	// Declare "result.score" on Assignment events as education_record.
	if err := gdb.Create(&models.GamificationFerpaFieldTag{
		ObjectType:     "Assignment",
		FieldPath:      "result.score",
		Classification: "education_record",
	}).Error; err != nil {
		t.Fatalf("create ferpa tag: %v", err)
	}

	// Event carries result.score but no policy_flags.
	resultBlob, err := json.Marshal(map[string]any{"score": 91})
	if err != nil {
		t.Fatalf("marshal result: %v", err)
	}
	event := &models.GamificationEvent{
		OccurredAt: time.Now(),
		TenantID:   fx.tenantID,
		ActorID:    fx.userID,
		Verb:       "completed",
		ObjectType: "Assignment",
		ObjectID:   &fx.assignmentID,
		Result:     datatypes.JSON(resultBlob),
		Source:     "internal",
	}
	_, err = fx.emitter.Emit(context.Background(), event)
	if err == nil {
		t.Fatalf("expected FERPA violation error, got nil")
	}
	if !strings.Contains(err.Error(), "ferpa") {
		t.Fatalf("expected error mentioning ferpa, got %v", err)
	}

	// Confirm no event row was persisted (FERPA fails before Create).
	var eventCount int64
	if err := gdb.Model(&models.GamificationEvent{}).Count(&eventCount).Error; err != nil {
		t.Fatalf("count events: %v", err)
	}
	if eventCount != 0 {
		t.Fatalf("expected zero events persisted on FERPA failure, got %d", eventCount)
	}

	// Now retry with the required policy flags — should succeed.
	event2 := &models.GamificationEvent{
		OccurredAt:  time.Now(),
		TenantID:    fx.tenantID,
		ActorID:     fx.userID,
		Verb:        "completed",
		ObjectType:  "Assignment",
		ObjectID:    &fx.assignmentID,
		Result:      datatypes.JSON(resultBlob),
		PolicyFlags: pqStringArray("ferpa_protected", "education_record"),
		Source:      "internal",
	}
	if _, err := fx.emitter.Emit(context.Background(), event2); err != nil {
		t.Fatalf("expected emit to succeed once flags set, got %v", err)
	}
}

// TestEmit_EffectFailure_StopOnError proves the dispatcher's stop-on-
// first-error semantics through Emit. We rig a rule whose AwardCurrency
// effect references a currency that doesn't exist; the effect errors,
// later effects (if any) are skipped, and no wallet transaction is
// produced. The audit row records the failure in effects_fired.
func TestEmit_EffectFailure_StopOnError(t *testing.T) {
	fx := setupEmitterFixture(t, nil)
	gdb := fx.db

	// Point the rule at a non-existent currency code so AwardCurrency
	// fails at ResolveCurrencyByCode. A second AwardCurrency follows;
	// we expect it to be marked skipped.
	effectsJSON, err := json.Marshal([]map[string]any{
		{"kind": "AwardCurrency", "code": "does_not_exist", "amount": 50},
		{"kind": "AwardCurrency", "code": "xp", "amount": 25},
	})
	if err != nil {
		t.Fatalf("marshal effects: %v", err)
	}
	if err := gdb.Model(&models.GamificationRule{}).Where("id = ?", fx.ruleID).Update("effects", datatypes.JSON(effectsJSON)).Error; err != nil {
		t.Fatalf("update rule: %v", err)
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
	// One rule was considered; predicate evaluated true; one effect failed
	// so the rule's Fired flag is false even though predicates matched.
	if len(result.Dispatch.Outcomes) != 1 {
		t.Fatalf("expected 1 outcome, got %+v", result.Dispatch)
	}
	out := result.Dispatch.Outcomes[0]
	if out.Fired {
		t.Fatalf("expected Fired=false on effect failure, got %+v", out)
	}
	if len(out.EffectErrors) != 2 {
		t.Fatalf("expected 2 effect entries, got %v", out.EffectErrors)
	}
	if out.EffectErrors[0] == "" {
		t.Fatalf("expected effect[0] to carry an error, got empty string")
	}
	if out.EffectErrors[1] != "skipped" {
		t.Fatalf("expected effect[1] to be skipped, got %q", out.EffectErrors[1])
	}

	// Effect 1 (xp +25) must NOT have produced a wallet tx — stop-on-
	// first-error means effects after the failure don't run.
	var txCount int64
	if err := gdb.Model(&models.GamificationWalletTransaction{}).Count(&txCount).Error; err != nil {
		t.Fatalf("count txs: %v", err)
	}
	if txCount != 0 {
		t.Fatalf("expected zero wallet txs on effect-0 failure, got %d", txCount)
	}

	// The rule_evaluation row should still record the attempt so the
	// audit trail is honest (result=true since predicates matched, but
	// effects_fired carries the partial-failure record).
	var evals []models.GamificationRuleEvaluation
	if err := gdb.Where("rule_id = ?", fx.ruleID).Find(&evals).Error; err != nil {
		t.Fatalf("list evals: %v", err)
	}
	if len(evals) != 1 {
		t.Fatalf("expected one eval row, got %d", len(evals))
	}
	if !evals[0].Result {
		t.Fatalf("expected eval.Result=true (predicate matched), got %+v", evals[0])
	}
}

// pqStringArray constructs a pq.StringArray inline so test cases can
// stay terse. Equivalent to pq.StringArray{...} but conveys intent.
func pqStringArray(flags ...string) pq.StringArray {
	return pq.StringArray(flags)
}

