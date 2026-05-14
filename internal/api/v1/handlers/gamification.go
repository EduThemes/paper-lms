package handlers

import (
	"errors"
	"regexp"
	"strconv"
	"strings"
	"time"

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
	userRepo     repository.UserRepository
}

// NewGamificationHandler wires the read-side handlers. The userRepo
// dependency arrived in W2-C for the leaderboard opt-out preference
// (the toggle lives on `users.leaderboard_opt_out`).
func NewGamificationHandler(
	walletRepo repository.GamificationWalletRepository,
	currencyRepo repository.GamificationCurrencyTypeRepository,
	userRepo repository.UserRepository,
) *GamificationHandler {
	return &GamificationHandler{
		walletRepo:   walletRepo,
		currencyRepo: currencyRepo,
		userRepo:     userRepo,
	}
}

// walletBalanceJSON is the topbar-pill payload: balance plus enough currency
// metadata for the frontend to render label/icon/color without a follow-up
// fetch. currency_type_id is exposed so the wallet drawer can fetch
// per-currency transaction history without re-resolving by code (codes can
// repeat across scopes).
type walletBalanceJSON struct {
	CurrencyTypeID     uint   `json:"currency_type_id"`
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
// frontend topbar + admin-debug screens + W2-B editor.
type currencyJSON struct {
	ID                 uint   `json:"id"`
	ScopeType          string `json:"scope_type"`
	ScopeID            uint   `json:"scope_id"`
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
				CurrencyTypeID: b.CurrencyTypeID,
				Code:           "",
				Balance:        b.Balance,
				LifetimeEarned: b.LifetimeEarned,
			})
			continue
		}
		out.Balances = append(out.Balances, walletBalanceJSON{
			CurrencyTypeID:     currency.ID,
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
		out.Currencies = append(out.Currencies, currencyJSONFor(&currencies[i]))
	}

	return c.JSON(out)
}

// walletTxJSON is one ledger entry projected for the wallet drawer.
type walletTxJSON struct {
	ID                uint   `json:"id"`
	Delta             int64  `json:"delta"`
	Reason            string `json:"reason"`
	TriggeringEventID *uint  `json:"triggering_event_id,omitempty"`
	TriggeringRuleID  *uint  `json:"triggering_rule_id,omitempty"`
	OccurredAt        string `json:"occurred_at"`
}

type walletTransactionsResponse struct {
	UserID         uint           `json:"user_id"`
	CurrencyTypeID uint           `json:"currency_type_id"`
	Transactions   []walletTxJSON `json:"transactions"`
	TotalCount     int64          `json:"total_count"`
	Page           int            `json:"page"`
	PerPage        int            `json:"per_page"`
}

// ListUserWalletTransactions handles
// GET /api/v1/users/:id/wallet/transactions?currency_type_id=N&page=&per_page=.
//
// Authorization: same self-or-admin rule as GetUserWallet — the caller must
// be the :id user or hold the cached is_admin flag.
//
// currency_type_id is required; the drawer always shows one currency at a
// time. Pagination defaults to page=1, per_page=20 (max 100).
func (h *GamificationHandler) ListUserWalletTransactions(c *fiber.Ctx) error {
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

	currencyTypeRaw := c.Query("currency_type_id")
	if currencyTypeRaw == "" {
		return responses.BadRequest(c, "currency_type_id is required")
	}
	currencyTypeParsed, err := strconv.ParseUint(currencyTypeRaw, 10, 64)
	if err != nil || currencyTypeParsed == 0 {
		return responses.BadRequest(c, "invalid currency_type_id")
	}
	currencyTypeID := uint(currencyTypeParsed)

	page, _ := strconv.Atoi(c.Query("page", "1"))
	if page < 1 {
		page = 1
	}
	perPage, _ := strconv.Atoi(c.Query("per_page", "20"))
	if perPage < 1 {
		perPage = 20
	}
	if perPage > 100 {
		perPage = 100
	}

	result, err := h.walletRepo.ListTransactionsForUserAndCurrency(
		c.Context(),
		targetUserID,
		currencyTypeID,
		repository.PaginationParams{Page: page, PerPage: perPage},
	)
	if err != nil {
		return responses.InternalError(c, "could not fetch wallet transactions")
	}

	out := walletTransactionsResponse{
		UserID:         targetUserID,
		CurrencyTypeID: currencyTypeID,
		Transactions:   make([]walletTxJSON, 0, len(result.Items)),
		TotalCount:     result.TotalCount,
		Page:           result.Page,
		PerPage:        result.PerPage,
	}
	for i := range result.Items {
		tx := &result.Items[i]
		out.Transactions = append(out.Transactions, walletTxJSON{
			ID:                tx.ID,
			Delta:             tx.Delta,
			Reason:            tx.Reason,
			TriggeringEventID: tx.TriggeringEventID,
			TriggeringRuleID:  tx.TriggeringRuleID,
			OccurredAt:        tx.OccurredAt.UTC().Format(time.RFC3339),
		})
	}

	return c.JSON(out)
}

// ---------------------------------------------------------------------------
// Sprint W2-B — Currency CRUD (POST/PATCH/DELETE).
//
// Two URL surfaces sharing the same handler logic:
//
//   Site scope    (admin):
//     POST   /api/v1/gamification/currencies
//     PATCH  /api/v1/gamification/currencies/:id
//     DELETE /api/v1/gamification/currencies/:id
//
//   Course scope (course instructor):
//     POST   /api/v1/courses/:course_id/gamification/currencies
//     PATCH  /api/v1/courses/:course_id/gamification/currencies/:id
//     DELETE /api/v1/courses/:course_id/gamification/currencies/:id
//
// The site routes infer scope from the absence of :course_id; the
// instructor routes infer scope from the presence of :course_id. The
// authorization is enforced by router-level middleware (RequireAdmin /
// RequireInstructor), not re-checked in the handler.
// ---------------------------------------------------------------------------

// codeRE pins the user-defined-currency code shape: lowercase, must start
// with a letter, then [a-z0-9_], 2–32 chars total. Matches the predicate
// engine's resolution scheme (rules reference currencies by code). The
// 2-char minimum accommodates seeded "xp"; "rep" / "gems" / longer
// teacher-defined codes (`coins`, `class_bucks`) all fit comfortably.
var codeRE = regexp.MustCompile(`^[a-z][a-z0-9_]{1,31}$`)

// colorRE accepts a 6-digit hex color or the empty string (which means
// "fall through to the frontend's default chip color").
var colorRE = regexp.MustCompile(`^(#[0-9A-Fa-f]{6})?$`)

// createCurrencyInput is the JSON body for POST. Fields default to
// sensible "non_PII, visible to student, visible in topbar, monotonic"
// values so a teacher who only fills in {code, display_label} gets a
// working currency.
type createCurrencyInput struct {
	Code               string  `json:"code"`
	DisplayLabel       string  `json:"display_label"`
	DisplayLabelPlural string  `json:"display_label_plural"`
	Icon               string  `json:"icon"`
	Color              string  `json:"color"`
	DisplayOrder       int     `json:"display_order"`
	Spendable          bool    `json:"spendable"`
	Monotonic          *bool   `json:"monotonic"`
	VisibleToStudent   *bool   `json:"visible_to_student"`
	VisibleInTopbar    *bool   `json:"visible_in_topbar"`
	Description        string  `json:"description"`
}

// patchCurrencyInput accepts only the fields a teacher/admin is allowed
// to change. Code, scope, system_owned, and tenant are intentionally
// absent — those are immutable post-create. Pointers everywhere so we
// can distinguish "field not provided" from "field set to zero value".
type patchCurrencyInput struct {
	DisplayLabel       *string `json:"display_label"`
	DisplayLabelPlural *string `json:"display_label_plural"`
	Icon               *string `json:"icon"`
	Color              *string `json:"color"`
	DisplayOrder       *int    `json:"display_order"`
	Spendable          *bool   `json:"spendable"`
	Monotonic          *bool   `json:"monotonic"`
	VisibleToStudent   *bool   `json:"visible_to_student"`
	VisibleInTopbar    *bool   `json:"visible_in_topbar"`
	Description        *string `json:"description"`
}

func validateCommonFields(label, color, description string) error {
	if l := strings.TrimSpace(label); l == "" || len(l) > 64 {
		return errors.New("display_label is required, max 64 chars")
	}
	if !colorRE.MatchString(color) {
		return errors.New("color must be a 6-digit hex like #A855F7, or empty")
	}
	if len(description) > 500 {
		return errors.New("description max 500 chars")
	}
	return nil
}

// resolveScope reads :course_id from the URL if present, returning the
// derived (scope_type, scope_id) for the handler. No :course_id → site
// scope keyed to the caller's account.
func resolveScope(c *fiber.Ctx) (models.GamificationScopeType, uint) {
	if cs := c.Params("course_id"); cs != "" {
		if id, err := strconv.ParseUint(cs, 10, 64); err == nil && id > 0 {
			return models.ScopeCourse, uint(id)
		}
	}
	return models.ScopeSite, callerAccountID(c)
}

// currencyJSONFor projects a model row to the same shape as ListCurrencies.
func currencyJSONFor(ct *models.GamificationCurrencyType) currencyJSON {
	return currencyJSON{
		ID:                 ct.ID,
		ScopeType:          string(ct.ScopeType),
		ScopeID:            ct.ScopeID,
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
	}
}

// CreateCurrency handles POST. Always creates with `system_owned=false`
// (the four system rows are reserved for the per-tenant seeder).
// Conflicts on (tenant_id, scope_type, scope_id, code) return 409.
func (h *GamificationHandler) CreateCurrency(c *fiber.Ctx) error {
	var in createCurrencyInput
	if err := c.BodyParser(&in); err != nil {
		return responses.BadRequest(c, "invalid request body")
	}
	in.Code = strings.TrimSpace(in.Code)
	if !codeRE.MatchString(in.Code) {
		return responses.BadRequest(c, "code must match ^[a-z][a-z0-9_]{1,31}$ (lowercase, starts with a letter, 2–32 chars)")
	}
	if err := validateCommonFields(in.DisplayLabel, in.Color, in.Description); err != nil {
		return responses.BadRequest(c, err.Error())
	}

	scopeType, scopeID := resolveScope(c)
	tenantID := callerAccountID(c)

	row := &models.GamificationCurrencyType{
		TenantID:            tenantID,
		ScopeType:           scopeType,
		ScopeID:             scopeID,
		Code:                in.Code,
		DisplayLabel:        strings.TrimSpace(in.DisplayLabel),
		DisplayLabelPlural:  strings.TrimSpace(in.DisplayLabelPlural),
		Icon:                strings.TrimSpace(in.Icon),
		Color:               strings.TrimSpace(in.Color),
		DisplayOrder:        in.DisplayOrder,
		Spendable:           in.Spendable,
		Monotonic:           derefBool(in.Monotonic, true),
		FerpaClassification: "non_PII",
		VisibleToStudent:    derefBool(in.VisibleToStudent, true),
		VisibleInTopbar:     derefBool(in.VisibleInTopbar, true),
		SystemOwned:         false,
		Description:         strings.TrimSpace(in.Description),
	}
	// Duplicate detection is atomic in the repo via ON CONFLICT DO NOTHING
	// + sentinel translation. Avoids the TOCTOU window a "FindByCode then
	// Create" sequence would open under concurrent admin POSTs.
	if err := h.currencyRepo.Create(c.Context(), row); err != nil {
		if errors.Is(err, repository.ErrCurrencyDuplicate) {
			return responses.Error(c, fiber.StatusConflict, "a currency with this code already exists in this scope")
		}
		return responses.InternalError(c, "could not create currency")
	}
	return c.Status(fiber.StatusCreated).JSON(currencyJSONFor(row))
}

// UpdateCurrency handles PATCH. system_owned rows allow every editable
// field except code/scope; non-system rows allow the same set (the code
// is treated as immutable post-create regardless of system_owned status
// because rules reference currencies by code — renaming a code breaks
// every authored rule).
func (h *GamificationHandler) UpdateCurrency(c *fiber.Ctx) error {
	idRaw, err := strconv.ParseUint(c.Params("id"), 10, 64)
	if err != nil {
		return responses.BadRequest(c, "invalid currency id")
	}
	row, err := h.currencyRepo.FindByID(c.Context(), uint(idRaw))
	if err != nil {
		return responses.InternalError(c, "could not load currency")
	}
	if row == nil {
		return responses.NotFound(c, "currency")
	}

	// Scope guard: route-derived scope must match the row's stored scope.
	// Prevents an instructor on course A from PATCHing a currency that
	// lives on course B (or at site scope).
	scopeType, scopeID := resolveScope(c)
	tenantID := callerAccountID(c)
	if row.TenantID != tenantID || row.ScopeType != scopeType || row.ScopeID != scopeID {
		return responses.Error(c, fiber.StatusForbidden, "currency is not in the requested scope")
	}

	var in patchCurrencyInput
	if err := c.BodyParser(&in); err != nil {
		return responses.BadRequest(c, "invalid request body")
	}

	if in.DisplayLabel != nil {
		row.DisplayLabel = strings.TrimSpace(*in.DisplayLabel)
	}
	if in.DisplayLabelPlural != nil {
		row.DisplayLabelPlural = strings.TrimSpace(*in.DisplayLabelPlural)
	}
	if in.Icon != nil {
		row.Icon = strings.TrimSpace(*in.Icon)
	}
	if in.Color != nil {
		row.Color = strings.TrimSpace(*in.Color)
	}
	if in.DisplayOrder != nil {
		row.DisplayOrder = *in.DisplayOrder
	}
	if in.Spendable != nil {
		row.Spendable = *in.Spendable
	}
	if in.Monotonic != nil {
		row.Monotonic = *in.Monotonic
	}
	if in.VisibleToStudent != nil {
		row.VisibleToStudent = *in.VisibleToStudent
	}
	if in.VisibleInTopbar != nil {
		row.VisibleInTopbar = *in.VisibleInTopbar
	}
	if in.Description != nil {
		row.Description = strings.TrimSpace(*in.Description)
	}

	if err := validateCommonFields(row.DisplayLabel, row.Color, row.Description); err != nil {
		return responses.BadRequest(c, err.Error())
	}

	if err := h.currencyRepo.Update(c.Context(), row); err != nil {
		return responses.InternalError(c, "could not update currency")
	}
	return c.JSON(currencyJSONFor(row))
}

// DeleteCurrency handles DELETE. system_owned rows return 409 — those
// codes are referenced by rules and capability unlocks; deleting one
// would silently break authored content. Tenants may rename them via
// PATCH but never remove them.
func (h *GamificationHandler) DeleteCurrency(c *fiber.Ctx) error {
	idRaw, err := strconv.ParseUint(c.Params("id"), 10, 64)
	if err != nil {
		return responses.BadRequest(c, "invalid currency id")
	}
	row, err := h.currencyRepo.FindByID(c.Context(), uint(idRaw))
	if err != nil {
		return responses.InternalError(c, "could not load currency")
	}
	if row == nil {
		return responses.NotFound(c, "currency")
	}

	scopeType, scopeID := resolveScope(c)
	tenantID := callerAccountID(c)
	if row.TenantID != tenantID || row.ScopeType != scopeType || row.ScopeID != scopeID {
		return responses.Error(c, fiber.StatusForbidden, "currency is not in the requested scope")
	}
	if row.SystemOwned {
		return responses.Error(c, fiber.StatusConflict, "system currencies cannot be deleted")
	}
	if err := h.currencyRepo.Delete(c.Context(), row.ID); err != nil {
		return responses.InternalError(c, "could not delete currency")
	}
	return c.SendStatus(fiber.StatusNoContent)
}

func derefBool(p *bool, def bool) bool {
	if p == nil {
		return def
	}
	return *p
}

// ---------------------------------------------------------------------------
// Sprint W2-C — per-learner gamification preferences.
//
// One toggle today: leaderboard_opt_out. The endpoints sit on
// /users/self/* (self-only; we don't expose another user's prefs even
// to admins — those settings belong to the learner). Self-scope is
// enforced by the existing RequireSelfOrAdmin pattern at the route
// layer, but the handler reads `user_id` from Locals directly to
// avoid leaking an admin-override path.
// ---------------------------------------------------------------------------

type gamificationPreferencesJSON struct {
	LeaderboardOptOut bool `json:"leaderboard_opt_out"`
}

// GetMyGamificationPreferences returns the signed-in learner's
// gamification preferences. Always 200 with a stable shape — missing
// users get default values, never 404.
func (h *GamificationHandler) GetMyGamificationPreferences(c *fiber.Ctx) error {
	userID, ok := c.Locals("user_id").(uint)
	if !ok || userID == 0 {
		return responses.Unauthorized(c)
	}
	user, err := h.userRepo.FindByID(c.Context(), userID)
	if err != nil || user == nil {
		return responses.InternalError(c, "could not load preferences")
	}
	return c.JSON(gamificationPreferencesJSON{
		LeaderboardOptOut: user.LeaderboardOptOut,
	})
}

// UpdateMyGamificationPreferences writes the signed-in learner's
// preferences. Uses a pointer-typed PATCH body so omitting a field is
// distinguishable from setting it to its zero value (this is the same
// bool-default lesson the currency editor pinned in W2-B).
//
// Side-effect contract: writing leaderboard_opt_out=true does NOT zero
// the learner's wallet / awards / mastery. SYNTHESIS §5: opting out is
// a visibility control, not an awards reset.
func (h *GamificationHandler) UpdateMyGamificationPreferences(c *fiber.Ctx) error {
	userID, ok := c.Locals("user_id").(uint)
	if !ok || userID == 0 {
		return responses.Unauthorized(c)
	}
	var in struct {
		LeaderboardOptOut *bool `json:"leaderboard_opt_out"`
	}
	if err := c.BodyParser(&in); err != nil {
		return responses.BadRequest(c, "invalid request body")
	}
	user, err := h.userRepo.FindByID(c.Context(), userID)
	if err != nil || user == nil {
		return responses.InternalError(c, "could not load user")
	}
	if in.LeaderboardOptOut != nil {
		user.LeaderboardOptOut = *in.LeaderboardOptOut
	}
	if err := h.userRepo.Update(c.Context(), user); err != nil {
		return responses.InternalError(c, "could not save preferences")
	}
	return c.JSON(gamificationPreferencesJSON{
		LeaderboardOptOut: user.LeaderboardOptOut,
	})
}
