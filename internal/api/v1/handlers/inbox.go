package handlers

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"

	"github.com/EduThemes/paper-lms/internal/api/v1/responses"
	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/repository"
	"github.com/EduThemes/paper-lms/internal/repository/postgres"
	"github.com/EduThemes/paper-lms/internal/service"
	"github.com/EduThemes/paper-lms/internal/settingsctx"
	"github.com/EduThemes/paper-lms/internal/storage"
)

// Inbox completeness (G092) and the recipient address book (G093).

const (
	// maxConversationRecipients caps one send (after expanding course,
	// section and group recipients).
	maxConversationRecipients = 1000
	// maxGroupConversationSize: above this many recipients a send always
	// becomes one private conversation per recipient (Canvas does the
	// same with its group-conversation limit).
	maxGroupConversationSize = 100
	// maxMessageAttachments caps the files on one message.
	maxMessageAttachments = 10
)

// Aliases of the repository event names, used by UpdateConversation.
const (
	inboxEventMarkRead   = postgres.InboxEventMarkRead
	inboxEventMarkUnread = postgres.InboxEventMarkUnread
	inboxEventStar       = postgres.InboxEventStar
	inboxEventUnstar     = postgres.InboxEventUnstar
	inboxEventArchive    = postgres.InboxEventArchive
	inboxEventUnarchive  = postgres.InboxEventUnarchive
	inboxEventDestroy    = postgres.InboxEventDestroy
)

// InboxStore is the per-participant inbox persistence
// (postgres.InboxRepository).
type InboxStore interface {
	ListForUser(ctx context.Context, userID, accountID uint, f postgres.InboxFilter, params repository.PaginationParams) ([]postgres.InboxConversation, int64, error)
	GetForUser(ctx context.Context, convID, userID, accountID uint) (*postgres.InboxConversation, error)
	UnreadCount(ctx context.Context, userID, accountID uint) (int64, error)
	UpdateParticipants(ctx context.Context, userID uint, convIDs []uint, event string) (int64, error)
	OnNewMessage(ctx context.Context, convID, senderID uint, restrictTo []uint) error
	AddMessageRecipients(ctx context.Context, messageID uint, userIDs []uint) error
	LinkAttachments(ctx context.Context, messageID uint, attachmentIDs []uint) error
	AttachmentsForMessages(ctx context.Context, messageIDs []uint) (map[uint][]models.Attachment, error)
	RecipientsForMessages(ctx context.Context, messageIDs []uint) (map[uint][]uint, error)
	ListVisibleMessages(ctx context.Context, convID, userID uint, params repository.PaginationParams) (*repository.PaginatedResult[models.ConversationMessage], error)
	VisibleMessages(ctx context.Context, convID, userID uint, messageIDs []uint) ([]models.ConversationMessage, error)
	CanSeeAttachment(ctx context.Context, convID, userID, attachmentID uint) (bool, error)
}

// AddressBook scopes who a user may message (postgres.RecipientSearchRepository).
type AddressBook interface {
	Unaddressable(ctx context.Context, callerID, accountID uint, recipientIDs []uint, allInAccount bool) ([]uint, error)
	ListContexts(ctx context.Context, callerID, accountID uint) ([]postgres.AddressBookContext, error)
	CourseRole(ctx context.Context, callerID, courseID, accountID uint) (member, instructor bool, err error)
	GroupInfo(ctx context.Context, callerID, groupID, accountID uint) (found, member bool, courseID uint, err error)
	SectionCourse(ctx context.Context, sectionID, accountID uint) (uint, error)
	ContextMembers(ctx context.Context, accountID uint, q postgres.ContextMemberQuery) ([]models.User, error)
}

// SetInbox wires the per-participant inbox store.
func (h *ConversationHandler) SetInbox(s InboxStore) { h.inbox = s }

// SetAddressBook wires recipient scoping and context recipients.
func (h *ConversationHandler) SetAddressBook(a AddressBook) { h.addressBook = a }

// SetFileService wires message attachments.
func (h *ConversationHandler) SetFileService(f *service.FileService) { h.fileService = f }

// contextCode renders a Canvas context code ("course_5", "group_7").
func contextCode(contextType string, contextID uint) string {
	if contextType == "" || contextID == 0 {
		return ""
	}
	return strings.ToLower(contextType) + "_" + strconv.FormatUint(uint64(contextID), 10)
}

// recipientCode is a parsed context recipient: "course_5_students" →
// {Course, 5, students}.
type recipientCode struct {
	Kind string // course, section, group
	ID   uint
	Role string // "", students, teachers, tas, observers
}

var recipientRoleTypes = map[string][]string{
	"students":  {"StudentEnrollment"},
	"teachers":  {"TeacherEnrollment"},
	"tas":       {"TaEnrollment"},
	"observers": {"ObserverEnrollment"},
}

func parseRecipientCode(s string) (recipientCode, bool) {
	parts := strings.Split(strings.ToLower(strings.TrimSpace(s)), "_")
	if len(parts) < 2 || len(parts) > 3 {
		return recipientCode{}, false
	}
	switch parts[0] {
	case "course", "section", "group":
	default:
		return recipientCode{}, false
	}
	id, err := strconv.ParseUint(parts[1], 10, 64)
	if err != nil || id == 0 {
		return recipientCode{}, false
	}
	rc := recipientCode{Kind: parts[0], ID: uint(id)}
	if len(parts) == 3 {
		if parts[0] == "group" {
			return recipientCode{}, false
		}
		if _, ok := recipientRoleTypes[parts[2]]; !ok {
			return recipientCode{}, false
		}
		rc.Role = parts[2]
	}
	return rc, true
}

// resolveRecipients turns the recipients array (user ids and context
// codes) into a de-duplicated user id list, and reports the first context
// used so the conversation can be tagged with it. Follows the
// (result, wrote, err) convention: wrote=true means a response was
// written and the caller must return err immediately.
func (h *ConversationHandler) resolveRecipients(c *fiber.Ctx, callerID uint, raw []any) ([]uint, string, uint, bool, error) {
	seen := map[uint]bool{}
	var ids []uint
	add := func(id uint) {
		if id != 0 && id != callerID && !seen[id] {
			seen[id] = true
			ids = append(ids, id)
		}
	}
	var ctxType string
	var ctxID uint
	isAdmin := -1 // lazily resolved
	admin := func() bool {
		if isAdmin < 0 {
			isAdmin = 0
			if h.callerIsAccountAdmin(c, callerID) {
				isAdmin = 1
			}
		}
		return isAdmin == 1
	}
	for _, item := range raw {
		switch v := item.(type) {
		case float64:
			if v <= 0 || v != float64(uint(v)) {
				return nil, "", 0, true, responses.BadRequest(c, "Invalid recipient")
			}
			add(uint(v))
		case string:
			if n, err := strconv.ParseUint(strings.TrimSpace(v), 10, 64); err == nil {
				add(uint(n))
				continue
			}
			rc, ok := parseRecipientCode(v)
			if !ok {
				return nil, "", 0, true, responses.BadRequest(c, "Invalid recipient "+strconv.Quote(v))
			}
			members, t, id, wrote, err := h.expandContext(c, callerID, admin, rc)
			if wrote {
				return nil, "", 0, true, err
			}
			if ctxType == "" {
				ctxType, ctxID = t, id
			}
			for _, u := range members {
				add(u.ID)
			}
		default:
			return nil, "", 0, true, responses.BadRequest(c, "Invalid recipient")
		}
	}
	return ids, ctxType, ctxID, false, nil
}

// expandContext authorizes and expands one context recipient. Course and
// section recipients are for the course's instructors and admins; a
// group's members (and its course's instructors) may message the group.
// A context the caller has no standing in is a 404.
func (h *ConversationHandler) expandContext(c *fiber.Ctx, callerID uint, admin func() bool, rc recipientCode) ([]models.User, string, uint, bool, error) {
	if h.addressBook == nil {
		return nil, "", 0, true, responses.BadRequest(c, "Context recipients are not available")
	}
	ctx := c.Context()
	acct := callerAccountID(c)
	q := postgres.ContextMemberQuery{ContextID: rc.ID, EnrollmentTypes: recipientRoleTypes[rc.Role], ExcludeUserID: callerID}
	var tagType string
	var tagID uint
	switch rc.Kind {
	case "course", "section":
		courseID := rc.ID
		q.ContextType = "Course"
		if rc.Kind == "section" {
			cid, err := h.addressBook.SectionCourse(ctx, rc.ID, acct)
			if err != nil {
				return nil, "", 0, true, responses.InternalError(c, "Could not resolve recipients")
			}
			if cid == 0 {
				return nil, "", 0, true, responses.NotFound(c, "section")
			}
			courseID = cid
			q.ContextType = "Section"
		}
		member, instructor, err := h.addressBook.CourseRole(ctx, callerID, courseID, acct)
		if err != nil {
			return nil, "", 0, true, responses.InternalError(c, "Could not resolve recipients")
		}
		if !instructor && !admin() {
			if !member {
				return nil, "", 0, true, responses.NotFound(c, "course")
			}
			return nil, "", 0, true, responses.Forbidden(c, "Only the course's instructors can message a whole course or section")
		}
		if !member && admin() {
			// An admin addressing a course of another tenant: the
			// members query is tenant-scoped, so it simply finds nobody
			// — but say so instead of sending to no one.
			members, err := h.addressBook.ContextMembers(ctx, acct, postgres.ContextMemberQuery{ContextType: "Course", ContextID: courseID, Limit: 1})
			if err != nil || len(members) == 0 {
				return nil, "", 0, true, responses.NotFound(c, "course")
			}
		}
		tagType, tagID = "Course", courseID
	case "group":
		found, member, courseID, err := h.addressBook.GroupInfo(ctx, callerID, rc.ID, acct)
		if err != nil {
			return nil, "", 0, true, responses.InternalError(c, "Could not resolve recipients")
		}
		if !found {
			return nil, "", 0, true, responses.NotFound(c, "group")
		}
		allowed := member || admin()
		if !allowed && courseID != 0 {
			_, instructor, err := h.addressBook.CourseRole(ctx, callerID, courseID, acct)
			if err != nil {
				return nil, "", 0, true, responses.InternalError(c, "Could not resolve recipients")
			}
			allowed = instructor
		}
		if !allowed {
			return nil, "", 0, true, responses.NotFound(c, "group")
		}
		q.ContextType = "Group"
		tagType, tagID = "Group", rc.ID
	}
	members, err := h.addressBook.ContextMembers(ctx, acct, q)
	if err != nil {
		return nil, "", 0, true, responses.InternalError(c, "Could not resolve recipients")
	}
	return members, tagType, tagID, false, nil
}

// resolveContextCode validates an explicit context_code ("course_5",
// "group_7") the caller belongs to (admins: any of their account).
func (h *ConversationHandler) resolveContextCode(c *fiber.Ctx, callerID uint, code string) (string, uint, bool, error) {
	rc, ok := parseRecipientCode(code)
	if !ok || rc.Role != "" || rc.Kind == "section" {
		return "", 0, true, responses.BadRequest(c, "context_code must be course_<id> or group_<id>")
	}
	if h.addressBook == nil {
		return "", 0, true, responses.BadRequest(c, "Conversation contexts are not available")
	}
	acct := callerAccountID(c)
	if rc.Kind == "course" {
		member, _, err := h.addressBook.CourseRole(c.Context(), callerID, rc.ID, acct)
		if err != nil {
			return "", 0, true, responses.InternalError(c, "Could not resolve context")
		}
		if !member && !h.callerIsAccountAdmin(c, callerID) {
			return "", 0, true, responses.NotFound(c, "course")
		}
		return "Course", rc.ID, false, nil
	}
	found, member, _, err := h.addressBook.GroupInfo(c.Context(), callerID, rc.ID, acct)
	if err != nil {
		return "", 0, true, responses.InternalError(c, "Could not resolve context")
	}
	if !found || (!member && !h.callerIsAccountAdmin(c, callerID)) {
		return "", 0, true, responses.NotFound(c, "group")
	}
	return "Group", rc.ID, false, nil
}

// browseContext lists the members of a course / section / group for the
// address book. Any member of the context may browse it; admins any
// context of their account.
func (h *ConversationHandler) browseContext(c *fiber.Ctx, callerID uint, isAdmin bool, code, term string, limit int) ([]models.User, bool, error) {
	if h.addressBook == nil {
		return nil, true, responses.Error(c, fiber.StatusServiceUnavailable, "Recipient search is not available")
	}
	rc, ok := parseRecipientCode(code)
	if !ok {
		return nil, true, responses.BadRequest(c, "context must be course_<id>, section_<id> or group_<id>")
	}
	acct := callerAccountID(c)
	q := postgres.ContextMemberQuery{ContextID: rc.ID, EnrollmentTypes: recipientRoleTypes[rc.Role], ExcludeUserID: callerID, Search: term, Limit: limit}
	switch rc.Kind {
	case "course", "section":
		courseID := rc.ID
		q.ContextType = "Course"
		if rc.Kind == "section" {
			cid, err := h.addressBook.SectionCourse(c.Context(), rc.ID, acct)
			if err != nil {
				return nil, true, responses.InternalError(c, "Could not search recipients")
			}
			if cid == 0 {
				return nil, true, responses.NotFound(c, "section")
			}
			courseID = cid
			q.ContextType = "Section"
		}
		member, _, err := h.addressBook.CourseRole(c.Context(), callerID, courseID, acct)
		if err != nil {
			return nil, true, responses.InternalError(c, "Could not search recipients")
		}
		if !member && !isAdmin {
			return nil, true, responses.NotFound(c, "course")
		}
	case "group":
		found, member, courseID, err := h.addressBook.GroupInfo(c.Context(), callerID, rc.ID, acct)
		if err != nil {
			return nil, true, responses.InternalError(c, "Could not search recipients")
		}
		allowed := found && (member || isAdmin)
		if found && !allowed && courseID != 0 {
			courseMember, _, err := h.addressBook.CourseRole(c.Context(), callerID, courseID, acct)
			if err != nil {
				return nil, true, responses.InternalError(c, "Could not search recipients")
			}
			allowed = courseMember
		}
		if !allowed {
			return nil, true, responses.NotFound(c, "group")
		}
		q.ContextType = "Group"
	}
	users, err := h.addressBook.ContextMembers(c.Context(), acct, q)
	if err != nil {
		return nil, true, responses.InternalError(c, "Could not search recipients")
	}
	return users, false, nil
}

// ListInboxContexts handles GET /conversations/contexts: the courses and
// groups the caller can message in (address book roots and the inbox
// course/group filter). permissions.send_messages_all says whether the
// caller may address the whole context.
func (h *ConversationHandler) ListInboxContexts(c *fiber.Ctx) error {
	userID, err := getUserID(c)
	if err != nil {
		return err
	}
	if h.addressBook == nil {
		return c.JSON([]fiber.Map{})
	}
	ctxs, err := h.addressBook.ListContexts(c.Context(), userID, callerAccountID(c))
	if err != nil {
		return responses.InternalError(c, "Could not load contexts")
	}
	isAdmin := h.callerIsAccountAdmin(c, userID)
	out := make([]fiber.Map, len(ctxs))
	for i, x := range ctxs {
		all := x.Instructor || isAdmin || x.ContextType == "Group"
		out[i] = fiber.Map{
			"context_type": x.ContextType,
			"context_id":   x.ContextID,
			"context_code": contextCode(x.ContextType, x.ContextID),
			"name":         x.Name,
			"permissions":  fiber.Map{"send_messages_all": all},
		}
	}
	return c.JSON(out)
}

// replySubset validates a "Reply" recipient subset: every id must be
// another live participant. A subset that covers every other participant
// is the same as "Reply all" and is returned empty.
func (h *ConversationHandler) replySubset(c *fiber.Ctx, convID, senderID uint, recipients []uint) ([]uint, bool, error) {
	if len(recipients) == 0 {
		return nil, false, nil
	}
	if h.inbox == nil {
		return nil, true, responses.BadRequest(c, "Replying to part of a conversation is not available")
	}
	participants, err := h.conversationService.GetParticipants(c.Context(), convID)
	if err != nil {
		return nil, true, responses.InternalError(c, "Could not load participants")
	}
	others := map[uint]bool{}
	for _, p := range participants {
		if p.UserID != senderID {
			others[p.UserID] = true
		}
	}
	seen := map[uint]bool{}
	var out []uint
	for _, id := range recipients {
		if id == senderID || seen[id] {
			continue
		}
		if !others[id] {
			return nil, true, responses.BadRequest(c, "Every recipient must be a participant of the conversation")
		}
		seen[id] = true
		out = append(out, id)
	}
	if len(out) == len(others) {
		return nil, false, nil
	}
	return out, false, nil
}

// ownAttachments checks that every id is a live file the caller uploaded
// to their own (User) files in their tenant — the only files a message may
// carry. Anything else is a 404.
func (h *ConversationHandler) ownAttachments(c *fiber.Ctx, callerID uint, ids []uint) ([]uint, bool, error) {
	if len(ids) == 0 {
		return nil, false, nil
	}
	if h.fileService == nil || h.inbox == nil {
		return nil, true, responses.BadRequest(c, "Attachments are not available")
	}
	if len(ids) > maxMessageAttachments {
		return nil, true, responses.BadRequest(c, fmt.Sprintf("A message can carry at most %d attachments", maxMessageAttachments))
	}
	seen := map[uint]bool{}
	var out []uint
	for _, id := range ids {
		if seen[id] {
			continue
		}
		seen[id] = true
		a, err := h.fileService.GetAttachment(c.Context(), id, callerAccountID(c))
		if err != nil || a == nil || a.UserID != callerID || a.ContextType != "User" || a.ContextID != callerID {
			return nil, true, responses.NotFound(c, "attachment")
		}
		out = append(out, id)
	}
	return out, false, nil
}

func messageAttachmentsJSON(conversationID uint, atts []models.Attachment) []fiber.Map {
	out := make([]fiber.Map, len(atts))
	for i, a := range atts {
		out[i] = fiber.Map{
			"id":           a.ID,
			"display_name": a.DisplayName,
			"filename":     a.Filename,
			"content_type": a.ContentType,
			"size":         a.Size,
			"url":          fmt.Sprintf("/api/v1/conversations/%d/attachments/%d", conversationID, a.ID),
		}
	}
	return out
}

func inboxFilterFromQuery(c *fiber.Ctx) (postgres.InboxFilter, bool) {
	f := postgres.InboxFilter{Scope: strings.TrimSpace(c.Query("scope")), Search: c.Query("search")}
	switch f.Scope {
	case "", postgres.InboxScopeInbox, postgres.InboxScopeUnread, postgres.InboxScopeStarred,
		postgres.InboxScopeArchived, postgres.InboxScopeSent:
	default:
		return f, false
	}
	if code := strings.TrimSpace(c.Query("filter")); code != "" {
		rc, ok := parseRecipientCode(code)
		if !ok || rc.Role != "" || rc.Kind == "section" {
			return f, false
		}
		f.ContextType = map[string]string{"course": "Course", "group": "Group"}[rc.Kind]
		f.ContextID = rc.ID
	}
	return f, true
}

func inboxConversationToJSON(r *postgres.InboxConversation) fiber.Map {
	preview := []rune(r.LastMessage)
	if len(preview) > 140 {
		preview = append(preview[:139], '…')
	}
	lastAt := r.LastMessageAt
	if r.LastVisibleAt != nil {
		lastAt = *r.LastVisibleAt
	}
	state := r.ParticipantState
	if state == "" || state == "active" {
		state = models.ConversationParticipantRead
	}
	return fiber.Map{
		"id":                     r.ID,
		"subject":                r.Subject,
		"created_by_user_id":     r.CreatedByUserID,
		"last_message_at":        lastAt,
		"last_message":           string(preview),
		"last_message_author_id": r.LastMessageAuthorID,
		"message_count":          r.MessageCount,
		"workflow_state":         state,
		"starred":                r.Starred,
		"has_attachments":        r.HasAttachments,
		"context_type":           r.ContextType,
		"context_id":             r.ContextID,
		"context_code":           contextCode(r.ContextType, r.ContextID),
		"context_name":           r.ContextName,
		"created_at":             r.CreatedAt,
		"updated_at":             r.UpdatedAt,
	}
}

// BatchUpdateConversations handles PUT /conversations: Canvas's batch
// update of the caller's own inbox state.
//
//	{"conversation_ids": [1,2], "event": "mark_as_read|mark_as_unread|star|unstar|archive|unarchive|destroy"}
//
// Only the caller's participant rows change; ids of conversations they are
// not part of are ignored.
func (h *ConversationHandler) BatchUpdateConversations(c *fiber.Ctx) error {
	userID, err := getUserID(c)
	if err != nil {
		return err
	}
	if h.inbox == nil {
		return responses.Error(c, fiber.StatusServiceUnavailable, "Inbox state is not available")
	}
	var input struct {
		ConversationIDs []uint `json:"conversation_ids"`
		Event           string `json:"event"`
	}
	if err := c.BodyParser(&input); err != nil {
		return responses.BadRequest(c, "Invalid input")
	}
	if !postgres.IsInboxEvent(input.Event) {
		return responses.BadRequest(c, "event must be one of: mark_as_read, mark_as_unread, star, unstar, archive, unarchive, destroy")
	}
	if len(input.ConversationIDs) == 0 || len(input.ConversationIDs) > 500 {
		return responses.BadRequest(c, "conversation_ids must list 1 to 500 conversations")
	}
	n, err := h.inbox.UpdateParticipants(c.Context(), userID, input.ConversationIDs, input.Event)
	if err != nil {
		return responses.InternalError(c, "Could not update conversations")
	}
	return c.JSON(fiber.Map{"updated": n, "event": input.Event})
}

// DeleteConversation handles DELETE /conversations/:id: removes the
// conversation from the CALLER's inbox only (Canvas semantics). The other
// participants keep it; a new message brings it back for the caller.
func (h *ConversationHandler) DeleteConversation(c *fiber.Ctx) error {
	id, err := c.ParamsInt("id")
	if err != nil || id <= 0 {
		return responses.BadRequest(c, "Invalid conversation ID")
	}
	if err := h.requireParticipant(c, uint(id)); err != nil {
		return err
	}
	if h.inbox == nil {
		return responses.Error(c, fiber.StatusServiceUnavailable, "Inbox state is not available")
	}
	userID, _ := c.Locals("user_id").(uint)
	if _, err := h.inbox.UpdateParticipants(c.Context(), userID, []uint{uint(id)}, inboxEventDestroy); err != nil {
		return responses.InternalError(c, "Could not delete conversation")
	}
	return c.JSON(fiber.Map{"id": id, "deleted": true})
}

// UnreadCount handles GET /conversations/unread_count.
func (h *ConversationHandler) UnreadCount(c *fiber.Ctx) error {
	userID, err := getUserID(c)
	if err != nil {
		return err
	}
	if h.inbox == nil {
		return c.JSON(fiber.Map{"unread_count": 0})
	}
	n, err := h.inbox.UnreadCount(c.Context(), userID, callerAccountID(c))
	if err != nil {
		return responses.InternalError(c, "Could not count unread conversations")
	}
	return c.JSON(fiber.Map{"unread_count": n})
}

// UploadAttachment handles POST /conversations/attachments (multipart
// "file"): stores the file in the caller's own User files so it can be
// attached to a message. Only the uploader can attach it; recipients
// download it through the conversation.
func (h *ConversationHandler) UploadAttachment(c *fiber.Ctx) error {
	userID, err := getUserID(c)
	if err != nil {
		return err
	}
	if h.fileService == nil {
		return responses.Error(c, fiber.StatusServiceUnavailable, "Attachments are not available")
	}
	fileHeader, err := c.FormFile("file")
	if err != nil {
		return responses.BadRequest(c, "File is required")
	}
	file, err := fileHeader.Open()
	if err != nil {
		return responses.InternalError(c, "Could not open uploaded file")
	}
	defer file.Close()

	// Same rule as course uploads: never trust a claimed inline-safe image
	// type whose bytes say otherwise.
	contentType := fileHeader.Header.Get("Content-Type")
	sniff := make([]byte, 512)
	n, _ := io.ReadFull(file, sniff)
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		return responses.InternalError(c, "Could not read uploaded file")
	}
	if n > 0 {
		sniffed := http.DetectContentType(sniff[:n])
		if isInlineSafeMIME(contentType) && !isInlineSafeMIME(sniffed) && !strings.HasPrefix(sniffed, "application/octet-stream") {
			contentType = sniffed
		}
	}
	if contentType == "" {
		contentType = "application/octet-stream"
	}

	att := &models.Attachment{
		ContextType: "User",
		ContextID:   userID,
		UserID:      userID,
		DisplayName: fileHeader.Filename,
		Filename:    fileHeader.Filename,
		ContentType: contentType,
		Size:        fileHeader.Size,
	}
	uploadCtx := settingsctx.WithAccountID(c.Context(), callerAccountID(c))
	if err := h.fileService.UploadFile(uploadCtx, att, file); err != nil {
		return responses.BadRequest(c, "Could not upload file: "+err.Error())
	}
	return c.Status(fiber.StatusCreated).JSON(fiber.Map{
		"id":           att.ID,
		"display_name": att.DisplayName,
		"filename":     att.Filename,
		"content_type": att.ContentType,
		"size":         att.Size,
	})
}

// DownloadAttachment handles GET /conversations/:id/attachments/:attachment_id.
// The caller must be a participant who can see a message of this
// conversation carrying the file; anything else is a 404.
func (h *ConversationHandler) DownloadAttachment(c *fiber.Ctx) error {
	convID, err := c.ParamsInt("id")
	if err != nil || convID <= 0 {
		return responses.BadRequest(c, "Invalid conversation ID")
	}
	attID, err := c.ParamsInt("attachment_id")
	if err != nil || attID <= 0 {
		return responses.BadRequest(c, "Invalid attachment ID")
	}
	if err := h.requireParticipant(c, uint(convID)); err != nil {
		return err
	}
	if h.inbox == nil || h.fileService == nil {
		return responses.NotFound(c, "file")
	}
	userID, _ := c.Locals("user_id").(uint)
	ok, err := h.inbox.CanSeeAttachment(c.Context(), uint(convID), userID, uint(attID))
	if err != nil || !ok {
		return responses.NotFound(c, "file")
	}
	att, err := h.fileService.GetAttachment(c.Context(), uint(attID), callerAccountID(c))
	if err != nil || att == nil {
		return responses.NotFound(c, "file")
	}

	safeName := filepath.Base(att.Filename)
	safeName = strings.NewReplacer("\"", "", "\r", "", "\n", "").Replace(safeName)
	disposition := "attachment"
	if isInlineSafeMIME(att.ContentType) {
		disposition = "inline"
	}
	c.Set("Content-Disposition", fmt.Sprintf("%s; filename=\"%s\"; filename*=UTF-8''%s", disposition, safeName, url.PathEscape(safeName)))

	backend := h.fileService.StorageBackend()
	if _, isS3 := backend.(*storage.S3Backend); isS3 {
		u, err := h.fileService.GetFileURL(c.Context(), att.ID, callerAccountID(c))
		if err != nil {
			return responses.InternalError(c, "Could not generate download URL")
		}
		return c.Redirect(u, fiber.StatusTemporaryRedirect)
	}
	reader, err := backend.Get(settingsctx.WithAccountID(c.Context(), callerAccountID(c)), att.StoragePath)
	if err != nil {
		return responses.InternalError(c, "Could not locate file")
	}
	// fasthttp closes the reader after streaming (see FileHandler.DownloadFile).
	c.Set("Content-Type", att.ContentType)
	return c.SendStream(reader)
}

// ForwardMessages handles POST /conversations/:id/forward: starts a new
// conversation with new recipients that quotes messages of this one (all
// visible messages, or message_ids) and carries their attachments.
//
//	{"recipients": [30], "body": "FYI", "message_ids": [7], "subject": "optional"}
func (h *ConversationHandler) ForwardMessages(c *fiber.Ctx) error {
	convID, err := c.ParamsInt("id")
	if err != nil || convID <= 0 {
		return responses.BadRequest(c, "Invalid conversation ID")
	}
	if err := h.requireParticipant(c, uint(convID)); err != nil {
		return err
	}
	if h.inbox == nil {
		return responses.Error(c, fiber.StatusServiceUnavailable, "Forwarding is not available")
	}
	var input struct {
		Recipients []any  `json:"recipients"`
		Body       string `json:"body"`
		MessageIDs []uint `json:"message_ids"`
		Subject    string `json:"subject"`
	}
	if err := c.BodyParser(&input); err != nil {
		return responses.BadRequest(c, "Invalid input")
	}
	userID, _ := c.Locals("user_id").(uint)
	src, err := h.conversationService.GetConversation(c.Context(), uint(convID), callerAccountID(c))
	if err != nil || src == nil {
		return responses.NotFound(c, "conversation")
	}
	msgs, err := h.inbox.VisibleMessages(c.Context(), uint(convID), userID, input.MessageIDs)
	if err != nil {
		return responses.InternalError(c, "Could not load messages")
	}
	if len(msgs) == 0 || (len(input.MessageIDs) > 0 && len(msgs) != len(uniqueUints(input.MessageIDs))) {
		return responses.NotFound(c, "message")
	}

	recipients, _, _, wrote, err := h.resolveRecipients(c, userID, input.Recipients)
	if wrote {
		return err
	}
	if len(recipients) == 0 {
		return responses.BadRequest(c, "At least one recipient is required")
	}
	if len(recipients) > maxGroupConversationSize {
		return responses.BadRequest(c, "Too many recipients")
	}
	if h.userService != nil {
		for _, rid := range recipients {
			if u, err := h.userService.GetByID(c.Context(), rid, callerAccountID(c)); err != nil || u == nil {
				return responses.NotFound(c, "recipient")
			}
		}
	}
	if h.addressBook != nil {
		bad, err := h.addressBook.Unaddressable(c.Context(), userID, callerAccountID(c), recipients, h.callerIsAccountAdmin(c, userID))
		if err != nil {
			return responses.InternalError(c, "Could not check recipients")
		}
		if len(bad) > 0 {
			return responses.NotFound(c, "recipient")
		}
	}
	if h.accountRepo != nil {
		if account, err := h.accountRepo.FindByID(c.Context(), callerAccountID(c)); err == nil && account != nil && isCOPPATenant(account) {
			if !h.senderIsTeacherInSharedCourseWithAll(c, userID, recipients) {
				return responses.Error(c, fiber.StatusForbidden, "Direct messages between students require parental consent in this school's privacy mode.")
			}
		}
	}

	subject := strings.TrimSpace(input.Subject)
	if subject == "" {
		subject = "Fwd: " + src.Subject
	}
	body := buildForwardBody(c, h, input.Body, msgs)

	msgIDs := make([]uint, len(msgs))
	for i, m := range msgs {
		msgIDs[i] = m.ID
	}
	var attIDs []uint
	if atts, err := h.inbox.AttachmentsForMessages(c.Context(), msgIDs); err == nil {
		for _, id := range msgIDs {
			for _, a := range atts[id] {
				attIDs = append(attIDs, a.ID)
			}
		}
	}

	conv := &models.Conversation{Subject: subject, CreatedByUserID: userID, ContextType: src.ContextType, ContextID: src.ContextID}
	if err := h.conversationService.CreateConversation(c.Context(), conv, recipients); err != nil {
		return responses.BadRequest(c, err.Error())
	}
	// The forwarded files keep their original owner; linking them here is
	// what lets the new participants download them.
	if _, err := h.postMessage(c, conv, userID, body, nil, attIDs); err != nil {
		return responses.BadRequest(c, err.Error())
	}
	return c.Status(fiber.StatusCreated).JSON(conversationToJSON(conv))
}

func uniqueUints(ids []uint) []uint {
	seen := map[uint]bool{}
	var out []uint
	for _, id := range ids {
		if !seen[id] {
			seen[id] = true
			out = append(out, id)
		}
	}
	return out
}

// buildForwardBody quotes the forwarded messages under the sender's note
// (plain text, like every inbox message).
func buildForwardBody(c *fiber.Ctx, h *ConversationHandler, note string, msgs []models.ConversationMessage) string {
	var b strings.Builder
	if strings.TrimSpace(note) != "" {
		b.WriteString(strings.TrimSpace(note))
		b.WriteString("\n\n")
	}
	for _, m := range msgs {
		name := "Unknown"
		if h.userService != nil {
			if u, err := h.userService.GetByID(c.Context(), m.UserID, callerAccountID(c)); err == nil && u != nil {
				name = u.Name
			}
		}
		b.WriteString("---------- Forwarded message ----------\n")
		b.WriteString("From: " + name + "\n")
		b.WriteString("Date: " + m.CreatedAt.UTC().Format(time.RFC1123) + "\n\n")
		b.WriteString(m.Body)
		b.WriteString("\n\n")
	}
	return strings.TrimSpace(b.String())
}
