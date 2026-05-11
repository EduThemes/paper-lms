package handlers

import (
	"github.com/gofiber/fiber/v2"
	"github.com/EduThemes/paper-lms/internal/api/v1/middleware"
	"github.com/EduThemes/paper-lms/internal/api/v1/responses"
	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/service"
)

type DiscussionEntryHandler struct {
	discussionService *service.DiscussionService
}

func NewDiscussionEntryHandler(discussionService *service.DiscussionService) *DiscussionEntryHandler {
	return &DiscussionEntryHandler{discussionService: discussionService}
}

func entryToJSON(e *models.DiscussionEntry) fiber.Map {
	return fiber.Map{
		"id":                  e.ID,
		"discussion_topic_id": e.DiscussionTopicID,
		"user_id":             e.UserID,
		"parent_id":           e.ParentID,
		"message":             e.Message,
		"rating_count":        e.RatingCount,
		"rating_sum":          e.RatingSum,
		"workflow_state":      e.WorkflowState,
		"created_at":          e.CreatedAt,
		"updated_at":          e.UpdatedAt,
	}
}

func (h *DiscussionEntryHandler) ListEntries(c *fiber.Ctx) error {
	topicID, err := c.ParamsInt("topic_id")
	if err != nil {
		return responses.BadRequest(c, "Invalid topic ID")
	}

	params := middleware.GetPagination(c)

	result, err := h.discussionService.ListEntries(c.Context(), uint(topicID), params)
	if err != nil {
		return responses.InternalError(c, "Could not fetch discussion entries")
	}

	responses.SetPaginationHeaders(c, result.TotalCount, result.Page, result.PerPage)

	entries := make([]fiber.Map, len(result.Items))
	for i, e := range result.Items {
		entries[i] = entryToJSON(&e)
	}

	return c.JSON(entries)
}

func (h *DiscussionEntryHandler) CreateEntry(c *fiber.Ctx) error {
	topicID, err := c.ParamsInt("topic_id")
	if err != nil {
		return responses.BadRequest(c, "Invalid topic ID")
	}

	var input struct {
		Message string `json:"message"`
	}

	if err := c.BodyParser(&input); err != nil {
		return responses.BadRequest(c, "Invalid input")
	}

	userID, _ := c.Locals("user_id").(uint)

	entry := &models.DiscussionEntry{
		DiscussionTopicID: uint(topicID),
		UserID:            userID,
		Message:           input.Message,
	}

	if err := h.discussionService.CreateEntry(c.Context(), entry); err != nil {
		return responses.BadRequest(c, err.Error())
	}

	return c.Status(fiber.StatusCreated).JSON(entryToJSON(entry))
}

func (h *DiscussionEntryHandler) UpdateEntry(c *fiber.Ctx) error {
	id, err := c.ParamsInt("entry_id")
	if err != nil {
		return responses.BadRequest(c, "Invalid entry ID")
	}

	entry, err := h.discussionService.GetEntry(c.Context(), uint(id))
	if err != nil {
		return responses.NotFound(c, "discussion entry")
	}

	var input struct {
		Message string `json:"message"`
	}

	if err := c.BodyParser(&input); err != nil {
		return responses.BadRequest(c, "Invalid input")
	}

	if input.Message != "" {
		entry.Message = input.Message
	}

	if err := h.discussionService.UpdateEntry(c.Context(), entry); err != nil {
		return responses.InternalError(c, "Could not update discussion entry")
	}

	return c.JSON(entryToJSON(entry))
}

func (h *DiscussionEntryHandler) DeleteEntry(c *fiber.Ctx) error {
	id, err := c.ParamsInt("entry_id")
	if err != nil {
		return responses.BadRequest(c, "Invalid entry ID")
	}

	if err := h.discussionService.DeleteEntry(c.Context(), uint(id)); err != nil {
		return responses.InternalError(c, "Could not delete discussion entry")
	}

	return c.JSON(fiber.Map{"delete": true})
}

func (h *DiscussionEntryHandler) ListReplies(c *fiber.Ctx) error {
	entryID, err := c.ParamsInt("entry_id")
	if err != nil {
		return responses.BadRequest(c, "Invalid entry ID")
	}

	params := middleware.GetPagination(c)

	result, err := h.discussionService.ListReplies(c.Context(), uint(entryID), params)
	if err != nil {
		return responses.InternalError(c, "Could not fetch replies")
	}

	responses.SetPaginationHeaders(c, result.TotalCount, result.Page, result.PerPage)

	entries := make([]fiber.Map, len(result.Items))
	for i, e := range result.Items {
		entries[i] = entryToJSON(&e)
	}

	return c.JSON(entries)
}

func (h *DiscussionEntryHandler) CreateReply(c *fiber.Ctx) error {
	topicID, err := c.ParamsInt("topic_id")
	if err != nil {
		return responses.BadRequest(c, "Invalid topic ID")
	}

	entryID, err := c.ParamsInt("entry_id")
	if err != nil {
		return responses.BadRequest(c, "Invalid entry ID")
	}

	var input struct {
		Message string `json:"message"`
	}

	if err := c.BodyParser(&input); err != nil {
		return responses.BadRequest(c, "Invalid input")
	}

	userID, _ := c.Locals("user_id").(uint)
	parentID := uint(entryID)

	entry := &models.DiscussionEntry{
		DiscussionTopicID: uint(topicID),
		UserID:            userID,
		ParentID:          &parentID,
		Message:           input.Message,
	}

	if err := h.discussionService.CreateEntry(c.Context(), entry); err != nil {
		return responses.BadRequest(c, err.Error())
	}

	return c.Status(fiber.StatusCreated).JSON(entryToJSON(entry))
}

func (h *DiscussionEntryHandler) RateEntry(c *fiber.Ctx) error {
	entryID, err := c.ParamsInt("entry_id")
	if err != nil {
		return responses.BadRequest(c, "Invalid entry ID")
	}

	var input struct {
		Rating int `json:"rating"`
	}

	if err := c.BodyParser(&input); err != nil {
		return responses.BadRequest(c, "Invalid input")
	}

	userID, _ := c.Locals("user_id").(uint)

	if err := h.discussionService.RateEntry(c.Context(), uint(entryID), userID, input.Rating); err != nil {
		return responses.InternalError(c, "Could not rate entry")
	}

	// Return the updated entry
	entry, err := h.discussionService.GetEntry(c.Context(), uint(entryID))
	if err != nil {
		return responses.InternalError(c, "Could not fetch updated entry")
	}

	return c.JSON(entryToJSON(entry))
}
