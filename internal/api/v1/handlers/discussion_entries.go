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
	// v2 records edit history (UpdateEntryWithHistory); without it V1
	// edits overwrite the message with no version kept.
	v2 *service.DiscussionV2Service
	// parity adds attachments, anonymous posts, @mentions and graded
	// discussion submissions. Optional (nil in some tests).
	parity *service.DiscussionParityService
}

// SetVersionHistory routes V1 entry edits through the versioned update the
// /v2 edit route uses, so API edits keep the same history as UI edits.
func (h *DiscussionEntryHandler) SetVersionHistory(v2 *service.DiscussionV2Service) *DiscussionEntryHandler {
	h.v2 = v2
	return h
}

// SetParity enables the Canvas-parity discussion features on entries.
func (h *DiscussionEntryHandler) SetParity(p *service.DiscussionParityService) *DiscussionEntryHandler {
	h.parity = p
	return h
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
		"attachment_id":       e.AttachmentID,
		"is_anonymous_author": e.IsAnonymousAuthor,
		"created_at":          e.CreatedAt,
		"updated_at":          e.UpdatedAt,
	}
}

// requirePostsVisible writes a 403 and returns wrote=true when the
// caller must post before seeing other people's posts
// (require_initial_post, Canvas initial_post_required?).
func requirePostsVisible(c *fiber.Ctx, svc interface{}, topic *models.DiscussionTopic) bool {
	if discussionInitialPostRequired(c, svc, topic) {
		_ = responses.Forbidden(c, "You must post before seeing replies")
		return true
	}
	return false
}

func (h *DiscussionEntryHandler) ListEntries(c *fiber.Ctx) error {
	topic, wrote := requireTopicInCourse(c, h.discussionService)
	if wrote {
		return nil
	}
	if requirePostsVisible(c, h.discussionService, topic) {
		return nil
	}

	params := middleware.GetPagination(c)

	result, err := h.discussionService.ListEntries(c.Context(), topic.ID, callerAccountID(c), params)
	if err != nil {
		return responses.InternalError(c, "Could not fetch discussion entries")
	}

	responses.SetPaginationHeaders(c, result.TotalCount, result.Page, result.PerPage)

	return c.JSON(presentEntries(c, h.parity, topic, result.Items))
}

// entryCreateInput is the body of a new post or reply.
type entryCreateInput struct {
	Message string `json:"message"`
	// AttachmentID is a file the poster uploaded through
	// POST .../discussion_topics/:topic_id/attachments.
	AttachmentID *uint `json:"attachment_id"`
	// Anonymous posts under the poster's alias in a partially anonymous
	// topic (ignored elsewhere).
	Anonymous bool `json:"anonymous"`
	// MentionedUserIDs are the people picked in the @mention picker.
	MentionedUserIDs []uint `json:"mentioned_user_ids"`
}

// createEntry is the shared body of CreateEntry and CreateReply.
func (h *DiscussionEntryHandler) createEntry(c *fiber.Ctx, topic *models.DiscussionTopic, parentID *uint) error {
	var input entryCreateInput
	if err := c.BodyParser(&input); err != nil {
		return responses.BadRequest(c, "Invalid input")
	}

	userID, _ := c.Locals("user_id").(uint)

	entry := &models.DiscussionEntry{
		DiscussionTopicID: topic.ID,
		UserID:            userID,
		ParentID:          parentID,
		Message:           service.SanitizeHTML(input.Message),
	}

	if input.AttachmentID != nil && *input.AttachmentID != 0 {
		if h.parity == nil {
			return responses.BadRequest(c, "Attachments are not enabled")
		}
		if err := h.parity.ValidatePostAttachment(c.Context(), *input.AttachmentID, userID, callerAccountID(c)); err != nil {
			return responses.BadRequest(c, err.Error())
		}
		id := *input.AttachmentID
		entry.AttachmentID = &id
		if entry.Message == "" {
			// A file-only post still needs a body for the thread view.
			entry.Message = "<p></p>"
		}
	}
	if input.Anonymous && topic.AnonymousState != nil && *topic.AnonymousState == models.DiscussionPartialAnonymity {
		entry.IsAnonymousAuthor = true
	}

	if err := h.discussionService.CreateEntry(c.Context(), entry); err != nil {
		return responses.BadRequest(c, err.Error())
	}

	if h.parity != nil && len(input.MentionedUserIDs) > 0 {
		_, _ = h.parity.ProcessMentions(c.Context(), topic, entry, input.MentionedUserIDs)
	}

	return c.Status(fiber.StatusCreated).JSON(presentEntry(c, h.parity, topic, entry))
}

func (h *DiscussionEntryHandler) CreateEntry(c *fiber.Ctx) error {
	// The topic must be in this course and tenant, visible to the caller
	// (not deleted / unpublished / delayed / outside their section or
	// group) and not locked for them.
	topic, wrote := requireTopicInCourse(c, h.discussionService)
	if wrote {
		return nil
	}
	if requireTopicOpenForCaller(c, topic) {
		return nil
	}
	if requireTopicAcceptsPosts(c, topic) {
		return nil
	}
	return h.createEntry(c, topic, nil)
}

// entryUpdateInput is the body of an edit.
type entryUpdateInput struct {
	Message          string `json:"message"`
	MentionedUserIDs []uint `json:"mentioned_user_ids"`
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

	var input entryUpdateInput
	if err := c.BodyParser(&input); err != nil {
		return responses.BadRequest(c, "Invalid input")
	}

	if input.Message != "" {
		msg := service.SanitizeHTML(input.Message)
		if h.v2 != nil {
			editorID, _ := c.Locals("user_id").(uint)
			if err := h.v2.UpdateEntryWithHistory(c.Context(), entry, editorID, msg); err != nil {
				return responses.InternalError(c, "Could not update discussion entry")
			}
			h.mentionsAfterEdit(c, topic, entry, input.MentionedUserIDs)
			return c.JSON(presentEntry(c, h.parity, topic, entry))
		}
		entry.Message = msg
	}

	if err := h.discussionService.UpdateEntry(c.Context(), entry); err != nil {
		return responses.InternalError(c, "Could not update discussion entry")
	}
	h.mentionsAfterEdit(c, topic, entry, input.MentionedUserIDs)

	return c.JSON(presentEntry(c, h.parity, topic, entry))
}

// mentionsAfterEdit notifies people newly @mentioned by an edit. Only
// the author's own edits mention on their behalf.
func (h *DiscussionEntryHandler) mentionsAfterEdit(c *fiber.Ctx, topic *models.DiscussionTopic, entry *models.DiscussionEntry, ids []uint) {
	if h.parity == nil || len(ids) == 0 {
		return
	}
	if editor, _ := c.Locals("user_id").(uint); editor != entry.UserID {
		return
	}
	_, _ = h.parity.ProcessMentions(c.Context(), topic, entry, ids)
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
	if h.parity != nil {
		// A graded discussion's submission goes back to unsubmitted once
		// the student's last post is gone.
		_ = h.parity.SyncSubmissionForUser(c.Context(), topic, entry.UserID)
	}

	return c.JSON(fiber.Map{"delete": true})
}

func (h *DiscussionEntryHandler) ListReplies(c *fiber.Ctx) error {
	topic, wrote := requireTopicInCourse(c, h.discussionService)
	if wrote {
		return nil
	}
	if requirePostsVisible(c, h.discussionService, topic) {
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

	return c.JSON(presentEntries(c, h.parity, topic, result.Items))
}

func (h *DiscussionEntryHandler) CreateReply(c *fiber.Ctx) error {
	topic, wrote := requireTopicInCourse(c, h.discussionService)
	if wrote {
		return nil
	}
	if requireTopicOpenForCaller(c, topic) {
		return nil
	}
	if requireTopicAcceptsPosts(c, topic) {
		return nil
	}
	// Someone who still has to make their initial post can't see the
	// entries, so they can't reply to one either.
	if requirePostsVisible(c, h.discussionService, topic) {
		return nil
	}
	// The parent must be a live entry on this same topic; a parent from
	// another topic would otherwise be orphaned into a root post.
	parent, wrote := requireEntryInTopic(c, h.discussionService, topic.ID, "entry_id")
	if wrote {
		return nil
	}

	parentID := parent.ID
	return h.createEntry(c, topic, &parentID)
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
	if requirePostsVisible(c, h.discussionService, topic) {
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

	out := presentEntry(c, h.parity, topic, updated)
	out["rating"] = *input.Rating
	return c.JSON(out)
}
