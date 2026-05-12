package wiring_test

// Sprint D-1 end-to-end test: prove that the full callback wiring works
// against real services. Build SubmissionService with the
// GradedSubmissionEmitCallback registered, call SubmissionService.Grade
// directly, and assert the rule fires + wallet ledger lands.

import (
	"context"
	"encoding/json"
	"strconv"
	"testing"
	"time"

	"github.com/EduThemes/paper-lms/internal/domain/models"
	pgrepo "github.com/EduThemes/paper-lms/internal/repository/postgres"
	"github.com/EduThemes/paper-lms/internal/service"
	"github.com/EduThemes/paper-lms/internal/service/gamification"
	"github.com/EduThemes/paper-lms/internal/service/gamification/wiring"
	"gorm.io/datatypes"
)

func TestGradeSubmission_TriggersRuleViaCallback(t *testing.T) {
	g, cleanup := freshDB(t)
	defer cleanup()
	ctx := context.Background()

	// Tenant + course + assignment.
	account := models.Account{Name: "E2E Tenant", WorkflowState: "active"}
	if err := g.Create(&account).Error; err != nil {
		t.Fatalf("create account: %v", err)
	}
	course := models.Course{
		AccountID:     account.ID,
		Name:          "E2E Course",
		CourseCode:    "E2E101",
		WorkflowState: "available",
	}
	if err := g.Create(&course).Error; err != nil {
		t.Fatalf("create course: %v", err)
	}
	pointsPossible := 100.0
	assignment := models.Assignment{
		CourseID:       course.ID,
		Name:           "Graded Assignment",
		WorkflowState:  "published",
		PointsPossible: &pointsPossible,
	}
	if err := g.Create(&assignment).Error; err != nil {
		t.Fatalf("create assignment: %v", err)
	}

	const learnerID uint = 1001

	// Pre-existing submission (Grade() updates rather than creates).
	zeroScore := 0.0
	now := time.Now()
	sub := models.Submission{
		AssignmentID:  assignment.ID,
		UserID:        learnerID,
		Score:         &zeroScore,
		SubmittedAt:   &now,
		WorkflowState: "submitted",
	}
	if err := g.Create(&sub).Error; err != nil {
		t.Fatalf("create submission: %v", err)
	}

	// Seed system currencies (so the AwardCurrency effect can resolve xp).
	if err := gamification.SeedSystemCurrenciesForTenant(ctx, g, account.ID); err != nil {
		t.Fatalf("seed currencies: %v", err)
	}
	var xp models.GamificationCurrencyType
	if err := g.Where("tenant_id = ? AND code = ?", account.ID, "xp").First(&xp).Error; err != nil {
		t.Fatalf("look up xp: %v", err)
	}

	// Rule: when assignment graded with score >= 90, award 50 xp.
	conditionSet, _ := json.Marshal(map[string]any{
		"kind":          "SubmittedAssignment",
		"assignment_id": assignment.ID,
		"min_score":     90.0,
	})
	effectsJSON, _ := json.Marshal([]map[string]any{
		{"kind": "AwardCurrency", "code": "xp", "amount": 50},
	})
	trigger, _ := json.Marshal(map[string]any{
		"kind":        "OnEvent",
		"verb":        "graded",
		"object_type": "Submission",
	})
	rule := models.GamificationRule{
		TenantID:      account.ID,
		ScopeType:     models.ScopeSite,
		ScopeID:       account.ID,
		AudienceLevel: models.AudienceHigherEd,
		Name:          "Award XP on graded assignment ≥ 90",
		Enabled:       true,
		TriggerEvent:  datatypes.JSON(trigger),
		ConditionSet:  datatypes.JSON(conditionSet),
		Effects:       datatypes.JSON(effectsJSON),
	}
	if err := g.Create(&rule).Error; err != nil {
		t.Fatalf("create rule: %v", err)
	}

	// Wire the SubmissionService with the real callback. Mirrors
	// cmd/server/main.go's wiring exactly.
	submissionRepo := pgrepo.NewSubmissionRepository(g)
	assignmentRepo := pgrepo.NewAssignmentRepository(g)
	enrollmentRepo := pgrepo.NewEnrollmentRepository(g)
	latePolicyRepo := pgrepo.NewLatePolicyRepository(g)
	courseRepo := pgrepo.NewCourseRepository(g)
	gradingPeriodGroupRepo := pgrepo.NewGradingPeriodGroupRepository(g)
	gradingPeriodRepo := pgrepo.NewGradingPeriodRepository(g)
	groupMembershipRepo := pgrepo.NewGroupMembershipRepository(g)
	submissionService := service.NewSubmissionService(
		submissionRepo, assignmentRepo, enrollmentRepo, latePolicyRepo,
		courseRepo, gradingPeriodGroupRepo, gradingPeriodRepo, groupMembershipRepo,
	)

	emitter := buildEmitter(t, g)
	submissionService.OnGraded(wiring.GradedSubmissionEmitCallback(
		emitter, submissionRepo, assignmentRepo, courseRepo,
	))

	// Grade the submission to 95.
	gradedSub, err := submissionService.Grade(ctx, assignment.ID, learnerID, 0, "95")
	if err != nil {
		t.Fatalf("grade: %v", err)
	}
	if gradedSub.Score == nil || *gradedSub.Score != 95 {
		t.Fatalf("expected score 95, got %+v", gradedSub.Score)
	}

	// The OnGraded callback fires in a goroutine. Poll briefly for the
	// downstream wallet transaction to appear so the assertion isn't
	// flaky on slow runners.
	deadline := time.Now().Add(5 * time.Second)
	var tx models.GamificationWalletTransaction
	for {
		err := g.Where("user_id = ? AND currency_type_id = ?", learnerID, xp.ID).First(&tx).Error
		if err == nil {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("wallet transaction never landed; last err: %v", err)
		}
		time.Sleep(50 * time.Millisecond)
	}
	if tx.Delta != 50 {
		t.Fatalf("expected delta=50, got %d", tx.Delta)
	}

	var balance models.GamificationWalletBalance
	if err := g.Where("user_id = ? AND currency_type_id = ?", learnerID, xp.ID).First(&balance).Error; err != nil {
		t.Fatalf("get balance: %v", err)
	}
	if balance.Balance != 50 {
		t.Fatalf("expected balance=50, got %+v", balance)
	}

	// rule_evaluation row should exist.
	var evalCount int64
	if err := g.Model(&models.GamificationRuleEvaluation{}).
		Where("rule_id = ? AND user_id = ?", rule.ID, learnerID).
		Count(&evalCount).Error; err != nil {
		t.Fatalf("count evals: %v", err)
	}
	if evalCount != 1 {
		t.Fatalf("expected 1 rule_evaluation row, got %d", evalCount)
	}

	// strconv is imported because future variations might parse the
	// score-string back; keep it here so the import doesn't drift.
	_ = strconv.Itoa
}
