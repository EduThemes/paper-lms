package handlers

import (
	"errors"
	"strconv"
	"time"

	"github.com/gofiber/fiber/v2"

	"github.com/EduThemes/paper-lms/internal/api/v1/responses"
	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/repository"
	"github.com/EduThemes/paper-lms/internal/service/gamification"
)

// GamificationHandler exposes the Phase 6 REST API surface for the
// gamification engine: per-user wallet balances, per-tenant currency
// metadata, badge CRUD, gamification preferences, recipe-builder write API +
// vocabulary discovery, leaderboard surfaces, and pseudonym switching.
//
// Wave 5 refactor (refactor/wave5-gamification-portfolio-services): business
// logic and repo orchestration now live in the gamification service package;
// these handlers parse Fiber input, call the service, and serialize the
// response.
type GamificationHandler struct {
	walletService      *gamification.WalletService
	currencyService    *gamification.CurrencyService
	badgeService       *gamification.BadgeService
	ruleService        *gamification.RuleService
	leaderboardService *gamification.LeaderboardService
}

// NewGamificationHandler wires the handler from repos. Construction stays
// repo-shaped because cmd/server/main.go already builds the repos at the top
// of init; the handler builds its own service composition internally so the
// caller doesn't need to thread five new services through main.
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
		walletService:      gamification.NewWalletService(walletRepo, currencyRepo, userRepo),
		currencyService:    gamification.NewCurrencyService(currencyRepo),
		badgeService:       gamification.NewBadgeService(badgeRepo, badgeAwardRepo),
		ruleService:        gamification.NewRuleService(ruleRepo),
		leaderboardService: gamification.NewLeaderboardService(walletRepo, currencyRepo, userRepo, enrollmentRepo, accountRepo, snapshotRepo),
	}
}

// ---------------------------------------------------------------------------
// JSON projections.
// ---------------------------------------------------------------------------

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

type gamificationPreferencesJSON struct {
	LeaderboardOptOut bool `json:"leaderboard_opt_out"`
}

// ---------------------------------------------------------------------------
// Inputs.
// ---------------------------------------------------------------------------

type createCurrencyInput struct {
	Code               string `json:"code"`
	DisplayLabel       string `json:"display_label"`
	DisplayLabelPlural string `json:"display_label_plural"`
	Icon               string `json:"icon"`
	Color              string `json:"color"`
	DisplayOrder       int    `json:"display_order"`
	Spendable          bool   `json:"spendable"`
	Monotonic          *bool  `json:"monotonic"`
	VisibleToStudent   *bool  `json:"visible_to_student"`
	VisibleInTopbar    *bool  `json:"visible_in_topbar"`
	Description        string `json:"description"`
}

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

type createBadgeInput struct {
	Code          string `json:"code"`
	Name          string `json:"name"`
	Description   string `json:"description"`
	Icon          string `json:"icon"`
	ImageURL      string `json:"image_url"`
	Color         string `json:"color"`
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

// ---------------------------------------------------------------------------
// Helpers shared with rules / leaderboards handler files.
// ---------------------------------------------------------------------------

// resolveScope reads :course_id from the URL if present.
func resolveScope(c *fiber.Ctx) (models.GamificationScopeType, uint) {
	if cs := c.Params("course_id"); cs != "" {
		if id, err := strconv.ParseUint(cs, 10, 64); err == nil && id > 0 {
			return models.ScopeCourse, uint(id)
		}
	}
	return models.ScopeSite, callerAccountID(c)
}

func derefBool(p *bool, def bool) bool {
	if p == nil {
		return def
	}
	return *p
}

// mapCurrencyServiceError maps service-layer sentinels to Fiber responses.
// Returns nil if err is nil; the (wrote, fiberErr) shape lets callers
// short-circuit cleanly.
func mapCurrencyServiceError(c *fiber.Ctx, err error) error {
	switch {
	case errors.Is(err, gamification.ErrCurrencyNotFound):
		return responses.NotFound(c, "currency")
	case errors.Is(err, gamification.ErrCurrencyOutOfScope):
		// 13.1.E: existence leak — return 404 not 403 on cross-tenant
		// (and on cross-scope; either reveals the row exists in some
		// scope the caller isn't entitled to know about).
		return responses.NotFound(c, "currency")
	case errors.Is(err, gamification.ErrSystemCurrencyImmutable):
		return responses.Error(c, fiber.StatusConflict, "system currencies cannot be deleted")
	case errors.Is(err, gamification.ErrInvalidCurrencyCode),
		errors.Is(err, gamification.ErrInvalidColor),
		errors.Is(err, gamification.ErrInvalidLabel),
		errors.Is(err, gamification.ErrInvalidDescription):
		return responses.BadRequest(c, err.Error())
	case errors.Is(err, repository.ErrCurrencyDuplicate):
		return responses.Error(c, fiber.StatusConflict, "a currency with this code already exists in this scope")
	default:
		return responses.InternalError(c, "currency operation failed")
	}
}

func mapBadgeServiceError(c *fiber.Ctx, err error) error {
	switch {
	case errors.Is(err, gamification.ErrBadgeNotFound):
		return responses.NotFound(c, "badge")
	case errors.Is(err, gamification.ErrBadgeOutOfScope):
		// 13.1.E: existence leak — return 404 not 403 on cross-tenant.
		return responses.NotFound(c, "badge")
	case errors.Is(err, gamification.ErrSystemBadgeImmutable):
		return responses.Error(c, fiber.StatusConflict, "system badges cannot be deleted")
	case errors.Is(err, gamification.ErrBadgeWrongTenant):
		// 13.1.E: existence leak — return 404 not 403 on cross-tenant.
		return responses.NotFound(c, "badge")
	case errors.Is(err, gamification.ErrInvalidBadgeName),
		errors.Is(err, gamification.ErrInvalidCurrencyCode),
		errors.Is(err, gamification.ErrInvalidColor),
		errors.Is(err, gamification.ErrInvalidDescription):
		return responses.BadRequest(c, err.Error())
	case errors.Is(err, repository.ErrBadgeDuplicate):
		return responses.Error(c, fiber.StatusConflict, "a badge with this code already exists in this scope")
	default:
		return responses.InternalError(c, "badge operation failed")
	}
}

// ---------------------------------------------------------------------------
// Wallet handlers.
// ---------------------------------------------------------------------------

// GetUserWallet handles GET /api/v1/users/:id/wallet.
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

	rows, err := h.walletService.GetUserWallet(c.Context(), targetUserID)
	if err != nil {
		return responses.InternalError(c, "could not fetch wallet balances")
	}

	out := userWalletResponse{
		UserID:   targetUserID,
		Balances: make([]walletBalanceJSON, 0, len(rows)),
	}
	for _, row := range rows {
		b := row.Balance
		if row.Currency == nil {
			// Stale balance row pointing at a deleted currency: surface a
			// minimal entry so the frontend doesn't choke.
			out.Balances = append(out.Balances, walletBalanceJSON{
				CurrencyTypeID: b.CurrencyTypeID,
				Balance:        b.Balance,
				LifetimeEarned: b.LifetimeEarned,
			})
			continue
		}
		currency := row.Currency
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

// ListUserWalletTransactions handles
// GET /api/v1/users/:id/wallet/transactions?currency_type_id=N&page=&per_page=.
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

	result, err := h.walletService.ListTransactions(c.Context(), targetUserID, currencyTypeID, repository.PaginationParams{Page: page, PerPage: perPage})
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
// Currency handlers.
// ---------------------------------------------------------------------------

// ListCurrencies handles GET /api/v1/gamification/currencies.
func (h *GamificationHandler) ListCurrencies(c *fiber.Ctx) error {
	if _, ok := c.Locals("user_id").(uint); !ok {
		return responses.Unauthorized(c)
	}
	currencies, err := h.currencyService.List(c.Context(), callerAccountID(c), c.Query("topbar_only") == "true")
	if err != nil {
		return responses.InternalError(c, "could not fetch currencies")
	}
	out := listCurrenciesResponse{Currencies: make([]currencyJSON, 0, len(currencies))}
	for i := range currencies {
		out.Currencies = append(out.Currencies, currencyJSONFor(&currencies[i]))
	}
	return c.JSON(out)
}

// CreateCurrency handles POST. Always creates with system_owned=false.
func (h *GamificationHandler) CreateCurrency(c *fiber.Ctx) error {
	var in createCurrencyInput
	if err := c.BodyParser(&in); err != nil {
		return responses.BadRequest(c, "invalid request body")
	}
	scopeType, scopeID := resolveScope(c)
	row, err := h.currencyService.Create(c.Context(), callerAccountID(c), scopeType, scopeID, gamification.CurrencyCreateInput{
		Code:               in.Code,
		DisplayLabel:       in.DisplayLabel,
		DisplayLabelPlural: in.DisplayLabelPlural,
		Icon:               in.Icon,
		Color:              in.Color,
		DisplayOrder:       in.DisplayOrder,
		Spendable:          in.Spendable,
		Monotonic:          derefBool(in.Monotonic, true),
		VisibleToStudent:   derefBool(in.VisibleToStudent, true),
		VisibleInTopbar:    derefBool(in.VisibleInTopbar, true),
		Description:        in.Description,
	})
	if err != nil {
		return mapCurrencyServiceError(c, err)
	}
	return c.Status(fiber.StatusCreated).JSON(currencyJSONFor(row))
}

// UpdateCurrency handles PATCH.
func (h *GamificationHandler) UpdateCurrency(c *fiber.Ctx) error {
	idRaw, err := strconv.ParseUint(c.Params("id"), 10, 64)
	if err != nil {
		return responses.BadRequest(c, "invalid currency id")
	}
	var in patchCurrencyInput
	if err := c.BodyParser(&in); err != nil {
		return responses.BadRequest(c, "invalid request body")
	}
	scopeType, scopeID := resolveScope(c)
	row, err := h.currencyService.Update(c.Context(), uint(idRaw), callerAccountID(c), scopeType, scopeID, gamification.CurrencyPatchInput{
		DisplayLabel:       in.DisplayLabel,
		DisplayLabelPlural: in.DisplayLabelPlural,
		Icon:               in.Icon,
		Color:              in.Color,
		DisplayOrder:       in.DisplayOrder,
		Spendable:          in.Spendable,
		Monotonic:          in.Monotonic,
		VisibleToStudent:   in.VisibleToStudent,
		VisibleInTopbar:    in.VisibleInTopbar,
		Description:        in.Description,
	})
	if err != nil {
		return mapCurrencyServiceError(c, err)
	}
	return c.JSON(currencyJSONFor(row))
}

// DeleteCurrency handles DELETE.
func (h *GamificationHandler) DeleteCurrency(c *fiber.Ctx) error {
	idRaw, err := strconv.ParseUint(c.Params("id"), 10, 64)
	if err != nil {
		return responses.BadRequest(c, "invalid currency id")
	}
	scopeType, scopeID := resolveScope(c)
	if err := h.currencyService.Delete(c.Context(), uint(idRaw), callerAccountID(c), scopeType, scopeID); err != nil {
		return mapCurrencyServiceError(c, err)
	}
	return c.SendStatus(fiber.StatusNoContent)
}

// ---------------------------------------------------------------------------
// Preferences handlers.
// ---------------------------------------------------------------------------

func (h *GamificationHandler) GetMyGamificationPreferences(c *fiber.Ctx) error {
	userID, ok := c.Locals("user_id").(uint)
	if !ok || userID == 0 {
		return responses.Unauthorized(c)
	}
	optOut, err := h.walletService.GetPreferences(c.Context(), userID)
	if err != nil {
		return responses.InternalError(c, "could not load preferences")
	}
	return c.JSON(gamificationPreferencesJSON{LeaderboardOptOut: optOut})
}

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
	optOut, err := h.walletService.UpdatePreferences(c.Context(), userID, in.LeaderboardOptOut)
	if err != nil {
		if errors.Is(err, gamification.ErrUserNotFound) {
			return responses.InternalError(c, "could not load user")
		}
		return responses.InternalError(c, "could not save preferences")
	}
	return c.JSON(gamificationPreferencesJSON{LeaderboardOptOut: optOut})
}

// ---------------------------------------------------------------------------
// Badge handlers.
// ---------------------------------------------------------------------------

func (h *GamificationHandler) ListBadges(c *fiber.Ctx) error {
	if _, ok := c.Locals("user_id").(uint); !ok {
		return responses.Unauthorized(c)
	}
	badges, err := h.badgeService.List(c.Context(), callerAccountID(c))
	if err != nil {
		return responses.InternalError(c, "could not fetch badges")
	}
	out := listBadgesResponse{Badges: make([]badgeJSON, 0, len(badges))}
	for i := range badges {
		out.Badges = append(out.Badges, badgeJSONFor(&badges[i]))
	}
	return c.JSON(out)
}

func (h *GamificationHandler) CreateBadge(c *fiber.Ctx) error {
	var in createBadgeInput
	if err := c.BodyParser(&in); err != nil {
		return responses.BadRequest(c, "invalid request body")
	}
	scopeType, scopeID := resolveScope(c)
	creatorID, _ := c.Locals("user_id").(uint)
	row, err := h.badgeService.Create(c.Context(), callerAccountID(c), scopeType, scopeID, creatorID, gamification.BadgeCreateInput{
		Code:          in.Code,
		Name:          in.Name,
		Description:   in.Description,
		Icon:          in.Icon,
		ImageURL:      in.ImageURL,
		Color:         in.Color,
		InternalOnly:  derefBool(in.InternalOnly, true),
		AudienceLevel: in.AudienceLevel,
	})
	if err != nil {
		return mapBadgeServiceError(c, err)
	}
	return c.Status(fiber.StatusCreated).JSON(badgeJSONFor(row))
}

func (h *GamificationHandler) UpdateBadge(c *fiber.Ctx) error {
	idRaw, err := strconv.ParseUint(c.Params("id"), 10, 64)
	if err != nil {
		return responses.BadRequest(c, "invalid badge id")
	}
	var in patchBadgeInput
	if err := c.BodyParser(&in); err != nil {
		return responses.BadRequest(c, "invalid request body")
	}
	scopeType, scopeID := resolveScope(c)
	row, err := h.badgeService.Update(c.Context(), uint(idRaw), callerAccountID(c), scopeType, scopeID, gamification.BadgePatchInput{
		Name:          in.Name,
		Description:   in.Description,
		Icon:          in.Icon,
		ImageURL:      in.ImageURL,
		Color:         in.Color,
		InternalOnly:  in.InternalOnly,
		AudienceLevel: in.AudienceLevel,
	})
	if err != nil {
		return mapBadgeServiceError(c, err)
	}
	return c.JSON(badgeJSONFor(row))
}

func (h *GamificationHandler) DeleteBadge(c *fiber.Ctx) error {
	idRaw, err := strconv.ParseUint(c.Params("id"), 10, 64)
	if err != nil {
		return responses.BadRequest(c, "invalid badge id")
	}
	scopeType, scopeID := resolveScope(c)
	if err := h.badgeService.Delete(c.Context(), uint(idRaw), callerAccountID(c), scopeType, scopeID); err != nil {
		return mapBadgeServiceError(c, err)
	}
	return c.SendStatus(fiber.StatusNoContent)
}

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

	awards, err := h.badgeService.ListAwardsForUser(c.Context(), targetUserID)
	if err != nil {
		return responses.InternalError(c, "could not fetch badges")
	}
	out := make([]earnedBadgeJSON, 0, len(awards))
	for i := range awards {
		a := &awards[i]
		badge, _ := h.badgeService.FindByID(c.Context(), a.BadgeID)
		if badge == nil {
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

	awarderID, _ := c.Locals("user_id").(uint)
	outcome, err := h.badgeService.AwardToUser(c.Context(), targetUserID, in.BadgeID, awarderID, callerAccountID(c))
	if err != nil {
		return mapBadgeServiceError(c, err)
	}
	status := fiber.StatusCreated
	if !outcome.Created {
		status = fiber.StatusOK
	}
	return c.Status(status).JSON(fiber.Map{
		"award_id": outcome.Award.ID,
		"badge_id": outcome.Badge.ID,
		"user_id":  targetUserID,
		"created":  outcome.Created,
	})
}

func (h *GamificationHandler) RevokeBadgeFromUser(c *fiber.Ctx) error {
	uidRaw, err := strconv.ParseUint(c.Params("user_id"), 10, 64)
	if err != nil {
		return responses.BadRequest(c, "invalid user_id")
	}
	bidRaw, err := strconv.ParseUint(c.Params("badge_id"), 10, 64)
	if err != nil {
		return responses.BadRequest(c, "invalid badge_id")
	}
	if err := h.badgeService.Revoke(c.Context(), uint(uidRaw), uint(bidRaw)); err != nil {
		return responses.InternalError(c, "could not revoke badge")
	}
	return c.SendStatus(fiber.StatusNoContent)
}

// ---------------------------------------------------------------------------
// Vocabulary discovery (W2-E.1).
// ---------------------------------------------------------------------------

type vocabularyResponse struct {
	Triggers      []gamification.KindSpec `json:"triggers"`
	Predicates    []gamification.KindSpec `json:"predicates"`
	Effects       []gamification.KindSpec `json:"effects"`
	SetOps        []string                `json:"set_ops"`
	Audiences     []string                `json:"audiences"`
	Scopes        []string                `json:"scopes"`
	Windows       []string                `json:"windows"`
	MasteryLevels []string                `json:"mastery_levels"`
}

func (h *GamificationHandler) GetVocabulary(c *fiber.Ctx) error {
	return c.JSON(vocabularyResponse{
		Triggers:      gamification.TriggerCatalog,
		Predicates:    gamification.PredicateCatalog,
		Effects:       gamification.EffectCatalog,
		SetOps:        gamification.SetOps,
		Audiences:     gamification.AudienceLevels,
		Scopes:        gamification.ScopeTypes,
		Windows:       gamification.WindowKinds,
		MasteryLevels: gamification.MasteryLevels,
	})
}

