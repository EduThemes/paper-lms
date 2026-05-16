package gamification_test

// DB-integration tests for the snapshot loader. Gated on
// PARITY_DB_URL/DATABASE_URL exactly like seed_test.go in this same
// package — reuses the freshDB helper there so concurrent test runs
// don't collide on the shared Postgres container.

import (
	"context"
	"testing"
	"time"

	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/repository/postgres"
	"github.com/EduThemes/paper-lms/internal/service/gamification"
	"github.com/EduThemes/paper-lms/internal/service/gamification/predicates"
	"gorm.io/gorm"
)

const testTenantID uint = 1

func TestLoadSnapshot_EmptyNeeds(t *testing.T) {
	g, cleanup := freshDB(t)
	defer cleanup()

	deps := gamification.SnapshotDeps{
		Submissions:     postgres.NewSubmissionRepository(g),
		QuizSubmissions: postgres.NewQuizSubmissionRepository(g),
		OutcomeResults:  postgres.NewLearningOutcomeResultRepository(g),
		ContentViews:    postgres.NewContentViewRepository(g),
		Wallet:          postgres.NewGamificationWalletRepository(g),
		CurrencyType:    postgres.NewGamificationCurrencyTypeRepository(g),
	}

	before := time.Now()
	snap, err := gamification.LoadSnapshot(context.Background(), deps, 42, testTenantID, predicates.Needs{}, "")
	if err != nil {
		t.Fatalf("LoadSnapshot empty needs: %v", err)
	}

	if snap.UserID != 42 {
		t.Errorf("UserID = %d, want 42", snap.UserID)
	}
	if snap.TenantID != testTenantID {
		t.Errorf("TenantID = %d, want %d", snap.TenantID, testTenantID)
	}
	if snap.Now.Before(before) || snap.Now.After(time.Now().Add(time.Second)) {
		t.Errorf("Now = %v outside expected window", snap.Now)
	}
	if snap.Submissions != nil {
		t.Errorf("Submissions = %v, want nil", snap.Submissions)
	}
	if snap.QuizAttempts != nil {
		t.Errorf("QuizAttempts = %v, want nil", snap.QuizAttempts)
	}
	if snap.ViewedContent != nil {
		t.Errorf("ViewedContent = %v, want nil", snap.ViewedContent)
	}
	if snap.OutcomeMastery != nil {
		t.Errorf("OutcomeMastery = %v, want nil", snap.OutcomeMastery)
	}
	if snap.WalletBalances != nil {
		t.Errorf("WalletBalances = %v, want nil", snap.WalletBalances)
	}
	if snap.CurrencyByCode != nil {
		t.Errorf("CurrencyByCode = %v, want nil", snap.CurrencyByCode)
	}
}

func TestLoadSnapshot_Assignments(t *testing.T) {
	g, cleanup := freshDB(t)
	defer cleanup()

	user := seedUser(t, g, "snap-assign@example.com")
	course := models.Course{Name: "Snapshot Course", AccountID: user.AccountID, WorkflowState: "available"}
	if err := g.Create(&course).Error; err != nil {
		t.Fatalf("create course: %v", err)
	}

	assignment := models.Assignment{
		CourseID:      course.ID,
		Name:          "Snapshot Test Assignment",
		WorkflowState: "published",
	}
	if err := g.Create(&assignment).Error; err != nil {
		t.Fatalf("create assignment: %v", err)
	}

	submittedAt := time.Now().Add(-2 * time.Hour).Round(time.Microsecond).UTC()
	score := 92.5
	sub := models.Submission{
		AssignmentID:  assignment.ID,
		UserID:        user.ID,
		Score:         &score,
		SubmittedAt:   &submittedAt,
		Attempt:       1,
		Late:          false,
		WorkflowState: "submitted",
	}
	if err := g.Create(&sub).Error; err != nil {
		t.Fatalf("create submission: %v", err)
	}

	deps := gamification.SnapshotDeps{
		Submissions: postgres.NewSubmissionRepository(g),
	}
	snap, err := gamification.LoadSnapshot(context.Background(), deps, user.ID, testTenantID, predicates.Needs{
		AssignmentIDs: []uint{assignment.ID},
	}, "")
	if err != nil {
		t.Fatalf("LoadSnapshot: %v", err)
	}

	got, ok := snap.Submissions[assignment.ID]
	if !ok {
		t.Fatalf("expected Submissions[%d] in snapshot, got %v", assignment.ID, snap.Submissions)
	}
	if got.AssignmentID != assignment.ID {
		t.Errorf("AssignmentID = %d, want %d", got.AssignmentID, assignment.ID)
	}
	if got.Score == nil || *got.Score != score {
		t.Errorf("Score = %v, want %v", got.Score, score)
	}
	if got.SubmittedAt == nil || !got.SubmittedAt.Equal(submittedAt) {
		t.Errorf("SubmittedAt = %v, want %v", got.SubmittedAt, submittedAt)
	}
	if !got.OnTime {
		t.Errorf("OnTime = false, want true (Late was false)")
	}
	if got.WorkflowState != "submitted" {
		t.Errorf("WorkflowState = %q, want 'submitted'", got.WorkflowState)
	}
	if got.AttemptCount != 1 {
		t.Errorf("AttemptCount = %d, want 1", got.AttemptCount)
	}
}

func TestLoadSnapshot_ContentViews(t *testing.T) {
	g, cleanup := freshDB(t)
	defer cleanup()

	user := seedUser(t, g, "snap-content@example.com")

	repo := postgres.NewContentViewRepository(g)
	ctx := context.Background()
	if err := repo.IncrementView(ctx, user.ID, gamification.DefaultContentObjectType, 7, 30); err != nil {
		t.Fatalf("increment 1: %v", err)
	}
	if err := repo.IncrementView(ctx, user.ID, gamification.DefaultContentObjectType, 7, 45); err != nil {
		t.Fatalf("increment 2: %v", err)
	}

	deps := gamification.SnapshotDeps{ContentViews: repo}
	snap, err := gamification.LoadSnapshot(ctx, deps, user.ID, testTenantID, predicates.Needs{
		ContentIDs: []uint{7},
	}, "")
	if err != nil {
		t.Fatalf("LoadSnapshot: %v", err)
	}

	got, ok := snap.ViewedContent[7]
	if !ok {
		t.Fatalf("expected ViewedContent[7], got %v", snap.ViewedContent)
	}
	if got.ObjectID != 7 {
		t.Errorf("ObjectID = %d, want 7", got.ObjectID)
	}
	if got.ViewCount != 2 {
		t.Errorf("ViewCount = %d, want 2", got.ViewCount)
	}
	if got.TotalSeconds != 75 {
		t.Errorf("TotalSeconds = %d, want 75", got.TotalSeconds)
	}
	if got.FirstViewedAt.IsZero() {
		t.Errorf("FirstViewedAt is zero")
	}
	if got.LastViewedAt.IsZero() {
		t.Errorf("LastViewedAt is zero")
	}
	if got.LastViewedAt.Before(got.FirstViewedAt) {
		t.Errorf("LastViewedAt %v before FirstViewedAt %v", got.LastViewedAt, got.FirstViewedAt)
	}
}

func TestLoadSnapshot_Wallet(t *testing.T) {
	g, cleanup := freshDB(t)
	defer cleanup()

	ctx := context.Background()
	seedTenantAccount(t, g, testTenantID)
	if err := gamification.SeedSystemCurrenciesForTenant(ctx, g, testTenantID); err != nil {
		t.Fatalf("seed currencies: %v", err)
	}

	user := seedUser(t, g, "snap-wallet@example.com")

	deps := gamification.SnapshotDeps{
		Wallet:       postgres.NewGamificationWalletRepository(g),
		CurrencyType: postgres.NewGamificationCurrencyTypeRepository(g),
	}
	snap, err := gamification.LoadSnapshot(ctx, deps, user.ID, testTenantID, predicates.Needs{
		CurrencyCodes: []string{"xp"},
	}, "")
	if err != nil {
		t.Fatalf("LoadSnapshot: %v", err)
	}

	if len(snap.CurrencyByCode) != 4 {
		t.Fatalf("CurrencyByCode size = %d, want 4 (%v)", len(snap.CurrencyByCode), snap.CurrencyByCode)
	}
	for _, want := range []string{"xp", "gems", "mastery_points", "reputation"} {
		if _, ok := snap.CurrencyByCode[want]; !ok {
			t.Errorf("CurrencyByCode missing %q", want)
		}
	}
	// No transactions yet → balance map is empty/nil. That's correct;
	// CurrencyThreshold reads it via a map lookup which returns zero.
	if len(snap.WalletBalances) != 0 {
		t.Errorf("WalletBalances = %v, want empty", snap.WalletBalances)
	}
}

func TestLoadSnapshot_OutcomeMastery(t *testing.T) {
	g, cleanup := freshDB(t)
	defer cleanup()

	user := seedUser(t, g, "snap-outcome@example.com")

	outcome := models.LearningOutcome{
		ContextType:    "Course",
		ContextID:      1,
		OutcomeGroupID: 1,
		Title:          "Snapshot test outcome",
		RatingsData:    "[]",
		WorkflowState:  "active",
	}
	if err := g.Create(&outcome).Error; err != nil {
		t.Fatalf("create outcome: %v", err)
	}

	assessedAt := time.Now().Add(-time.Hour).Round(time.Microsecond).UTC()
	percent := 0.72
	result := models.LearningOutcomeResult{
		UserID:            user.ID,
		LearningOutcomeID: outcome.ID,
		ContextType:       "Course",
		ContextID:         1,
		Percent:           &percent,
		Attempt:           1,
		AssessedAt:        &assessedAt,
	}
	if err := g.Create(&result).Error; err != nil {
		t.Fatalf("create outcome result: %v", err)
	}

	deps := gamification.SnapshotDeps{
		OutcomeResults: postgres.NewLearningOutcomeResultRepository(g),
	}
	snap, err := gamification.LoadSnapshot(context.Background(), deps, user.ID, testTenantID, predicates.Needs{
		OutcomeIDs: []uint{outcome.ID},
	}, "")
	if err != nil {
		t.Fatalf("LoadSnapshot: %v", err)
	}

	got, ok := snap.OutcomeMastery[outcome.ID]
	if !ok {
		t.Fatalf("expected OutcomeMastery[%d], got %v", outcome.ID, snap.OutcomeMastery)
	}
	if got.OutcomeID != outcome.ID {
		t.Errorf("OutcomeID = %d, want %d", got.OutcomeID, outcome.ID)
	}
	if got.Value != percent {
		t.Errorf("Value = %v, want %v", got.Value, percent)
	}
	// 0.72 sits in the [0.65, 0.85) "proficient" bucket per LevelFor.
	if got.Level != "proficient" {
		t.Errorf("Level = %q, want 'proficient'", got.Level)
	}
	if !got.AsOf.Equal(assessedAt) {
		t.Errorf("AsOf = %v, want %v", got.AsOf, assessedAt)
	}
}

func TestLoadSnapshot_QuizAttempts(t *testing.T) {
	g, cleanup := freshDB(t)
	defer cleanup()

	user := seedUser(t, g, "snap-quiz@example.com")

	finishedAt := time.Now().Add(-30 * time.Minute).Round(time.Microsecond).UTC()
	score := 88.0
	quizSub := models.QuizSubmission{
		QuizID:          5,
		UserID:          user.ID,
		Attempt:         2,
		Score:           &score,
		FinishedAt:      &finishedAt,
		ValidationToken: "test-token",
		WorkflowState:   "complete",
	}
	if err := g.Create(&quizSub).Error; err != nil {
		t.Fatalf("create quiz submission: %v", err)
	}

	deps := gamification.SnapshotDeps{
		QuizSubmissions: postgres.NewQuizSubmissionRepository(g),
	}
	snap, err := gamification.LoadSnapshot(context.Background(), deps, user.ID, testTenantID, predicates.Needs{
		QuizIDs: []uint{5},
	}, "")
	if err != nil {
		t.Fatalf("LoadSnapshot: %v", err)
	}

	got, ok := snap.QuizAttempts[5]
	if !ok {
		t.Fatalf("expected QuizAttempts[5], got %v", snap.QuizAttempts)
	}
	if got.Score == nil || *got.Score != score {
		t.Errorf("Score = %v, want %v", got.Score, score)
	}
	if got.SubmittedAt == nil || !got.SubmittedAt.Equal(finishedAt) {
		t.Errorf("SubmittedAt = %v, want %v", got.SubmittedAt, finishedAt)
	}
	if got.AttemptCount != 2 {
		t.Errorf("AttemptCount = %d, want 2", got.AttemptCount)
	}
	if got.WorkflowState != "complete" {
		t.Errorf("WorkflowState = %q, want 'complete'", got.WorkflowState)
	}
}

// --- helpers ---

// seedUser inserts a minimal users row and returns the persisted record.
// The unique login_id is derived from the supplied email so multiple
// users in one test don't collide. Also ensures a parent account row
// exists so the users.account_id FK (added in migration 000052) holds.
// User.BeforeCreate defaults AccountID to 1 when unset.
func seedUser(t *testing.T, g *gorm.DB, email string) models.User {
	t.Helper()
	ensureRootAccount(t, g)
	user := models.User{
		Name:    "Snap Test " + email,
		Email:   email,
		LoginID: email,
		Role:    "user",
	}
	if err := user.HashPassword("placeholder-pw"); err != nil {
		t.Fatalf("hash pw: %v", err)
	}
	if err := g.Create(&user).Error; err != nil {
		t.Fatalf("create user: %v", err)
	}
	return user
}

// ensureRootAccount inserts an accounts row with id=1 if one is not
// already present. The BeforeCreate hook on User defaults AccountID=1
// for any seed that doesn't set it; that FK target needs to exist.
func ensureRootAccount(t *testing.T, g *gorm.DB) {
	t.Helper()
	var n int64
	if err := g.Model(&models.Account{}).Where("id = ?", 1).Count(&n).Error; err != nil {
		t.Fatalf("count root account: %v", err)
	}
	if n > 0 {
		return
	}
	// Use raw SQL so the id=1 collides with the FK target deterministically,
	// even on a database where the bigserial sequence would otherwise start
	// elsewhere.
	if err := g.Exec(
		`INSERT INTO accounts (id, name, workflow_state, mfa_policy, default_locale, tenant_mode, max_upload_size_mb)
		 VALUES (1, 'Root', 'active', 'off', 'en', 'higher_ed', 500)
		 ON CONFLICT (id) DO NOTHING`,
	).Error; err != nil {
		t.Fatalf("seed root account: %v", err)
	}
}
