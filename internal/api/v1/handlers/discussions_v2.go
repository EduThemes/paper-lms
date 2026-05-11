package handlers

import (
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/EduThemes/paper-lms/internal/api/v1/responses"
	"github.com/EduThemes/paper-lms/internal/service"
)

// DiscussionV2Handler provides enhanced discussion endpoints with read/unread tracking,
// user profile resolution, and edit history.
type DiscussionV2Handler struct {
	discussionV2Service *service.DiscussionV2Service
}

func NewDiscussionV2Handler(svc *service.DiscussionV2Service) *DiscussionV2Handler {
	return &DiscussionV2Handler{discussionV2Service: svc}
}

func entryViewV2ToJSON(e *service.EntryViewV2) fiber.Map {
	replies := make([]fiber.Map, len(e.Replies))
	for i := range e.Replies {
		replies[i] = entryViewV2ToJSON(&e.Replies[i])
	}

	return fiber.Map{
		"id":                  e.ID,
		"discussion_topic_id": e.DiscussionTopicID,
		"user_id":             e.UserID,
		"user_name":           e.UserName,
		"user_avatar_url":     e.UserAvatarURL,
		"parent_id":           e.ParentID,
		"message":             e.Message,
		"rating_count":        e.RatingCount,
		"rating_sum":          e.RatingSum,
		"read_state":          e.ReadState,
		"edited_at":           e.EditedAt,
		"version_count":       e.VersionCount,
		"workflow_state":      e.WorkflowState,
		"created_at":          e.CreatedAt,
		"updated_at":          e.UpdatedAt,
		"replies":             replies,
	}
}

// GetFullViewV2 returns the topic + tree with read states and user info.
// GET /topics/:topic_id/view
func (h *DiscussionV2Handler) GetFullViewV2(c *fiber.Ctx) error {
	topicID, err := c.ParamsInt("topic_id")
	if err != nil {
		return responses.BadRequest(c, "Invalid topic ID")
	}

	userID, _ := c.Locals("user_id").(uint)

	view, err := h.discussionV2Service.GetFullViewWithReadState(c.Context(), uint(topicID), userID)
	if err != nil {
		return responses.NotFound(c, "discussion topic")
	}

	entries := make([]fiber.Map, len(view.Entries))
	for i := range view.Entries {
		entries[i] = entryViewV2ToJSON(&view.Entries[i])
	}

	topicJSON := topicToJSON(view.Topic)
	// Resolve topic author name
	if view.Topic.UserID > 0 {
		userInfoMap := h.discussionV2Service.ResolveUserInfo(c.Context(), []uint{view.Topic.UserID})
		if info, ok := userInfoMap[view.Topic.UserID]; ok {
			topicJSON["user_name"] = info.Name
			topicJSON["user_avatar_url"] = info.AvatarURL
		}
	}

	return c.JSON(fiber.Map{
		"topic":   topicJSON,
		"entries": entries,
	})
}

// MarkEntryRead marks a single entry as read.
// POST /entries/:entry_id/read
func (h *DiscussionV2Handler) MarkEntryRead(c *fiber.Ctx) error {
	entryID, err := c.ParamsInt("entry_id")
	if err != nil {
		return responses.BadRequest(c, "Invalid entry ID")
	}

	userID, _ := c.Locals("user_id").(uint)

	if err := h.discussionV2Service.MarkEntryAsRead(c.Context(), uint(entryID), userID); err != nil {
		return responses.InternalError(c, "Could not mark entry as read")
	}

	return c.JSON(fiber.Map{"status": "read"})
}

// MarkTopicRead marks all entries in the topic as read.
// POST /topics/:topic_id/mark_all_read
func (h *DiscussionV2Handler) MarkTopicRead(c *fiber.Ctx) error {
	topicID, err := c.ParamsInt("topic_id")
	if err != nil {
		return responses.BadRequest(c, "Invalid topic ID")
	}

	userID, _ := c.Locals("user_id").(uint)

	if err := h.discussionV2Service.MarkTopicAsRead(c.Context(), uint(topicID), userID); err != nil {
		return responses.InternalError(c, "Could not mark topic as read")
	}

	return c.JSON(fiber.Map{"status": "all_read"})
}

// GetUnreadCount returns the unread entry count for a topic.
// GET /topics/:topic_id/unread_count
func (h *DiscussionV2Handler) GetUnreadCount(c *fiber.Ctx) error {
	topicID, err := c.ParamsInt("topic_id")
	if err != nil {
		return responses.BadRequest(c, "Invalid topic ID")
	}

	userID, _ := c.Locals("user_id").(uint)

	count, err := h.discussionV2Service.GetUnreadCount(c.Context(), uint(topicID), userID)
	if err != nil {
		return responses.InternalError(c, "Could not get unread count")
	}

	return c.JSON(fiber.Map{"unread_count": count})
}

// ToggleSubscription sets subscription for the topic.
// PUT /topics/:topic_id/subscription
func (h *DiscussionV2Handler) ToggleSubscription(c *fiber.Ctx) error {
	topicID, err := c.ParamsInt("topic_id")
	if err != nil {
		return responses.BadRequest(c, "Invalid topic ID")
	}

	var input struct {
		Subscribed bool `json:"subscribed"`
	}

	if err := c.BodyParser(&input); err != nil {
		return responses.BadRequest(c, "Invalid input")
	}

	userID, _ := c.Locals("user_id").(uint)

	if err := h.discussionV2Service.ToggleSubscription(c.Context(), uint(topicID), userID, input.Subscribed); err != nil {
		return responses.InternalError(c, "Could not update subscription")
	}

	return c.JSON(fiber.Map{"subscribed": input.Subscribed})
}

// GetEntryVersions returns the edit history for a discussion entry.
// GET /entries/:entry_id/versions
func (h *DiscussionV2Handler) GetEntryVersions(c *fiber.Ctx) error {
	entryID, err := c.ParamsInt("entry_id")
	if err != nil {
		return responses.BadRequest(c, "Invalid entry ID")
	}

	versions, err := h.discussionV2Service.GetEntryVersions(c.Context(), uint(entryID))
	if err != nil {
		return responses.InternalError(c, "Could not fetch entry versions")
	}

	result := make([]fiber.Map, len(versions))
	for i, v := range versions {
		result[i] = fiber.Map{
			"id":                  v.ID,
			"discussion_entry_id": v.DiscussionEntryID,
			"user_id":             v.UserID,
			"message":             v.Message,
			"version":             v.Version,
			"created_at":          v.CreatedAt,
		}
	}

	return c.JSON(result)
}

// UpdateEntryV2 updates a discussion entry, saving the old version to history.
// PUT /entries/:entry_id
func (h *DiscussionV2Handler) UpdateEntryV2(c *fiber.Ctx) error {
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

	if input.Message == "" {
		return responses.BadRequest(c, "Message is required")
	}

	userID, _ := c.Locals("user_id").(uint)

	if err := h.discussionV2Service.UpdateEntryWithHistory(c.Context(), uint(entryID), userID, input.Message); err != nil {
		return responses.InternalError(c, "Could not update entry")
	}

	// Return the updated entry with V2 fields
	entry, err := h.discussionV2Service.GetEntry(c.Context(), uint(entryID))
	if err != nil {
		return responses.InternalError(c, "Could not fetch updated entry")
	}

	versionCount, _ := h.discussionV2Service.GetEntryVersions(c.Context(), uint(entryID))
	userInfo := h.discussionV2Service.ResolveUserInfo(c.Context(), []uint{entry.UserID})
	info := userInfo[entry.UserID]

	var editedAt *time.Time
	if len(versionCount) > 0 {
		editedAt = &entry.UpdatedAt
	}

	return c.JSON(fiber.Map{
		"id":                  entry.ID,
		"discussion_topic_id": entry.DiscussionTopicID,
		"user_id":             entry.UserID,
		"user_name":           info.Name,
		"user_avatar_url":     info.AvatarURL,
		"parent_id":           entry.ParentID,
		"message":             entry.Message,
		"rating_count":        entry.RatingCount,
		"rating_sum":          entry.RatingSum,
		"edited_at":           editedAt,
		"version_count":       len(versionCount),
		"workflow_state":      entry.WorkflowState,
		"created_at":          entry.CreatedAt,
		"updated_at":          entry.UpdatedAt,
	})
}
