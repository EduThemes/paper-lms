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

// GamificationHandler exposes the Phase 6 REST API surface for the
// gamification engine: per-user wallet balances, per-tenant currency
// metadata, badge CRUD, gamification preferences, and (W2-E.1) the
// recipe-builder write API + vocabulary discovery endpoint. Write paths
// for *runtime* rule firing (AwardCurrency, AwardBadge) live in the
// dispatcher / effect package; these handlers are for *authoring* the
// rules themselves.
type GamificationHandler struct {
	walletRepo     repository.GamificationWalletRepository
	currencyRepo   repository.GamificationCurrencyTypeRepository
	userRepo       repository.UserRepository
	badgeRepo      repository.GamificationBadgeRepository
	badgeAwardRepo repository.GamificationBadgeAwardRepository
	ruleRepo       repository.GamificationRuleRepository
	enrollmentRepo repository.EnrollmentRepository
	accountRepo    repository.AccountRepository
	snapshotRepo   repository.GamificationLeaderboardSnapshotRepository
}

// NewGamificationHandler wires the handlers.
//
//   - userRepo: W2-C (leaderboard opt-out toggle lives on users.leaderboard_opt_out)
//   - badgeRepo + badgeAwardRepo: W2-D (badge CRUD + manual award)
//   - ruleRepo: W2-E.1 (recipe-builder CRUD)
//   - enrollmentRepo: W3-A (course-scoped leaderboard candidate set)
//   - accountRepo: W3-B (tenant_mode lookup drives pseudonym + top-N policy)
//   - snapshotRepo: 7-B (?offset_weeks=N reads from gamification_leaderboard_snapshots)
func NewGamificationHandler(
	walletRepo repository.GamificationWalletRepository,
	currencyRepo repository.GamificationCurrencyTypeRepository,
	userRepo repository.UserRepository,
	badgeRepo repository.GamificationBadgeRepository,
	badgeAwardRepo repository.GamificationBadgeAwardRepository,
	ruleRepo repository.GamificationRuleRepository,
	enrollmentRepo repository.EnrollmentRepository,
	accountRepo repository.AccountRepository,
	snapshotRepo repository.GamificationLeaderboardSnapshotRepository,
) *GamificationHandler {
	return &GamificationHandler{
		walletRepo:     walletRepo,
		currencyRepo:   currencyRepo,
		userRepo:       userRepo,
		badgeRepo:      badgeRepo,
		badgeAwardRepo: badgeAwardRepo,
		ruleRepo:       ruleRepo,
		enrollmentRepo: enrollmentRepo,
		accountRepo:    accountRepo,
		snapshotRepo:   snapshotRepo,
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
	// 13.1.E: existence leak — return 404 not 403 on cross-tenant (and
	// on cross-scope; either reveals the row exists in some scope the
	// caller isn't entitled to know about).
	scopeType, scopeID := resolveScope(c)
	tenantID := callerAccountID(c)
	if row.TenantID != tenantID || row.ScopeType != scopeType || row.ScopeID != scopeID {
		return responses.NotFound(c, "currency")
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
	// 13.1.E: existence leak — return 404 not 403 on cross-tenant.
	if row.TenantID != tenantID || row.ScopeType != scopeType || row.ScopeID != scopeID {
		return responses.NotFound(c, "currency")
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

// parseAudienceLevel normalizes user input to a typed GamificationAudience.
// Empty / whitespace-only input is treated as "no audience set" (nil).
// Invalid values are also normalized to nil — the DB enum CHECK is the
// last line of defense; this layer just keeps obviously bad input out
// of the round-trip. Closes F1.11 (post-migration-000050 alignment).
func parseAudienceLevel(raw string) *models.GamificationAudience {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return nil
	}
	a := models.GamificationAudience(trimmed)
	switch a {
	case models.AudienceK5,
		models.AudienceM68,
		models.AudienceH912,
		models.AudienceHigherEd,
		models.AudienceCorp,
		models.AudiencePro:
		return &a
	}
	return nil
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

// ---------------------------------------------------------------------------
// Sprint W2-D — Badge CRUD + admin manual-award + self list.
//
// URL surfaces mirror the W2-B currency CRUD pattern:
//
//   Site scope (admin):
//     GET    /api/v1/gamification/badges
//     POST   /api/v1/gamification/badges
//     PATCH  /api/v1/gamification/badges/:id
//     DELETE /api/v1/gamification/badges/:id
//
//   Course scope (instructor):
//     POST/PATCH/DELETE /api/v1/courses/:course_id/gamification/badges[/:id]
//
//   Per-user (self-or-admin):
//     GET    /api/v1/users/:id/badges
//
//   Manual award + revoke (admin or instructor):
//     POST   /api/v1/users/:user_id/badges       — body {badge_id, evidence?}
//     DELETE /api/v1/users/:user_id/badges/:badge_id
//
// The site/course scope split + system_owned + code-immutability rules
// are inherited verbatim from W2-B's currency surface.
// ---------------------------------------------------------------------------

type badgeJSON struct {
	ID            uint   `json:"id"`
	ScopeType     string `json:"scope_type"`
	ScopeID       uint   `json:"scope_id"`
	Code          string `json:"code"`
	Name          string `json:"name"`
	Description   string `json:"description"`
	Icon          string `json:"icon"`
	ImageURL      string `json:"image_url"`
	Color         string `json:"color"`
	InternalOnly  bool   `json:"internal_only"`
	SystemOwned   bool   `json:"system_owned"`
	AudienceLevel string `json:"audience_level"`
}

func badgeJSONFor(b *models.GamificationBadge) badgeJSON {
	audience := ""
	if b.AudienceLevel != nil {
		audience = string(*b.AudienceLevel)
	}
	return badgeJSON{
		ID:            b.ID,
		ScopeType:     string(b.ScopeType),
		ScopeID:       b.ScopeID,
		Code:          b.Code,
		Name:          b.Name,
		Description:   b.Description,
		Icon:          b.Icon,
		ImageURL:      b.ImageURL,
		Color:         b.Color,
		InternalOnly:  b.InternalOnly,
		SystemOwned:   b.SystemOwned,
		AudienceLevel: audience,
	}
}

type listBadgesResponse struct {
	Badges []badgeJSON `json:"badges"`
}

type createBadgeInput struct {
	Code          string `json:"code"`
	Name          string `json:"name"`
	Description   string `json:"description"`
	Icon          string `json:"icon"`
	ImageURL      string `json:"image_url"`
	Color         string `json:"color"`
	// InternalOnly defaults to true if omitted (SYNTHESIS §5: K-12 has
	// no production OB 3.0 wallet for under-13, so badges stay
	// internal). Tenants can flip per-badge in a future PATCH once OB
	// 3.0 export ships.
	InternalOnly  *bool  `json:"internal_only"`
	AudienceLevel string `json:"audience_level"`
}

type patchBadgeInput struct {
	Name          *string `json:"name"`
	Description   *string `json:"description"`
	Icon          *string `json:"icon"`
	ImageURL      *string `json:"image_url"`
	Color         *string `json:"color"`
	InternalOnly  *bool   `json:"internal_only"`
	AudienceLevel *string `json:"audience_level"`
}

// ListBadges handles GET /api/v1/gamification/badges. Any authenticated
// user — badge definitions are tenant-wide metadata, not FERPA-
// protected per-user data. Listing earned badges is a separate endpoint.
func (h *GamificationHandler) ListBadges(c *fiber.Ctx) error {
	if _, ok := c.Locals("user_id").(uint); !ok {
		return responses.Unauthorized(c)
	}
	tenantID := callerAccountID(c)
	badges, err := h.badgeRepo.ListByTenant(c.Context(), tenantID)
	if err != nil {
		return responses.InternalError(c, "could not fetch badges")
	}
	out := listBadgesResponse{Badges: make([]badgeJSON, 0, len(badges))}
	for i := range badges {
		out.Badges = append(out.Badges, badgeJSONFor(&badges[i]))
	}
	return c.JSON(out)
}

// CreateBadge handles POST. Always system_owned=false. Conflicts on
// (tenant_id, scope_type, scope_id, code) return 409 atomically via the
// repo's ON CONFLICT DO NOTHING translation.
func (h *GamificationHandler) CreateBadge(c *fiber.Ctx) error {
	var in createBadgeInput
	if err := c.BodyParser(&in); err != nil {
		return responses.BadRequest(c, "invalid request body")
	}
	in.Code = strings.TrimSpace(in.Code)
	if !codeRE.MatchString(in.Code) {
		return responses.BadRequest(c, "code must match ^[a-z][a-z0-9_]{1,31}$ (lowercase, starts with a letter, 2–32 chars)")
	}
	if l := strings.TrimSpace(in.Name); l == "" || len(l) > 80 {
		return responses.BadRequest(c, "name is required, max 80 chars")
	}
	if !colorRE.MatchString(in.Color) {
		return responses.BadRequest(c, "color must be a 6-digit hex like #A855F7, or empty")
	}
	if len(in.Description) > 500 {
		return responses.BadRequest(c, "description max 500 chars")
	}

	scopeType, scopeID := resolveScope(c)
	tenantID := callerAccountID(c)
	creatorID, _ := c.Locals("user_id").(uint)
	var createdBy *uint
	if creatorID > 0 {
		createdBy = &creatorID
	}

	audience := parseAudienceLevel(in.AudienceLevel)
	row := &models.GamificationBadge{
		TenantID:      tenantID,
		ScopeType:     scopeType,
		ScopeID:       scopeID,
		Code:          in.Code,
		Name:          strings.TrimSpace(in.Name),
		Description:   strings.TrimSpace(in.Description),
		Icon:          strings.TrimSpace(in.Icon),
		ImageURL:      strings.TrimSpace(in.ImageURL),
		Color:         strings.TrimSpace(in.Color),
		InternalOnly:  derefBool(in.InternalOnly, true),
		SystemOwned:   false,
		AudienceLevel: audience,
		CreatedBy:     createdBy,
	}
	if err := h.badgeRepo.Create(c.Context(), row); err != nil {
		if errors.Is(err, repository.ErrBadgeDuplicate) {
			return responses.Error(c, fiber.StatusConflict, "a badge with this code already exists in this scope")
		}
		return responses.InternalError(c, "could not create badge")
	}
	return c.Status(fiber.StatusCreated).JSON(badgeJSONFor(row))
}

// UpdateBadge handles PATCH. Code, scope, system_owned are immutable
// (rules reference badges by code; renaming breaks authored content).
func (h *GamificationHandler) UpdateBadge(c *fiber.Ctx) error {
	idRaw, err := strconv.ParseUint(c.Params("id"), 10, 64)
	if err != nil {
		return responses.BadRequest(c, "invalid badge id")
	}
	row, err := h.badgeRepo.FindByID(c.Context(), uint(idRaw))
	if err != nil {
		return responses.InternalError(c, "could not load badge")
	}
	if row == nil {
		return responses.NotFound(c, "badge")
	}

	scopeType, scopeID := resolveScope(c)
	tenantID := callerAccountID(c)
	// 13.1.E: existence leak — return 404 not 403 on cross-tenant.
	if row.TenantID != tenantID || row.ScopeType != scopeType || row.ScopeID != scopeID {
		return responses.NotFound(c, "badge")
	}

	var in patchBadgeInput
	if err := c.BodyParser(&in); err != nil {
		return responses.BadRequest(c, "invalid request body")
	}
	if in.Name != nil {
		row.Name = strings.TrimSpace(*in.Name)
	}
	if in.Description != nil {
		row.Description = strings.TrimSpace(*in.Description)
	}
	if in.Icon != nil {
		row.Icon = strings.TrimSpace(*in.Icon)
	}
	if in.ImageURL != nil {
		row.ImageURL = strings.TrimSpace(*in.ImageURL)
	}
	if in.Color != nil {
		row.Color = strings.TrimSpace(*in.Color)
	}
	if in.InternalOnly != nil {
		row.InternalOnly = *in.InternalOnly
	}
	if in.AudienceLevel != nil {
		row.AudienceLevel = parseAudienceLevel(*in.AudienceLevel)
	}
	if row.Name == "" || len(row.Name) > 80 {
		return responses.BadRequest(c, "name is required, max 80 chars")
	}
	if !colorRE.MatchString(row.Color) {
		return responses.BadRequest(c, "color must be a 6-digit hex or empty")
	}
	if len(row.Description) > 500 {
		return responses.BadRequest(c, "description max 500 chars")
	}
	if err := h.badgeRepo.Update(c.Context(), row); err != nil {
		return responses.InternalError(c, "could not update badge")
	}
	return c.JSON(badgeJSONFor(row))
}

// DeleteBadge handles DELETE. system_owned returns 409. ON DELETE
// CASCADE on gamification_badge_awards.badge_id means existing
// awards of this badge are also wiped — the client should surface a
// confirmation before calling.
func (h *GamificationHandler) DeleteBadge(c *fiber.Ctx) error {
	idRaw, err := strconv.ParseUint(c.Params("id"), 10, 64)
	if err != nil {
		return responses.BadRequest(c, "invalid badge id")
	}
	row, err := h.badgeRepo.FindByID(c.Context(), uint(idRaw))
	if err != nil {
		return responses.InternalError(c, "could not load badge")
	}
	if row == nil {
		return responses.NotFound(c, "badge")
	}
	scopeType, scopeID := resolveScope(c)
	tenantID := callerAccountID(c)
	// 13.1.E: existence leak — return 404 not 403 on cross-tenant.
	if row.TenantID != tenantID || row.ScopeType != scopeType || row.ScopeID != scopeID {
		return responses.NotFound(c, "badge")
	}
	if row.SystemOwned {
		return responses.Error(c, fiber.StatusConflict, "system badges cannot be deleted")
	}
	if err := h.badgeRepo.Delete(c.Context(), row.ID); err != nil {
		return responses.InternalError(c, "could not delete badge")
	}
	return c.SendStatus(fiber.StatusNoContent)
}

// earnedBadgeJSON is the projection returned by the per-user list — a
// flattened join of badge_award + badge metadata so the frontend renders
// the grid without a follow-up fetch.
type earnedBadgeJSON struct {
	AwardID         uint   `json:"award_id"`
	AwardedAt       string `json:"awarded_at"`
	EvidenceEventID *uint  `json:"evidence_event_id,omitempty"`
	AwardedBy       *uint  `json:"awarded_by,omitempty"`
	BadgeID         uint   `json:"badge_id"`
	Code            string `json:"code"`
	Name            string `json:"name"`
	Description     string `json:"description"`
	Icon            string `json:"icon"`
	ImageURL        string `json:"image_url"`
	Color           string `json:"color"`
}

// ListUserBadges handles GET /api/v1/users/:id/badges. Self-or-admin.
// Returns earned badges joined with badge metadata, most recent first.
// Empty array (200) for users with no awards.
func (h *GamificationHandler) ListUserBadges(c *fiber.Ctx) error {
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
		return responses.Error(c, fiber.StatusForbidden, "you can only view your own badges")
	}

	awards, err := h.badgeAwardRepo.ListForUser(c.Context(), targetUserID)
	if err != nil {
		return responses.InternalError(c, "could not fetch badges")
	}

	out := make([]earnedBadgeJSON, 0, len(awards))
	for i := range awards {
		a := &awards[i]
		badge, ferr := h.badgeRepo.FindByID(c.Context(), a.BadgeID)
		if ferr != nil || badge == nil {
			// Award row points at a deleted badge — minimal entry so the
			// frontend doesn't choke. ON DELETE CASCADE wipes these, so
			// this branch is mostly defensive.
			out = append(out, earnedBadgeJSON{
				AwardID:         a.ID,
				AwardedAt:       a.AwardedAt.UTC().Format(time.RFC3339),
				EvidenceEventID: a.EvidenceEventID,
				AwardedBy:       a.AwardedBy,
				BadgeID:         a.BadgeID,
			})
			continue
		}
		out = append(out, earnedBadgeJSON{
			AwardID:         a.ID,
			AwardedAt:       a.AwardedAt.UTC().Format(time.RFC3339),
			EvidenceEventID: a.EvidenceEventID,
			AwardedBy:       a.AwardedBy,
			BadgeID:         badge.ID,
			Code:            badge.Code,
			Name:            badge.Name,
			Description:     badge.Description,
			Icon:            badge.Icon,
			ImageURL:        badge.ImageURL,
			Color:           badge.Color,
		})
	}
	return c.JSON(fiber.Map{"user_id": targetUserID, "badges": out})
}

// AwardBadgeToUser handles POST /api/v1/users/:user_id/badges. Admin /
// instructor only (router-level middleware). Body: {badge_id}. Idempotent
// — re-awarding the same badge is a 200 with created=false.
//
// Wave 2 surface is intentionally minimal: admins manually grant by ID.
// Wave 2 W2-E lets rule authors use the AwardBadge effect; once that
// ships, this endpoint stays as the manual-grant fallback.
func (h *GamificationHandler) AwardBadgeToUser(c *fiber.Ctx) error {
	pathID, err := strconv.ParseUint(c.Params("user_id"), 10, 64)
	if err != nil {
		return responses.BadRequest(c, "invalid user_id")
	}
	targetUserID := uint(pathID)

	var in struct {
		BadgeID uint `json:"badge_id"`
	}
	if err := c.BodyParser(&in); err != nil {
		return responses.BadRequest(c, "invalid request body")
	}
	if in.BadgeID == 0 {
		return responses.BadRequest(c, "badge_id is required")
	}

	badge, err := h.badgeRepo.FindByID(c.Context(), in.BadgeID)
	if err != nil {
		return responses.InternalError(c, "could not load badge")
	}
	if badge == nil {
		return responses.NotFound(c, "badge")
	}
	// Tenant guard — admins can only award badges that belong to their
	// tenant. (The middleware-set is_admin doesn't carry tenant info
	// yet; this check uses callerAccountID for the same single-tenant
	// fallback the rest of W2-B/C uses.)
	// 13.1.E: existence leak — return 404 not 403 on cross-tenant.
	if badge.TenantID != callerAccountID(c) {
		return responses.NotFound(c, "badge")
	}

	awarderID, _ := c.Locals("user_id").(uint)
	award := &models.GamificationBadgeAward{
		UserID:    targetUserID,
		BadgeID:   badge.ID,
		AwardedBy: &awarderID,
	}
	created, err := h.badgeAwardRepo.Award(c.Context(), award)
	if err != nil {
		return responses.InternalError(c, "could not award badge")
	}
	status := fiber.StatusCreated
	if !created {
		status = fiber.StatusOK // idempotent — already held
	}
	return c.Status(status).JSON(fiber.Map{
		"award_id":  award.ID,
		"badge_id":  badge.ID,
		"user_id":   targetUserID,
		"created":   created,
	})
}

// RevokeBadgeFromUser handles DELETE /api/v1/users/:user_id/badges/:badge_id.
// Admin / instructor only (router-level middleware). Idempotent — the
// repo's Revoke returns nil when no award exists.
func (h *GamificationHandler) RevokeBadgeFromUser(c *fiber.Ctx) error {
	uidRaw, err := strconv.ParseUint(c.Params("user_id"), 10, 64)
	if err != nil {
		return responses.BadRequest(c, "invalid user_id")
	}
	bidRaw, err := strconv.ParseUint(c.Params("badge_id"), 10, 64)
	if err != nil {
		return responses.BadRequest(c, "invalid badge_id")
	}
	if err := h.badgeAwardRepo.Revoke(c.Context(), uint(uidRaw), uint(bidRaw)); err != nil {
		return responses.InternalError(c, "could not revoke badge")
	}
	return c.SendStatus(fiber.StatusNoContent)
}
