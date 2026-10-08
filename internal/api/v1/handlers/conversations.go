package handlers

import (
	"context"
	"html"
	"strings"

	"github.com/gofiber/fiber/v2"
	"github.com/EduThemes/paper-lms/internal/api/v1/middleware"
	"github.com/EduThemes/paper-lms/internal/api/v1/responses"
	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/repository"
	"github.com/EduThemes/paper-lms/internal/service"
)

type ConversationHandler struct {
	conversationService *service.ConversationService
	userService         *service.UserService
	// accountRepo + enrollmentRepo drive the 13.4 COPPA gate on
	// CreateConversation. auditService drives 13.5 LogPIIAccess on
	// reads. All three nil-safe — handler degrades to legacy
	// pre-Phase-13 behavior when any is unset.
	accountRepo    repository.AccountRepository
	enrollmentRepo repository.EnrollmentRepository
	auditService   *service.AuditService
	// notifier queues "new_message" notifications to the other
	// participants. Nil disables (tests).
	notifier service.Notifier
	// recipientSearch backs GET /search/recipients. Nil → 503.
	recipientSearch RecipientSearcher
	// inbox holds the per-participant inbox state (unread / starred /
	// archived / deleted-for-me), attachments and restricted replies
	// (G092). Nil keeps the legacy shared-thread behavior (unit tests).
	inbox InboxStore
	// addressBook scopes recipients to people who share a course or
	// group with the sender and expands course/section/group recipients
	// (G093). Nil keeps only the tenant check (unit tests).
	addressBook AddressBook
	// fileService stores and serves message attachments. Nil disables
	// attachments.
	fileService *service.FileService
}

// RecipientSearcher finds users the caller may message (see
// postgres.RecipientSearchRepository).
type RecipientSearcher interface {
	SearchRecipients(ctx context.Context, callerID, accountID uint, term string, allInAccount bool, limit int) ([]models.User, error)
}

// SetRecipientSearch wires GET /search/recipients.
func (h *ConversationHandler) SetRecipientSearch(s RecipientSearcher) {
	h.recipientSearch = s
}

// SearchRecipients handles GET /search/recipients?search=<term>&per_page=N
// and GET /search/recipients?context=course_5[_students]&search=<term>.
//
// The inbox recipient picker for every role. GET /users is admin-only, so
// students and teachers need a scoped search: like Canvas, candidates are
// the people who share an active course or group with the caller, within
// the caller's tenant. Admins may address anyone in their account. With
// context=, the members of that course / section / group the caller
// belongs to are listed (browsing the address book by context).
func (h *ConversationHandler) SearchRecipients(c *fiber.Ctx) error {
	userID, err := getUserID(c)
	if err != nil {
		return err
	}
	if h.recipientSearch == nil {
		return responses.Error(c, fiber.StatusServiceUnavailable, "Recipient search is not available")
	}
	term := strings.TrimSpace(c.Query("search"))
	if term == "" {
		term = strings.TrimSpace(c.Query("search_term"))
	}
	limit := c.QueryInt("per_page", 10)
	if limit < 1 || limit > 50 {
		limit = 10
	}
	isAdmin := h.callerIsAccountAdmin(c, userID)

	if code := strings.TrimSpace(c.Query("context")); code != "" {
		users, wrote, err := h.browseContext(c, userID, isAdmin, code, term, limit)
		if wrote {
			return err
		}
		return c.JSON(recipientsToJSON(users))
	}

	if len([]rune(term)) < 2 {
		return c.JSON([]fiber.Map{})
	}
	users, err := h.recipientSearch.SearchRecipients(c.Context(), userID, callerAccountID(c), term, isAdmin, limit)
	if err != nil {
		return responses.InternalError(c, "Could not search recipients")
	}
	return c.JSON(recipientsToJSON(users))
}

func recipientsToJSON(users []models.User) []fiber.Map {
	out := make([]fiber.Map, len(users))
	for i, u := range users {
		out[i] = fiber.Map{
			"id":            u.ID,
			"name":          u.Name,
			"sortable_name": u.SortableName,
			"avatar_url":    u.AvatarURL,
		}
	}
	return out
}

// callerIsAccountAdmin is the DB-backed role check (the JWT is_admin hint
// can be stale).
func (h *ConversationHandler) callerIsAccountAdmin(c *fiber.Ctx, userID uint) bool {
	if h.userService == nil {
		return false
	}
	me, err := h.userService.GetByID(c.Context(), userID, callerAccountID(c))
	return err == nil && me != nil && (me.Role == "admin" || me.Role == "super_admin")
}

// SetNotifier wires new-message notifications.
func (h *ConversationHandler) SetNotifier(n service.Notifier) {
	h.notifier = n
}

// notifyNewMessage queues a "new_message" notification for every
// participant except the sender — or, for a restricted reply, only the
// users in onlyTo. Best-effort: never fails the request.
func (h *ConversationHandler) notifyNewMessage(c *fiber.Ctx, conversationID, senderID uint, subject, body string, onlyTo []uint) {
	if h.notifier == nil {
		return
	}
	participants, err := h.conversationService.GetParticipants(c.Context(), conversationID)
	if err != nil {
		return
	}
	var allowed map[uint]bool
	if len(onlyTo) > 0 {
		allowed = map[uint]bool{}
		for _, id := range onlyTo {
			allowed[id] = true
		}
	}
	senderName := "Someone"
	if h.userService != nil {
		if u, err := h.userService.GetByID(c.Context(), senderID, callerAccountID(c)); err == nil && u != nil {
			senderName = u.Name
		}
	}
	title := "New message from " + senderName
	if strings.TrimSpace(subject) != "" {
		title += ": " + subject
	}
	for _, p := range participants {
		if p.UserID == senderID || (allowed != nil && !allowed[p.UserID]) {
			continue
		}
		// Defense in depth: only notify participants of the sender's
		// tenant (conversations created before recipients were
		// tenant-checked may hold foreign ids).
		if h.userService != nil {
			if u, err := h.userService.GetByID(c.Context(), p.UserID, callerAccountID(c)); err != nil || u == nil {
				continue
			}
		}
		_ = h.notifier.QueueNotification(c.Context(), p.UserID, "new_message", title, html.EscapeString(body), "Conversation", conversationID)
	}
}

func NewConversationHandler(conversationService *service.ConversationService, userService *service.UserService, accountRepo repository.AccountRepository, enrollmentRepo repository.EnrollmentRepository, auditService *service.AuditService) *ConversationHandler {
	return &ConversationHandler{
		conversationService: conversationService,
		userService:         userService,
		accountRepo:         accountRepo,
		enrollmentRepo:      enrollmentRepo,
		auditService:        auditService,
	}
}

// isCOPPATenant returns true when the account is in a privacy mode that
// requires the conversation gate: explicit CoppaStrict OR K-12 tenant
// modes (k5, m68). Mirrors the ai_assist.go gate.
func isCOPPATenant(account *models.Account) bool {
	if account == nil {
		return false
	}
	return account.CoppaStrict || string(account.TenantMode) == "k5" || string(account.TenantMode) == "m68"
}

// senderIsTeacherInSharedCourseWithAll returns true when the sender holds
// a Teacher/TA enrollment in at least one course where every recipient
// is also enrolled (any role). Used to gate student-to-student DMs in
// COPPA tenants.
func (h *ConversationHandler) senderIsTeacherInSharedCourseWithAll(c *fiber.Ctx, senderID uint, recipients []uint) bool {
	if h.enrollmentRepo == nil {
		// Fail-closed: the gate is on, but we can't verify. Reject the
		// DM rather than leak past the policy.
		return false
	}
	senderEnrollments, err := h.enrollmentRepo.ListByUserID(c.Context(), senderID, callerAccountID(c))
	if err != nil {
		return false
	}
	// Collect the courses where sender is a teacher/TA.
	teacherCourses := make(map[uint]bool)
	for _, e := range senderEnrollments {
		if e.Type == "TeacherEnrollment" || e.Type == "TaEnrollment" {
			teacherCourses[e.CourseID] = true
		}
	}
	if len(teacherCourses) == 0 {
		return false
	}
	// For every recipient, check they share at least one of those courses.
	for _, rid := range recipients {
		recipientEnrollments, err := h.enrollmentRepo.ListByUserID(c.Context(), rid, callerAccountID(c))
		if err != nil {
			return false
		}
		shared := false
		for _, e := range recipientEnrollments {
			if teacherCourses[e.CourseID] {
				shared = true
				break
			}
		}
		if !shared {
			return false
		}
	}
	return true
}

func conversationToJSON(c *models.Conversation) fiber.Map {
	return fiber.Map{
		"id":                 c.ID,
		"subject":            c.Subject,
		"created_by_user_id": c.CreatedByUserID,
		"last_message_at":    c.LastMessageAt,
		"workflow_state":     c.WorkflowState,
		"context_type":       c.ContextType,
		"context_id":         c.ContextID,
		"context_code":       contextCode(c.ContextType, c.ContextID),
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
// Non-participants get 404, not 403 — surfacing a 403 leaks the existence of the
// conversation across the tenant boundary and violates the 13.1.E contract. The
// not-found shape is also consistent with cross-tenant FindByID misses elsewhere.
//
// Returns a *fiber.Error (rendered by the app ErrorHandler) and does NOT
// write the response: it used to return responses.NotFound(...), which is
// nil after writing, so every `if err := h.requireParticipant(...); err !=
// nil { return err }` caller fell through and a non-participant could read,
// reply to, update or delete the conversation.
func (h *ConversationHandler) requireParticipant(c *fiber.Ctx, conversationID uint) error {
	notFound := fiber.NewError(fiber.StatusNotFound, "The specified conversation was not found")
	userID, _ := c.Locals("user_id").(uint)
	if userID == 0 {
		return notFound
	}
	participants, err := h.conversationService.GetParticipants(c.Context(), conversationID)
	if err != nil {
		return notFound
	}
	for _, p := range participants {
		if p.UserID == userID {
			return nil
		}
	}
	return notFound
}

func (h *ConversationHandler) resolveParticipants(c *fiber.Ctx, conversationID uint) []fiber.Map {
	participants, err := h.conversationService.GetParticipants(c.Context(), conversationID)
	if err != nil {
		return nil
	}
	result := make([]fiber.Map, 0, len(participants))
	for _, p := range participants {
		name := ""
		if user, err := h.userService.GetByID(c.Context(), p.UserID, callerAccountID(c)); err == nil {
			name = user.Name
		}
		result = append(result, fiber.Map{
			"id":   p.UserID,
			"name": name,
		})
	}
	return result
}

// ListConversations handles GET /conversations.
//
// With the inbox store wired it accepts Canvas's filters:
//
//	scope   = inbox (default) | unread | starred | archived | sent
//	filter  = course_<id> | group_<id>   (context filter)
//	search  = subject substring
//
// and each row carries the caller's own workflow_state (unread / read /
// archived), starred, message_count and a last_message preview.
func (h *ConversationHandler) ListConversations(c *fiber.Ctx) error {
	userID, _ := c.Locals("user_id").(uint)
	accountID := callerAccountID(c)

	params := middleware.GetPagination(c)

	if h.inbox != nil {
		filter, ok := inboxFilterFromQuery(c)
		if !ok {
			return responses.BadRequest(c, "Invalid filter; use course_<id> or group_<id>")
		}
		rows, total, err := h.inbox.ListForUser(c.Context(), userID, accountID, filter, params)
		if err != nil {
			return responses.InternalError(c, "Could not fetch conversations")
		}
		responses.SetPaginationHeaders(c, total, params.Page, params.PerPage)
		out := make([]fiber.Map, len(rows))
		for i := range rows {
			j := inboxConversationToJSON(&rows[i])
			j["participants"] = h.resolveParticipants(c, rows[i].ID)
			out[i] = j
		}
		return c.JSON(out)
	}

	result, err := h.conversationService.ListByUser(c.Context(), userID, accountID, params)
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

	conv, err := h.conversationService.GetConversation(c.Context(), uint(id), callerAccountID(c))
	if err != nil {
		return responses.NotFound(c, "conversation")
	}

	// 13.5 PII audit — conversations have N participants; per-row
	// emission would inflate the audit table for no investigative
	// win. One row per access with student_id=0 +
	// data_field="bulk_conversation_read" captures the meaningful
	// signal (who looked at which conversation, when).
	if callerID, _ := getUserID(c); callerID != 0 && h.auditService != nil {
		_ = h.auditService.LogPIIAccess(c.Context(), callerID, 0, "read", "bulk_conversation_read", "conversations", conv.ID, c.IP(), c.Get("User-Agent"))
	}

	j := conversationToJSON(conv)
	if h.inbox != nil {
		userID, _ := c.Locals("user_id").(uint)
		if row, err := h.inbox.GetForUser(c.Context(), conv.ID, userID, callerAccountID(c)); err == nil && row != nil {
			j = inboxConversationToJSON(row)
		}
	}
	j["participants"] = h.resolveParticipants(c, conv.ID)
	return c.JSON(j)
}

// CreateConversation handles POST /conversations.
//
//	{"conversation": {
//	   "subject": "...",
//	   "recipients": [12, "course_5_students", "section_3", "group_7"],
//	   "body": "first message (optional)",
//	   "attachment_ids": [41],
//	   "context_code": "course_5",
//	   "group_conversation": false   // false = one private conversation per recipient
//	}}
//
// Recipients must be in the caller's address book (G093): users who share
// an active course or group with the caller (admins: anyone in their
// account). Course / section recipients are for the course's instructors
// (and admins); a group's members may message the group.
func (h *ConversationHandler) CreateConversation(c *fiber.Ctx) error {
	var input struct {
		Conversation struct {
			Subject           string `json:"subject"`
			Recipients        []any  `json:"recipients"`
			Body              string `json:"body"`
			AttachmentIDs     []uint `json:"attachment_ids"`
			ContextCode       string `json:"context_code"`
			GroupConversation *bool  `json:"group_conversation"`
		} `json:"conversation"`
	}

	if err := c.BodyParser(&input); err != nil {
		return responses.BadRequest(c, "Invalid input")
	}

	userID, _ := c.Locals("user_id").(uint)
	in := input.Conversation

	recipients, ctxType, ctxID, wrote, err := h.resolveRecipients(c, userID, in.Recipients)
	if wrote {
		return err
	}
	if code := strings.TrimSpace(in.ContextCode); code != "" {
		t, id, wrote, err := h.resolveContextCode(c, userID, code)
		if wrote {
			return err
		}
		ctxType, ctxID = t, id
	}
	if len(recipients) == 0 {
		return responses.BadRequest(c, "At least one recipient is required")
	}
	if len(recipients) > maxConversationRecipients {
		return responses.BadRequest(c, "Too many recipients")
	}

	// 13.4 — COPPA gate. In tenants with coppa_strict=true OR
	// tenant_mode in {k5,m68}, student-to-student DMs are a non-starter
	// without parental consent. The narrow exception is teacher-led
	// communication: the sender must be a teacher/TA in a course where
	// every recipient is enrolled. Anything else is refused outright.
	if h.accountRepo != nil {
		accountID, _ := c.Locals("account_id").(uint)
		if accountID > 0 {
			if account, err := h.accountRepo.FindByID(c.Context(), accountID); err == nil && account != nil {
				if isCOPPATenant(account) {
					if !h.senderIsTeacherInSharedCourseWithAll(c, userID, recipients) {
						return responses.Error(c, fiber.StatusForbidden, "Direct messages between students require parental consent in this school's privacy mode.")
					}
				}
			}
		}
	}

	// Every recipient must be a user of the caller's tenant. A message
	// now also notifies (in-app and by email) every participant, so an
	// unchecked id would let anyone mail users of another school.
	// Unknown / other-tenant ids are a 404, never a hint they exist.
	if h.userService != nil {
		for _, rid := range recipients {
			if rid == userID {
				continue
			}
			if u, err := h.userService.GetByID(c.Context(), rid, callerAccountID(c)); err != nil || u == nil {
				return responses.NotFound(c, "recipient")
			}
		}
	}
	// ...and in the caller's address book (G093).
	if h.addressBook != nil {
		bad, err := h.addressBook.Unaddressable(c.Context(), userID, callerAccountID(c), recipients, h.callerIsAccountAdmin(c, userID))
		if err != nil {
			return responses.InternalError(c, "Could not check recipients")
		}
		if len(bad) > 0 {
			return responses.NotFound(c, "recipient")
		}
	}

	attachments, wrote, err := h.ownAttachments(c, userID, in.AttachmentIDs)
	if wrote {
		return err
	}

	group := len(recipients) <= maxGroupConversationSize
	if in.GroupConversation != nil {
		group = *in.GroupConversation && len(recipients) <= maxGroupConversationSize
	}
	sets := [][]uint{recipients}
	if !group && len(recipients) > 1 {
		sets = make([][]uint, 0, len(recipients))
		for _, r := range recipients {
			sets = append(sets, []uint{r})
		}
	}

	var created []*models.Conversation
	for _, set := range sets {
		conv := &models.Conversation{
			Subject:         in.Subject,
			CreatedByUserID: userID,
			ContextType:     ctxType,
			ContextID:       ctxID,
		}
		if err := h.conversationService.CreateConversation(c.Context(), conv, set); err != nil {
			return responses.BadRequest(c, err.Error())
		}
		if strings.TrimSpace(in.Body) != "" {
			if _, err := h.postMessage(c, conv, userID, in.Body, nil, attachments); err != nil {
				return responses.BadRequest(c, err.Error())
			}
		}
		created = append(created, conv)
	}

	j := conversationToJSON(created[0])
	ids := make([]uint, len(created))
	for i, cv := range created {
		ids[i] = cv.ID
	}
	j["conversation_ids"] = ids
	return c.Status(fiber.StatusCreated).JSON(j)
}

// UpdateConversation handles PUT /conversations/:id.
//
// subject is shared by every participant. workflow_state ("read",
// "unread", "archived") and starred are the CALLER's own inbox state
// (Canvas semantics): they only ever touch the caller's participant row,
// so one participant can't archive or star the thread for anyone else.
func (h *ConversationHandler) UpdateConversation(c *fiber.Ctx) error {
	id, err := c.ParamsInt("id")
	if err != nil {
		return responses.BadRequest(c, "Invalid conversation ID")
	}

	if err := h.requireParticipant(c, uint(id)); err != nil {
		return err
	}

	conv, err := h.conversationService.GetConversation(c.Context(), uint(id), callerAccountID(c))
	if err != nil {
		return responses.NotFound(c, "conversation")
	}

	var input struct {
		Conversation struct {
			Subject       *string `json:"subject"`
			WorkflowState *string `json:"workflow_state"`
			Starred       *bool   `json:"starred"`
		} `json:"conversation"`
	}

	if err := c.BodyParser(&input); err != nil {
		return responses.BadRequest(c, "Invalid input")
	}

	userID, _ := c.Locals("user_id").(uint)
	if input.Conversation.WorkflowState != nil || input.Conversation.Starred != nil {
		if h.inbox == nil {
			return responses.BadRequest(c, "Inbox state is not available")
		}
		var events []string
		if ws := input.Conversation.WorkflowState; ws != nil {
			switch *ws {
			case models.ConversationParticipantRead:
				events = append(events, inboxEventMarkRead, inboxEventUnarchive)
			case models.ConversationParticipantUnread:
				events = append(events, inboxEventMarkUnread)
			case models.ConversationParticipantArchived:
				events = append(events, inboxEventArchive)
			default:
				return responses.BadRequest(c, "workflow_state must be one of: read, unread, archived")
			}
		}
		if st := input.Conversation.Starred; st != nil {
			if *st {
				events = append(events, inboxEventStar)
			} else {
				events = append(events, inboxEventUnstar)
			}
		}
		for _, ev := range events {
			if _, err := h.inbox.UpdateParticipants(c.Context(), userID, []uint{conv.ID}, ev); err != nil {
				return responses.InternalError(c, "Could not update conversation")
			}
		}
	}

	if input.Conversation.Subject != nil {
		conv.Subject = *input.Conversation.Subject
		if err := h.conversationService.UpdateConversation(c.Context(), conv); err != nil {
			return responses.InternalError(c, "Could not update conversation")
		}
	}

	j := conversationToJSON(conv)
	if h.inbox != nil {
		if row, err := h.inbox.GetForUser(c.Context(), conv.ID, userID, callerAccountID(c)); err == nil && row != nil {
			j = inboxConversationToJSON(row)
		}
	}
	return c.JSON(j)
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
	userID, _ := c.Locals("user_id").(uint)

	var result *repository.PaginatedResult[models.ConversationMessage]
	if h.inbox != nil {
		// Only the messages this participant can see: not the ones they
		// deleted, and not "Reply" messages addressed to someone else.
		result, err = h.inbox.ListVisibleMessages(c.Context(), uint(id), userID, params)
	} else {
		result, err = h.conversationService.ListMessages(c.Context(), uint(id), params)
	}
	if err != nil {
		return responses.InternalError(c, "Could not fetch messages")
	}

	responses.SetPaginationHeaders(c, result.TotalCount, result.Page, result.PerPage)

	messages := h.messagesToJSON(c, uint(id), result.Items)

	// 13.5 PII audit — bulk-read semantics on a conversation message
	// list. Same rationale as GetConversation: messages have N
	// authors; emit one row per call keyed to the conversation ID.
	if callerID, _ := getUserID(c); callerID != 0 && h.auditService != nil {
		_ = h.auditService.LogPIIAccess(c.Context(), callerID, 0, "read", "bulk_conversation_message_read", "conversations", uint(id), c.IP(), c.Get("User-Agent"))
	}

	return c.JSON(messages)
}

// messagesToJSON renders messages with their author name, attachments and
// (for a restricted reply) recipient ids.
func (h *ConversationHandler) messagesToJSON(c *fiber.Ctx, conversationID uint, msgs []models.ConversationMessage) []fiber.Map {
	ids := make([]uint, len(msgs))
	for i, m := range msgs {
		ids[i] = m.ID
	}
	var atts map[uint][]models.Attachment
	var recips map[uint][]uint
	if h.inbox != nil && len(ids) > 0 {
		atts, _ = h.inbox.AttachmentsForMessages(c.Context(), ids)
		recips, _ = h.inbox.RecipientsForMessages(c.Context(), ids)
	}
	names := map[uint]string{}
	out := make([]fiber.Map, len(msgs))
	for i := range msgs {
		m := &msgs[i]
		j := conversationMessageToJSON(m)
		name, ok := names[m.UserID]
		if !ok {
			if user, err := h.userService.GetByID(c.Context(), m.UserID, callerAccountID(c)); err == nil && user != nil {
				name = user.Name
			}
			names[m.UserID] = name
		}
		if name != "" {
			j["user_name"] = name
		}
		j["attachments"] = messageAttachmentsJSON(conversationID, atts[m.ID])
		if r := recips[m.ID]; len(r) > 0 {
			j["recipient_ids"] = r
		}
		out[i] = j
	}
	return out
}

// CreateMessage handles POST /conversations/:id/messages.
//
//	{"message": "...", "attachment_ids": [41], "recipients": [20]}
//
// recipients (optional) restricts the message to a subset of the other
// participants — Canvas's "Reply" as opposed to "Reply all".
func (h *ConversationHandler) CreateMessage(c *fiber.Ctx) error {
	id, err := c.ParamsInt("id")
	if err != nil {
		return responses.BadRequest(c, "Invalid conversation ID")
	}

	if err := h.requireParticipant(c, uint(id)); err != nil {
		return err
	}

	var input struct {
		Message       string `json:"message"`
		Body          string `json:"body"`
		AttachmentIDs []uint `json:"attachment_ids"`
		Recipients    []uint `json:"recipients"`
	}

	if err := c.BodyParser(&input); err != nil {
		return responses.BadRequest(c, "Invalid input")
	}
	if input.Message == "" {
		input.Message = input.Body
	}

	userID, _ := c.Locals("user_id").(uint)

	onlyTo, wrote, err := h.replySubset(c, uint(id), userID, input.Recipients)
	if wrote {
		return err
	}
	attachments, wrote, err := h.ownAttachments(c, userID, input.AttachmentIDs)
	if wrote {
		return err
	}

	conv, err := h.conversationService.GetConversation(c.Context(), uint(id), callerAccountID(c))
	if err != nil || conv == nil {
		// The participant gate passed; fall back to a bare row so the
		// message can still be written (legacy cross-tenant rows).
		conv = &models.Conversation{ID: uint(id)}
	}
	msg, err := h.postMessage(c, conv, userID, input.Message, onlyTo, attachments)
	if err != nil {
		return responses.BadRequest(c, err.Error())
	}

	j := h.messagesToJSON(c, uint(id), []models.ConversationMessage{*msg})[0]
	return c.Status(fiber.StatusCreated).JSON(j)
}

// postMessage writes a message, its attachment links and restricted
// recipients, updates the participants' inbox state and notifies.
func (h *ConversationHandler) postMessage(c *fiber.Ctx, conv *models.Conversation, senderID uint, body string, onlyTo []uint, attachments []uint) (*models.ConversationMessage, error) {
	msg := &models.ConversationMessage{
		ConversationID: conv.ID,
		UserID:         senderID,
		Body:           body,
	}
	if err := h.conversationService.CreateMessage(c.Context(), msg); err != nil {
		return nil, err
	}
	if h.inbox != nil {
		_ = h.inbox.AddMessageRecipients(c.Context(), msg.ID, onlyTo)
		_ = h.inbox.LinkAttachments(c.Context(), msg.ID, attachments)
		_ = h.inbox.OnNewMessage(c.Context(), conv.ID, senderID, onlyTo)
	}
	h.notifyNewMessage(c, conv.ID, senderID, conv.Subject, body, onlyTo)
	return msg, nil
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
