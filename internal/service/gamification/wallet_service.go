package gamification

import (
	"context"
	"errors"

	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/repository"
)

// ErrUserNotFound is returned when a user lookup misses.
var ErrUserNotFound = errors.New("user not found")

// WalletService orchestrates per-user wallet balance + transaction reads, and
// the per-learner gamification preferences (leaderboard opt-out).
type WalletService struct {
	walletRepo   repository.GamificationWalletRepository
	currencyRepo repository.GamificationCurrencyTypeRepository
	userRepo     repository.UserRepository
}

// NewWalletService wires the service.
func NewWalletService(
	walletRepo repository.GamificationWalletRepository,
	currencyRepo repository.GamificationCurrencyTypeRepository,
	userRepo repository.UserRepository,
) *WalletService {
	return &WalletService{
		walletRepo:   walletRepo,
		currencyRepo: currencyRepo,
		userRepo:     userRepo,
	}
}

// WalletBalanceWithCurrency packages a balance row with its currency metadata
// resolved (or nil when the currency was deleted post-balance — stale row).
type WalletBalanceWithCurrency struct {
	Balance  models.GamificationWalletBalance
	Currency *models.GamificationCurrencyType
}

// GetUserWallet returns the user's wallet balances with currency metadata
// resolved. Stale balances pointing at deleted currencies surface with
// Currency=nil so the caller can render a minimal entry.
func (s *WalletService) GetUserWallet(ctx context.Context, userID uint) ([]WalletBalanceWithCurrency, error) {
	balances, err := s.walletRepo.ListBalancesForUser(ctx, userID)
	if err != nil {
		return nil, err
	}
	out := make([]WalletBalanceWithCurrency, 0, len(balances))
	for i := range balances {
		b := balances[i]
		currency, ferr := s.currencyRepo.FindByID(ctx, b.CurrencyTypeID)
		if ferr != nil {
			currency = nil
		}
		out = append(out, WalletBalanceWithCurrency{Balance: b, Currency: currency})
	}
	return out, nil
}

// ListTransactions returns paginated wallet transactions for a (user,
// currency) pair.
func (s *WalletService) ListTransactions(ctx context.Context, userID, currencyTypeID uint, params repository.PaginationParams) (*repository.PaginatedResult[models.GamificationWalletTransaction], error) {
	return s.walletRepo.ListTransactionsForUserAndCurrency(ctx, userID, currencyTypeID, params)
}

// GetPreferences loads the leaderboard_opt_out flag for the user.
// Self lookup: userID is the JWT subject. accountID=0 is safe.
func (s *WalletService) GetPreferences(ctx context.Context, userID uint) (bool, error) {
	user, err := s.userRepo.FindByID(ctx, userID, 0)
	if err != nil {
		return false, err
	}
	if user == nil {
		return false, ErrUserNotFound
	}
	return user.LeaderboardOptOut, nil
}

// UpdatePreferences applies a partial update to the user's gamification
// preferences. optOut is a pointer so omitted=no-op.
// Self lookup: userID is the JWT subject. accountID=0 is safe.
func (s *WalletService) UpdatePreferences(ctx context.Context, userID uint, optOut *bool) (bool, error) {
	user, err := s.userRepo.FindByID(ctx, userID, 0)
	if err != nil {
		return false, err
	}
	if user == nil {
		return false, ErrUserNotFound
	}
	if optOut != nil {
		user.LeaderboardOptOut = *optOut
	}
	if err := s.userRepo.Update(ctx, user); err != nil {
		return false, err
	}
	return user.LeaderboardOptOut, nil
}
