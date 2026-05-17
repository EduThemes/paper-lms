package handlers

import (
	"context"
	"strconv"

	"github.com/gofiber/fiber/v2"

	"github.com/EduThemes/paper-lms/internal/api/v1/responses"
	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/service/settings"
)

// SuperAdminSettingsHandler exposes the read-only surface of the
// Super-Admin Settings Engine. Wave 2 ships three endpoints:
//
//	GET /api/v1/superadmin/settings              — every catalog
//	                                               entry resolved at
//	                                               the current scope
//	                                               hints (effective
//	                                               values, secrets
//	                                               masked).
//	GET /api/v1/superadmin/settings/:key         — one entry resolved.
//	GET /api/v1/superadmin/settings/groups       — pure vocabulary
//	                                               (no live values),
//	                                               for building the
//	                                               UI form.
//
// Threat model — Canvas-LMS site-admin precedent
// ───────────────────────────────────────────────
// Canvas's site_admin CVEs (e.g. CVE-2021-32585) trace to a
// cross-tenant superuser role where individual gates forgot to
// re-check correctly. Defensive properties this handler is required
// to maintain:
//
//  1. EVERY route mounted by Register MUST sit behind both
//     Protected() AND RequireSuperAdmin(). Forgetting either is a
//     deployment-wide privilege escalation. The router has a
//     compile-time-friendly mount helper (registerSuperAdminRoutes)
//     so adding a fourth endpoint can't accidentally skip the gate.
//
//  2. Secret-typed catalog entries route through EffectiveValue.Mask()
//     before serialization, here and at every future write/read
//     endpoint. The handler NEVER serializes the raw EffectiveValue.
//
//  3. The vocabulary endpoint returns Definition only, NEVER mixed
//     with live values from any scope. The frontend builds the form
//     from the vocabulary and asks for live values separately — a
//     single response that mixes "this is what could be set" with
//     "this is the current secret" would invite the kind of UI bug
//     that leaks an unmask through a debug serialization.
//
//  4. The ?account_id=N query parameter expands the resolution-chain
//     hint a super-admin sees, but it does NOT widen what they can
//     see — a super-admin already crosses tenant boundaries; the
//     param just selects WHICH tenant's resolution to project. It is
//     ignored entirely if the auth layer didn't already set
//     is_super_admin=true (handled implicitly by RequireSuperAdmin's
//     403 short-circuit; this handler never executes otherwise).
type SuperAdminSettingsHandler struct {
	svc            *settings.Service
	accountChecker accountExistenceChecker
	audit          settingsAuditSink
}

// accountExistenceChecker is a narrow interface for "does this
// account row exist." The PUT handler uses this to reject writes
// targeted at a nonexistent account_id before delegating to the
// service. The wider AccountRepository satisfies this structurally.
type accountExistenceChecker interface {
	FindByID(ctx context.Context, id uint) (*models.Account, error)
}

// settingsAuditSink mirrors settings.AuditSink — duplicated here so
// the handler can emit `setting.tested` events directly for the test
// actions (the service only emits set/clear). The wider AuditService
// satisfies it structurally.
type settingsAuditSink interface {
	LogEvent(ctx context.Context, eventType string, userID uint, courseID, accountID *uint, contextType string, contextID uint, action, payload, ipAddress, userAgent string) error
}

// NewSuperAdminSettingsHandler constructs the handler. `accountChecker`
// and `audit` may be nil — the write API will reject if accountChecker
// is nil + a write targets account scope; the audit sink degrades to
// no-op when nil (matches the service-layer contract).
func NewSuperAdminSettingsHandler(svc *settings.Service, accountChecker accountExistenceChecker, audit settingsAuditSink) *SuperAdminSettingsHandler {
	return &SuperAdminSettingsHandler{svc: svc, accountChecker: accountChecker, audit: audit}
}

// settingResponse is the JSON shape returned by the live-value
// endpoints. Value is the empty string for any secret-typed entry;
// callers should NOT distinguish "secret with empty plaintext" from
// "secret with a value" by .Value alone — has_value is the right
// signal. updated_at + updated_by are populated only when the source
// is user/account/instance (a stored row); for env/default they're
// nil so the UI renders "configured via environment" instead of a
// timestamp.
type settingResponse struct {
	Key       string  `json:"key"`
	Group     string  `json:"group"`
	Label     string  `json:"label"`
	ValueType string  `json:"value_type"`
	IsSecret  bool    `json:"is_secret"`
	Source    string  `json:"source"`
	HasValue  bool    `json:"has_value"`
	Value     string  `json:"value,omitempty"`
	ScopeID   uint    `json:"scope_id,omitempty"`
	UpdatedAt *string `json:"updated_at,omitempty"`
	UpdatedBy *uint   `json:"updated_by,omitempty"`
}

// definitionResponse is the JSON shape returned by the vocabulary
// endpoint. NO live value field — vocabulary is a description of what
// COULD be set, not what IS set.
type definitionResponse struct {
	Key         string   `json:"key"`
	Group       string   `json:"group"`
	Label       string   `json:"label"`
	Description string   `json:"description"`
	ValueType   string   `json:"value_type"`
	IsSecret    bool     `json:"is_secret"`
	Scopes      []string `json:"scopes"`
	EnvFallback string   `json:"env_fallback,omitempty"`
	HasDefault  bool     `json:"has_default"`
	TestAction  string   `json:"test_action,omitempty"`
}

// hintsFromRequest reads ?account_id=N if present. Default = 0 (no
// account context) — the resolution chain skips the account walk and
// falls straight through to instance/env/default.
//
// The plain integer parse is sufficient — RequireSuperAdmin has
// already verified the caller is a platform operator, and a super-
// admin's tenant scope is the whole deployment. We don't validate
// the account exists; a non-existent account_id simply causes the
// chain to skip the account scope (settings repo returns NotFound,
// ancestry FindByID returns an error which the service treats as an
// abort-walk signal). No information leak, since the response shape
// doesn't change based on whether the account exists.
func hintsFromRequest(c *fiber.Ctx) settings.ScopeHints {
	var hints settings.ScopeHints
	if raw := c.Query("account_id"); raw != "" {
		if v, err := strconv.ParseUint(raw, 10, 64); err == nil {
			hints.AccountID = uint(v)
		}
	}
	return hints
}

// toResponse converts an EffectiveValue + its Definition into the
// JSON shape. SECRETS ARE MASKED HERE — every code path that
// serializes a value MUST go through this function. There is no
// other authorized way to render a setting value.
func toResponse(def settings.Definition, ev settings.EffectiveValue) settingResponse {
	masked := ev.Mask()
	out := settingResponse{
		Key:       def.Key,
		Group:     def.Group,
		Label:     def.Label,
		ValueType: string(def.ValueType),
		IsSecret:  def.IsSecret(),
		Source:    string(masked.Source),
		HasValue:  masked.HasValue,
		Value:     masked.Value, // empty string when IsSecret, by Mask()
		ScopeID:   masked.ScopeID,
		UpdatedBy: masked.UpdatedBy,
	}
	if masked.UpdatedAt != nil {
		s := masked.UpdatedAt.UTC().Format("2006-01-02T15:04:05Z07:00")
		out.UpdatedAt = &s
	}
	return out
}

// List handles GET /api/v1/superadmin/settings[?account_id=N].
// Returns every catalog entry, resolved at the current scope hints,
// with secrets masked.
func (h *SuperAdminSettingsHandler) List(c *fiber.Ctx) error {
	hints := hintsFromRequest(c)
	effective, err := h.svc.GetEffective(c.Context(), "", hints)
	if err != nil {
		// The service only returns errors for catalog-misses
		// (impossible here — we iterate the catalog) or transport
		// errors. A 500 with the sanitized error path is fine —
		// the ErrorHandler in Phase 12 already redacts 5xx bodies.
		return responses.InternalError(c, "Could not load settings")
	}

	items := make([]settingResponse, 0, len(settings.Catalog))
	for _, def := range settings.Catalog {
		ev := effective[def.Key]
		items = append(items, toResponse(def, ev))
	}
	return c.JSON(fiber.Map{"settings": items})
}

// Get handles GET /api/v1/superadmin/settings/:key[?account_id=N].
// Returns a single catalog entry's effective value, secrets masked.
func (h *SuperAdminSettingsHandler) Get(c *fiber.Ctx) error {
	key := c.Params("key")
	if key == "" {
		return responses.BadRequest(c, "key is required")
	}

	def, ok := settings.Find(key)
	if !ok {
		// 404 (not 400) — mirrors the "we don't acknowledge a thing
		// we don't recognize" stance from assertSameTenant. Keeps
		// the API surface from being a key-enumeration oracle for
		// settings the deployment doesn't support.
		return responses.NotFound(c, "setting")
	}

	ev, err := h.svc.Get(c.Context(), key, hintsFromRequest(c))
	if err != nil {
		return responses.InternalError(c, "Could not load setting")
	}
	return c.JSON(toResponse(def, ev))
}

// Groups handles GET /api/v1/superadmin/settings/groups. Returns the
// catalog vocabulary (no live values). The UI uses this to build the
// settings form; live values come from List/Get above.
func (h *SuperAdminSettingsHandler) Groups(c *fiber.Ctx) error {
	defs := make([]definitionResponse, 0, len(settings.Catalog))
	for _, def := range settings.Catalog {
		scopes := make([]string, 0, len(def.Scopes))
		for _, s := range def.Scopes {
			scopes = append(scopes, string(s))
		}
		defs = append(defs, definitionResponse{
			Key:         def.Key,
			Group:       def.Group,
			Label:       def.Label,
			Description: def.Description,
			ValueType:   string(def.ValueType),
			IsSecret:    def.IsSecret(),
			Scopes:      scopes,
			EnvFallback: def.EnvFallback,
			HasDefault:  def.Default != "",
			TestAction:  def.TestAction,
		})
	}
	return c.JSON(fiber.Map{"definitions": defs})
}
