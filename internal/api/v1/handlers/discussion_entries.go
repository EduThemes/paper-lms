package handlers

import (
	"errors"

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
	topic, wrote := requireTopicInCourse(c, h.discussionService)
	if wrote {
		return nil
	}

	params := middleware.GetPagination(c)

	result, err := h.discussionService.ListEntries(c.Context(), topic.ID, callerAccountID(c), params)
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
	// The topic must be in this course and tenant, visible to the caller
	// (not deleted / unpublished / delayed) and not locked for them.
	topic, wrote := requireTopicInCourse(c, h.discussionService)
	if wrote {
		return nil
	}
	if requireTopicOpenForCaller(c, topic) {
		return nil
	}

	var input struct {
		Message string `json:"message"`
	}

	if err := c.BodyParser(&input); err != nil {
		return responses.BadRequest(c, "Invalid input")
	}

	userID, _ := c.Locals("user_id").(uint)

	entry := &models.DiscussionEntry{
		DiscussionTopicID: topic.ID,
		UserID:            userID,
		Message:           service.SanitizeHTML(input.Message),
	}

	if err := h.discussionService.CreateEntry(c.Context(), entry); err != nil {
		return responses.BadRequest(c, err.Error())
	}

	return c.Status(fiber.StatusCreated).JSON(entryToJSON(entry))
}

// UpdateEntry — PUT /courses/:course_id/discussion_topics/:topic_id/entries/:id
func (h *DiscussionEntryHandler) UpdateEntry(c *fiber.Ctx) error {
	topic, wrote := requireTopicInCourse(c, h.discussionService)
	if wrote {
		return nil
	}
	entry, wrote := requireEntryInTopic(c, h.discussionService, topic.ID, "id")
	if wrote {
		return nil
	}
	if requireEntryModerator(c, topic, entry) {
		return nil
	}

	var input struct {
		Message string `json:"message"`
	}

	if err := c.BodyParser(&input); err != nil {
		return responses.BadRequest(c, "Invalid input")
	}

	if input.Message != "" {
		entry.Message = service.SanitizeHTML(input.Message)
	}

	if err := h.discussionService.UpdateEntry(c.Context(), entry); err != nil {
		return responses.InternalError(c, "Could not update discussion entry")
	}

	return c.JSON(entryToJSON(entry))
}

// DeleteEntry — DELETE /courses/:course_id/discussion_topics/:topic_id/entries/:id
// The author may delete their own post (while the topic is open for
// them); course staff may delete any post.
func (h *DiscussionEntryHandler) DeleteEntry(c *fiber.Ctx) error {
	topic, wrote := requireTopicInCourse(c, h.discussionService)
	if wrote {
		return nil
	}
	entry, wrote := requireEntryInTopic(c, h.discussionService, topic.ID, "id")
	if wrote {
		return nil
	}
	if requireEntryModerator(c, topic, entry) {
		return nil
	}

	if err := h.discussionService.DeleteEntry(c.Context(), entry.ID); err != nil {
		return responses.InternalError(c, "Could not delete discussion entry")
	}

	return c.JSON(fiber.Map{"delete": true})
}

func (h *DiscussionEntryHandler) ListReplies(c *fiber.Ctx) error {
	topic, wrote := requireTopicInCourse(c, h.discussionService)
	if wrote {
		return nil
	}
	parent, wrote := requireEntryInTopic(c, h.discussionService, topic.ID, "entry_id")
	if wrote {
		return nil
	}

	params := middleware.GetPagination(c)

	result, err := h.discussionService.ListReplies(c.Context(), parent.ID, callerAccountID(c), params)
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
	topic, wrote := requireTopicInCourse(c, h.discussionService)
	if wrote {
		return nil
	}
	if requireTopicOpenForCaller(c, topic) {
		return nil
	}
	// The parent must be a live entry on this same topic; a parent from
	// another topic would otherwise be orphaned into a root post.
	parent, wrote := requireEntryInTopic(c, h.discussionService, topic.ID, "entry_id")
	if wrote {
		return nil
	}

	var input struct {
		Message string `json:"message"`
	}

	if err := c.BodyParser(&input); err != nil {
		return responses.BadRequest(c, "Invalid input")
	}

	userID, _ := c.Locals("user_id").(uint)
	parentID := parent.ID

	entry := &models.DiscussionEntry{
		DiscussionTopicID: topic.ID,
		UserID:            userID,
		ParentID:          &parentID,
		Message:           service.SanitizeHTML(input.Message),
	}

	if err := h.discussionService.CreateEntry(c.Context(), entry); err != nil {
		return responses.BadRequest(c, err.Error())
	}

	return c.Status(fiber.StatusCreated).JSON(entryToJSON(entry))
}

// RateEntry — POST .../entries/:entry_id/rating with {"rating": 0|1}.
// Matches Canvas rate_entry: 1 likes, 0 removes the like; any other
// value is rejected. Only allowed when the topic allows rating, and only
// for graders when only_graders_can_rate is set.
func (h *DiscussionEntryHandler) RateEntry(c *fiber.Ctx) error {
	topic, wrote := requireTopicInCourse(c, h.discussionService)
	if wrote {
		return nil
	}
	entry, wrote := requireEntryInTopic(c, h.discussionService, topic.ID, "entry_id")
	if wrote {
		return nil
	}
	if !discussionCallerMayRate(c, topic) {
		return responses.Forbidden(c, "Rating is not allowed on this discussion")
	}

	var input struct {
		Rating *int `json:"rating"`
	}

	if err := c.BodyParser(&input); err != nil {
		return responses.BadRequest(c, "Invalid input")
	}
	if input.Rating == nil || (*input.Rating != 0 && *input.Rating != 1) {
		return responses.BadRequest(c, "rating must be 0 or 1")
	}

	userID, _ := c.Locals("user_id").(uint)

	if err := h.discussionService.RateEntry(c.Context(), entry.ID, userID, *input.Rating); err != nil {
		if errors.Is(err, service.ErrDiscussionInvalidRating) {
			return responses.BadRequest(c, err.Error())
		}
		return responses.InternalError(c, "Could not rate entry")
	}

	// Return the updated entry
	updated, err := h.discussionService.GetEntry(c.Context(), entry.ID, callerAccountID(c))
	if err != nil {
		return responses.InternalError(c, "Could not fetch updated entry")
	}

	out := entryToJSON(updated)
	out["rating"] = *input.Rating
	return c.JSON(out)
}
