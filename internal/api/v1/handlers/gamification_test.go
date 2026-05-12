package handlers_test

import (
	"context"
	"errors"
	"net/http"
	"testing"

	"github.com/gofiber/fiber/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/EduThemes/paper-lms/internal/api/v1/handlers"
	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/repository"
	"github.com/EduThemes/paper-lms/internal/testutil"
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

// Compile-time check: our mocks satisfy the production interfaces.
var (
	_ repository.GamificationWalletRepository       = (*mockGamWalletRepo)(nil)
	_ repository.GamificationCurrencyTypeRepository = (*mockGamCurrencyRepo)(nil)
)

// ---------------------------------------------------------------------------
// Fixtures + harness.
// ---------------------------------------------------------------------------

// setupGamificationHandler wires the handler to fresh mocks and registers
// the two routes on a test app. The auth-stub middleware injects the given
// callerID + isAdmin flag into Locals so the handler's self-or-admin check
// has something to read.
func setupGamificationHandler(callerID uint, isAdmin bool) (*fiber.App, *mockGamWalletRepo, *mockGamCurrencyRepo) {
	walletRepo := new(mockGamWalletRepo)
	currencyRepo := new(mockGamCurrencyRepo)
	h := handlers.NewGamificationHandler(walletRepo, currencyRepo)

	app := testutil.SetupTestApp()
	app.Use(func(c *fiber.Ctx) error {
		c.Locals("user_id", callerID)
		c.Locals("is_admin", isAdmin)
		return c.Next()
	})

	app.Get("/api/v1/users/:id/wallet", h.GetUserWallet)
	app.Get("/api/v1/gamification/currencies", h.ListCurrencies)
	return app, walletRepo, currencyRepo
}

func fixtureXP() models.GamificationCurrencyType {
	return models.GamificationCurrencyType{
		ID:                 11,
		TenantID:           1,
		Code:               "xp",
		DisplayLabel:       "XP",
		DisplayLabelPlural: "XP",
		Icon:               "zap",
		Color:              "#F59E0B",
		DisplayOrder:       1,
		Spendable:          false,
		Monotonic:          true,
		VisibleToStudent:   true,
		VisibleInTopbar:    true,
		SystemOwned:        true,
		Description:        "Experience points",
	}
}

func fixtureGems() models.GamificationCurrencyType {
	return models.GamificationCurrencyType{
		ID:                 12,
		TenantID:           1,
		Code:               "gems",
		DisplayLabel:       "Gem",
		DisplayLabelPlural: "Gems",
		Icon:               "gem",
		Color:              "#10B981",
		DisplayOrder:       2,
		Spendable:          true,
		Monotonic:          false,
		VisibleToStudent:   true,
		VisibleInTopbar:    true,
		SystemOwned:        true,
	}
}

// fixtureHidden is a non-topbar currency used to verify topbar_only filter.
func fixtureHidden() models.GamificationCurrencyType {
	return models.GamificationCurrencyType{
		ID:               13,
		TenantID:         1,
		Code:             "mastery_points",
		DisplayLabel:     "Mastery",
		DisplayOrder:     3,
		VisibleToStudent: true,
		VisibleInTopbar:  false,
		SystemOwned:      true,
	}
}

// ---------------------------------------------------------------------------
// GetUserWallet.
// ---------------------------------------------------------------------------

func TestGetUserWallet_Self_HappyPath(t *testing.T) {
	app, walletRepo, currencyRepo := setupGamificationHandler(42, false)

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
	assert.Equal(t, "gems", second["code"])
	assert.Equal(t, true, second["spendable"])
	assert.Equal(t, float64(8), second["balance"])
	assert.Equal(t, float64(12), second["lifetime_earned"])

	walletRepo.AssertExpectations(t)
	currencyRepo.AssertExpectations(t)
}

func TestGetUserWallet_AdminViewingOtherUser(t *testing.T) {
	app, walletRepo, currencyRepo := setupGamificationHandler(99, true /*isAdmin*/)

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
	app, walletRepo, currencyRepo := setupGamificationHandler(99, false)

	resp := testutil.MakeRequest(app, http.MethodGet, "/api/v1/users/42/wallet", nil)
	assert.Equal(t, http.StatusForbidden, resp.StatusCode)

	// Repo must not be touched on a 403.
	walletRepo.AssertNotCalled(t, "ListBalancesForUser", mock.Anything, mock.Anything)
	currencyRepo.AssertNotCalled(t, "FindByID", mock.Anything, mock.Anything)
}

func TestGetUserWallet_EmptyBalances_ReturnsEmptyArrayNot404(t *testing.T) {
	app, walletRepo, _ := setupGamificationHandler(42, false)

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
	app, _, _ := setupGamificationHandler(1, true)

	resp := testutil.MakeRequest(app, http.MethodGet, "/api/v1/users/abc/wallet", nil)
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

func TestGetUserWallet_WalletRepoError(t *testing.T) {
	app, walletRepo, _ := setupGamificationHandler(42, false)

	walletRepo.On("ListBalancesForUser", mock.Anything, uint(42)).
		Return(nil, errors.New("db down"))

	resp := testutil.MakeRequest(app, http.MethodGet, "/api/v1/users/42/wallet", nil)
	assert.Equal(t, http.StatusInternalServerError, resp.StatusCode)
}

// ---------------------------------------------------------------------------
// ListCurrencies.
// ---------------------------------------------------------------------------

func TestListCurrencies_All(t *testing.T) {
	app, _, currencyRepo := setupGamificationHandler(7, false)

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
	app, _, currencyRepo := setupGamificationHandler(7, false)

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
	app, _, currencyRepo := setupGamificationHandler(7, false)

	currencyRepo.On("ListByTenant", mock.Anything, uint(1)).Return(nil, errors.New("db down"))

	resp := testutil.MakeRequest(app, http.MethodGet, "/api/v1/gamification/currencies", nil)
	assert.Equal(t, http.StatusInternalServerError, resp.StatusCode)
}
