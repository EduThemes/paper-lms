package handlers

import (
	"github.com/gofiber/fiber/v2"
	"github.com/EduThemes/paper-lms/internal/api/v1/middleware"
	"github.com/EduThemes/paper-lms/internal/api/v1/responses"
	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/service"
)

type ConversationHandler struct {
	conversationService *service.ConversationService
	userService         *service.UserService
}

func NewConversationHandler(conversationService *service.ConversationService, userService *service.UserService) *ConversationHandler {
	return &ConversationHandler{conversationService: conversationService, userService: userService}
}

func conversationToJSON(c *models.Conversation) fiber.Map {
	return fiber.Map{
		"id":                 c.ID,
		"subject":            c.Subject,
		"created_by_user_id": c.CreatedByUserID,
		"last_message_at":    c.LastMessageAt,
		"workflow_state":     c.WorkflowState,
		"created_at":         c.CreatedAt,
		"updated_at":         c.UpdatedAt,
	}
}

func conversationMessageToJSON(m *models.ConversationMessage) fiber.Map {
	return fiber.Map{
		"id":              m.ID,
		"conversation_id": m.ConversationID,
		"user_id":         m.UserID,
		"body":            m.Body,
		"workflow_state":  m.WorkflowState,
		"created_at":      m.CreatedAt,
		"updated_at":      m.UpdatedAt,
	}
}

// requireParticipant checks the authenticated user is a participant of the conversation.
func (h *ConversationHandler) requireParticipant(c *fiber.Ctx, conversationID uint) error {
	userID, _ := c.Locals("user_id").(uint)
	participants, err := h.conversationService.GetParticipants(c.Context(), conversationID)
	if err != nil {
		return responses.Error(c, fiber.StatusForbidden, "Could not verify conversation access")
	}
	for _, p := range participants {
		if p.UserID == userID {
			return nil
		}
	}
	return responses.Error(c, fiber.StatusForbidden, "You are not a participant in this conversation")
}

func (h *ConversationHandler) resolveParticipants(c *fiber.Ctx, conversationID uint) []fiber.Map {
	participants, err := h.conversationService.GetParticipants(c.Context(), conversationID)
	if err != nil {
		return nil
	}
	result := make([]fiber.Map, 0, len(participants))
	for _, p := range participants {
		name := ""
		if user, err := h.userService.GetByID(c.Context(), p.UserID); err == nil {
			name = user.Name
		}
		result = append(result, fiber.Map{
			"id":   p.UserID,
			"name": name,
		})
	}
	return result
}

func (h *ConversationHandler) ListConversations(c *fiber.Ctx) error {
	userID, _ := c.Locals("user_id").(uint)

	params := middleware.GetPagination(c)

	result, err := h.conversationService.ListByUser(c.Context(), userID, params)
	if err != nil {
		return responses.InternalError(c, "Could not fetch conversations")
	}

	responses.SetPaginationHeaders(c, result.TotalCount, result.Page, result.PerPage)

	conversations := make([]fiber.Map, len(result.Items))
	for i, conv := range result.Items {
		j := conversationToJSON(&conv)
		j["participants"] = h.resolveParticipants(c, conv.ID)
		conversations[i] = j
	}

	return c.JSON(conversations)
}

func (h *ConversationHandler) GetConversation(c *fiber.Ctx) error {
	id, err := c.ParamsInt("id")
	if err != nil {
		return responses.BadRequest(c, "Invalid conversation ID")
	}

	if err := h.requireParticipant(c, uint(id)); err != nil {
		return err
	}

	conv, err := h.conversationService.GetConversation(c.Context(), uint(id))
	if err != nil {
		return responses.NotFound(c, "conversation")
	}

	j := conversationToJSON(conv)
	j["participants"] = h.resolveParticipants(c, conv.ID)
	return c.JSON(j)
}

func (h *ConversationHandler) CreateConversation(c *fiber.Ctx) error {
	var input struct {
		Conversation struct {
			Subject    string `json:"subject"`
			Recipients []uint `json:"recipients"`
		} `json:"conversation"`
	}

	if err := c.BodyParser(&input); err != nil {
		return responses.BadRequest(c, "Invalid input")
	}

	userID, _ := c.Locals("user_id").(uint)

	conv := &models.Conversation{
		Subject:         input.Conversation.Subject,
		CreatedByUserID: userID,
	}

	if err := h.conversationService.CreateConversation(c.Context(), conv, input.Conversation.Recipients); err != nil {
		return responses.BadRequest(c, err.Error())
	}

	return c.Status(fiber.StatusCreated).JSON(conversationToJSON(conv))
}

func (h *ConversationHandler) UpdateConversation(c *fiber.Ctx) error {
	id, err := c.ParamsInt("id")
	if err != nil {
		return responses.BadRequest(c, "Invalid conversation ID")
	}

	if err := h.requireParticipant(c, uint(id)); err != nil {
		return err
	}

	conv, err := h.conversationService.GetConversation(c.Context(), uint(id))
	if err != nil {
		return responses.NotFound(c, "conversation")
	}

	var input struct {
		Conversation struct {
			WorkflowState *string `json:"workflow_state"`
			Subject       *string `json:"subject"`
		} `json:"conversation"`
	}

	if err := c.BodyParser(&input); err != nil {
		return responses.BadRequest(c, "Invalid input")
	}

	if input.Conversation.WorkflowState != nil {
		conv.WorkflowState = *input.Conversation.WorkflowState
	}
	if input.Conversation.Subject != nil {
		conv.Subject = *input.Conversation.Subject
	}

	if err := h.conversationService.UpdateConversation(c.Context(), conv); err != nil {
		return responses.InternalError(c, "Could not update conversation")
	}

	return c.JSON(conversationToJSON(conv))
}

func (h *ConversationHandler) ListMessages(c *fiber.Ctx) error {
	id, err := c.ParamsInt("id")
	if err != nil {
		return responses.BadRequest(c, "Invalid conversation ID")
	}

	if err := h.requireParticipant(c, uint(id)); err != nil {
		return err
	}

	params := middleware.GetPagination(c)

	result, err := h.conversationService.ListMessages(c.Context(), uint(id), params)
	if err != nil {
		return responses.InternalError(c, "Could not fetch messages")
	}

	responses.SetPaginationHeaders(c, result.TotalCount, result.Page, result.PerPage)

	messages := make([]fiber.Map, len(result.Items))
	for i, m := range result.Items {
		j := conversationMessageToJSON(&m)
		if user, err := h.userService.GetByID(c.Context(), m.UserID); err == nil {
			j["user_name"] = user.Name
		}
		messages[i] = j
	}

	return c.JSON(messages)
}

func (h *ConversationHandler) CreateMessage(c *fiber.Ctx) error {
	id, err := c.ParamsInt("id")
	if err != nil {
		return responses.BadRequest(c, "Invalid conversation ID")
	}

	if err := h.requireParticipant(c, uint(id)); err != nil {
		return err
	}

	var input struct {
		Message string `json:"message"`
	}

	if err := c.BodyParser(&input); err != nil {
		return responses.BadRequest(c, "Invalid input")
	}

	userID, _ := c.Locals("user_id").(uint)

	msg := &models.ConversationMessage{
		ConversationID: uint(id),
		UserID:         userID,
		Body:           input.Message,
	}

	if err := h.conversationService.CreateMessage(c.Context(), msg); err != nil {
		return responses.BadRequest(c, err.Error())
	}

	j := conversationMessageToJSON(msg)
	if user, err := h.userService.GetByID(c.Context(), userID); err == nil {
		j["user_name"] = user.Name
	}
	return c.Status(fiber.StatusCreated).JSON(j)
}

func (h *ConversationHandler) MarkAsRead(c *fiber.Ctx) error {
	id, err := c.ParamsInt("id")
	if err != nil {
		return responses.BadRequest(c, "Invalid conversation ID")
	}

	userID, _ := c.Locals("user_id").(uint)

	if err := h.conversationService.MarkConversationAsRead(c.Context(), uint(id), userID); err != nil {
		return responses.InternalError(c, "Could not mark conversation as read")
	}

	return c.JSON(fiber.Map{"status": "ok"})
}
