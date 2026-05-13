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
	return app, walletRepo, currencyRepo
}

func fixtureXP() models.GamificationCurrencyType {
	return models.GamificationCurrencyType{
		ID:                 11,
		TenantID:           1,
		ScopeType:          models.ScopeSite,
		ScopeID:            1,
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
		ScopeType:          models.ScopeSite,
		ScopeID:            1,
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

// ---------------------------------------------------------------------------
// ListUserWalletTransactions (Wave 2 W2-A: powers the wallet drawer).
// ---------------------------------------------------------------------------

func TestListUserWalletTransactions_Self_HappyPath(t *testing.T) {
	app, walletRepo, _ := setupGamificationHandler(42, false)

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
	app, walletRepo, _ := setupGamificationHandler(99, true)

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
	app, walletRepo, _ := setupGamificationHandler(99, false)

	resp := testutil.MakeRequest(app, http.MethodGet,
		"/api/v1/users/42/wallet/transactions?currency_type_id=11", nil)
	assert.Equal(t, http.StatusForbidden, resp.StatusCode)
	walletRepo.AssertNotCalled(t, "ListTransactionsForUserAndCurrency",
		mock.Anything, mock.Anything, mock.Anything, mock.Anything)
}

func TestListUserWalletTransactions_MissingCurrencyTypeID(t *testing.T) {
	app, _, _ := setupGamificationHandler(42, false)

	resp := testutil.MakeRequest(app, http.MethodGet,
		"/api/v1/users/42/wallet/transactions", nil)
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

func TestListUserWalletTransactions_InvalidCurrencyTypeID(t *testing.T) {
	app, _, _ := setupGamificationHandler(42, false)

	resp := testutil.MakeRequest(app, http.MethodGet,
		"/api/v1/users/42/wallet/transactions?currency_type_id=0", nil)
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

func TestListUserWalletTransactions_PerPageClampedTo100(t *testing.T) {
	app, walletRepo, _ := setupGamificationHandler(42, false)

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
	app, walletRepo, _ := setupGamificationHandler(42, false)

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
	app, _, currencyRepo := setupGamificationHandler(7, true)

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
	app, _, currencyRepo := setupGamificationHandler(7, false)

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
	app, _, _ := setupGamificationHandler(7, true)
	for _, code := range []string{"", "X", "XP", "1coin", "has space", "WAY_TOO_LONG_CODE_THAT_EXCEEDS_THIRTY_TWO_CHARS"} {
		body := `{"code":"` + code + `","display_label":"Test"}`
		resp := postJSON(app, "/api/v1/gamification/currencies", body)
		assert.Equal(t, http.StatusBadRequest, resp.StatusCode, "code %q must be rejected", code)
	}
}

func TestCreateCurrency_RejectsBadColor(t *testing.T) {
	app, _, _ := setupGamificationHandler(7, true)
	resp := postJSON(app, "/api/v1/gamification/currencies",
		`{"code":"coins","display_label":"Coin","color":"not-a-hex"}`)
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

func TestCreateCurrency_RejectsEmptyLabel(t *testing.T) {
	app, _, _ := setupGamificationHandler(7, true)
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
	app, _, currencyRepo := setupGamificationHandler(7, true)
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
	app, _, currencyRepo := setupGamificationHandler(7, true)
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
	app, _, currencyRepo := setupGamificationHandler(7, true)
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
	app, _, currencyRepo := setupGamificationHandler(7, true)
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

func TestUpdateCurrency_ScopeMismatch_403(t *testing.T) {
	// A site-scoped XP currency cannot be PATCHed via the course-scoped
	// route. Prevents a course instructor on course A from touching a
	// site-scoped currency they don't own.
	app, _, currencyRepo := setupGamificationHandler(7, false)
	xp := fixtureXP() // site/1
	currencyRepo.On("FindByID", mock.Anything, uint(11)).Return(&xp, nil)

	resp := patchJSON(app, "/api/v1/courses/99/gamification/currencies/11",
		`{"display_label":"Hijack"}`)
	assert.Equal(t, http.StatusForbidden, resp.StatusCode)
	currencyRepo.AssertNotCalled(t, "Update", mock.Anything, mock.Anything)
}

func TestUpdateCurrency_NotFound(t *testing.T) {
	app, _, currencyRepo := setupGamificationHandler(7, true)
	currencyRepo.On("FindByID", mock.Anything, uint(99)).Return(nil, nil)
	resp := patchJSON(app, "/api/v1/gamification/currencies/99", `{"display_label":"X"}`)
	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
}

func TestDeleteCurrency_SystemOwned_409(t *testing.T) {
	app, _, currencyRepo := setupGamificationHandler(7, true)
	xp := fixtureXP()
	currencyRepo.On("FindByID", mock.Anything, uint(11)).Return(&xp, nil)

	resp := deleteReq(app, "/api/v1/gamification/currencies/11")
	assert.Equal(t, http.StatusConflict, resp.StatusCode)
	currencyRepo.AssertNotCalled(t, "Delete", mock.Anything, mock.Anything)
}

func TestDeleteCurrency_CustomRow_204(t *testing.T) {
	app, _, currencyRepo := setupGamificationHandler(7, true)
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

func TestDeleteCurrency_ScopeMismatch_403(t *testing.T) {
	app, _, currencyRepo := setupGamificationHandler(7, false)
	custom := models.GamificationCurrencyType{
		ID: 50, TenantID: 1, ScopeType: models.ScopeCourse, ScopeID: 99,
		Code: "stars", SystemOwned: false,
	}
	currencyRepo.On("FindByID", mock.Anything, uint(50)).Return(&custom, nil)

	resp := deleteReq(app, "/api/v1/courses/77/gamification/currencies/50")
	assert.Equal(t, http.StatusForbidden, resp.StatusCode)
	currencyRepo.AssertNotCalled(t, "Delete", mock.Anything, mock.Anything)
}
