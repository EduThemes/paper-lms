// Package handlers: AI Assist proxy endpoints for the RCE V2 toolbar.
//
// POST /api/v1/ai_assist/:action where action ∈ {outline, summarize, rewrite}.
// Body: {"text": "...", "style": "..."} (style only used for "rewrite").
// Returns: {"result": "..."}.
//
// Auth: requires an authenticated session (router applies RequireAuth before
// these routes — Locals("user_id") must be set).
package handlers

import (
	"errors"

	"github.com/gofiber/fiber/v2"
	"github.com/EduThemes/paper-lms/internal/api/v1/responses"
	"github.com/EduThemes/paper-lms/internal/repository"
	"github.com/EduThemes/paper-lms/internal/service"
)

// AIAssistHandler wires HTTP requests to AIAssistService.
type AIAssistHandler struct {
	service     *service.AIAssistService
	accountRepo repository.AccountRepository
}

// NewAIAssistHandler constructs the handler. The service may be nil-keyed
// (no ANTHROPIC_API_KEY) — in that case Dispatch returns 503. accountRepo
// drives the 13.4 COPPA gate; a nil repo skips the gate (development
// fallback), production wires the real one.
func NewAIAssistHandler(svc *service.AIAssistService, accountRepo repository.AccountRepository) *AIAssistHandler {
	return &AIAssistHandler{service: svc, accountRepo: accountRepo}
}

type aiAssistRequest struct {
	Text  string `json:"text"`
	Style string `json:"style"`
}

// Dispatch handles POST /api/v1/ai_assist/:action.
func (h *AIAssistHandler) Dispatch(c *fiber.Ctx) error {
	// Auth check — handler is mounted behind RequireAuth, but defense in depth.
	userID, ok := c.Locals("user_id").(uint)
	if !ok || userID == 0 {
		return responses.Unauthorized(c)
	}

	if h.service == nil || !h.service.Configured() {
		return responses.Error(c, fiber.StatusServiceUnavailable, "AI Assist not configured")
	}

	// 13.4 — COPPA gate. AI Assist sends student writing to Anthropic;
	// for accounts with coppa_strict=true or tenant_mode in {k5,m68},
	// that's a non-starter without explicit parental consent (which
	// the audit found is not yet wired). Refuse outright; the toolbar
	// shows "AI Assist disabled for your school" on 403.
	if h.accountRepo != nil {
		accountID, _ := c.Locals("account_id").(uint)
		if accountID > 0 {
			if account, err := h.accountRepo.FindByID(c.Context(), accountID); err == nil && account != nil {
				if account.CoppaStrict || string(account.TenantMode) == "k5" || string(account.TenantMode) == "m68" {
					return responses.Error(c, fiber.StatusForbidden, "AI Assist is disabled for your school's privacy mode")
				}
			}
		}
	}

	action := c.Params("action")
	var input aiAssistRequest
	if err := c.BodyParser(&input); err != nil {
		return responses.BadRequest(c, "Invalid input")
	}
	if input.Text == "" {
		return responses.BadRequest(c, "text is required")
	}

	ctx := c.Context()
	var (
		result string
		err    error
	)
	switch action {
	case "outline":
		result, err = h.service.Outline(ctx, input.Text)
	case "summarize":
		result, err = h.service.Summarize(ctx, input.Text)
	case "rewrite":
		result, err = h.service.Rewrite(ctx, input.Text, input.Style)
	default:
		return responses.BadRequest(c, "Unknown AI Assist action: "+action)
	}

	if err != nil {
		if errors.Is(err, service.ErrAIAssistNotConfigured) {
			return responses.Error(c, fiber.StatusServiceUnavailable, "AI Assist not configured")
		}
		return responses.Error(c, fiber.StatusBadGateway, "AI Assist request failed: "+err.Error())
	}

	return c.JSON(fiber.Map{"result": result})
}
