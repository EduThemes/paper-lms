package handlers

import (
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/gofiber/fiber/v2"
	"gorm.io/datatypes"

	"github.com/EduThemes/paper-lms/internal/api/v1/responses"
	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/repository"
	"github.com/EduThemes/paper-lms/internal/service/gamification"
	"github.com/EduThemes/paper-lms/internal/service/gamification/effects"
	"github.com/EduThemes/paper-lms/internal/service/gamification/predicates"
)

// Sprint W2-E.1 — recipe-builder write API + vocabulary discovery
// endpoint. Handlers here live on the existing *GamificationHandler
// struct (gamification.go) so route registration matches the
// W2-B/W2-D shape exactly.
//
// Validation pulls from the canonical decoders the runtime uses:
//   - predicates.DecodePredicate for condition_set trees,
//   - effects.DecodeEffects for the effects array.
//
// Vocabulary endpoint serializes the catalog declared in
// service/gamification/vocabulary.go — one source of truth that both
// validates writes and shapes the recipe builder's inline editors.

// ----------------------------------------------------------------------
// JSON projection.
// ----------------------------------------------------------------------

// ruleJSON is the read projection a recipe-builder UI consumes. JSONB
// fields are emitted as raw bytes so the frontend's editor doesn't need
// to re-encode them on edit. tenant/scope are inferred at write time
// from caller + route, not echoed for write — but they ARE echoed here
// so a list-view chip can show "site" vs "course/42" at a glance.
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

// ----------------------------------------------------------------------
// Write inputs.
// ----------------------------------------------------------------------

// createRuleInput is the POST body shape. Tenant + scope come from the
// caller/route, never the body. Code-style immutability isn't needed
// (rules have no natural-key `code` field that other rules reference);
// the schema-level CASCADE on rule_evaluations.rule_id is the
// referential safety net.
type createRuleInput struct {
	Name            string          `json:"name"`
	Description     string          `json:"description"`
	AudienceLevel   string          `json:"audience_level"`
	Enabled         *bool           `json:"enabled"` // default true
	TriggerEvent    json.RawMessage `json:"trigger_event"`
	ConditionSet    json.RawMessage `json:"condition_set"`
	Effects         json.RawMessage `json:"effects"`
	CooldownSeconds *int            `json:"cooldown_seconds"`
	MaxPerWindow    json.RawMessage `json:"max_per_window"`
}

// patchRuleInput accepts the editable subset. tenant/scope/audience are
// immutable post-create — changing audience changes pedagogical
// defaults that ripple through rule_evaluation history.
type patchRuleInput struct {
	Name            *string         `json:"name"`
	Description     *string         `json:"description"`
	Enabled         *bool           `json:"enabled"`
	TriggerEvent    json.RawMessage `json:"trigger_event"`
	ConditionSet    json.RawMessage `json:"condition_set"`
	Effects         json.RawMessage `json:"effects"`
	CooldownSeconds *int            `json:"cooldown_seconds"`
	MaxPerWindow    json.RawMessage `json:"max_per_window"`
	// ClearCooldown / ClearMaxPerWindow let the caller explicitly null
	// out an optional field. Useful for the recipe editor's "remove
	// cooldown" affordance — leaving these false preserves whatever was
	// there. (CooldownSeconds = 0 is meaningfully different from
	// "remove the limit" — 0 means "no rate limit applies in practice,"
	// NULL means "no rate limit configured.")
	ClearCooldown     bool `json:"clear_cooldown"`
	ClearMaxPerWindow bool `json:"clear_max_per_window"`
}

// ----------------------------------------------------------------------
// Validators.
// ----------------------------------------------------------------------

func validateAudience(level string) error {
	for _, ok := range gamification.AudienceLevels {
		if level == ok {
			return nil
		}
	}
	return fmt.Errorf("audience_level must be one of %v", gamification.AudienceLevels)
}

func enumContains(haystack []string, needle string) bool {
	for _, h := range haystack {
		if h == needle {
			return true
		}
	}
	return false
}

// validateTriggerEvent decodes and validates the trigger_event JSON
// blob. The runtime rule index (rule_index.go) only fires on the three
// kinds enumerated here — anything else would silently never trigger,
// so reject at write time with a clear error rather than a 200 + a
// rule that never runs.
func validateTriggerEvent(raw json.RawMessage) error {
	if len(raw) == 0 {
		return errors.New("trigger_event is required")
	}
	var head struct {
		Kind string `json:"kind"`
	}
	if err := json.Unmarshal(raw, &head); err != nil {
		return fmt.Errorf("trigger_event: %w", err)
	}
	switch head.Kind {
	case "OnEvent":
		var p struct {
			Verb       string `json:"verb"`
			ObjectType string `json:"object_type"`
		}
		if err := json.Unmarshal(raw, &p); err != nil {
			return fmt.Errorf("trigger_event OnEvent: %w", err)
		}
		if !enumContains(gamification.VerbCatalog, p.Verb) {
			return fmt.Errorf("trigger_event OnEvent.verb must be one of %v (got %q)", gamification.VerbCatalog, p.Verb)
		}
		if !enumContains(gamification.ObjectCatalog, p.ObjectType) {
			return fmt.Errorf("trigger_event OnEvent.object_type must be one of %v (got %q)", gamification.ObjectCatalog, p.ObjectType)
		}
	case "OnSchedule":
		var p struct {
			Cron string `json:"cron"`
		}
		if err := json.Unmarshal(raw, &p); err != nil {
			return fmt.Errorf("trigger_event OnSchedule: %w", err)
		}
		if strings.TrimSpace(p.Cron) == "" {
			return errors.New("trigger_event OnSchedule.cron must be non-empty")
		}
	case "OnManualTrigger":
		var p struct {
			Handle string `json:"handle"`
		}
		if err := json.Unmarshal(raw, &p); err != nil {
			return fmt.Errorf("trigger_event OnManualTrigger: %w", err)
		}
		if strings.TrimSpace(p.Handle) == "" {
			return errors.New("trigger_event OnManualTrigger.handle must be non-empty")
		}
	case "":
		return errors.New("trigger_event missing required field \"kind\"")
	default:
		return fmt.Errorf("unknown trigger_event kind: %q", head.Kind)
	}
	return nil
}

// validateMaxPerWindow checks the optional rate-limit shape:
// `{"window":"day"|"week"|"lifetime","count": >0}`.
func validateMaxPerWindow(raw json.RawMessage) error {
	if len(raw) == 0 {
		return nil // optional
	}
	var p struct {
		Window string `json:"window"`
		Count  int    `json:"count"`
	}
	if err := json.Unmarshal(raw, &p); err != nil {
		return fmt.Errorf("max_per_window: %w", err)
	}
	if !enumContains(gamification.WindowKinds, p.Window) {
		return fmt.Errorf("max_per_window.window must be one of %v (got %q)", gamification.WindowKinds, p.Window)
	}
	if p.Count <= 0 {
		return fmt.Errorf("max_per_window.count must be > 0 (got %d)", p.Count)
	}
	return nil
}

// validateRuleBody runs the full validator on the merged Create / Patch
// payload. It's invoked AFTER the Patch's pointer-merge so a partial
// update that yields an invalid rule is rejected before the row is
// persisted.
func validateRuleBody(audience string, trigger, condition, effectsRaw, maxPerWindow json.RawMessage, cooldown *int) error {
	if err := validateAudience(audience); err != nil {
		return err
	}
	if err := validateTriggerEvent(trigger); err != nil {
		return err
	}
	if len(condition) == 0 {
		return errors.New("condition_set is required")
	}
	if _, err := predicates.DecodePredicate(condition); err != nil {
		return fmt.Errorf("condition_set: %w", err)
	}
	if len(effectsRaw) == 0 {
		return errors.New("effects is required (use [] for no-effect rules)")
	}
	if _, err := effects.DecodeEffects(effectsRaw); err != nil {
		return fmt.Errorf("effects: %w", err)
	}
	if cooldown != nil && *cooldown <= 0 {
		return fmt.Errorf("cooldown_seconds must be > 0 when set (got %d)", *cooldown)
	}
	if err := validateMaxPerWindow(maxPerWindow); err != nil {
		return err
	}
	return nil
}

// ----------------------------------------------------------------------
// Handlers.
// ----------------------------------------------------------------------

// ListRules handles GET — paginated rules at the route-derived scope.
//
// Returns rules at the exact (tenant, scope_type, scope_id) the route
// resolved to: admin at site sees site rules; instructor at course X
// sees course X rules. The dispatcher walks the org hierarchy itself at
// runtime; the list view is intentionally scope-precise so a recipe
// builder doesn't show authors rules they can't edit.
func (h *GamificationHandler) ListRules(c *fiber.Ctx) error {
	tenantID := callerAccountID(c)
	scopeType, scopeID := resolveScope(c)
	page, perPage := paginationParams(c)

	res, err := h.ruleRepo.ListByScope(c.Context(), tenantID, scopeType, scopeID, repository.PaginationParams{Page: page, PerPage: perPage})
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
	row, err := h.ruleRepo.FindByID(c.Context(), uint(id))
	if err != nil {
		return responses.InternalError(c, "could not load rule")
	}
	if row == nil {
		return responses.NotFound(c, "rule")
	}
	if !ruleInScope(c, row) {
		return responses.Error(c, fiber.StatusForbidden, "rule is not in the requested scope")
	}
	return c.JSON(ruleJSONFor(row))
}

// CreateRule handles POST.
func (h *GamificationHandler) CreateRule(c *fiber.Ctx) error {
	var in createRuleInput
	if err := c.BodyParser(&in); err != nil {
		return responses.BadRequest(c, "invalid request body")
	}
	if strings.TrimSpace(in.Name) == "" || len(in.Name) > 200 {
		return responses.BadRequest(c, "name is required, max 200 chars")
	}
	if len(in.Description) > 2000 {
		return responses.BadRequest(c, "description max 2000 chars")
	}
	if err := validateRuleBody(in.AudienceLevel, in.TriggerEvent, in.ConditionSet, in.Effects, in.MaxPerWindow, in.CooldownSeconds); err != nil {
		return responses.BadRequest(c, err.Error())
	}

	scopeType, scopeID := resolveScope(c)
	tenantID := callerAccountID(c)
	creator := callerUserID(c)

	row := &models.GamificationRule{
		TenantID:        tenantID,
		ScopeType:       scopeType,
		ScopeID:         scopeID,
		AudienceLevel:   models.GamificationAudience(in.AudienceLevel),
		Name:            strings.TrimSpace(in.Name),
		Description:     strings.TrimSpace(in.Description),
		Enabled:         derefBool(in.Enabled, true),
		TriggerEvent:    datatypes.JSON(in.TriggerEvent),
		ConditionSet:    datatypes.JSON(in.ConditionSet),
		Effects:         datatypes.JSON(in.Effects),
		CooldownSeconds: in.CooldownSeconds,
		MaxPerWindow:    datatypes.JSON(in.MaxPerWindow),
	}
	if creator != 0 {
		c := creator
		row.CreatedBy = &c
	}

	if err := h.ruleRepo.Create(c.Context(), row); err != nil {
		return responses.InternalError(c, "could not create rule")
	}
	return c.Status(fiber.StatusCreated).JSON(ruleJSONFor(row))
}

// PatchRule handles PATCH /rules/:id. tenant/scope/audience immutable.
func (h *GamificationHandler) PatchRule(c *fiber.Ctx) error {
	id, err := strconv.ParseUint(c.Params("id"), 10, 64)
	if err != nil {
		return responses.BadRequest(c, "invalid rule id")
	}
	row, err := h.ruleRepo.FindByID(c.Context(), uint(id))
	if err != nil {
		return responses.InternalError(c, "could not load rule")
	}
	if row == nil {
		return responses.NotFound(c, "rule")
	}
	if !ruleInScope(c, row) {
		return responses.Error(c, fiber.StatusForbidden, "rule is not in the requested scope")
	}

	var in patchRuleInput
	if err := c.BodyParser(&in); err != nil {
		return responses.BadRequest(c, "invalid request body")
	}

	if in.Name != nil {
		n := strings.TrimSpace(*in.Name)
		if n == "" || len(n) > 200 {
			return responses.BadRequest(c, "name must be non-empty, max 200 chars")
		}
		row.Name = n
	}
	if in.Description != nil {
		if len(*in.Description) > 2000 {
			return responses.BadRequest(c, "description max 2000 chars")
		}
		row.Description = strings.TrimSpace(*in.Description)
	}
	if in.Enabled != nil {
		row.Enabled = *in.Enabled
	}
	if len(in.TriggerEvent) > 0 {
		row.TriggerEvent = datatypes.JSON(in.TriggerEvent)
	}
	if len(in.ConditionSet) > 0 {
		row.ConditionSet = datatypes.JSON(in.ConditionSet)
	}
	if len(in.Effects) > 0 {
		row.Effects = datatypes.JSON(in.Effects)
	}
	if in.ClearCooldown {
		row.CooldownSeconds = nil
	} else if in.CooldownSeconds != nil {
		row.CooldownSeconds = in.CooldownSeconds
	}
	if in.ClearMaxPerWindow {
		row.MaxPerWindow = nil
	} else if len(in.MaxPerWindow) > 0 {
		row.MaxPerWindow = datatypes.JSON(in.MaxPerWindow)
	}

	// Validate the merged result. A patch that takes a previously-valid
	// rule into an invalid state (e.g., clears effects to []) is
	// rejected before persistence.
	if err := validateRuleBody(
		string(row.AudienceLevel),
		json.RawMessage(row.TriggerEvent),
		json.RawMessage(row.ConditionSet),
		json.RawMessage(row.Effects),
		json.RawMessage(row.MaxPerWindow),
		row.CooldownSeconds,
	); err != nil {
		return responses.BadRequest(c, err.Error())
	}

	if err := h.ruleRepo.Update(c.Context(), row); err != nil {
		return responses.InternalError(c, "could not update rule")
	}
	return c.JSON(ruleJSONFor(row))
}

// DeleteRule handles DELETE /rules/:id. CASCADE on the SQL chain
// removes the linked rule_evaluation audit rows; if that's ever a
// concern (compliance retention, etc.) a soft-delete flag goes onto
// the model first, not into this handler.
func (h *GamificationHandler) DeleteRule(c *fiber.Ctx) error {
	id, err := strconv.ParseUint(c.Params("id"), 10, 64)
	if err != nil {
		return responses.BadRequest(c, "invalid rule id")
	}
	row, err := h.ruleRepo.FindByID(c.Context(), uint(id))
	if err != nil {
		return responses.InternalError(c, "could not load rule")
	}
	if row == nil {
		return responses.NotFound(c, "rule")
	}
	if !ruleInScope(c, row) {
		return responses.Error(c, fiber.StatusForbidden, "rule is not in the requested scope")
	}
	if err := h.ruleRepo.Delete(c.Context(), row.ID); err != nil {
		return responses.InternalError(c, "could not delete rule")
	}
	return c.SendStatus(fiber.StatusNoContent)
}

// ----------------------------------------------------------------------
// Vocabulary discovery.
// ----------------------------------------------------------------------

// vocabularyResponse is the serialized catalog the recipe builder reads
// at editor-mount time. Static per server build — the catalog has no
// per-tenant variation in W2-E.1. Authenticated route, no admin gate.
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

// GetVocabulary handles GET /api/v1/gamification/vocabulary.
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

// ----------------------------------------------------------------------
// Local helpers.
// ----------------------------------------------------------------------

// ruleInScope is the defence-in-depth scope guard. Middleware already
// gates the route by role; this handler check stops a course-instructor
// from PATCHing a site rule by paste-bombing its id into the URL.
func ruleInScope(c *fiber.Ctx, row *models.GamificationRule) bool {
	tenantID := callerAccountID(c)
	scopeType, scopeID := resolveScope(c)
	return row.TenantID == tenantID && row.ScopeType == scopeType && row.ScopeID == scopeID
}

func callerUserID(c *fiber.Ctx) uint {
	if v, ok := c.Locals("user_id").(uint); ok {
		return v
	}
	return 0
}

// paginationParams pulls page/per_page from query with sane defaults.
// Defaults match the rest of the gamification paginated endpoints.
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
