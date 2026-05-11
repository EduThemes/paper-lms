package handlers

import (
	"github.com/gofiber/fiber/v2"
	"github.com/EduThemes/paper-lms/internal/api/v1/middleware"
	"github.com/EduThemes/paper-lms/internal/api/v1/responses"
	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/service"
)

type NotificationHandler struct {
	notificationService *service.NotificationService
}

func NewNotificationHandler(notificationService *service.NotificationService) *NotificationHandler {
	return &NotificationHandler{notificationService: notificationService}
}

func notificationToJSON(n *models.Notification) fiber.Map {
	return fiber.Map{
		"id":                n.ID,
		"user_id":           n.UserID,
		"notification_type": n.NotificationType,
		"title":             n.Title,
		"message":           n.Message,
		"context_type":      n.ContextType,
		"context_id":        n.ContextID,
		"related_user_id":   n.RelatedUserID,
		"is_read":           n.IsRead,
		"sent_at":           n.SentAt,
		"created_at":        n.CreatedAt,
		"updated_at":        n.UpdatedAt,
	}
}

func notificationPreferenceToJSON(p *models.NotificationPreference) fiber.Map {
	return fiber.Map{
		"id":                      p.ID,
		"user_id":                 p.UserID,
		"policy":                  p.Policy,
		"notify_new_message":      p.NotifyNewMessage,
		"notify_event_start":      p.NotifyEventStart,
		"notify_submission_grade": p.NotifySubmissionGrade,
		"notify_new_announcement": p.NotifyNewAnnouncement,
		"created_at":              p.CreatedAt,
		"updated_at":              p.UpdatedAt,
	}
}

// ListNotifications returns a paginated list of notifications for the authenticated user.
// GET /api/v1/notifications
// Query param "unread=true" filters to unread only.
func (h *NotificationHandler) ListNotifications(c *fiber.Ctx) error {
	userID, err := getUserID(c)
	if err != nil {
		return err
	}
	params := middleware.GetPagination(c)

	unread := c.Query("unread")
	if unread == "true" {
		r, err := h.notificationService.ListUnreadByUser(c.Context(), userID, params)
		if err != nil {
			return responses.InternalError(c, "Could not fetch notifications")
		}
		responses.SetPaginationHeaders(c, r.TotalCount, r.Page, r.PerPage)
		items := make([]fiber.Map, len(r.Items))
		for i, n := range r.Items {
			items[i] = notificationToJSON(&n)
		}
		return c.JSON(items)
	}

	r, err := h.notificationService.ListByUser(c.Context(), userID, params)
	if err != nil {
		return responses.InternalError(c, "Could not fetch notifications")
	}

	responses.SetPaginationHeaders(c, r.TotalCount, r.Page, r.PerPage)

	items := make([]fiber.Map, len(r.Items))
	for i, n := range r.Items {
		items[i] = notificationToJSON(&n)
	}

	return c.JSON(items)
}

// MarkAsRead marks a single notification as read.
// PUT /api/v1/notifications/:id/mark_as_read
func (h *NotificationHandler) MarkAsRead(c *fiber.Ctx) error {
	userID, err := getUserID(c)
	if err != nil {
		return err
	}

	notificationID, err := c.ParamsInt("id")
	if err != nil {
		return responses.BadRequest(c, "Invalid notification ID")
	}

	if err := h.notificationService.MarkAsRead(c.Context(), userID, uint(notificationID)); err != nil {
		return responses.InternalError(c, "Could not mark notification as read")
	}

	return c.JSON(fiber.Map{"success": true})
}

// MarkAllAsRead marks all notifications as read for the authenticated user.
// PUT /api/v1/notifications/mark_all_as_read
func (h *NotificationHandler) MarkAllAsRead(c *fiber.Ctx) error {
	userID, err := getUserID(c)
	if err != nil {
		return err
	}

	if err := h.notificationService.MarkAllAsRead(c.Context(), userID); err != nil {
		return responses.InternalError(c, "Could not mark notifications as read")
	}

	return c.JSON(fiber.Map{"success": true})
}

// GetPreferences returns the notification preferences for the authenticated user.
// GET /api/v1/users/self/notification_preferences
func (h *NotificationHandler) GetPreferences(c *fiber.Ctx) error {
	userID, err := getUserID(c)
	if err != nil {
		return err
	}

	prefs, err := h.notificationService.GetOrCreatePreferences(c.Context(), userID)
	if err != nil {
		return responses.InternalError(c, "Could not fetch notification preferences")
	}

	return c.JSON(fiber.Map{"notification_preferences": notificationPreferenceToJSON(prefs)})
}

// UpdatePreferences updates the notification preferences for the authenticated user.
// PUT /api/v1/users/self/notification_preferences
func (h *NotificationHandler) UpdatePreferences(c *fiber.Ctx) error {
	userID, err := getUserID(c)
	if err != nil {
		return err
	}

	var input struct {
		NotificationPreference struct {
			Policy                *string `json:"policy"`
			NotifyNewMessage      *bool   `json:"notify_new_message"`
			NotifyEventStart      *bool   `json:"notify_event_start"`
			NotifySubmissionGrade *bool   `json:"notify_submission_grade"`
			NotifyNewAnnouncement *bool   `json:"notify_new_announcement"`
		} `json:"notification_preference"`
	}

	if err := c.BodyParser(&input); err != nil {
		return responses.BadRequest(c, "Invalid input")
	}

	prefs, err := h.notificationService.GetOrCreatePreferences(c.Context(), userID)
	if err != nil {
		return responses.InternalError(c, "Could not fetch notification preferences")
	}

	if input.NotificationPreference.Policy != nil {
		prefs.Policy = *input.NotificationPreference.Policy
	}
	if input.NotificationPreference.NotifyNewMessage != nil {
		prefs.NotifyNewMessage = *input.NotificationPreference.NotifyNewMessage
	}
	if input.NotificationPreference.NotifyEventStart != nil {
		prefs.NotifyEventStart = *input.NotificationPreference.NotifyEventStart
	}
	if input.NotificationPreference.NotifySubmissionGrade != nil {
		prefs.NotifySubmissionGrade = *input.NotificationPreference.NotifySubmissionGrade
	}
	if input.NotificationPreference.NotifyNewAnnouncement != nil {
		prefs.NotifyNewAnnouncement = *input.NotificationPreference.NotifyNewAnnouncement
	}

	if err := h.notificationService.UpdatePreferences(c.Context(), prefs); err != nil {
		return responses.BadRequest(c, err.Error())
	}

	return c.JSON(fiber.Map{"notification_preferences": notificationPreferenceToJSON(prefs)})
}
