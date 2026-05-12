package postgres

import (
	"context"
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

		// Lock the balance row (or treat absence as zero).
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

		// Append the immutable transaction row.
		if err := g.Create(tx).Error; err != nil {
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
