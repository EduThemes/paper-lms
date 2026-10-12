package handlers

import (
	"errors"
	"net/mail"
	"strings"

	"github.com/gofiber/fiber/v2"
	"github.com/EduThemes/paper-lms/internal/security"
	"github.com/EduThemes/paper-lms/internal/api/v1/middleware"
	"github.com/EduThemes/paper-lms/internal/api/v1/responses"
	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/repository/postgres"
	"github.com/EduThemes/paper-lms/internal/service"
)

// NotificationDeliveryHandler handles notification delivery and communication channel endpoints.
type NotificationDeliveryHandler struct {
	deliveryService *service.NotificationDeliveryService
	channelRepo     postgres.CommunicationChannelRepository
}

// NewNotificationDeliveryHandler creates a new NotificationDeliveryHandler.
func NewNotificationDeliveryHandler(
	deliveryService *service.NotificationDeliveryService,
	channelRepo postgres.CommunicationChannelRepository,
) *NotificationDeliveryHandler {
	return &NotificationDeliveryHandler{
		deliveryService: deliveryService,
		channelRepo:     channelRepo,
	}
}

func deliveryToJSON(d *models.NotificationDelivery) fiber.Map {
	return fiber.Map{
		"id":              d.ID,
		"notification_id": d.NotificationID,
		"user_id":         d.UserID,
		"channel_type":    d.ChannelType,
		"address":         d.Address,
		"subject":         d.Subject,
		"body":            d.Body,
		"delivery_status": d.DeliveryStatus,
		"digest_type":     d.DigestType,
		"retry_count":     d.RetryCount,
		"max_retries":     d.MaxRetries,
		"last_error":      d.LastError,
		"sent_at":         d.SentAt,
		"delivered_at":    d.DeliveredAt,
		"scheduled_for":   d.ScheduledFor,
		"created_at":      d.CreatedAt,
		"updated_at":      d.UpdatedAt,
	}
}

func channelToJSON(ch *models.CommunicationChannel) fiber.Map {
	return fiber.Map{
		"id":             ch.ID,
		"user_id":        ch.UserID,
		"channel_type":   ch.ChannelType,
		"address":        ch.Address,
		"position":       ch.Position,
		"confirmed":      ch.Confirmed,
		"confirmed_at":   ch.ConfirmedAt,
		"workflow_state": ch.WorkflowState,
		"created_at":     ch.CreatedAt,
		"updated_at":     ch.UpdatedAt,
	}
}

// ListDeliveries returns a paginated delivery log for the current user.
// GET /api/v1/users/self/notification_deliveries
func (h *NotificationDeliveryHandler) ListDeliveries(c *fiber.Ctx) error {
	userID, err := getUserID(c)
	if err != nil {
		return err
	}
	params := middleware.GetPagination(c)
	status := c.Query("status")

	if status != "" {
		r, err := h.deliveryService.GetDeliveryLogByStatus(c.Context(), userID, status, params.Page, params.PerPage)
		if err != nil {
			return responses.InternalError(c, "Could not fetch delivery log")
		}
		responses.SetPaginationHeaders(c, r.TotalCount, r.Page, r.PerPage)
		items := make([]fiber.Map, len(r.Items))
		for i, d := range r.Items {
			items[i] = deliveryToJSON(&d)
		}
		return c.JSON(items)
	}

	r, err := h.deliveryService.GetDeliveryLog(c.Context(), userID, params.Page, params.PerPage)
	if err != nil {
		return responses.InternalError(c, "Could not fetch delivery log")
	}

	responses.SetPaginationHeaders(c, r.TotalCount, r.Page, r.PerPage)

	items := make([]fiber.Map, len(r.Items))
	for i, d := range r.Items {
		items[i] = deliveryToJSON(&d)
	}
	return c.JSON(items)
}

// GetDeliveryStats returns delivery statistics for admins.
// GET /api/v1/admin/notification_stats
func (h *NotificationDeliveryHandler) GetDeliveryStats(c *fiber.Ctx) error {
	stats, err := h.deliveryService.GetDeliveryStats(c.Context())
	if err != nil {
		return responses.InternalError(c, "Could not fetch delivery stats")
	}
	return c.JSON(fiber.Map{
		"delivery_stats": stats,
	})
}

// RetryFailedDeliveries retries all failed deliveries that haven't exceeded max retries.
// POST /api/v1/admin/notification_deliveries/retry
func (h *NotificationDeliveryHandler) RetryFailedDeliveries(c *fiber.Ctx) error {
	count, err := h.deliveryService.RetryFailedDeliveries(c.Context())
	if err != nil {
		return responses.InternalError(c, "Could not retry failed deliveries")
	}
	return c.JSON(fiber.Map{
		"retried": count,
		"message": "Failed deliveries have been queued for retry",
	})
}

// ListChannels lists the current user's communication channels.
// GET /api/v1/users/self/communication_channels
func (h *NotificationDeliveryHandler) ListChannels(c *fiber.Ctx) error {
	userID, err := getUserID(c)
	if err != nil {
		return err
	}

	channels, err := h.channelRepo.ListByUserID(c.Context(), userID)
	if err != nil {
		return responses.InternalError(c, "Could not fetch communication channels")
	}

	items := make([]fiber.Map, len(channels))
	for i, ch := range channels {
		items[i] = channelToJSON(&ch)
	}
	return c.JSON(items)
}

// CreateChannel adds a new communication channel for the current user.
// POST /api/v1/users/self/communication_channels
func (h *NotificationDeliveryHandler) CreateChannel(c *fiber.Ctx) error {
	userID, err := getUserID(c)
	if err != nil {
		return err
	}

	var input struct {
		CommunicationChannel struct {
			ChannelType string `json:"channel_type"`
			Address     string `json:"address"`
		} `json:"communication_channel"`
	}

	if err := c.BodyParser(&input); err != nil {
		return responses.BadRequest(c, "Invalid input")
	}

	channelType := input.CommunicationChannel.ChannelType
	address := input.CommunicationChannel.Address

	address = strings.TrimSpace(address)
	if channelType == "" || address == "" {
		return responses.BadRequest(c, "channel_type and address are required")
	}

	// Only channel types the delivery engine can actually send to. There
	// is no push transport: a "push" channel used to be accepted and then
	// fail every delivery queued for it.
	switch channelType {
	case "email":
		parsed, perr := mail.ParseAddress(address)
		if perr != nil || parsed.Address != address {
			return responses.BadRequest(c, "address must be a valid email address")
		}
	case "webhook":
		if err := security.ValidateExternalURL(c.Context(), address); err != nil {
			return responses.BadRequest(c, "webhook address must be a public http(s) URL")
		}
	default:
		return responses.BadRequest(c, "channel_type must be one of: email, webhook")
	}

	// Determine position (next available)
	existing, _ := h.channelRepo.ListByUserID(c.Context(), userID)
	position := len(existing) + 1

	channel := &models.CommunicationChannel{
		UserID:        userID,
		ChannelType:   channelType,
		Address:       address,
		Position:      position,
		WorkflowState: "active",
	}

	// Email channels stay unconfirmed (and receive nothing) until the
	// owner enters the code mailed to the address.
	code, err := h.deliveryService.BeginChannelConfirmation(channel)
	if err != nil {
		return responses.InternalError(c, "Could not create communication channel")
	}

	if err := h.channelRepo.Create(c.Context(), channel); err != nil {
		return responses.InternalError(c, "Could not create communication channel")
	}

	if code != "" {
		_ = h.deliveryService.SendChannelConfirmation(c.Context(), channel, code)
	}

	return c.Status(fiber.StatusCreated).JSON(channelToJSON(channel))
}

// ConfirmChannel confirms an email channel with the code mailed to it.
// POST /api/v1/users/self/communication_channels/:id/confirm  {code}
func (h *NotificationDeliveryHandler) ConfirmChannel(c *fiber.Ctx) error {
	userID, err := getUserID(c)
	if err != nil {
		return err
	}

	channelID, err := c.ParamsInt("id")
	if err != nil {
		return responses.BadRequest(c, "Invalid channel ID")
	}

	channel, err := h.channelRepo.FindByID(c.Context(), uint(channelID))
	// 404 (not 403) for another user's channel: existence-leak contract.
	if err != nil || channel.UserID != userID || channel.WorkflowState != "active" {
		return responses.NotFound(c, "communication channel")
	}

	var input struct {
		Code string `json:"code"`
	}
	if err := c.BodyParser(&input); err != nil {
		return responses.BadRequest(c, "Invalid input")
	}

	if err := h.deliveryService.ConfirmChannel(c.Context(), channel, input.Code); err != nil {
		if errors.Is(err, service.ErrInvalidConfirmationCode) {
			return responses.BadRequest(c, "Invalid confirmation code")
		}
		return responses.InternalError(c, "Could not confirm communication channel")
	}

	return c.JSON(channelToJSON(channel))
}

// DeleteChannel removes a communication channel for the current user (soft-delete via workflow_state).
// DELETE /api/v1/users/self/communication_channels/:id
func (h *NotificationDeliveryHandler) DeleteChannel(c *fiber.Ctx) error {
	userID, err := getUserID(c)
	if err != nil {
		return err
	}

	channelID, err := c.ParamsInt("id")
	if err != nil {
		return responses.BadRequest(c, "Invalid channel ID")
	}

	// Verify the channel belongs to the current user
	channel, err := h.channelRepo.FindByID(c.Context(), uint(channelID))
	if err != nil {
		return responses.NotFound(c, "communication channel")
	}

	// F-010: 404, not 403, on cross-user channel access. Per the
	// 13.1.E existence-leak contract, distinguishing "exists but
	// wrong owner" from "doesn't exist" leaks IDs.
	if channel.UserID != userID {
		return responses.NotFound(c, "communication channel")
	}

	if err := h.channelRepo.Delete(c.Context(), uint(channelID)); err != nil {
		return responses.InternalError(c, "Could not delete communication channel")
	}

	return c.JSON(fiber.Map{"success": true})
}
