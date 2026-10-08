package handlers

import (
	"errors"
	"regexp"

	"github.com/gofiber/fiber/v2"

	"github.com/EduThemes/paper-lms/internal/api/v1/responses"
	"github.com/EduThemes/paper-lms/internal/service"
)

// e164Phone accepts an international phone number: "+", a non-zero country
// digit, then 7-14 more digits.
var e164Phone = regexp.MustCompile(`^\+[1-9][0-9]{7,14}$`)

func isE164Phone(s string) bool { return e164Phone.MatchString(s) }

// GetNotificationPolicies returns the caller's per-category x per-channel
// notification grid (G091).
// GET /api/v1/users/self/notification_policies
func (h *NotificationDeliveryHandler) GetNotificationPolicies(c *fiber.Ctx) error {
	userID, err := getUserID(c)
	if err != nil {
		return err
	}
	grid, err := h.deliveryService.EffectivePolicies(c.Context(), userID)
	if err != nil {
		return responses.InternalError(c, "Could not load notification preferences")
	}
	return c.JSON(grid)
}

// UpdateNotificationPolicies sets one or more cells of the caller's grid.
// PUT /api/v1/users/self/notification_policies
//
//	{"policies":[{"channel_id":0,"category":"announcement","frequency":"immediately"}]}
//
// frequency "default" (or "") removes the explicit cell so it inherits the
// global preference again. Only the caller's own channels (0 = account
// email) are accepted; any other id is a 404.
func (h *NotificationDeliveryHandler) UpdateNotificationPolicies(c *fiber.Ctx) error {
	userID, err := getUserID(c)
	if err != nil {
		return err
	}
	var input struct {
		Policies []struct {
			ChannelID uint   `json:"channel_id"`
			Category  string `json:"category"`
			Frequency string `json:"frequency"`
		} `json:"policies"`
	}
	if err := c.BodyParser(&input); err != nil || len(input.Policies) == 0 {
		return responses.BadRequest(c, "policies is required")
	}
	if len(input.Policies) > 500 {
		return responses.BadRequest(c, "too many policies in one request")
	}
	for _, p := range input.Policies {
		if err := h.deliveryService.SetPolicy(c.Context(), userID, p.ChannelID, p.Category, p.Frequency); err != nil {
			if errors.Is(err, service.ErrUnknownPolicyChannel) {
				return responses.NotFound(c, "communication channel")
			}
			return responses.BadRequest(c, err.Error())
		}
	}
	grid, err := h.deliveryService.EffectivePolicies(c.Context(), userID)
	if err != nil {
		return responses.InternalError(c, "Could not load notification preferences")
	}
	return c.JSON(grid)
}

// ListChannelTypes reports which channel types this site can deliver to
// (SMS and push are stubs until a provider is registered).
// GET /api/v1/users/self/communication_channel_types
func (h *NotificationDeliveryHandler) ListChannelTypes(c *fiber.Ctx) error {
	return c.JSON(h.deliveryService.ChannelTypes())
}
