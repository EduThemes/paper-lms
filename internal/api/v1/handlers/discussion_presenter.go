package handlers

import (
	"fmt"

	"github.com/gofiber/fiber/v2"

	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/service"
)

// discussionPresenter decorates discussion entry JSON for one viewer:
// authors of anonymous posts are replaced by their alias (G087), and an
// attached file is described with its topic-scoped download URL (G086).
//
// A nil parity service (tests, or a deployment without it) leaves the
// JSON as it is.
type discussionPresenter struct {
	topic       *models.DiscussionTopic
	callerID    uint
	aliases     map[uint]string
	attachments map[uint]*models.Attachment
}

// discussionEntryRef is what the presenter needs to know about an entry.
type discussionEntryRef struct {
	ID                uint
	UserID            uint
	IsAnonymousAuthor bool
	AttachmentID      *uint
	Deleted           bool
}

func newDiscussionPresenter(c *fiber.Ctx, parity *service.DiscussionParityService, topic *models.DiscussionTopic, refs []discussionEntryRef) *discussionPresenter {
	callerID, _ := c.Locals("user_id").(uint)
	p := &discussionPresenter{topic: topic, callerID: callerID}
	if parity == nil || topic == nil {
		return p
	}
	var anonUsers, attachmentIDs []uint
	for _, r := range refs {
		if service.EntryIsAnonymous(topic, &models.DiscussionEntry{IsAnonymousAuthor: r.IsAnonymousAuthor}) {
			anonUsers = append(anonUsers, r.UserID)
		}
		if r.AttachmentID != nil && !r.Deleted {
			attachmentIDs = append(attachmentIDs, *r.AttachmentID)
		}
	}
	if len(anonUsers) > 0 {
		p.aliases = parity.AnonymousAliases(c.Context(), topic, anonUsers)
	}
	if len(attachmentIDs) > 0 {
		p.attachments = parity.Attachments(c.Context(), attachmentIDs)
	}
	return p
}

// discussionAttachmentURL is the topic-scoped download URL of an entry's
// attachment (access follows the discussion, not the uploader's files).
func discussionAttachmentURL(courseID, topicID, entryID uint) string {
	return fmt.Sprintf("/api/v1/courses/%d/discussion_topics/%d/entries/%d/attachment", courseID, topicID, entryID)
}

// decorate rewrites one entry's JSON in place.
func (p *discussionPresenter) decorate(m fiber.Map, r discussionEntryRef) {
	if p == nil || p.topic == nil {
		return
	}
	if service.EntryIsAnonymous(p.topic, &models.DiscussionEntry{IsAnonymousAuthor: r.IsAnonymousAuthor}) {
		alias := p.aliases[r.UserID]
		name := service.AnonymousName(alias)
		m["anonymous_author"] = fiber.Map{"id": alias, "short_name": name}
		m["user_name"] = name
		m["user_avatar_url"] = ""
		m["is_anonymous_author"] = true
		if r.UserID != p.callerID {
			m["user_id"] = nil
		}
	}
	if r.AttachmentID != nil && !r.Deleted {
		if a, ok := p.attachments[*r.AttachmentID]; ok {
			m["attachment"] = fiber.Map{
				"id":           a.ID,
				"display_name": a.DisplayName,
				"filename":     a.Filename,
				"content_type": a.ContentType,
				"size":         a.Size,
				"url":          discussionAttachmentURL(p.topic.CourseID, p.topic.ID, r.ID),
			}
		}
	}
}

func entryRef(e *models.DiscussionEntry) discussionEntryRef {
	return discussionEntryRef{
		ID:                e.ID,
		UserID:            e.UserID,
		IsAnonymousAuthor: e.IsAnonymousAuthor,
		AttachmentID:      e.AttachmentID,
		Deleted:           e.WorkflowState == discussionEntryDeletedState,
	}
}

// presentEntries renders entries with entryToJSON and decorates them.
func presentEntries(c *fiber.Ctx, parity *service.DiscussionParityService, topic *models.DiscussionTopic, entries []models.DiscussionEntry) []fiber.Map {
	refs := make([]discussionEntryRef, len(entries))
	for i := range entries {
		refs[i] = entryRef(&entries[i])
	}
	p := newDiscussionPresenter(c, parity, topic, refs)
	out := make([]fiber.Map, len(entries))
	for i := range entries {
		out[i] = entryToJSON(&entries[i])
		p.decorate(out[i], refs[i])
	}
	return out
}

// presentEntry renders a single entry.
func presentEntry(c *fiber.Ctx, parity *service.DiscussionParityService, topic *models.DiscussionTopic, e *models.DiscussionEntry) fiber.Map {
	return presentEntries(c, parity, topic, []models.DiscussionEntry{*e})[0]
}

// collectV2Refs walks a V2 entry tree.
func collectV2Refs(entries []service.EntryViewV2, out *[]discussionEntryRef) {
	for i := range entries {
		e := &entries[i]
		*out = append(*out, discussionEntryRef{
			ID: e.ID, UserID: e.UserID, IsAnonymousAuthor: e.IsAnonymousAuthor,
			AttachmentID: e.AttachmentID, Deleted: e.WorkflowState == discussionEntryDeletedState,
		})
		collectV2Refs(e.Replies, out)
	}
}

// collectV1Refs walks a V1 entry tree.
func collectV1Refs(entries []service.EntryView, out *[]discussionEntryRef) {
	for i := range entries {
		e := &entries[i]
		*out = append(*out, discussionEntryRef{
			ID: e.ID, UserID: e.UserID, IsAnonymousAuthor: e.IsAnonymousAuthor,
			AttachmentID: e.AttachmentID, Deleted: e.WorkflowState == discussionEntryDeletedState,
		})
		collectV1Refs(e.Replies, out)
	}
}

// entryViewV1ToJSON renders the V1 threaded view tree as maps so the
// presenter can decorate it.
func entryViewV1ToJSON(e *service.EntryView, p *discussionPresenter) fiber.Map {
	replies := make([]fiber.Map, len(e.Replies))
	for i := range e.Replies {
		replies[i] = entryViewV1ToJSON(&e.Replies[i], p)
	}
	m := fiber.Map{
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
		"replies":             replies,
	}
	p.decorate(m, discussionEntryRef{
		ID: e.ID, UserID: e.UserID, IsAnonymousAuthor: e.IsAnonymousAuthor,
		AttachmentID: e.AttachmentID, Deleted: e.WorkflowState == discussionEntryDeletedState,
	})
	return m
}
