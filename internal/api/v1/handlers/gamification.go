package handlers

import (
	"strconv"

	"github.com/gofiber/fiber/v2"

	"github.com/EduThemes/paper-lms/internal/api/v1/responses"
	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/repository"
)

// GamificationHandler exposes the Phase 6 / Wave 1 read-side REST API for the
// gamification engine: per-user wallet balances and per-tenant currency type
// metadata. Write paths (rule-fired AwardCurrency, manual grants) live in the
// dispatcher / effect package, not here.
type GamificationHandler struct {
	walletRepo   repository.GamificationWalletRepository
	currencyRepo repository.GamificationCurrencyTypeRepository
}

// NewGamificationHandler wires the read-side handlers.
func NewGamificationHandler(
	walletRepo repository.GamificationWalletRepository,
	currencyRepo repository.GamificationCurrencyTypeRepository,
) *GamificationHandler {
	return &GamificationHandler{
		walletRepo:   walletRepo,
		currencyRepo: currencyRepo,
	}
}

// walletBalanceJSON is the topbar-pill payload: balance plus enough currency
// metadata for the frontend to render label/icon/color without a follow-up
// fetch.
type walletBalanceJSON struct {
	Code               string `json:"code"`
	DisplayLabel       string `json:"display_label"`
	DisplayLabelPlural string `json:"display_label_plural"`
	Icon               string `json:"icon"`
	Color              string `json:"color"`
	Balance            int64  `json:"balance"`
	LifetimeEarned     int64  `json:"lifetime_earned"`
	Spendable          bool   `json:"spendable"`
	Monotonic          bool   `json:"monotonic"`
	VisibleInTopbar    bool   `json:"visible_in_topbar"`
	DisplayOrder       int    `json:"display_order"`
}

type userWalletResponse struct {
	UserID   uint                `json:"user_id"`
	Balances []walletBalanceJSON `json:"balances"`
}

// currencyJSON is the per-tenant currency-type descriptor consumed by the
// frontend topbar + admin-debug screens.
type currencyJSON struct {
	ID                 uint   `json:"id"`
	Code               string `json:"code"`
	DisplayLabel       string `json:"display_label"`
	DisplayLabelPlural string `json:"display_label_plural"`
	Icon               string `json:"icon"`
	Color              string `json:"color"`
	DisplayOrder       int    `json:"display_order"`
	Spendable          bool   `json:"spendable"`
	Monotonic          bool   `json:"monotonic"`
	VisibleToStudent   bool   `json:"visible_to_student"`
	VisibleInTopbar    bool   `json:"visible_in_topbar"`
	SystemOwned        bool   `json:"system_owned"`
	Description        string `json:"description"`
}

type listCurrenciesResponse struct {
	Currencies []currencyJSON `json:"currencies"`
}

// GetUserWallet handles GET /api/v1/users/:id/wallet.
//
// Authorization: requester must be the path-:id user themselves OR a cached
// admin (is_admin Locals flag set by RequireAdmin / RequireCourseRole). Wave
// 1 has no per-account role lookup primitive in the handler layer, so any
// teacher/admin not flagged by middleware is denied — Phase 3 wiring may
// promote a richer check.
//
// A user with no wallet activity returns 200 with an empty array, not 404.
//
// N+1 note: ListBalancesForUser returns ≤10 rows in Wave 1 (one per currency
// the tenant has defined and the user has touched); the per-balance
// FindByID is acceptable here. A future projection table can collapse this.
func (h *GamificationHandler) GetUserWallet(c *fiber.Ctx) error {
	pathID, err := strconv.ParseUint(c.Params("id"), 10, 64)
	if err != nil {
		return responses.BadRequest(c, "invalid user id")
	}
	targetUserID := uint(pathID)

	callerID, ok := c.Locals("user_id").(uint)
	if !ok || callerID == 0 {
		return responses.Unauthorized(c)
	}

	isAdmin, _ := c.Locals("is_admin").(bool)
	if callerID != targetUserID && !isAdmin {
		return responses.Error(c, fiber.StatusForbidden, "you can only access your own wallet")
	}

	balances, err := h.walletRepo.ListBalancesForUser(c.Context(), targetUserID)
	if err != nil {
		return responses.InternalError(c, "could not fetch wallet balances")
	}

	out := userWalletResponse{
		UserID:   targetUserID,
		Balances: make([]walletBalanceJSON, 0, len(balances)),
	}

	for i := range balances {
		b := &balances[i]
		currency, ferr := h.currencyRepo.FindByID(c.Context(), b.CurrencyTypeID)
		if ferr != nil || currency == nil {
			// Stale balance row pointing at a deleted currency: surface a
			// minimal entry so the frontend doesn't choke on the missing
			// metadata. system_owned currencies cannot be deleted, so this
			// only triggers for instructor-defined currencies post-delete.
			out.Balances = append(out.Balances, walletBalanceJSON{
				Code:           "",
				Balance:        b.Balance,
				LifetimeEarned: b.LifetimeEarned,
			})
			continue
		}
		out.Balances = append(out.Balances, walletBalanceJSON{
			Code:               currency.Code,
			DisplayLabel:       currency.DisplayLabel,
			DisplayLabelPlural: currency.DisplayLabelPlural,
			Icon:               currency.Icon,
			Color:              currency.Color,
			Balance:            b.Balance,
			LifetimeEarned:     b.LifetimeEarned,
			Spendable:          currency.Spendable,
			Monotonic:          currency.Monotonic,
			VisibleInTopbar:    currency.VisibleInTopbar,
			DisplayOrder:       currency.DisplayOrder,
		})
	}

	return c.JSON(out)
}

// ListCurrencies handles GET /api/v1/gamification/currencies.
//
// Authorization: any authenticated user — currency-type rows are tenant
// metadata, not FERPA-protected balances. (The auth middleware on the route
// group already enforces authentication.)
//
// Query params:
//   - topbar_only=true → filter to visible_in_topbar=true (topbar pills only).
//     Anything else / unset → return all currencies for the tenant.
func (h *GamificationHandler) ListCurrencies(c *fiber.Ctx) error {
	if _, ok := c.Locals("user_id").(uint); !ok {
		return responses.Unauthorized(c)
	}

	tenantID := callerAccountID(c)

	var (
		currencies []models.GamificationCurrencyType
		err        error
	)
	if c.Query("topbar_only") == "true" {
		currencies, err = h.currencyRepo.ListInTopbar(c.Context(), tenantID)
	} else {
		currencies, err = h.currencyRepo.ListByTenant(c.Context(), tenantID)
	}
	if err != nil {
		return responses.InternalError(c, "could not fetch currencies")
	}

	out := listCurrenciesResponse{
		Currencies: make([]currencyJSON, 0, len(currencies)),
	}
	for i := range currencies {
		ct := &currencies[i]
		out.Currencies = append(out.Currencies, currencyJSON{
			ID:                 ct.ID,
			Code:               ct.Code,
			DisplayLabel:       ct.DisplayLabel,
			DisplayLabelPlural: ct.DisplayLabelPlural,
			Icon:               ct.Icon,
			Color:              ct.Color,
			DisplayOrder:       ct.DisplayOrder,
			Spendable:          ct.Spendable,
			Monotonic:          ct.Monotonic,
			VisibleToStudent:   ct.VisibleToStudent,
			VisibleInTopbar:    ct.VisibleInTopbar,
			SystemOwned:        ct.SystemOwned,
			Description:        ct.Description,
		})
	}

	return c.JSON(out)
}
