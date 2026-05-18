package handlers

import (
	"encoding/json"
	"errors"
	"strconv"

	"github.com/gofiber/fiber/v2"

	"github.com/EduThemes/paper-lms/internal/api/v1/responses"
	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/repository"
	"github.com/EduThemes/paper-lms/internal/service/gamification"
)

// ruleJSON is the read projection a recipe-builder UI consumes.
type ruleJSON struct {
	ID              uint            `json:"id"`
	TenantID        uint            `json:"tenant_id"`
	ScopeType       string          `json:"scope_type"`
	ScopeID         uint            `json:"scope_id"`
	AudienceLevel   string          `json:"audience_level"`
	Name            string          `json:"name"`
	Description     string          `json:"description"`
	Enabled         bool            `json:"enabled"`
	TriggerEvent    json.RawMessage `json:"trigger_event"`
	ConditionSet    json.RawMessage `json:"condition_set"`
	Effects         json.RawMessage `json:"effects"`
	CooldownSeconds *int            `json:"cooldown_seconds,omitempty"`
	MaxPerWindow    json.RawMessage `json:"max_per_window,omitempty"`
	CreatedBy       *uint           `json:"created_by,omitempty"`
	CreatedAt       string          `json:"created_at"`
	UpdatedAt       string          `json:"updated_at"`
}

func ruleJSONFor(r *models.GamificationRule) ruleJSON {
	out := ruleJSON{
		ID:              r.ID,
		TenantID:        r.TenantID,
		ScopeType:       string(r.ScopeType),
		ScopeID:         r.ScopeID,
		AudienceLevel:   string(r.AudienceLevel),
		Name:            r.Name,
		Description:     r.Description,
		Enabled:         r.Enabled,
		TriggerEvent:    json.RawMessage(r.TriggerEvent),
		ConditionSet:    json.RawMessage(r.ConditionSet),
		Effects:         json.RawMessage(r.Effects),
		CooldownSeconds: r.CooldownSeconds,
		CreatedBy:       r.CreatedBy,
		CreatedAt:       r.CreatedAt.UTC().Format("2006-01-02T15:04:05Z"),
		UpdatedAt:       r.UpdatedAt.UTC().Format("2006-01-02T15:04:05Z"),
	}
	if len(r.MaxPerWindow) > 0 {
		out.MaxPerWindow = json.RawMessage(r.MaxPerWindow)
	}
	return out
}

type listRulesResponse struct {
	Rules      []ruleJSON `json:"rules"`
	TotalCount int64      `json:"total_count"`
	Page       int        `json:"page"`
	PerPage    int        `json:"per_page"`
}

type createRuleInput struct {
	Name            string          `json:"name"`
	Description     string          `json:"description"`
	AudienceLevel   string          `json:"audience_level"`
	Enabled         *bool           `json:"enabled"`
	TriggerEvent    json.RawMessage `json:"trigger_event"`
	ConditionSet    json.RawMessage `json:"condition_set"`
	Effects         json.RawMessage `json:"effects"`
	CooldownSeconds *int            `json:"cooldown_seconds"`
	MaxPerWindow    json.RawMessage `json:"max_per_window"`
}

type patchRuleInput struct {
	Name              *string         `json:"name"`
	Description       *string         `json:"description"`
	Enabled           *bool           `json:"enabled"`
	TriggerEvent      json.RawMessage `json:"trigger_event"`
	ConditionSet      json.RawMessage `json:"condition_set"`
	Effects           json.RawMessage `json:"effects"`
	CooldownSeconds   *int            `json:"cooldown_seconds"`
	MaxPerWindow      json.RawMessage `json:"max_per_window"`
	ClearCooldown     bool            `json:"clear_cooldown"`
	ClearMaxPerWindow bool            `json:"clear_max_per_window"`
}

// mapRuleServiceError maps service-layer rule sentinels to Fiber responses.
func mapRuleServiceError(c *fiber.Ctx, err error) error {
	switch {
	case errors.Is(err, gamification.ErrRuleNotFound):
		return responses.NotFound(c, "rule")
	case errors.Is(err, gamification.ErrRuleOutOfScope):
		// 13.1.E: existence leak — return 404 not 403 on cross-tenant.
		return responses.NotFound(c, "rule")
	case errors.Is(err, gamification.ErrInvalidRuleName),
		errors.Is(err, gamification.ErrInvalidRuleDescription):
		return responses.BadRequest(c, err.Error())
	default:
		// Validators in the rule service return errors.New(...) for
		// trigger / condition / effects / audience errors. These are all
		// 400-class — we can't easily distinguish from genuine 500 here
		// without typed sentinels for every validator. Use err.Error() to
		// surface the actionable message.
		if err == nil {
			return responses.InternalError(c, "rule operation failed")
		}
		return responses.BadRequest(c, err.Error())
	}
}

// ListRules handles GET — paginated rules at the route-derived scope.
func (h *GamificationHandler) ListRules(c *fiber.Ctx) error {
	tenantID := callerAccountID(c)
	scopeType, scopeID := resolveScope(c)
	page, perPage := paginationParams(c)

	res, err := h.ruleService.List(c.Context(), tenantID, scopeType, scopeID, repository.PaginationParams{Page: page, PerPage: perPage})
	if err != nil {
		return responses.InternalError(c, "could not list rules")
	}
	out := listRulesResponse{
		Rules:      make([]ruleJSON, 0, len(res.Items)),
		TotalCount: res.TotalCount,
		Page:       res.Page,
		PerPage:    res.PerPage,
	}
	for i := range res.Items {
		out.Rules = append(out.Rules, ruleJSONFor(&res.Items[i]))
	}
	return c.JSON(out)
}

// GetRule handles GET /rules/:id.
func (h *GamificationHandler) GetRule(c *fiber.Ctx) error {
	id, err := strconv.ParseUint(c.Params("id"), 10, 64)
	if err != nil {
		return responses.BadRequest(c, "invalid rule id")
	}
	scopeType, scopeID := resolveScope(c)
	row, err := h.ruleService.Get(c.Context(), uint(id), callerAccountID(c), scopeType, scopeID)
	if err != nil {
		return mapRuleServiceError(c, err)
	}
	return c.JSON(ruleJSONFor(row))
}

// CreateRule handles POST.
func (h *GamificationHandler) CreateRule(c *fiber.Ctx) error {
	var in createRuleInput
	if err := c.BodyParser(&in); err != nil {
		return responses.BadRequest(c, "invalid request body")
	}
	scopeType, scopeID := resolveScope(c)
	row, err := h.ruleService.Create(c.Context(), callerAccountID(c), scopeType, scopeID, callerUserID(c), gamification.RuleCreateInput{
		Name:            in.Name,
		Description:     in.Description,
		AudienceLevel:   in.AudienceLevel,
		Enabled:         derefBool(in.Enabled, true),
		TriggerEvent:    in.TriggerEvent,
		ConditionSet:    in.ConditionSet,
		Effects:         in.Effects,
		CooldownSeconds: in.CooldownSeconds,
		MaxPerWindow:    in.MaxPerWindow,
	})
	if err != nil {
		return mapRuleServiceError(c, err)
	}
	return c.Status(fiber.StatusCreated).JSON(ruleJSONFor(row))
}

// PatchRule handles PATCH /rules/:id.
func (h *GamificationHandler) PatchRule(c *fiber.Ctx) error {
	id, err := strconv.ParseUint(c.Params("id"), 10, 64)
	if err != nil {
		return responses.BadRequest(c, "invalid rule id")
	}
	var in patchRuleInput
	if err := c.BodyParser(&in); err != nil {
		return responses.BadRequest(c, "invalid request body")
	}
	scopeType, scopeID := resolveScope(c)
	row, err := h.ruleService.Patch(c.Context(), uint(id), callerAccountID(c), scopeType, scopeID, gamification.RulePatchInput{
		Name:              in.Name,
		Description:       in.Description,
		Enabled:           in.Enabled,
		TriggerEvent:      in.TriggerEvent,
		ConditionSet:      in.ConditionSet,
		Effects:           in.Effects,
		CooldownSeconds:   in.CooldownSeconds,
		MaxPerWindow:      in.MaxPerWindow,
		ClearCooldown:     in.ClearCooldown,
		ClearMaxPerWindow: in.ClearMaxPerWindow,
	})
	if err != nil {
		return mapRuleServiceError(c, err)
	}
	return c.JSON(ruleJSONFor(row))
}

// DeleteRule handles DELETE /rules/:id.
func (h *GamificationHandler) DeleteRule(c *fiber.Ctx) error {
	id, err := strconv.ParseUint(c.Params("id"), 10, 64)
	if err != nil {
		return responses.BadRequest(c, "invalid rule id")
	}
	scopeType, scopeID := resolveScope(c)
	if err := h.ruleService.Delete(c.Context(), uint(id), callerAccountID(c), scopeType, scopeID); err != nil {
		return mapRuleServiceError(c, err)
	}
	return c.SendStatus(fiber.StatusNoContent)
}

// callerUserID reads user_id from Locals (0 if unset).
func callerUserID(c *fiber.Ctx) uint {
	if v, ok := c.Locals("user_id").(uint); ok {
		return v
	}
	return 0
}

// paginationParams pulls page/per_page from query with sane defaults.
func paginationParams(c *fiber.Ctx) (page, perPage int) {
	page = 1
	perPage = 50
	if p, err := strconv.Atoi(c.Query("page")); err == nil && p > 0 {
		page = p
	}
	if pp, err := strconv.Atoi(c.Query("per_page")); err == nil && pp > 0 && pp <= 200 {
		perPage = pp
	}
	return
}
