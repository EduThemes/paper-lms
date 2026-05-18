package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/repository"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// GamificationWalletRepo persists balances and the immutable transaction
// ledger.
type GamificationWalletRepo struct {
	db *gorm.DB
}

func NewGamificationWalletRepository(db *gorm.DB) *GamificationWalletRepo {
	return &GamificationWalletRepo{db: db}
}

func (r *GamificationWalletRepo) GetBalance(ctx context.Context, userID, currencyTypeID uint) (*models.GamificationWalletBalance, error) {
	var balance models.GamificationWalletBalance
	err := r.db.WithContext(ctx).
		Where("user_id = ? AND currency_type_id = ?", userID, currencyTypeID).
		First(&balance).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &balance, nil
}

func (r *GamificationWalletRepo) ListBalancesForUser(ctx context.Context, userID uint) ([]models.GamificationWalletBalance, error) {
	var balances []models.GamificationWalletBalance
	err := r.db.WithContext(ctx).
		Where("user_id = ?", userID).
		Find(&balances).Error
	return balances, err
}

// ApplyTransaction appends a transaction row and atomically updates the
// matching balance row. Negative deltas are rejected if the currency is
// monotonic or the resulting balance would be negative.
//
// The balance update uses a row-level lock (SELECT … FOR UPDATE) to
// serialize concurrent writers against the same (user_id, currency_type_id)
// pair, which is the right granularity: two users earning XP at the same
// moment don't contend, but two simultaneous awards to the same user
// linearize.
//
// Idempotency on (triggering_event_id, triggering_rule_id) — closes the
// TOCTOU race between CheckCooldown and the effects pass in the
// gamification dispatcher. When both fields are set, the INSERT uses
// `ON CONFLICT (triggering_event_id, triggering_rule_id) WHERE
// triggering_event_id IS NOT NULL DO NOTHING RETURNING id` against the
// partial unique index uniq_wallet_tx_event_rule (migration 000059). If
// a concurrent worker already wrote a row for the same (event, rule)
// pair, the RETURNING yields no row; we translate that into the typed
// sentinel repository.ErrDuplicateWalletTransaction and skip the balance
// update so the duplicate emit is a clean no-op. The row-level lock on
// the balance serializes the inference so we never double-count.
//
// Same pattern as gamification_currency_type Create — atomic
// duplicate detection without a TOCTOU pre-check window.
func (r *GamificationWalletRepo) ApplyTransaction(ctx context.Context, tx *models.GamificationWalletTransaction) error {
	if tx.Delta == 0 {
		return errors.New("wallet transaction delta must be non-zero")
	}

	return r.db.WithContext(ctx).Transaction(func(g *gorm.DB) error {
		// Look up the currency type for monotonic + spendable checks.
		var currency models.GamificationCurrencyType
		if err := g.First(&currency, tx.CurrencyTypeID).Error; err != nil {
			return fmt.Errorf("load currency type %d: %w", tx.CurrencyTypeID, err)
		}
		if tx.Delta < 0 {
			if currency.Monotonic {
				return fmt.Errorf("currency %q is monotonic; negative delta rejected", currency.Code)
			}
			if !currency.Spendable {
				return fmt.Errorf("currency %q is not spendable; negative delta rejected", currency.Code)
			}
		}

		// Lock the balance row (or treat absence as zero). The lock both
		// serializes concurrent (user, currency) writers AND serializes
		// the idempotency inference below — without it two workers could
		// race past the duplicate check before either INSERT lands.
		var balance models.GamificationWalletBalance
		err := g.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("user_id = ? AND currency_type_id = ?", tx.UserID, tx.CurrencyTypeID).
			First(&balance).Error
		exists := err == nil
		if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}

		newBalance := balance.Balance + tx.Delta
		if newBalance < 0 {
			return fmt.Errorf("insufficient balance: have %d, delta %d", balance.Balance, tx.Delta)
		}

		// Append the immutable transaction row. Use a raw INSERT so the
		// ON CONFLICT inference targets the partial unique index
		// uniq_wallet_tx_event_rule (migration 000059). When
		// triggering_event_id IS NULL the partial index predicate
		// excludes the row, so the INSERT always succeeds — manual
		// grants / seeds / spends bypass idempotency by design (their
		// natural keys live elsewhere).
		//
		// COALESCE(policy_flags, '{}') mirrors the column DEFAULT —
		// a nil pq.StringArray binds as NULL, which would trip the
		// NOT NULL constraint. The pre-fix Create path went through
		// GORM which applied the default; raw SQL needs to be explicit.
		const insertSQL = `
			INSERT INTO gamification_wallet_transactions
				(user_id, currency_type_id, delta, reason,
				 triggering_event_id, triggering_rule_id,
				 policy_flags, occurred_at)
			VALUES
				(?, ?, ?, ?, ?, ?, COALESCE(?, '{}'::text[]), COALESCE(?, NOW()))
			ON CONFLICT (triggering_event_id, triggering_rule_id)
			    WHERE triggering_event_id IS NOT NULL
			    DO NOTHING
			RETURNING id, occurred_at`
		var occurredArg interface{}
		if !tx.OccurredAt.IsZero() {
			occurredArg = tx.OccurredAt
		}
		var policyFlagsArg interface{}
		if tx.PolicyFlags != nil {
			policyFlagsArg = tx.PolicyFlags
		}
		row := g.Raw(insertSQL,
			tx.UserID, tx.CurrencyTypeID, tx.Delta, tx.Reason,
			tx.TriggeringEventID, tx.TriggeringRuleID,
			policyFlagsArg, occurredArg,
		).Row()
		if err := row.Scan(&tx.ID, &tx.OccurredAt); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				// Idempotency hit: a concurrent worker already wrote a
				// row for this (event, rule) pair. Bail without
				// touching the balance — the existing row's delta is
				// already reflected.
				return repository.ErrDuplicateWalletTransaction
			}
			return err
		}

		// Upsert the balance.
		balance.UserID = tx.UserID
		balance.CurrencyTypeID = tx.CurrencyTypeID
		balance.Balance = newBalance
		if tx.Delta > 0 {
			balance.LifetimeEarned += tx.Delta
		}
		if exists {
			return g.Save(&balance).Error
		}
		return g.Create(&balance).Error
	})
}

func (r *GamificationWalletRepo) ListTransactionsForUser(ctx context.Context, userID uint, params repository.PaginationParams) (*repository.PaginatedResult[models.GamificationWalletTransaction], error) {
	query := r.db.WithContext(ctx).
		Model(&models.GamificationWalletTransaction{}).
		Where("user_id = ?", userID)

	var count int64
	if err := query.Count(&count).Error; err != nil {
		return nil, err
	}

	var txs []models.GamificationWalletTransaction
	offset := (params.Page - 1) * params.PerPage
	if err := query.Offset(offset).Limit(params.PerPage).Order("occurred_at DESC").Find(&txs).Error; err != nil {
		return nil, err
	}

	return &repository.PaginatedResult[models.GamificationWalletTransaction]{
		Items:      txs,
		TotalCount: count,
		Page:       params.Page,
		PerPage:    params.PerPage,
	}, nil
}

// RankByCurrency ranks the supplied candidate users by lifetime_earned
// in the given currency, ties broken by earliest most-recent positive
// transaction. Candidates with no balance row surface at rank tail
// with lifetime_earned = 0 (ties among those are arbitrary).
//
// The query uses idx_wallet_balances_currency_lifetime (migration
// 000042) for the primary ordering. The tie-break is a correlated
// MAX subquery over the transactions ledger — cheap because the
// tied set is usually tiny.
func (r *GamificationWalletRepo) RankByCurrency(ctx context.Context, currencyTypeID uint, candidateUserIDs []uint) ([]repository.RankRow, error) {
	if len(candidateUserIDs) == 0 {
		return nil, nil
	}

	type row struct {
		UserID         uint
		LifetimeEarned int64
	}
	var rows []row

	// LEFT JOIN so candidates with no balance row in this currency still
	// appear (lifetime_earned = 0). MAX(occurred_at) is the tie-break
	// signal — earlier completers rank higher. NULL last so missing
	// balances sort after present ones at the same lifetime_earned tier.
	err := r.db.WithContext(ctx).
		Raw(`
			SELECT u.id AS user_id,
			       COALESCE(b.lifetime_earned, 0) AS lifetime_earned
			  FROM users u
			  LEFT JOIN gamification_wallet_balances b
			    ON b.user_id = u.id AND b.currency_type_id = ?
			  LEFT JOIN LATERAL (
			    SELECT MAX(t.occurred_at) AS last_earn
			      FROM gamification_wallet_transactions t
			     WHERE t.user_id = u.id
			       AND t.currency_type_id = ?
			       AND t.delta > 0
			  ) tx ON TRUE
			 WHERE u.id IN ?
			 ORDER BY COALESCE(b.lifetime_earned, 0) DESC,
			          tx.last_earn ASC NULLS LAST,
			          u.id ASC
		`, currencyTypeID, currencyTypeID, candidateUserIDs).
		Scan(&rows).Error
	if err != nil {
		return nil, err
	}

	out := make([]repository.RankRow, len(rows))
	for i, r := range rows {
		out[i] = repository.RankRow{
			UserID:         r.UserID,
			LifetimeEarned: r.LifetimeEarned,
			Rank:           i + 1,
		}
	}
	return out, nil
}

func (r *GamificationWalletRepo) ListTransactionsForUserAndCurrency(ctx context.Context, userID, currencyTypeID uint, params repository.PaginationParams) (*repository.PaginatedResult[models.GamificationWalletTransaction], error) {
	query := r.db.WithContext(ctx).
		Model(&models.GamificationWalletTransaction{}).
		Where("user_id = ? AND currency_type_id = ?", userID, currencyTypeID)

	var count int64
	if err := query.Count(&count).Error; err != nil {
		return nil, err
	}

	var txs []models.GamificationWalletTransaction
	offset := (params.Page - 1) * params.PerPage
	if err := query.Offset(offset).Limit(params.PerPage).Order("occurred_at DESC").Find(&txs).Error; err != nil {
		return nil, err
	}

	return &repository.PaginatedResult[models.GamificationWalletTransaction]{
		Items:      txs,
		TotalCount: count,
		Page:       params.Page,
		PerPage:    params.PerPage,
	}, nil
}
