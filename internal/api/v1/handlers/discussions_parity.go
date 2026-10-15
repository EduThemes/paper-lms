package handlers

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/gofiber/fiber/v2"

	"github.com/EduThemes/paper-lms/internal/api/v1/responses"
	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/service"
	"github.com/EduThemes/paper-lms/internal/settingsctx"
	"github.com/EduThemes/paper-lms/internal/storage"
)

// DiscussionParityHandler serves the discussion endpoints added for
// Canvas parity: post attachments (G086), the @mention picker (G088),
// and the graded-discussion lookups used by the assignment page and
// SpeedGrader (G084).
//
// Every topic is resolved with requireTopicInCourse (tenant + course tie
// + publish / section / group visibility) before anything else.
type DiscussionParityHandler struct {
	discussions *service.DiscussionService
	parity      *service.DiscussionParityService
	fileService *service.FileService
}

func NewDiscussionParityHandler(discussions *service.DiscussionService, parity *service.DiscussionParityService, fileService *service.FileService) *DiscussionParityHandler {
	return &DiscussionParityHandler{discussions: discussions, parity: parity, fileService: fileService}
}

// UploadAttachment stores a file a participant is about to attach to a
// post. Like Canvas, it lands in the poster's own (User) files; the post
// then references it and the topic-scoped download route governs who
// may read it.
//
//	POST /courses/:course_id/discussion_topics/:topic_id/attachments
func (h *DiscussionParityHandler) UploadAttachment(c *fiber.Ctx) error {
	if h.fileService == nil {
		return responses.InternalError(c, "File uploads are not configured")
	}
	topic, wrote := requireTopicInCourse(c, h.discussions)
	if wrote {
		return nil
	}
	if requireTopicOpenForCaller(c, topic) {
		return nil
	}
	if requireTopicAcceptsPosts(c, topic) {
		return nil
	}
	userID, _ := c.Locals("user_id").(uint)
	if userID == 0 {
		return responses.Unauthorized(c)
	}

	fileHeader, err := c.FormFile("file")
	if err != nil {
		return responses.BadRequest(c, "File is required")
	}
	file, err := fileHeader.Open()
	if err != nil {
		return responses.InternalError(c, "Could not open uploaded file")
	}
	defer func() { _ = file.Close() }()

	// Never trust a client claim of an inline-safe type the bytes
	// contradict (same hardening as the course and submission uploads).
	contentType := fileHeader.Header.Get("Content-Type")
	sniffBuf := make([]byte, 512)
	n, _ := io.ReadFull(file, sniffBuf)
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		return responses.InternalError(c, "Could not read uploaded file")
	}
	if n > 0 {
		sniffed := http.DetectContentType(sniffBuf[:n])
		if isInlineSafeMIME(contentType) && !isInlineSafeMIME(sniffed) &&
			!strings.HasPrefix(sniffed, "application/octet-stream") {
			contentType = sniffed
		}
	}

	attachment := &models.Attachment{
		ContextType: "User",
		ContextID:   userID,
		UserID:      userID,
		DisplayName: fileHeader.Filename,
		Filename:    fileHeader.Filename,
		ContentType: contentType,
		Size:        fileHeader.Size,
	}
	uploadCtx := settingsctx.WithAccountID(c.Context(), callerAccountID(c))
	if err := h.fileService.UploadFile(uploadCtx, attachment, file); err != nil {
		return responses.BadRequest(c, "Could not upload file: "+err.Error())
	}

	return c.Status(fiber.StatusCreated).JSON(fiber.Map{
		"id":           attachment.ID,
		"display_name": attachment.DisplayName,
		"filename":     attachment.Filename,
		"content_type": attachment.ContentType,
		"size":         attachment.Size,
	})
}

// DownloadEntryAttachment streams the file attached to a post. Access
// follows the discussion: anyone who can read the post (topic visible,
// initial post made where required) may download its file; every miss
// is a 404.
//
//	GET /courses/:course_id/discussion_topics/:topic_id/entries/:entry_id/attachment
func (h *DiscussionParityHandler) DownloadEntryAttachment(c *fiber.Ctx) error {
	if h.fileService == nil {
		return responses.NotFound(c, "file")
	}
	topic, wrote := requireTopicInCourse(c, h.discussions)
	if wrote {
		return nil
	}
	entry, wrote := requireEntryInTopic(c, h.discussions, topic.ID, "entry_id")
	if wrote {
		return nil
	}
	callerID, _ := c.Locals("user_id").(uint)
	if entry.UserID != callerID && discussionInitialPostRequired(c, h.discussions, topic) {
		return responses.NotFound(c, "file")
	}
	if entry.AttachmentID == nil {
		return responses.NotFound(c, "file")
	}
	attachment, err := h.fileService.GetAttachment(c.Context(), *entry.AttachmentID, callerAccountID(c))
	if err != nil || attachment == nil || attachment.UserID != entry.UserID || attachment.WorkflowState == models.AttachmentDeleted {
		return responses.NotFound(c, "file")
	}

	safeName := filepath.Base(attachment.Filename)
	safeName = strings.ReplaceAll(safeName, "\"", "")
	safeName = strings.ReplaceAll(safeName, "\r", "")
	safeName = strings.ReplaceAll(safeName, "\n", "")
	disposition := "attachment"
	if isInlineSafeMIME(attachment.ContentType) {
		disposition = "inline"
	}
	c.Set("Content-Disposition", fmt.Sprintf("%s; filename=\"%s\"; filename*=UTF-8''%s", disposition, safeName, url.PathEscape(safeName)))

	backend := h.fileService.StorageBackend()
	if _, isS3 := backend.(*storage.S3Backend); isS3 {
		downloadURL, err := h.fileService.GetFileURL(c.Context(), attachment.ID, callerAccountID(c))
		if err != nil {
			return responses.InternalError(c, "Could not generate download URL")
		}
		return c.Redirect(downloadURL, fiber.StatusTemporaryRedirect)
	}

	downloadCtx := settingsctx.WithAccountID(c.Context(), callerAccountID(c))
	reader, err := backend.Get(downloadCtx, attachment.StoragePath)
	if err != nil {
		return responses.InternalError(c, "Could not locate file")
	}
	// No defer Close: fasthttp streams the body after the handler
	// returns and closes the reader itself.
	c.Set("Content-Type", service.ServedContentType(attachment.ContentType))
	return c.SendStream(reader)
}

// MentionableUsers lists the people who can be @mentioned in a topic:
// active members of the course who can open it.
//
//	GET /courses/:course_id/discussion_topics/:topic_id/mentionable_users?search=
func (h *DiscussionParityHandler) MentionableUsers(c *fiber.Ctx) error {
	topic, wrote := requireTopicInCourse(c, h.discussions)
	if wrote {
		return nil
	}
	users, err := h.parity.MentionableUsers(c.Context(), topic, c.Query("search"))
	if err != nil {
		return responses.InternalError(c, "Could not list users")
	}
	return c.JSON(users)
}

// requireGradedTopic resolves the graded discussion backing
// :assignment_id in :course_id. Students must also be able to open the
// topic. Returns wrote=true after writing a 404.
func (h *DiscussionParityHandler) requireGradedTopic(c *fiber.Ctx) (*models.DiscussionTopic, bool) {
	courseID, err := c.ParamsInt("course_id")
	if err != nil || courseID <= 0 {
		_ = responses.BadRequest(c, "Invalid course ID")
		return nil, true
	}
	assignmentID, err := c.ParamsInt("assignment_id")
	if err != nil || assignmentID <= 0 {
		_ = responses.BadRequest(c, "Invalid assignment ID")
		return nil, true
	}
	found, err := h.parity.FindTopicByAssignment(c.Context(), uint(assignmentID), uint(courseID))
	if err != nil || found == nil {
		_ = responses.NotFound(c, "discussion topic")
		return nil, true
	}
	// Re-resolve under the caller's tenant with the usual visibility.
	topic, err := h.discussions.GetTopic(c.Context(), found.ID, callerAccountID(c))
	if err != nil || topic == nil || topic.CourseID != uint(courseID) ||
		topic.WorkflowState == models.DiscussionTopicDeleted ||
		!discussionTopicVisibleToCaller(c, topic) ||
		!discussionAudienceAllows(c, h.discussions, topic) {
		_ = responses.NotFound(c, "discussion topic")
		return nil, true
	}
	return topic, false
}

// AssignmentDiscussionTopic returns the graded discussion behind an
// assignment, so the assignment page can send students to it.
//
//	GET /courses/:course_id/assignments/:assignment_id/discussion_topic
func (h *DiscussionParityHandler) AssignmentDiscussionTopic(c *fiber.Ctx) error {
	topic, wrote := h.requireGradedTopic(c)
	if wrote {
		return nil
	}
	return c.JSON(fiber.Map{
		"id":                topic.ID,
		"title":             topic.Title,
		"course_id":         topic.CourseID,
		"group_category_id": topic.GroupCategoryID,
	})
}

// StudentEntries lists one student's posts on a graded discussion for
// SpeedGrader (Canvas shows a student's discussion posts as their
// submission). Course staff only (instructor route).
//
//	GET /courses/:course_id/assignments/:assignment_id/discussion_entries?user_id=
func (h *DiscussionParityHandler) StudentEntries(c *fiber.Ctx) error {
	topic, wrote := h.requireGradedTopic(c)
	if wrote {
		return nil
	}
	studentID, err := parseUintQuery(c, "user_id")
	if err != nil || studentID == 0 {
		return responses.BadRequest(c, "user_id is required")
	}
	entries, topicGroups, err := h.parity.StudentEntries(c.Context(), topic, studentID)
	if err != nil {
		return responses.InternalError(c, "Could not load discussion posts")
	}

	refs := make([]discussionEntryRef, len(entries))
	for i := range entries {
		refs[i] = entryRef(&entries[i])
	}
	// Attachments are linked through the topic each post lives on.
	p := newDiscussionPresenter(c, h.parity, topic, refs)
	out := make([]fiber.Map, len(entries))
	for i := range entries {
		m := entryToJSON(&entries[i])
		p.decorate(m, refs[i])
		if a, ok := m["attachment"].(fiber.Map); ok {
			a["url"] = discussionAttachmentURL(topic.CourseID, entries[i].DiscussionTopicID, entries[i].ID)
		}
		if name, ok := topicGroups[entries[i].DiscussionTopicID]; ok {
			m["group_name"] = name
		}
		out[i] = m
	}
	return c.JSON(fiber.Map{
		"topic":   fiber.Map{"id": topic.ID, "title": topic.Title, "message": topic.Message},
		"entries": out,
	})
}

func parseUintQuery(c *fiber.Ctx, key string) (uint, error) {
	raw := strings.TrimSpace(c.Query(key))
	if raw == "" {
		return 0, nil
	}
	v, err := strconv.ParseUint(raw, 10, 32)
	return uint(v), err
}
