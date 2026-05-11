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
	"github.com/EduThemes/paper-lms/internal/service"
)

// AIAssistHandler wires HTTP requests to AIAssistService.
type AIAssistHandler struct {
	service *service.AIAssistService
}

// NewAIAssistHandler constructs the handler. The service may be nil-keyed
// (no ANTHROPIC_API_KEY) — in that case Dispatch returns 503.
func NewAIAssistHandler(svc *service.AIAssistService) *AIAssistHandler {
	return &AIAssistHandler{service: svc}
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
