package postgres_test

// Integration test for the dispatcher-TOCTOU defense added in migration
// 000059. Verifies that two ApplyTransaction calls carrying the same
// (triggering_event_id, triggering_rule_id) pair surface
// ErrDuplicateWalletTransaction on the second call AND leave the balance
// unchanged.
//
// Pre-fix behavior: two concurrent emits could both pass CheckCooldown,
// both run AwardCurrency, both append wallet rows. The row-level balance
// lock serialized the writes but did not deduplicate them — both deltas
// accumulated. Post-fix: the partial UNIQUE index uniq_wallet_tx_event_rule
// causes the second INSERT's ON CONFLICT clause to skip the row; the
// repo translates sql.ErrNoRows into the sentinel and bails before
// touching the balance.

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/repository"
	"github.com/EduThemes/paper-lms/internal/repository/postgres"
	"gorm.io/gorm"
)

// seedTenantAccountForWallet inserts the accounts row that the
// gamification FK chain (000050 + 000052) walks up to. Idempotent.
func seedTenantAccountForWallet(t *testing.T, g *gorm.DB, id uint) {
	t.Helper()
	if err := g.Exec(
		`INSERT INTO accounts (id, name, workflow_state, mfa_policy, default_locale, tenant_mode, max_upload_size_mb)
		 VALUES (?, 'Wallet Test Tenant', 'active', 'off', 'en', 'higher_ed', 500)
		 ON CONFLICT (id) DO NOTHING`,
		id,
	).Error; err != nil {
		t.Fatalf("seed tenant account %d: %v", id, err)
	}
}

// seedUserForWallet inserts a user row via raw SQL so we don't reach into
// the User model's BeforeCreate hook (which expects optional fields).
// Also satisfies the gen_random_bytes default for webauthn_user_handle.
func seedUserForWallet(t *testing.T, g *gorm.DB, id, accountID uint, email string) {
	t.Helper()
	if err := g.Exec(
		`INSERT INTO users (id, account_id, name, email, login_id, password_hash, role)
		 VALUES (?, ?, ?, ?, ?, ?, 'user')
		 ON CONFLICT (id) DO NOTHING`,
		id, accountID, "Wallet Test "+email, email, email, "placeholder",
	).Error; err != nil {
		t.Fatalf("seed user %d: %v", id, err)
	}
}

// seedCurrencyForWallet inserts a single test currency. Bypasses GORM
// defaults to keep the row deterministic.
func seedCurrencyForWallet(t *testing.T, g *gorm.DB, tenantID uint) *models.GamificationCurrencyType {
	t.Helper()
	var id uint
	if err := g.Raw(`
		INSERT INTO gamification_currency_types
			(tenant_id, scope_type, scope_id, code, display_label,
			 display_order, spendable, monotonic, ferpa_classification,
			 visible_to_student, visible_in_topbar, system_owned)
		VALUES (?, 'site', ?, 'xp', 'XP', 0, false, true, 'non_PII', true, true, false)
		RETURNING id`, tenantID, tenantID).Scan(&id).Error; err != nil {
		t.Fatalf("seed currency: %v", err)
	}
	return &models.GamificationCurrencyType{
		ID:        id,
		TenantID:  tenantID,
		Code:      "xp",
		Monotonic: true,
	}
}

// seedEventForWallet inserts a gamification_events row so the wallet
// transaction's triggering_event_id FK (000034) resolves.
func seedEventForWallet(t *testing.T, g *gorm.DB, tenantID, actorID uint) uint {
	t.Helper()
	var id uint
	if err := g.Raw(`
		INSERT INTO gamification_events
			(occurred_at, emitted_at, tenant_id, actor_id, verb, object_type, source, policy_flags)
		VALUES (NOW(), NOW(), ?, ?, 'completed', 'assignment', 'internal', '{}')
		RETURNING id`, tenantID, actorID).Scan(&id).Error; err != nil {
		t.Fatalf("seed event: %v", err)
	}
	return id
}

// seedRuleForWallet inserts a gamification_rules row so the wallet
// transaction's triggering_rule_id FK resolves.
func seedRuleForWallet(t *testing.T, g *gorm.DB, tenantID uint) uint {
	t.Helper()
	var id uint
	if err := g.Raw(`
		INSERT INTO gamification_rules
			(tenant_id, scope_type, scope_id, audience_level, name, enabled,
			 trigger_event, condition_set, effects)
		VALUES (?, 'site', ?, 'higher_ed', 'test-rule', true,
			'{"verb":"completed","object_type":"assignment"}'::jsonb,
			'{}'::jsonb,
			'[]'::jsonb)
		RETURNING id`, tenantID, tenantID).Scan(&id).Error; err != nil {
		t.Fatalf("seed rule: %v", err)
	}
	return id
}

// TestApplyTransaction_IdempotentOnEventRulePair is the migration 000059
// contract: re-applying the same (event, rule) pair must be a no-op.
//
// This is the dispatcher-TOCTOU defense. Pre-fix, two concurrent
// dispatcher passes for the same emit could both pass CheckCooldown
// (which only reads rule_evaluations, no row lock) and both ledger
// currency. The balance row-lock serialized writes but did not
// deduplicate them — both deltas accumulated. Post-fix, the partial
// UNIQUE index on (triggering_event_id, triggering_rule_id) WHERE
// triggering_event_id IS NOT NULL causes the second INSERT to be
// skipped; ApplyTransaction returns ErrDuplicateWalletTransaction and
// the balance is untouched.
func TestApplyTransaction_IdempotentOnEventRulePair(t *testing.T) {
	g, cleanup := freshDB(t)
	defer cleanup()

	ctx := context.Background()
	seedTenantAccountForWallet(t, g, 1)
	seedUserForWallet(t, g, 42, 1, "alice@idempotency.test")
	currency := seedCurrencyForWallet(t, g, 1)
	eventID := seedEventForWallet(t, g, 1, 42)
	ruleID := seedRuleForWallet(t, g, 1)

	repo := postgres.NewGamificationWalletRepository(g)

	// First emit: succeeds, balance becomes 50.
	tx1 := &models.GamificationWalletTransaction{
		UserID:            42,
		CurrencyTypeID:    currency.ID,
		Delta:             50,
		Reason:            "rule:1",
		TriggeringEventID: &eventID,
		TriggeringRuleID:  &ruleID,
	}
	if err := repo.ApplyTransaction(ctx, tx1); err != nil {
		t.Fatalf("first ApplyTransaction: %v", err)
	}
	if tx1.ID == 0 {
		t.Fatalf("first ApplyTransaction: tx ID not populated")
	}

	balAfterFirst, err := repo.GetBalance(ctx, 42, currency.ID)
	if err != nil {
		t.Fatalf("get balance after first: %v", err)
	}
	if balAfterFirst == nil || balAfterFirst.Balance != 50 {
		t.Fatalf("after first emit: want balance 50, got %+v", balAfterFirst)
	}
	if balAfterFirst.LifetimeEarned != 50 {
		t.Fatalf("after first emit: want lifetime 50, got %d", balAfterFirst.LifetimeEarned)
	}

	// Second emit: same (event, rule) pair → sentinel + balance unchanged.
	tx2 := &models.GamificationWalletTransaction{
		UserID:            42,
		CurrencyTypeID:    currency.ID,
		Delta:             50,
		Reason:            "rule:1",
		TriggeringEventID: &eventID,
		TriggeringRuleID:  &ruleID,
	}
	err = repo.ApplyTransaction(ctx, tx2)
	if !errors.Is(err, repository.ErrDuplicateWalletTransaction) {
		t.Fatalf("second ApplyTransaction: want ErrDuplicateWalletTransaction, got %v", err)
	}

	balAfterSecond, err := repo.GetBalance(ctx, 42, currency.ID)
	if err != nil {
		t.Fatalf("get balance after second: %v", err)
	}
	if balAfterSecond == nil || balAfterSecond.Balance != 50 {
		t.Fatalf("after duplicate emit: balance must remain 50, got %+v", balAfterSecond)
	}
	if balAfterSecond.LifetimeEarned != 50 {
		t.Fatalf("after duplicate emit: lifetime must remain 50, got %d", balAfterSecond.LifetimeEarned)
	}

	// And only one ledger row exists.
	var rowCount int64
	if err := g.Model(&models.GamificationWalletTransaction{}).
		Where("triggering_event_id = ? AND triggering_rule_id = ?", eventID, ruleID).
		Count(&rowCount).Error; err != nil {
		t.Fatalf("count tx rows: %v", err)
	}
	if rowCount != 1 {
		t.Fatalf("want exactly 1 ledger row for the (event, rule) pair, got %d", rowCount)
	}
}

// TestApplyTransaction_NullTriggerBypassesIdempotency confirms that
// manual grants / seeds / spends (where triggering_event_id IS NULL)
// stay outside the idempotency check — the partial index predicate
// excludes those rows by design. Each call appends a new row, balance
// accumulates.
func TestApplyTransaction_NullTriggerBypassesIdempotency(t *testing.T) {
	g, cleanup := freshDB(t)
	defer cleanup()

	ctx := context.Background()
	seedTenantAccountForWallet(t, g, 1)
	seedUserForWallet(t, g, 42, 1, "bob@manual.test")
	currency := seedCurrencyForWallet(t, g, 1)

	repo := postgres.NewGamificationWalletRepository(g)

	for i := 0; i < 3; i++ {
		tx := &models.GamificationWalletTransaction{
			UserID:         42,
			CurrencyTypeID: currency.ID,
			Delta:          10,
			Reason:         "manual:admin",
		}
		if err := repo.ApplyTransaction(ctx, tx); err != nil {
			t.Fatalf("manual ApplyTransaction #%d: %v", i, err)
		}
	}

	bal, err := repo.GetBalance(ctx, 42, currency.ID)
	if err != nil {
		t.Fatalf("get balance: %v", err)
	}
	if bal == nil || bal.Balance != 30 {
		t.Fatalf("3 manual grants of 10 should accumulate to 30, got %+v", bal)
	}
}

// TestApplyTransaction_ConcurrentSameEventRule simulates the original
// race: two goroutines racing past a hypothetical cooldown gate and
// both trying to ledger the same (event, rule). Exactly one must
// succeed and one must receive the sentinel; the balance lands at the
// single-emit value, not double.
func TestApplyTransaction_ConcurrentSameEventRule(t *testing.T) {
	g, cleanup := freshDB(t)
	defer cleanup()

	ctx := context.Background()
	seedTenantAccountForWallet(t, g, 1)
	seedUserForWallet(t, g, 42, 1, "carol@race.test")
	currency := seedCurrencyForWallet(t, g, 1)
	eventID := seedEventForWallet(t, g, 1, 42)
	ruleID := seedRuleForWallet(t, g, 1)

	repo := postgres.NewGamificationWalletRepository(g)

	const workers = 2
	const award = int64(25)

	results := make([]error, workers)
	var wg sync.WaitGroup
	var start sync.WaitGroup
	start.Add(1)

	for i := 0; i < workers; i++ {
		wg.Add(1)
		idx := i
		go func() {
			defer wg.Done()
			start.Wait()
			tx := &models.GamificationWalletTransaction{
				UserID:            42,
				CurrencyTypeID:    currency.ID,
				Delta:             award,
				Reason:            "rule:1",
				TriggeringEventID: &eventID,
				TriggeringRuleID:  &ruleID,
			}
			results[idx] = repo.ApplyTransaction(ctx, tx)
		}()
	}
	start.Done()
	wg.Wait()

	var successes, dupes int
	for _, err := range results {
		switch {
		case err == nil:
			successes++
		case errors.Is(err, repository.ErrDuplicateWalletTransaction):
			dupes++
		default:
			t.Fatalf("unexpected error in concurrent worker: %v", err)
		}
	}
	if successes != 1 || dupes != workers-1 {
		t.Fatalf("want 1 success + %d duplicates; got %d successes, %d duplicates", workers-1, successes, dupes)
	}

	bal, err := repo.GetBalance(ctx, 42, currency.ID)
	if err != nil {
		t.Fatalf("get balance: %v", err)
	}
	if bal == nil || bal.Balance != award {
		t.Fatalf("balance must equal single award (%d), got %+v", award, bal)
	}
	if bal.LifetimeEarned != award {
		t.Fatalf("lifetime_earned must equal single award (%d), got %d", award, bal.LifetimeEarned)
	}
}
