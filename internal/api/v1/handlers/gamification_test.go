package handlers_test

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/EduThemes/paper-lms/internal/api/v1/handlers"
	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/repository"
	"github.com/EduThemes/paper-lms/internal/testutil"
	"github.com/EduThemes/paper-lms/internal/testutil/mocks"
)

// ---------------------------------------------------------------------------
// Local mock repositories.
//
// Wave 1 sprint D-1 doesn't ship gamification mocks into the shared
// internal/testutil/mocks package yet; doing so would conflict with the
// parallel agents touching that file. These local stubs implement the two
// interfaces this handler needs and nothing more.
// ---------------------------------------------------------------------------

type mockGamWalletRepo struct{ mock.Mock }

func (m *mockGamWalletRepo) GetBalance(ctx context.Context, userID, currencyTypeID uint) (*models.GamificationWalletBalance, error) {
	args := m.Called(ctx, userID, currencyTypeID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.GamificationWalletBalance), args.Error(1)
}

func (m *mockGamWalletRepo) ListBalancesForUser(ctx context.Context, userID uint) ([]models.GamificationWalletBalance, error) {
	args := m.Called(ctx, userID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]models.GamificationWalletBalance), args.Error(1)
}

func (m *mockGamWalletRepo) ApplyTransaction(ctx context.Context, tx *models.GamificationWalletTransaction) error {
	args := m.Called(ctx, tx)
	return args.Error(0)
}

func (m *mockGamWalletRepo) ListTransactionsForUser(ctx context.Context, userID uint, params repository.PaginationParams) (*repository.PaginatedResult[models.GamificationWalletTransaction], error) {
	args := m.Called(ctx, userID, params)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*repository.PaginatedResult[models.GamificationWalletTransaction]), args.Error(1)
}

func (m *mockGamWalletRepo) ListTransactionsForUserAndCurrency(ctx context.Context, userID, currencyTypeID uint, params repository.PaginationParams) (*repository.PaginatedResult[models.GamificationWalletTransaction], error) {
	args := m.Called(ctx, userID, currencyTypeID, params)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*repository.PaginatedResult[models.GamificationWalletTransaction]), args.Error(1)
}

func (m *mockGamWalletRepo) RankByCurrency(ctx context.Context, currencyTypeID uint, candidateUserIDs []uint) ([]repository.RankRow, error) {
	args := m.Called(ctx, currencyTypeID, candidateUserIDs)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]repository.RankRow), args.Error(1)
}

type mockGamCurrencyRepo struct{ mock.Mock }

func (m *mockGamCurrencyRepo) Create(ctx context.Context, c *models.GamificationCurrencyType) error {
	args := m.Called(ctx, c)
	return args.Error(0)
}

func (m *mockGamCurrencyRepo) FindByID(ctx context.Context, id uint) (*models.GamificationCurrencyType, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.GamificationCurrencyType), args.Error(1)
}

func (m *mockGamCurrencyRepo) FindByCode(ctx context.Context, tenantID uint, scopeType models.GamificationScopeType, scopeID uint, code string) (*models.GamificationCurrencyType, error) {
	args := m.Called(ctx, tenantID, scopeType, scopeID, code)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.GamificationCurrencyType), args.Error(1)
}

func (m *mockGamCurrencyRepo) Update(ctx context.Context, c *models.GamificationCurrencyType) error {
	args := m.Called(ctx, c)
	return args.Error(0)
}

func (m *mockGamCurrencyRepo) Delete(ctx context.Context, id uint) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

func (m *mockGamCurrencyRepo) ListByTenant(ctx context.Context, tenantID uint) ([]models.GamificationCurrencyType, error) {
	args := m.Called(ctx, tenantID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]models.GamificationCurrencyType), args.Error(1)
}

func (m *mockGamCurrencyRepo) ListInTopbar(ctx context.Context, tenantID uint) ([]models.GamificationCurrencyType, error) {
	args := m.Called(ctx, tenantID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]models.GamificationCurrencyType), args.Error(1)
}

// W2-D: badge + badge-award mocks.

type mockGamBadgeRepo struct{ mock.Mock }

func (m *mockGamBadgeRepo) Create(ctx context.Context, b *models.GamificationBadge) error {
	args := m.Called(ctx, b)
	return args.Error(0)
}
func (m *mockGamBadgeRepo) FindByID(ctx context.Context, id uint) (*models.GamificationBadge, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.GamificationBadge), args.Error(1)
}
func (m *mockGamBadgeRepo) FindByCode(ctx context.Context, tenantID uint, scopeType models.GamificationScopeType, scopeID uint, code string) (*models.GamificationBadge, error) {
	args := m.Called(ctx, tenantID, scopeType, scopeID, code)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.GamificationBadge), args.Error(1)
}
func (m *mockGamBadgeRepo) Update(ctx context.Context, b *models.GamificationBadge) error {
	args := m.Called(ctx, b)
	return args.Error(0)
}
func (m *mockGamBadgeRepo) Delete(ctx context.Context, id uint) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}
func (m *mockGamBadgeRepo) ListByTenant(ctx context.Context, tenantID uint) ([]models.GamificationBadge, error) {
	args := m.Called(ctx, tenantID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]models.GamificationBadge), args.Error(1)
}

type mockGamBadgeAwardRepo struct{ mock.Mock }

func (m *mockGamBadgeAwardRepo) Award(ctx context.Context, a *models.GamificationBadgeAward) (bool, error) {
	args := m.Called(ctx, a)
	return args.Bool(0), args.Error(1)
}
func (m *mockGamBadgeAwardRepo) Revoke(ctx context.Context, userID, badgeID uint) error {
	args := m.Called(ctx, userID, badgeID)
	return args.Error(0)
}
func (m *mockGamBadgeAwardRepo) ListForUser(ctx context.Context, userID uint) ([]models.GamificationBadgeAward, error) {
	args := m.Called(ctx, userID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]models.GamificationBadgeAward), args.Error(1)
}
func (m *mockGamBadgeAwardRepo) FindByUserAndBadge(ctx context.Context, userID, badgeID uint) (*models.GamificationBadgeAward, error) {
	args := m.Called(ctx, userID, badgeID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.GamificationBadgeAward), args.Error(1)
}

// Compile-time check: our mocks satisfy the production interfaces.
var (
	_ repository.GamificationWalletRepository         = (*mockGamWalletRepo)(nil)
	_ repository.GamificationCurrencyTypeRepository   = (*mockGamCurrencyRepo)(nil)
	_ repository.GamificationBadgeRepository          = (*mockGamBadgeRepo)(nil)
	_ repository.GamificationBadgeAwardRepository     = (*mockGamBadgeAwardRepo)(nil)
)

// ---------------------------------------------------------------------------
// Fixtures + harness.
// ---------------------------------------------------------------------------

// setupGamificationHandler wires the handler to fresh mocks and registers
// the two routes on a test app. The auth-stub middleware injects the given
// callerID + isAdmin flag into Locals so the handler's self-or-admin check
// has something to read.
func setupGamificationHandler(callerID uint, isAdmin bool) (*fiber.App, *mockGamWalletRepo, *mockGamCurrencyRepo, *mocks.MockUserRepository, *mockGamBadgeRepo, *mockGamBadgeAwardRepo) {
	walletRepo := new(mockGamWalletRepo)
	currencyRepo := new(mockGamCurrencyRepo)
	userRepo := new(mocks.MockUserRepository)
	badgeRepo := new(mockGamBadgeRepo)
	badgeAwardRepo := new(mockGamBadgeAwardRepo)
	// ruleRepo (W2-E.1) — wired so the handler constructor stays
	// satisfied. Existing W2-A..W2-D tests don't exercise rule paths,
	// so an inert mock is fine here. Rule-specific tests live in
	// gamification_rules_test.go with their own focused harness.
	ruleRepo := new(mockGamRuleRepo)
	// enrollmentRepo (W3-A) — leaderboard candidate set. Inert here
	// for the same reason as ruleRepo; leaderboard-specific tests live
	// in gamification_leaderboards_test.go with their own harness.
	enrollmentRepo := new(mocks.MockEnrollmentRepository)
	// accountRepo (W3-B) — tenant_mode lookup for render policy. Inert
	// for the W2-A..D paths exercised in this file.
	accountRepo := new(mocks.MockAccountRepository)
	snapshotRepo := new(mocks.MockGamificationLeaderboardSnapshotRepository)
	h := handlers.NewGamificationHandler(walletRepo, currencyRepo, userRepo, badgeRepo, badgeAwardRepo, ruleRepo, enrollmentRepo, accountRepo, snapshotRepo)

	app := testutil.SetupTestApp()
	app.Use(func(c *fiber.Ctx) error {
		c.Locals("user_id", callerID)
		c.Locals("is_admin", isAdmin)
		c.Locals("account_id", uint(1))
		return c.Next()
	})

	app.Get("/api/v1/users/:id/wallet", h.GetUserWallet)
	app.Get("/api/v1/users/:id/wallet/transactions", h.ListUserWalletTransactions)
	app.Get("/api/v1/gamification/currencies", h.ListCurrencies)
	// W2-B: currency CRUD. Site-scope routes; course-scope use a separate
	// helper because they need :course_id URL params.
	app.Post("/api/v1/gamification/currencies", h.CreateCurrency)
	app.Patch("/api/v1/gamification/currencies/:id", h.UpdateCurrency)
	app.Delete("/api/v1/gamification/currencies/:id", h.DeleteCurrency)
	app.Post("/api/v1/courses/:course_id/gamification/currencies", h.CreateCurrency)
	app.Patch("/api/v1/courses/:course_id/gamification/currencies/:id", h.UpdateCurrency)
	app.Delete("/api/v1/courses/:course_id/gamification/currencies/:id", h.DeleteCurrency)
	// W2-C: self-only gamification preferences.
	app.Get("/api/v1/users/self/gamification_preferences", h.GetMyGamificationPreferences)
	app.Put("/api/v1/users/self/gamification_preferences", h.UpdateMyGamificationPreferences)
	// W2-D: badge CRUD + per-user list + manual award/revoke.
	app.Get("/api/v1/gamification/badges", h.ListBadges)
	app.Post("/api/v1/gamification/badges", h.CreateBadge)
	app.Patch("/api/v1/gamification/badges/:id", h.UpdateBadge)
	app.Delete("/api/v1/gamification/badges/:id", h.DeleteBadge)
	app.Post("/api/v1/courses/:course_id/gamification/badges", h.CreateBadge)
	app.Patch("/api/v1/courses/:course_id/gamification/badges/:id", h.UpdateBadge)
	app.Delete("/api/v1/courses/:course_id/gamification/badges/:id", h.DeleteBadge)
	app.Get("/api/v1/users/:id/badges", h.ListUserBadges)
	app.Post("/api/v1/users/:user_id/badges", h.AwardBadgeToUser)
	app.Delete("/api/v1/users/:user_id/badges/:badge_id", h.RevokeBadgeFromUser)
	return app, walletRepo, currencyRepo, userRepo, badgeRepo, badgeAwardRepo
}

// fixtureXP / fixtureGems / fixtureHidden / fixtureBadge moved to
// gamification_fixtures_test.go (F2.5 closeout).

// ---------------------------------------------------------------------------
// GetUserWallet.
// ---------------------------------------------------------------------------

func TestGetUserWallet_Self_HappyPath(t *testing.T) {
	app, walletRepo, currencyRepo, _, _, _ := setupGamificationHandler(42, false)

	balances := []models.GamificationWalletBalance{
		{UserID: 42, CurrencyTypeID: 11, Balance: 250, LifetimeEarned: 250},
		{UserID: 42, CurrencyTypeID: 12, Balance: 8, LifetimeEarned: 12},
	}
	walletRepo.On("ListBalancesForUser", mock.Anything, uint(42)).Return(balances, nil)

	xp := fixtureXP()
	gems := fixtureGems()
	currencyRepo.On("FindByID", mock.Anything, uint(11)).Return(&xp, nil)
	currencyRepo.On("FindByID", mock.Anything, uint(12)).Return(&gems, nil)

	resp := testutil.MakeRequest(app, http.MethodGet, "/api/v1/users/42/wallet", nil)
	require.Equal(t, http.StatusOK, resp.StatusCode)

	body, err := testutil.ParseJSONMap(resp)
	require.NoError(t, err)
	assert.Equal(t, float64(42), body["user_id"])
	rows, ok := body["balances"].([]interface{})
	require.True(t, ok, "balances must be an array")
	require.Len(t, rows, 2)

	first := rows[0].(map[string]interface{})
	assert.Equal(t, float64(11), first["currency_type_id"])
	assert.Equal(t, "xp", first["code"])
	assert.Equal(t, "XP", first["display_label"])
	assert.Equal(t, "zap", first["icon"])
	assert.Equal(t, "#F59E0B", first["color"])
	assert.Equal(t, float64(250), first["balance"])
	assert.Equal(t, float64(250), first["lifetime_earned"])
	assert.Equal(t, false, first["spendable"])
	assert.Equal(t, true, first["monotonic"])
	assert.Equal(t, true, first["visible_in_topbar"])
	assert.Equal(t, float64(1), first["display_order"])

	second := rows[1].(map[string]interface{})
	assert.Equal(t, float64(12), second["currency_type_id"])
	assert.Equal(t, "gems", second["code"])
	assert.Equal(t, true, second["spendable"])
	assert.Equal(t, float64(8), second["balance"])
	assert.Equal(t, float64(12), second["lifetime_earned"])

	walletRepo.AssertExpectations(t)
	currencyRepo.AssertExpectations(t)
}

func TestGetUserWallet_AdminViewingOtherUser(t *testing.T) {
	app, walletRepo, currencyRepo, _, _, _ := setupGamificationHandler(99, true /*isAdmin*/)

	balances := []models.GamificationWalletBalance{
		{UserID: 42, CurrencyTypeID: 11, Balance: 100, LifetimeEarned: 100},
	}
	walletRepo.On("ListBalancesForUser", mock.Anything, uint(42)).Return(balances, nil)
	xp := fixtureXP()
	currencyRepo.On("FindByID", mock.Anything, uint(11)).Return(&xp, nil)

	resp := testutil.MakeRequest(app, http.MethodGet, "/api/v1/users/42/wallet", nil)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	walletRepo.AssertExpectations(t)
	currencyRepo.AssertExpectations(t)
}

func TestGetUserWallet_Unauthorized_NotSelfNotAdmin(t *testing.T) {
	// caller=99 trying to view user 42's wallet, not an admin.
	app, walletRepo, currencyRepo, _, _, _ := setupGamificationHandler(99, false)

	resp := testutil.MakeRequest(app, http.MethodGet, "/api/v1/users/42/wallet", nil)
	assert.Equal(t, http.StatusForbidden, resp.StatusCode)

	// Repo must not be touched on a 403.
	walletRepo.AssertNotCalled(t, "ListBalancesForUser", mock.Anything, mock.Anything)
	currencyRepo.AssertNotCalled(t, "FindByID", mock.Anything, mock.Anything)
}

func TestGetUserWallet_EmptyBalances_ReturnsEmptyArrayNot404(t *testing.T) {
	app, walletRepo, _, _, _, _ := setupGamificationHandler(42, false)

	walletRepo.On("ListBalancesForUser", mock.Anything, uint(42)).
		Return([]models.GamificationWalletBalance{}, nil)

	resp := testutil.MakeRequest(app, http.MethodGet, "/api/v1/users/42/wallet", nil)
	require.Equal(t, http.StatusOK, resp.StatusCode)

	body, err := testutil.ParseJSONMap(resp)
	require.NoError(t, err)
	rows, ok := body["balances"].([]interface{})
	require.True(t, ok)
	assert.Len(t, rows, 0)

	walletRepo.AssertExpectations(t)
}

func TestGetUserWallet_InvalidUserID(t *testing.T) {
	app, _, _, _, _, _ := setupGamificationHandler(1, true)

	resp := testutil.MakeRequest(app, http.MethodGet, "/api/v1/users/abc/wallet", nil)
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

func TestGetUserWallet_WalletRepoError(t *testing.T) {
	app, walletRepo, _, _, _, _ := setupGamificationHandler(42, false)

	walletRepo.On("ListBalancesForUser", mock.Anything, uint(42)).
		Return(nil, errors.New("db down"))

	resp := testutil.MakeRequest(app, http.MethodGet, "/api/v1/users/42/wallet", nil)
	assert.Equal(t, http.StatusInternalServerError, resp.StatusCode)
}

// ---------------------------------------------------------------------------
// ListCurrencies.
// ---------------------------------------------------------------------------

func TestListCurrencies_All(t *testing.T) {
	app, _, currencyRepo, _, _, _ := setupGamificationHandler(7, false)

	xp := fixtureXP()
	gems := fixtureGems()
	hidden := fixtureHidden()
	currencyRepo.On("ListByTenant", mock.Anything, uint(1)).
		Return([]models.GamificationCurrencyType{xp, gems, hidden}, nil)

	resp := testutil.MakeRequest(app, http.MethodGet, "/api/v1/gamification/currencies", nil)
	require.Equal(t, http.StatusOK, resp.StatusCode)

	body, err := testutil.ParseJSONMap(resp)
	require.NoError(t, err)
	items, ok := body["currencies"].([]interface{})
	require.True(t, ok)
	require.Len(t, items, 3)

	first := items[0].(map[string]interface{})
	assert.Equal(t, "xp", first["code"])
	assert.Equal(t, float64(11), first["id"])
	assert.Equal(t, true, first["visible_in_topbar"])
	assert.Equal(t, true, first["system_owned"])

	third := items[2].(map[string]interface{})
	assert.Equal(t, "mastery_points", third["code"])
	assert.Equal(t, false, third["visible_in_topbar"])

	currencyRepo.AssertExpectations(t)
	currencyRepo.AssertNotCalled(t, "ListInTopbar", mock.Anything, mock.Anything)
}

func TestListCurrencies_TopbarOnly(t *testing.T) {
	app, _, currencyRepo, _, _, _ := setupGamificationHandler(7, false)

	xp := fixtureXP()
	gems := fixtureGems()
	currencyRepo.On("ListInTopbar", mock.Anything, uint(1)).
		Return([]models.GamificationCurrencyType{xp, gems}, nil)

	resp := testutil.MakeRequest(app, http.MethodGet, "/api/v1/gamification/currencies?topbar_only=true", nil)
	require.Equal(t, http.StatusOK, resp.StatusCode)

	body, err := testutil.ParseJSONMap(resp)
	require.NoError(t, err)
	items, ok := body["currencies"].([]interface{})
	require.True(t, ok)
	require.Len(t, items, 2)
	for _, raw := range items {
		row := raw.(map[string]interface{})
		assert.Equal(t, true, row["visible_in_topbar"], "topbar_only must filter to visible currencies")
	}

	currencyRepo.AssertExpectations(t)
	currencyRepo.AssertNotCalled(t, "ListByTenant", mock.Anything, mock.Anything)
}

func TestListCurrencies_RepoError(t *testing.T) {
	app, _, currencyRepo, _, _, _ := setupGamificationHandler(7, false)

	currencyRepo.On("ListByTenant", mock.Anything, uint(1)).Return(nil, errors.New("db down"))

	resp := testutil.MakeRequest(app, http.MethodGet, "/api/v1/gamification/currencies", nil)
	assert.Equal(t, http.StatusInternalServerError, resp.StatusCode)
}

// ---------------------------------------------------------------------------
// ListUserWalletTransactions (Wave 2 W2-A: powers the wallet drawer).
// ---------------------------------------------------------------------------

func TestListUserWalletTransactions_Self_HappyPath(t *testing.T) {
	app, walletRepo, _, _, _, _ := setupGamificationHandler(42, false)

	occurredAt := time.Date(2026, 5, 13, 9, 30, 0, 0, time.UTC)
	ruleID := uint(7)
	page := &repository.PaginatedResult[models.GamificationWalletTransaction]{
		Items: []models.GamificationWalletTransaction{
			{ID: 101, UserID: 42, CurrencyTypeID: 11, Delta: 25, Reason: "rule:7", TriggeringRuleID: &ruleID, OccurredAt: occurredAt},
			{ID: 100, UserID: 42, CurrencyTypeID: 11, Delta: 10, Reason: "rule:7", TriggeringRuleID: &ruleID, OccurredAt: occurredAt.Add(-time.Hour)},
		},
		TotalCount: 2, Page: 1, PerPage: 20,
	}
	walletRepo.On(
		"ListTransactionsForUserAndCurrency", mock.Anything,
		uint(42), uint(11),
		repository.PaginationParams{Page: 1, PerPage: 20},
	).Return(page, nil)

	resp := testutil.MakeRequest(app, http.MethodGet,
		"/api/v1/users/42/wallet/transactions?currency_type_id=11", nil)
	require.Equal(t, http.StatusOK, resp.StatusCode)

	body, err := testutil.ParseJSONMap(resp)
	require.NoError(t, err)
	assert.Equal(t, float64(42), body["user_id"])
	assert.Equal(t, float64(11), body["currency_type_id"])
	assert.Equal(t, float64(2), body["total_count"])
	rows := body["transactions"].([]interface{})
	require.Len(t, rows, 2)
	first := rows[0].(map[string]interface{})
	assert.Equal(t, float64(101), first["id"])
	assert.Equal(t, float64(25), first["delta"])
	assert.Equal(t, "rule:7", first["reason"])
	assert.Equal(t, "2026-05-13T09:30:00Z", first["occurred_at"])

	walletRepo.AssertExpectations(t)
}

func TestListUserWalletTransactions_Admin_OtherUser(t *testing.T) {
	app, walletRepo, _, _, _, _ := setupGamificationHandler(99, true)

	walletRepo.On(
		"ListTransactionsForUserAndCurrency", mock.Anything,
		uint(42), uint(11),
		repository.PaginationParams{Page: 1, PerPage: 20},
	).Return(&repository.PaginatedResult[models.GamificationWalletTransaction]{
		Items: []models.GamificationWalletTransaction{}, TotalCount: 0, Page: 1, PerPage: 20,
	}, nil)

	resp := testutil.MakeRequest(app, http.MethodGet,
		"/api/v1/users/42/wallet/transactions?currency_type_id=11", nil)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
}

func TestListUserWalletTransactions_Forbidden(t *testing.T) {
	app, walletRepo, _, _, _, _ := setupGamificationHandler(99, false)

	resp := testutil.MakeRequest(app, http.MethodGet,
		"/api/v1/users/42/wallet/transactions?currency_type_id=11", nil)
	assert.Equal(t, http.StatusForbidden, resp.StatusCode)
	walletRepo.AssertNotCalled(t, "ListTransactionsForUserAndCurrency",
		mock.Anything, mock.Anything, mock.Anything, mock.Anything)
}

func TestListUserWalletTransactions_MissingCurrencyTypeID(t *testing.T) {
	app, _, _, _, _, _ := setupGamificationHandler(42, false)

	resp := testutil.MakeRequest(app, http.MethodGet,
		"/api/v1/users/42/wallet/transactions", nil)
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

func TestListUserWalletTransactions_InvalidCurrencyTypeID(t *testing.T) {
	app, _, _, _, _, _ := setupGamificationHandler(42, false)

	resp := testutil.MakeRequest(app, http.MethodGet,
		"/api/v1/users/42/wallet/transactions?currency_type_id=0", nil)
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

func TestListUserWalletTransactions_PerPageClampedTo100(t *testing.T) {
	app, walletRepo, _, _, _, _ := setupGamificationHandler(42, false)

	walletRepo.On(
		"ListTransactionsForUserAndCurrency", mock.Anything,
		uint(42), uint(11),
		repository.PaginationParams{Page: 1, PerPage: 100},
	).Return(&repository.PaginatedResult[models.GamificationWalletTransaction]{
		Items: []models.GamificationWalletTransaction{}, TotalCount: 0, Page: 1, PerPage: 100,
	}, nil)

	resp := testutil.MakeRequest(app, http.MethodGet,
		"/api/v1/users/42/wallet/transactions?currency_type_id=11&per_page=9999", nil)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	walletRepo.AssertExpectations(t)
}

func TestListUserWalletTransactions_RepoError(t *testing.T) {
	app, walletRepo, _, _, _, _ := setupGamificationHandler(42, false)

	walletRepo.On(
		"ListTransactionsForUserAndCurrency", mock.Anything,
		uint(42), uint(11),
		repository.PaginationParams{Page: 1, PerPage: 20},
	).Return(nil, errors.New("db down"))

	resp := testutil.MakeRequest(app, http.MethodGet,
		"/api/v1/users/42/wallet/transactions?currency_type_id=11", nil)
	assert.Equal(t, http.StatusInternalServerError, resp.StatusCode)
}

// ---------------------------------------------------------------------------
// W2-B: CreateCurrency / UpdateCurrency / DeleteCurrency.
//
// Authorization is enforced by router middleware (RequireAdmin /
// RequireInstructor) which isn't mounted in these tests; what's
// exercised here is handler-level invariants — input validation, scope
// guards, system_owned protections, conflict semantics. Route wiring is
// covered separately by integration smoke against the real DB.
// ---------------------------------------------------------------------------

func postJSON(app *fiber.App, path, body string) *http.Response {
	return testutil.MakeRequest(app, http.MethodPost, path, strings.NewReader(body))
}
func patchJSON(app *fiber.App, path, body string) *http.Response {
	return testutil.MakeRequest(app, http.MethodPatch, path, strings.NewReader(body))
}
func deleteReq(app *fiber.App, path string) *http.Response {
	return testutil.MakeRequest(app, http.MethodDelete, path, nil)
}

func TestCreateCurrency_HappyPath_SiteScope(t *testing.T) {
	app, _, currencyRepo, _, _, _ := setupGamificationHandler(7, true)

	currencyRepo.On("Create", mock.Anything, mock.MatchedBy(func(c *models.GamificationCurrencyType) bool {
		return c.Code == "coins" &&
			c.ScopeType == models.ScopeSite &&
			c.ScopeID == 1 &&
			c.DisplayLabel == "Coin" &&
			c.Spendable == true &&
			c.Monotonic == false && // explicit false; bool-default regression class
			c.SystemOwned == false &&
			c.FerpaClassification == "non_PII"
	})).Run(func(args mock.Arguments) {
		c := args.Get(1).(*models.GamificationCurrencyType)
		c.ID = 42
	}).Return(nil)

	resp := postJSON(app, "/api/v1/gamification/currencies", `{
		"code": "coins",
		"display_label": "Coin",
		"display_label_plural": "Coins",
		"icon": "coins",
		"color": "#A855F7",
		"spendable": true,
		"monotonic": false,
		"description": "Class economy currency"
	}`)
	require.Equal(t, http.StatusCreated, resp.StatusCode)

	body, err := testutil.ParseJSONMap(resp)
	require.NoError(t, err)
	assert.Equal(t, float64(42), body["id"])
	assert.Equal(t, "coins", body["code"])
	assert.Equal(t, false, body["system_owned"])
	currencyRepo.AssertExpectations(t)
}

func TestCreateCurrency_CourseScope_ResolvedFromURL(t *testing.T) {
	app, _, currencyRepo, _, _, _ := setupGamificationHandler(7, false)

	currencyRepo.On("Create", mock.Anything, mock.MatchedBy(func(c *models.GamificationCurrencyType) bool {
		return c.ScopeType == models.ScopeCourse && c.ScopeID == 99 && c.Code == "stars"
	})).Run(func(args mock.Arguments) {
		args.Get(1).(*models.GamificationCurrencyType).ID = 50
	}).Return(nil)

	resp := postJSON(app, "/api/v1/courses/99/gamification/currencies",
		`{"code":"stars","display_label":"Star"}`)
	require.Equal(t, http.StatusCreated, resp.StatusCode)
	currencyRepo.AssertExpectations(t)
}

func TestCreateCurrency_RejectsBadCode(t *testing.T) {
	app, _, _, _, _, _ := setupGamificationHandler(7, true)
	for _, code := range []string{"", "X", "XP", "1coin", "has space", "WAY_TOO_LONG_CODE_THAT_EXCEEDS_THIRTY_TWO_CHARS"} {
		body := `{"code":"` + code + `","display_label":"Test"}`
		resp := postJSON(app, "/api/v1/gamification/currencies", body)
		assert.Equal(t, http.StatusBadRequest, resp.StatusCode, "code %q must be rejected", code)
	}
}

func TestCreateCurrency_RejectsBadColor(t *testing.T) {
	app, _, _, _, _, _ := setupGamificationHandler(7, true)
	resp := postJSON(app, "/api/v1/gamification/currencies",
		`{"code":"coins","display_label":"Coin","color":"not-a-hex"}`)
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

func TestCreateCurrency_RejectsEmptyLabel(t *testing.T) {
	app, _, _, _, _, _ := setupGamificationHandler(7, true)
	resp := postJSON(app, "/api/v1/gamification/currencies",
		`{"code":"coins","display_label":""}`)
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

func TestCreateCurrency_Duplicate_Returns409(t *testing.T) {
	// Atomic duplicate detection: the repo's INSERT ... ON CONFLICT DO
	// NOTHING returns ErrCurrencyDuplicate when the (tenant, scope, code)
	// tuple already exists. The handler maps that to 409. This collapses
	// what would otherwise be a "FindByCode then Create" sequence with a
	// TOCTOU window into a single round-trip — concurrent admins minting
	// the same code each get a deterministic 409 (or one of them gets
	// 201) instead of one getting a 500 from the unique constraint.
	app, _, currencyRepo, _, _, _ := setupGamificationHandler(7, true)
	currencyRepo.On("Create", mock.Anything, mock.MatchedBy(func(c *models.GamificationCurrencyType) bool {
		return c.Code == "xp"
	})).Return(repository.ErrCurrencyDuplicate)

	resp := postJSON(app, "/api/v1/gamification/currencies",
		`{"code":"xp","display_label":"Duplicate"}`)
	assert.Equal(t, http.StatusConflict, resp.StatusCode)
	currencyRepo.AssertExpectations(t)
}

func TestCreateCurrency_ForcesSystemOwnedFalse(t *testing.T) {
	// Even if a malicious client sends system_owned=true in the JSON, the
	// handler should ignore it. (The struct doesn't even unmarshal it.)
	app, _, currencyRepo, _, _, _ := setupGamificationHandler(7, true)
	currencyRepo.On("Create", mock.Anything, mock.MatchedBy(func(c *models.GamificationCurrencyType) bool {
		return !c.SystemOwned
	})).Run(func(args mock.Arguments) {
		args.Get(1).(*models.GamificationCurrencyType).ID = 99
	}).Return(nil)

	resp := postJSON(app, "/api/v1/gamification/currencies",
		`{"code":"evil","display_label":"Evil","system_owned":true}`)
	assert.Equal(t, http.StatusCreated, resp.StatusCode)
	currencyRepo.AssertExpectations(t)
}

func TestUpdateCurrency_HappyPath_RenameSystem(t *testing.T) {
	// A tenant admin can rename system_owned currencies (label/icon/color)
	// but not their code. The code field isn't in patchCurrencyInput so
	// any attempt to change it via JSON is silently ignored.
	app, _, currencyRepo, _, _, _ := setupGamificationHandler(7, true)
	xp := fixtureXP() // system_owned=true, code="xp"
	currencyRepo.On("FindByID", mock.Anything, uint(11)).Return(&xp, nil)
	currencyRepo.On("Update", mock.Anything, mock.MatchedBy(func(c *models.GamificationCurrencyType) bool {
		return c.Code == "xp" && // unchanged
			c.DisplayLabel == "Energy" && // new label
			c.Color == "#10B981" // new color
	})).Return(nil)

	resp := patchJSON(app, "/api/v1/gamification/currencies/11",
		`{"display_label":"Energy","color":"#10B981","code":"renamed"}`)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	currencyRepo.AssertExpectations(t)
}

func TestUpdateCurrency_TogglesVisibleInTopbar(t *testing.T) {
	// Verifies the bool-default class regression is closed for PATCH: a
	// teacher setting visible_in_topbar=false on a currency that was true
	// must actually persist false. db.Save (not db.Updates) handles this.
	app, _, currencyRepo, _, _, _ := setupGamificationHandler(7, true)
	gems := fixtureGems()
	currencyRepo.On("FindByID", mock.Anything, uint(12)).Return(&gems, nil)
	currencyRepo.On("Update", mock.Anything, mock.MatchedBy(func(c *models.GamificationCurrencyType) bool {
		return c.VisibleInTopbar == false
	})).Return(nil)

	resp := patchJSON(app, "/api/v1/gamification/currencies/12",
		`{"visible_in_topbar":false}`)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	currencyRepo.AssertExpectations(t)
}

// TestUpdateCurrency_ScopeMismatch_404 locks the 13.1.E existence-leak
// contract: scope-mismatched PATCHes must return 404, not 403. A 403
// would confirm the currency exists somewhere (possibly another scope
// or tenant); 404 keeps that signal silent.
func TestUpdateCurrency_ScopeMismatch_404(t *testing.T) {
	// A site-scoped XP currency cannot be PATCHed via the course-scoped
	// route. Prevents a course instructor on course A from touching a
	// site-scoped currency they don't own.
	app, _, currencyRepo, _, _, _ := setupGamificationHandler(7, false)
	xp := fixtureXP() // site/1
	currencyRepo.On("FindByID", mock.Anything, uint(11)).Return(&xp, nil)

	resp := patchJSON(app, "/api/v1/courses/99/gamification/currencies/11",
		`{"display_label":"Hijack"}`)
	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
	currencyRepo.AssertNotCalled(t, "Update", mock.Anything, mock.Anything)
}

func TestUpdateCurrency_NotFound(t *testing.T) {
	app, _, currencyRepo, _, _, _ := setupGamificationHandler(7, true)
	currencyRepo.On("FindByID", mock.Anything, uint(99)).Return(nil, nil)
	resp := patchJSON(app, "/api/v1/gamification/currencies/99", `{"display_label":"X"}`)
	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
}

func TestDeleteCurrency_SystemOwned_409(t *testing.T) {
	app, _, currencyRepo, _, _, _ := setupGamificationHandler(7, true)
	xp := fixtureXP()
	currencyRepo.On("FindByID", mock.Anything, uint(11)).Return(&xp, nil)

	resp := deleteReq(app, "/api/v1/gamification/currencies/11")
	assert.Equal(t, http.StatusConflict, resp.StatusCode)
	currencyRepo.AssertNotCalled(t, "Delete", mock.Anything, mock.Anything)
}

func TestDeleteCurrency_CustomRow_204(t *testing.T) {
	app, _, currencyRepo, _, _, _ := setupGamificationHandler(7, true)
	custom := models.GamificationCurrencyType{
		ID: 50, TenantID: 1, ScopeType: models.ScopeSite, ScopeID: 1,
		Code: "coins", DisplayLabel: "Coin", SystemOwned: false,
	}
	currencyRepo.On("FindByID", mock.Anything, uint(50)).Return(&custom, nil)
	currencyRepo.On("Delete", mock.Anything, uint(50)).Return(nil)

	resp := deleteReq(app, "/api/v1/gamification/currencies/50")
	assert.Equal(t, http.StatusNoContent, resp.StatusCode)
	currencyRepo.AssertExpectations(t)
}

// TestDeleteCurrency_ScopeMismatch_404 — 13.1.E existence-leak fix.
// See TestUpdateCurrency_ScopeMismatch_404 for the rationale.
func TestDeleteCurrency_ScopeMismatch_404(t *testing.T) {
	app, _, currencyRepo, _, _, _ := setupGamificationHandler(7, false)
	custom := models.GamificationCurrencyType{
		ID: 50, TenantID: 1, ScopeType: models.ScopeCourse, ScopeID: 99,
		Code: "stars", SystemOwned: false,
	}
	currencyRepo.On("FindByID", mock.Anything, uint(50)).Return(&custom, nil)

	resp := deleteReq(app, "/api/v1/courses/77/gamification/currencies/50")
	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
	currencyRepo.AssertNotCalled(t, "Delete", mock.Anything, mock.Anything)
}

// ---------------------------------------------------------------------------
// W2-C: gamification preferences (leaderboard opt-out).
// ---------------------------------------------------------------------------

func TestGetMyGamificationPreferences_HappyPath(t *testing.T) {
	app, _, _, userRepo, _, _ := setupGamificationHandler(42, false)
	userRepo.On("FindByID", mock.Anything, uint(42)).
		Return(&models.User{ID: 42, LeaderboardOptOut: true}, nil)

	resp := testutil.MakeRequest(app, http.MethodGet,
		"/api/v1/users/self/gamification_preferences", nil)
	require.Equal(t, http.StatusOK, resp.StatusCode)

	body, err := testutil.ParseJSONMap(resp)
	require.NoError(t, err)
	assert.Equal(t, true, body["leaderboard_opt_out"])
	userRepo.AssertExpectations(t)
}

func TestGetMyGamificationPreferences_DefaultsToFalse(t *testing.T) {
	app, _, _, userRepo, _, _ := setupGamificationHandler(42, false)
	userRepo.On("FindByID", mock.Anything, uint(42)).
		Return(&models.User{ID: 42}, nil) // LeaderboardOptOut zero-value

	resp := testutil.MakeRequest(app, http.MethodGet,
		"/api/v1/users/self/gamification_preferences", nil)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	body, _ := testutil.ParseJSONMap(resp)
	assert.Equal(t, false, body["leaderboard_opt_out"])
}

func TestUpdateMyGamificationPreferences_TogglesOn(t *testing.T) {
	// Verifies the bool-default class is closed for this PATCH too:
	// learner opts IN (false→true) via a PUT with a single field.
	app, _, _, userRepo, _, _ := setupGamificationHandler(42, false)
	existing := &models.User{ID: 42, LeaderboardOptOut: false}
	userRepo.On("FindByID", mock.Anything, uint(42)).Return(existing, nil)
	userRepo.On("Update", mock.Anything, mock.MatchedBy(func(u *models.User) bool {
		return u.ID == 42 && u.LeaderboardOptOut == true
	})).Return(nil)

	resp := testutil.MakeRequest(app, http.MethodPut,
		"/api/v1/users/self/gamification_preferences",
		strings.NewReader(`{"leaderboard_opt_out":true}`))
	require.Equal(t, http.StatusOK, resp.StatusCode)

	body, _ := testutil.ParseJSONMap(resp)
	assert.Equal(t, true, body["leaderboard_opt_out"])
	userRepo.AssertExpectations(t)
}

func TestUpdateMyGamificationPreferences_TogglesOff(t *testing.T) {
	// Verifies the inverse: learner opts back IN (true→false). The pointer
	// PATCH body distinguishes "explicitly set to false" from "field
	// omitted", so db.Save writes the false explicitly.
	app, _, _, userRepo, _, _ := setupGamificationHandler(42, false)
	existing := &models.User{ID: 42, LeaderboardOptOut: true}
	userRepo.On("FindByID", mock.Anything, uint(42)).Return(existing, nil)
	userRepo.On("Update", mock.Anything, mock.MatchedBy(func(u *models.User) bool {
		return u.ID == 42 && u.LeaderboardOptOut == false
	})).Return(nil)

	resp := testutil.MakeRequest(app, http.MethodPut,
		"/api/v1/users/self/gamification_preferences",
		strings.NewReader(`{"leaderboard_opt_out":false}`))
	require.Equal(t, http.StatusOK, resp.StatusCode)
	userRepo.AssertExpectations(t)
}

func TestUpdateMyGamificationPreferences_OmittedFieldIsNoop(t *testing.T) {
	// A PUT body with no leaderboard_opt_out leaves the existing value
	// unchanged. This is the contract for adding more prefs over time:
	// each PUT is a partial update, never a wipe.
	app, _, _, userRepo, _, _ := setupGamificationHandler(42, false)
	existing := &models.User{ID: 42, LeaderboardOptOut: true}
	userRepo.On("FindByID", mock.Anything, uint(42)).Return(existing, nil)
	userRepo.On("Update", mock.Anything, mock.MatchedBy(func(u *models.User) bool {
		return u.ID == 42 && u.LeaderboardOptOut == true // unchanged
	})).Return(nil)

	resp := testutil.MakeRequest(app, http.MethodPut,
		"/api/v1/users/self/gamification_preferences",
		strings.NewReader(`{}`))
	require.Equal(t, http.StatusOK, resp.StatusCode)
	userRepo.AssertExpectations(t)
}

func TestUpdateMyGamificationPreferences_RejectsBadBody(t *testing.T) {
	app, _, _, _, _, _ := setupGamificationHandler(42, false)

	resp := testutil.MakeRequest(app, http.MethodPut,
		"/api/v1/users/self/gamification_preferences",
		strings.NewReader(`not-json`))
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

func TestGetMyGamificationPreferences_Unauthenticated(t *testing.T) {
	// callerID=0 — middleware Locals not set; handler must 401.
	app, _, _, _, _, _ := setupGamificationHandler(0, false)

	resp := testutil.MakeRequest(app, http.MethodGet,
		"/api/v1/users/self/gamification_preferences", nil)
	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
}

// ---------------------------------------------------------------------------
// W2-D: Badge CRUD + per-user list + manual award/revoke.
// ---------------------------------------------------------------------------

// fixtureBadge moved to gamification_fixtures_test.go (F2.5 closeout).

func TestCreateBadge_HappyPath_SiteScope(t *testing.T) {
	app, _, _, _, badgeRepo, _ := setupGamificationHandler(7, true)
	badgeRepo.On("Create", mock.Anything, mock.MatchedBy(func(b *models.GamificationBadge) bool {
		return b.Code == "first_quiz" &&
			b.Name == "First Quiz" &&
			b.ScopeType == models.ScopeSite && b.ScopeID == 1 &&
			b.InternalOnly == true && // default-on
			b.SystemOwned == false
	})).Run(func(args mock.Arguments) {
		args.Get(1).(*models.GamificationBadge).ID = 99
	}).Return(nil)

	resp := postJSON(app, "/api/v1/gamification/badges",
		`{"code":"first_quiz","name":"First Quiz","description":"Pass your first quiz."}`)
	require.Equal(t, http.StatusCreated, resp.StatusCode)
	body, _ := testutil.ParseJSONMap(resp)
	assert.Equal(t, float64(99), body["id"])
	assert.Equal(t, true, body["internal_only"])
	badgeRepo.AssertExpectations(t)
}

func TestCreateBadge_CourseScope(t *testing.T) {
	app, _, _, _, badgeRepo, _ := setupGamificationHandler(7, false)
	badgeRepo.On("Create", mock.Anything, mock.MatchedBy(func(b *models.GamificationBadge) bool {
		return b.ScopeType == models.ScopeCourse && b.ScopeID == 99
	})).Run(func(args mock.Arguments) {
		args.Get(1).(*models.GamificationBadge).ID = 100
	}).Return(nil)

	resp := postJSON(app, "/api/v1/courses/99/gamification/badges",
		`{"code":"explorer","name":"Course Explorer"}`)
	require.Equal(t, http.StatusCreated, resp.StatusCode)
	badgeRepo.AssertExpectations(t)
}

func TestCreateBadge_AtomicDuplicate_409(t *testing.T) {
	// Mirrors W2-B's race-safe path: repo returns ErrBadgeDuplicate from
	// ON CONFLICT DO NOTHING; handler maps to 409.
	app, _, _, _, badgeRepo, _ := setupGamificationHandler(7, true)
	badgeRepo.On("Create", mock.Anything, mock.Anything).Return(repository.ErrBadgeDuplicate)

	resp := postJSON(app, "/api/v1/gamification/badges",
		`{"code":"dupe","name":"Dupe"}`)
	assert.Equal(t, http.StatusConflict, resp.StatusCode)
	badgeRepo.AssertExpectations(t)
}

func TestCreateBadge_RejectsBadInputs(t *testing.T) {
	app, _, _, _, _, _ := setupGamificationHandler(7, true)
	for _, body := range []string{
		`{"code":"","name":"X"}`,                 // bad code
		`{"code":"X","name":"Y"}`,                // uppercase
		`{"code":"good","name":""}`,              // empty name
		`{"code":"good","name":"X","color":"oops"}`, // bad color
	} {
		resp := postJSON(app, "/api/v1/gamification/badges", body)
		assert.Equal(t, http.StatusBadRequest, resp.StatusCode, "body %q must be rejected", body)
	}
}

func TestUpdateBadge_HappyPath(t *testing.T) {
	app, _, _, _, badgeRepo, _ := setupGamificationHandler(7, true)
	row := fixtureBadge()
	badgeRepo.On("FindByID", mock.Anything, uint(50)).Return(&row, nil)
	badgeRepo.On("Update", mock.Anything, mock.MatchedBy(func(b *models.GamificationBadge) bool {
		return b.Code == "first_quiz" && // unchanged
			b.Name == "Renamed" &&
			b.InternalOnly == false // toggled — bool-default class
	})).Return(nil)

	resp := patchJSON(app, "/api/v1/gamification/badges/50",
		`{"name":"Renamed","internal_only":false,"code":"hijack_attempt"}`)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	badgeRepo.AssertExpectations(t)
}

// TestUpdateBadge_ScopeMismatch_404 — 13.1.E existence-leak fix.
// See TestUpdateCurrency_ScopeMismatch_404 for rationale.
func TestUpdateBadge_ScopeMismatch_404(t *testing.T) {
	// A site-scoped badge can't be PATCHed via the course-scoped route.
	app, _, _, _, badgeRepo, _ := setupGamificationHandler(7, false)
	row := fixtureBadge() // site
	badgeRepo.On("FindByID", mock.Anything, uint(50)).Return(&row, nil)

	resp := patchJSON(app, "/api/v1/courses/99/gamification/badges/50",
		`{"name":"Hijack"}`)
	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
	badgeRepo.AssertNotCalled(t, "Update", mock.Anything, mock.Anything)
}

func TestDeleteBadge_SystemOwned_409(t *testing.T) {
	app, _, _, _, badgeRepo, _ := setupGamificationHandler(7, true)
	sys := fixtureBadge()
	sys.SystemOwned = true
	badgeRepo.On("FindByID", mock.Anything, uint(50)).Return(&sys, nil)

	resp := deleteReq(app, "/api/v1/gamification/badges/50")
	assert.Equal(t, http.StatusConflict, resp.StatusCode)
	badgeRepo.AssertNotCalled(t, "Delete", mock.Anything, mock.Anything)
}

func TestDeleteBadge_CustomRow_204(t *testing.T) {
	app, _, _, _, badgeRepo, _ := setupGamificationHandler(7, true)
	row := fixtureBadge() // system_owned=false
	badgeRepo.On("FindByID", mock.Anything, uint(50)).Return(&row, nil)
	badgeRepo.On("Delete", mock.Anything, uint(50)).Return(nil)

	resp := deleteReq(app, "/api/v1/gamification/badges/50")
	assert.Equal(t, http.StatusNoContent, resp.StatusCode)
	badgeRepo.AssertExpectations(t)
}

func TestListUserBadges_Self_HappyPath(t *testing.T) {
	app, _, _, _, badgeRepo, awardRepo := setupGamificationHandler(42, false)
	row := fixtureBadge()
	awardRepo.On("ListForUser", mock.Anything, uint(42)).
		Return([]models.GamificationBadgeAward{
			{ID: 1, UserID: 42, BadgeID: 50, AwardedAt: time.Date(2026, 5, 13, 9, 0, 0, 0, time.UTC)},
		}, nil)
	badgeRepo.On("FindByID", mock.Anything, uint(50)).Return(&row, nil)

	resp := testutil.MakeRequest(app, http.MethodGet, "/api/v1/users/42/badges", nil)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	body, _ := testutil.ParseJSONMap(resp)
	assert.Equal(t, float64(42), body["user_id"])
	rows := body["badges"].([]interface{})
	require.Len(t, rows, 1)
	first := rows[0].(map[string]interface{})
	assert.Equal(t, "first_quiz", first["code"])
	assert.Equal(t, "First Quiz", first["name"])
	assert.Equal(t, "2026-05-13T09:00:00Z", first["awarded_at"])
}

func TestListUserBadges_Forbidden(t *testing.T) {
	app, _, _, _, _, _ := setupGamificationHandler(99, false)
	resp := testutil.MakeRequest(app, http.MethodGet, "/api/v1/users/42/badges", nil)
	assert.Equal(t, http.StatusForbidden, resp.StatusCode)
}

func TestAwardBadgeToUser_Created(t *testing.T) {
	app, _, _, _, badgeRepo, awardRepo := setupGamificationHandler(7, true)
	row := fixtureBadge()
	badgeRepo.On("FindByID", mock.Anything, uint(50)).Return(&row, nil)
	awardRepo.On("Award", mock.Anything, mock.MatchedBy(func(a *models.GamificationBadgeAward) bool {
		return a.UserID == 42 && a.BadgeID == 50 && a.AwardedBy != nil && *a.AwardedBy == 7
	})).Run(func(args mock.Arguments) {
		args.Get(1).(*models.GamificationBadgeAward).ID = 11
	}).Return(true, nil)

	resp := postJSON(app, "/api/v1/users/42/badges", `{"badge_id":50}`)
	require.Equal(t, http.StatusCreated, resp.StatusCode)
	body, _ := testutil.ParseJSONMap(resp)
	assert.Equal(t, true, body["created"])
}

func TestAwardBadgeToUser_Idempotent200(t *testing.T) {
	// Re-awarding the same badge → 200 OK with created=false (no error).
	app, _, _, _, badgeRepo, awardRepo := setupGamificationHandler(7, true)
	row := fixtureBadge()
	badgeRepo.On("FindByID", mock.Anything, uint(50)).Return(&row, nil)
	awardRepo.On("Award", mock.Anything, mock.Anything).Return(false, nil)

	resp := postJSON(app, "/api/v1/users/42/badges", `{"badge_id":50}`)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	body, _ := testutil.ParseJSONMap(resp)
	assert.Equal(t, false, body["created"])
}

func TestAwardBadgeToUser_RejectsBadInput(t *testing.T) {
	app, _, _, _, _, _ := setupGamificationHandler(7, true)
	resp := postJSON(app, "/api/v1/users/42/badges", `{}`)
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

// TestAwardBadgeToUser_TenantMismatch_404 — 13.1.E existence-leak fix.
// Awarding a badge that belongs to a different tenant must return 404,
// not 403; 403 would confirm the badge ID exists somewhere.
func TestAwardBadgeToUser_TenantMismatch_404(t *testing.T) {
	app, _, _, _, badgeRepo, _ := setupGamificationHandler(7, true)
	row := fixtureBadge()
	row.TenantID = 99 // someone else's tenant
	badgeRepo.On("FindByID", mock.Anything, uint(50)).Return(&row, nil)

	resp := postJSON(app, "/api/v1/users/42/badges", `{"badge_id":50}`)
	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
}

func TestRevokeBadgeFromUser_NoContent(t *testing.T) {
	app, _, _, _, _, awardRepo := setupGamificationHandler(7, true)
	awardRepo.On("Revoke", mock.Anything, uint(42), uint(50)).Return(nil)
	resp := deleteReq(app, "/api/v1/users/42/badges/50")
	assert.Equal(t, http.StatusNoContent, resp.StatusCode)
	awardRepo.AssertExpectations(t)
}

func TestListBadges_HappyPath(t *testing.T) {
	app, _, _, _, badgeRepo, _ := setupGamificationHandler(7, false)
	row := fixtureBadge()
	badgeRepo.On("ListByTenant", mock.Anything, uint(1)).
		Return([]models.GamificationBadge{row}, nil)

	resp := testutil.MakeRequest(app, http.MethodGet, "/api/v1/gamification/badges", nil)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	body, _ := testutil.ParseJSONMap(resp)
	rows := body["badges"].([]interface{})
	require.Len(t, rows, 1)
	assert.Equal(t, "first_quiz", rows[0].(map[string]interface{})["code"])
}
