package handlers

import (
	"time"

	"github.com/EduThemes/paper-lms/internal/api/v1/responses"
	"github.com/EduThemes/paper-lms/internal/service"
	"github.com/gofiber/fiber/v2"
)

// DiscussionV2Handler provides enhanced discussion endpoints with read/unread tracking,
// user profile resolution, and edit history.
//
// Every endpoint resolves :topic_id with requireTopicInCourse (tenant +
// F-013 course tie + visibility) and any entry id with
// requireEntryInTopic before touching the service.
type DiscussionV2Handler struct {
	discussionV2Service *service.DiscussionV2Service
	// parity adds attachments, anonymity, group and graded topic details
	// and @mentions. Optional (nil in some tests).
	parity *service.DiscussionParityService
}

func NewDiscussionV2Handler(svc *service.DiscussionV2Service) *DiscussionV2Handler {
	return &DiscussionV2Handler{discussionV2Service: svc}
}

// SetParity enables the Canvas-parity discussion features on the V2 view.
func (h *DiscussionV2Handler) SetParity(p *service.DiscussionParityService) *DiscussionV2Handler {
	h.parity = p
	return h
}

func entryViewV2ToJSON(e *service.EntryViewV2, p *discussionPresenter) fiber.Map {
	replies := make([]fiber.Map, len(e.Replies))
	for i := range e.Replies {
		replies[i] = entryViewV2ToJSON(&e.Replies[i], p)
	}

	m := fiber.Map{
		"id":                  e.ID,
		"discussion_topic_id": e.DiscussionTopicID,
		"user_id":             e.UserID,
		"user_name":           e.UserName,
		"user_avatar_url":     e.UserAvatarURL,
		"parent_id":           e.ParentID,
		"message":             e.Message,
		"rating_count":        e.RatingCount,
		"rating_sum":          e.RatingSum,
		"rating":              e.Rating,
		"read_state":          e.ReadState,
		"edited_at":           e.EditedAt,
		"version_count":       e.VersionCount,
		"workflow_state":      e.WorkflowState,
		"attachment_id":       e.AttachmentID,
		"is_anonymous_author": e.IsAnonymousAuthor,
		"created_at":          e.CreatedAt,
		"updated_at":          e.UpdatedAt,
		"replies":             replies,
	}
	p.decorate(m, discussionEntryRef{
		ID: e.ID, UserID: e.UserID, IsAnonymousAuthor: e.IsAnonymousAuthor,
		AttachmentID: e.AttachmentID, Deleted: e.WorkflowState == discussionEntryDeletedState,
	})
	return m
}

// GetFullViewV2 returns the topic + tree with read states and user info.
// GET /courses/:course_id/discussion_topics/:topic_id/view_v2
func (h *DiscussionV2Handler) GetFullViewV2(c *fiber.Ctx) error {
	topic, wrote := requireTopicInCourse(c, h.discussionV2Service)
	if wrote {
		return nil
	}

	userID, _ := c.Locals("user_id").(uint)

	view, err := h.discussionV2Service.GetFullViewWithReadState(c.Context(), topic, userID, callerAccountID(c))
	if err != nil {
		return responses.InternalError(c, "Could not load discussion")
	}

	topicJSON := topicToJSON(view.Topic)
	addDiscussionTopicExtras(c, h.discussionV2Service, h.parity, view.Topic, topicJSON)
	topicJSON["subscribed"] = view.Subscribed
	topicJSON["can_rate"] = discussionCallerMayRate(c, view.Topic)
	topicJSON["locked_for_user"] = discussionTopicLockedForCaller(c, view.Topic)

	// Someone who must post first, and the root of a group discussion
	// (posts live on the group topics), get no entries.
	entries := []fiber.Map{}
	if !discussionInitialPostRequired(c, h.discussionV2Service, view.Topic) && !discussionIsGroupRoot(view.Topic) {
		var refs []discussionEntryRef
		collectV2Refs(view.Entries, &refs)
		p := newDiscussionPresenter(c, h.parity, view.Topic, refs)
		entries = make([]fiber.Map, len(view.Entries))
		for i := range view.Entries {
			entries[i] = entryViewV2ToJSON(&view.Entries[i], p)
		}
	}
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
// POST .../entries/:entry_id/read
func (h *DiscussionV2Handler) MarkEntryRead(c *fiber.Ctx) error {
	topic, wrote := requireTopicInCourse(c, h.discussionV2Service)
	if wrote {
		return nil
	}
	entry, wrote := requireEntryInTopic(c, h.discussionV2Service, topic.ID, "entry_id")
	if wrote {
		return nil
	}

	userID, _ := c.Locals("user_id").(uint)

	if err := h.discussionV2Service.MarkEntryAsRead(c.Context(), entry.ID, userID); err != nil {
		return responses.InternalError(c, "Could not mark entry as read")
	}

	return c.JSON(fiber.Map{"status": "read"})
}

// MarkTopicRead marks all entries in the topic as read.
// POST .../discussion_topics/:topic_id/mark_all_read
func (h *DiscussionV2Handler) MarkTopicRead(c *fiber.Ctx) error {
	topic, wrote := requireTopicInCourse(c, h.discussionV2Service)
	if wrote {
		return nil
	}

	userID, _ := c.Locals("user_id").(uint)

	if err := h.discussionV2Service.MarkTopicAsRead(c.Context(), topic, userID); err != nil {
		return responses.InternalError(c, "Could not mark topic as read")
	}

	return c.JSON(fiber.Map{"status": "all_read"})
}

// GetUnreadCount returns the unread entry count for a topic.
// GET .../discussion_topics/:topic_id/unread_count
func (h *DiscussionV2Handler) GetUnreadCount(c *fiber.Ctx) error {
	topic, wrote := requireTopicInCourse(c, h.discussionV2Service)
	if wrote {
		return nil
	}

	userID, _ := c.Locals("user_id").(uint)

	count, err := h.discussionV2Service.GetUnreadCount(c.Context(), topic.ID, userID)
	if err != nil {
		return responses.InternalError(c, "Could not get unread count")
	}

	return c.JSON(fiber.Map{"unread_count": count})
}

// ToggleSubscription sets subscription for the topic.
// PUT .../discussion_topics/:topic_id/subscription
func (h *DiscussionV2Handler) ToggleSubscription(c *fiber.Ctx) error {
	topic, wrote := requireTopicInCourse(c, h.discussionV2Service)
	if wrote {
		return nil
	}

	var input struct {
		Subscribed bool `json:"subscribed"`
	}

	if err := c.BodyParser(&input); err != nil {
		return responses.BadRequest(c, "Invalid input")
	}

	userID, _ := c.Locals("user_id").(uint)

	if err := h.discussionV2Service.ToggleSubscription(c.Context(), topic.ID, userID, input.Subscribed); err != nil {
		return responses.InternalError(c, "Could not update subscription")
	}

	return c.JSON(fiber.Map{"subscribed": input.Subscribed})
}

// GetEntryVersions returns the edit history for a discussion entry.
// GET .../entries/:entry_id/versions
func (h *DiscussionV2Handler) GetEntryVersions(c *fiber.Ctx) error {
	topic, wrote := requireTopicInCourse(c, h.discussionV2Service)
	if wrote {
		return nil
	}
	if requirePostsVisible(c, h.discussionV2Service, topic) {
		return nil
	}
	entry, wrote := requireEntryInTopic(c, h.discussionV2Service, topic.ID, "entry_id")
	if wrote {
		return nil
	}

	versions, err := h.discussionV2Service.GetEntryVersions(c.Context(), entry.ID)
	if err != nil {
		return responses.InternalError(c, "Could not fetch entry versions")
	}

	// An anonymous post's history must not name its editors either.
	callerID, _ := c.Locals("user_id").(uint)
	hideEditors := service.EntryIsAnonymous(topic, entry)
	result := make([]fiber.Map, len(versions))
	for i, v := range versions {
		var editor interface{} = v.UserID
		if hideEditors && v.UserID != callerID {
			editor = nil
		}
		result[i] = fiber.Map{
			"id":                  v.ID,
			"discussion_entry_id": v.DiscussionEntryID,
			"user_id":             editor,
			"message":             v.Message,
			"version":             v.Version,
			"created_at":          v.CreatedAt,
		}
	}

	return c.JSON(result)
}

// UpdateEntryV2 updates a discussion entry, recording the edit history.
// PUT .../entries/:entry_id/v2
//
// Only the entry's author (while the topic is open for them) or course
// staff may edit; the entry must belong to this topic, course and tenant.
func (h *DiscussionV2Handler) UpdateEntryV2(c *fiber.Ctx) error {
	topic, wrote := requireTopicInCourse(c, h.discussionV2Service)
	if wrote {
		return nil
	}
	entry, wrote := requireEntryInTopic(c, h.discussionV2Service, topic.ID, "entry_id")
	if wrote {
		return nil
	}
	if requireEntryModerator(c, topic, entry) {
		return nil
	}

	var input entryUpdateInput

	if err := c.BodyParser(&input); err != nil {
		return responses.BadRequest(c, "Invalid input")
	}

	if input.Message == "" {
		return responses.BadRequest(c, "Message is required")
	}

	userID, _ := c.Locals("user_id").(uint)

	if err := h.discussionV2Service.UpdateEntryWithHistory(c.Context(), entry, userID, service.SanitizeHTML(input.Message)); err != nil {
		return responses.InternalError(c, "Could not update entry")
	}

	versions, _ := h.discussionV2Service.GetEntryVersions(c.Context(), entry.ID)
	userInfo := h.discussionV2Service.ResolveUserInfo(c.Context(), []uint{entry.UserID})
	info := userInfo[entry.UserID]

	var editedAt *time.Time
	if len(versions) > 0 {
		editedAt = &entry.UpdatedAt
	}

	if h.parity != nil && len(input.MentionedUserIDs) > 0 && userID == entry.UserID {
		_, _ = h.parity.ProcessMentions(c.Context(), topic, entry, input.MentionedUserIDs)
	}

	out := fiber.Map{
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
		"version_count":       len(versions),
		"workflow_state":      entry.WorkflowState,
		"attachment_id":       entry.AttachmentID,
		"is_anonymous_author": entry.IsAnonymousAuthor,
		"created_at":          entry.CreatedAt,
		"updated_at":          entry.UpdatedAt,
	}
	ref := entryRef(entry)
	newDiscussionPresenter(c, h.parity, topic, []discussionEntryRef{ref}).decorate(out, ref)
	return c.JSON(out)
}
