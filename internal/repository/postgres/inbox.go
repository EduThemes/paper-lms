package postgres

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/repository"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// InboxRepository holds the per-participant inbox queries (G092): the
// filtered conversation list, unread/starred/archived state, attachments,
// and "Reply" messages restricted to a subset of the participants.
//
// Message visibility for participant p:
//   - the message is not deleted,
//   - it was sent after p.cleared_at (p deleted the conversation earlier),
//   - it is unrestricted, authored by p, or restricted to a set that
//     includes p (conversation_message_recipients).
type InboxRepository struct {
	db *gorm.DB
}

// NewInboxRepository creates the postgres-backed inbox store.
func NewInboxRepository(db *gorm.DB) *InboxRepository {
	return &InboxRepository{db: db}
}

// Inbox scopes (Canvas conversation scopes).
const (
	InboxScopeInbox    = "inbox"
	InboxScopeUnread   = "unread"
	InboxScopeStarred  = "starred"
	InboxScopeArchived = "archived"
	InboxScopeSent     = "sent"
)

// Batch participant events (Canvas PUT /conversations event values).
const (
	InboxEventMarkRead   = "mark_as_read"
	InboxEventMarkUnread = "mark_as_unread"
	InboxEventStar       = "star"
	InboxEventUnstar     = "unstar"
	InboxEventArchive    = "archive"
	InboxEventUnarchive  = "unarchive"
	InboxEventDestroy    = "destroy"
)

// IsInboxEvent reports whether e is a supported batch event.
func IsInboxEvent(e string) bool {
	switch e {
	case InboxEventMarkRead, InboxEventMarkUnread, InboxEventStar, InboxEventUnstar,
		InboxEventArchive, InboxEventUnarchive, InboxEventDestroy:
		return true
	}
	return false
}

// InboxFilter narrows the conversation list.
type InboxFilter struct {
	Scope       string // inbox (default), unread, starred, archived, sent
	ContextType string // "Course" / "Group" ("" = any)
	ContextID   uint
	Search      string // subject substring (case-insensitive)
	// ConversationID limits the query to one conversation (single GET).
	ConversationID uint
}

// InboxConversation is one row of the caller's inbox: the conversation
// plus the caller's own participant state and the last message they can
// see.
type InboxConversation struct {
	ID                  uint       `json:"id"`
	Subject             string     `json:"subject"`
	CreatedByUserID     uint       `json:"created_by_user_id"`
	LastMessageAt       time.Time  `json:"last_message_at"`
	ContextType         string     `json:"context_type"`
	ContextID           uint       `json:"context_id"`
	ContextName         string     `json:"context_name"`
	CreatedAt           time.Time  `json:"created_at"`
	UpdatedAt           time.Time  `json:"updated_at"`
	ParticipantState    string     `json:"participant_state"`
	Starred             bool       `json:"starred"`
	LastReadAt          *time.Time `json:"last_read_at"`
	MessageCount        int64      `json:"message_count"`
	LastMessage         string     `json:"last_message"`
	LastMessageAuthorID uint       `json:"last_message_author_id"`
	LastVisibleAt       *time.Time `json:"last_visible_at"`
	HasAttachments      bool       `json:"has_attachments"`
}

// visibleMessageSQL is the visibility predicate for message alias m and
// participant alias p (see the type comment).
const visibleMessageSQL = `COALESCE(m.workflow_state, 'active') <> 'deleted'
	AND (p.cleared_at IS NULL OR m.created_at > p.cleared_at)
	AND (m.user_id = p.user_id
	     OR NOT EXISTS (SELECT 1 FROM conversation_message_recipients r WHERE r.conversation_message_id = m.id)
	     OR EXISTS (SELECT 1 FROM conversation_message_recipients r WHERE r.conversation_message_id = m.id AND r.user_id = p.user_id))`

func (r *InboxRepository) baseQuery(ctx context.Context, userID, accountID uint, f InboxFilter) *gorm.DB {
	q := r.db.WithContext(ctx).Table("conversations AS c").
		Joins("JOIN conversation_participants p ON p.conversation_id = c.id AND p.user_id = ? AND COALESCE(p.workflow_state, '') <> ?", userID, models.ConversationParticipantRemoved).
		Where("COALESCE(c.workflow_state, 'active') <> 'deleted'").
		// A conversation the participant deleted stays hidden until a
		// message they can see arrives after the deletion.
		Where("(p.cleared_at IS NULL OR EXISTS (SELECT 1 FROM conversation_messages m WHERE m.conversation_id = c.id AND " + visibleMessageSQL + "))")
	if accountID != 0 {
		// Tenant scope through the creator (conversations carry no
		// account_id). accountID==0 is the internal escape hatch only.
		q = q.Where("c.created_by_user_id IN (SELECT id FROM users WHERE account_id = ?)", accountID)
	}
	if f.ConversationID != 0 {
		q = q.Where("c.id = ?", f.ConversationID)
	}
	switch f.Scope {
	case InboxScopeUnread:
		q = q.Where("p.workflow_state = ?", models.ConversationParticipantUnread)
	case InboxScopeStarred:
		q = q.Where("p.starred = ?", true)
	case InboxScopeArchived:
		q = q.Where("p.workflow_state = ?", models.ConversationParticipantArchived)
	case InboxScopeSent:
		q = q.Where("EXISTS (SELECT 1 FROM conversation_messages m WHERE m.conversation_id = c.id AND m.user_id = p.user_id AND " + visibleMessageSQL + ")")
	case "all":
		// every non-deleted conversation (single GET)
	default:
		q = q.Where("COALESCE(p.workflow_state, '') <> ?", models.ConversationParticipantArchived)
	}
	if f.ContextType != "" {
		q = q.Where("c.context_type = ? AND c.context_id = ?", f.ContextType, f.ContextID)
	}
	if s := strings.TrimSpace(f.Search); s != "" {
		q = q.Where(`LOWER(c.subject) LIKE ? ESCAPE '\'`, "%"+escapeLike(strings.ToLower(s))+"%")
	}
	return q
}

// ListForUser returns one page of the caller's conversations, newest
// visible activity first, with the total count.
func (r *InboxRepository) ListForUser(ctx context.Context, userID, accountID uint, f InboxFilter, params repository.PaginationParams) ([]InboxConversation, int64, error) {
	var total int64
	if err := r.baseQuery(ctx, userID, accountID, f).Distinct("c.id").Count(&total).Error; err != nil {
		return nil, 0, err
	}
	if params.PerPage <= 0 {
		params.PerPage = 20
	}
	if params.Page <= 0 {
		params.Page = 1
	}
	var rows []InboxConversation
	err := r.baseQuery(ctx, userID, accountID, f).
		Select(`c.id, c.subject, c.created_by_user_id, c.last_message_at,
			COALESCE(c.context_type, '') AS context_type, COALESCE(c.context_id, 0) AS context_id,
			COALESCE(co.name, g.name, '') AS context_name,
			c.created_at, c.updated_at,
			COALESCE(p.workflow_state, 'read') AS participant_state, p.starred, p.last_read_at,
			(SELECT COUNT(*) FROM conversation_messages m WHERE m.conversation_id = c.id AND ` + visibleMessageSQL + `) AS message_count,
			COALESCE(lm.body, '') AS last_message, COALESCE(lm.user_id, 0) AS last_message_author_id, lm.created_at AS last_visible_at,
			EXISTS (SELECT 1 FROM conversation_message_attachments a JOIN conversation_messages m ON m.id = a.conversation_message_id
			        WHERE m.conversation_id = c.id AND ` + visibleMessageSQL + `) AS has_attachments`).
		Joins(`LEFT JOIN LATERAL (SELECT m.body, m.user_id, m.created_at FROM conversation_messages m
			WHERE m.conversation_id = c.id AND ` + visibleMessageSQL + `
			ORDER BY m.created_at DESC, m.id DESC LIMIT 1) lm ON true`).
		Joins("LEFT JOIN courses co ON c.context_type = 'Course' AND co.id = c.context_id").
		Joins("LEFT JOIN groups g ON c.context_type = 'Group' AND g.id = c.context_id").
		Order("COALESCE(lm.created_at, c.last_message_at) DESC, c.id DESC").
		Offset((params.Page - 1) * params.PerPage).Limit(params.PerPage).
		Scan(&rows).Error
	return rows, total, err
}

// GetForUser returns the caller's view of one conversation (any scope).
func (r *InboxRepository) GetForUser(ctx context.Context, convID, userID, accountID uint) (*InboxConversation, error) {
	rows, _, err := r.ListForUser(ctx, userID, accountID, InboxFilter{Scope: "all", ConversationID: convID}, repository.PaginationParams{Page: 1, PerPage: 1})
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, gorm.ErrRecordNotFound
	}
	return &rows[0], nil
}

// UnreadCount is the number of unread, non-archived conversations.
func (r *InboxRepository) UnreadCount(ctx context.Context, userID, accountID uint) (int64, error) {
	var n int64
	err := r.baseQuery(ctx, userID, accountID, InboxFilter{Scope: InboxScopeUnread}).Distinct("c.id").Count(&n).Error
	return n, err
}

// UpdateParticipants applies a batch event to the caller's own participant
// rows of the given conversations and returns how many rows changed. Rows
// of conversations the caller is not part of are never touched.
func (r *InboxRepository) UpdateParticipants(ctx context.Context, userID uint, convIDs []uint, event string) (int64, error) {
	if len(convIDs) == 0 {
		return 0, nil
	}
	now := time.Now()
	var updates map[string]any
	switch event {
	case InboxEventMarkRead:
		updates = map[string]any{
			"workflow_state": gorm.Expr("CASE WHEN workflow_state = ? THEN workflow_state ELSE ? END", models.ConversationParticipantArchived, models.ConversationParticipantRead),
			"last_read_at":   now,
		}
	case InboxEventMarkUnread:
		updates = map[string]any{"workflow_state": models.ConversationParticipantUnread}
	case InboxEventStar:
		updates = map[string]any{"starred": true}
	case InboxEventUnstar:
		updates = map[string]any{"starred": false}
	case InboxEventArchive:
		updates = map[string]any{"workflow_state": models.ConversationParticipantArchived, "last_read_at": now}
	case InboxEventUnarchive:
		updates = map[string]any{"workflow_state": gorm.Expr("CASE WHEN workflow_state = ? THEN ? ELSE workflow_state END", models.ConversationParticipantArchived, models.ConversationParticipantRead)}
	case InboxEventDestroy:
		updates = map[string]any{"cleared_at": now, "workflow_state": models.ConversationParticipantRead, "starred": false, "last_read_at": now}
	default:
		return 0, errors.New("unknown inbox event")
	}
	updates["updated_at"] = now
	res := r.db.WithContext(ctx).Model(&models.ConversationParticipant{}).
		Where("user_id = ? AND conversation_id IN ? AND COALESCE(workflow_state, '') <> ?", userID, convIDs, models.ConversationParticipantRemoved).
		Updates(updates)
	return res.RowsAffected, res.Error
}

// OnNewMessage updates participant state after senderID posted a message:
// the recipients (all other participants, or only restrictTo when the
// message is a restricted "Reply") become unread and leave the archive;
// the sender's copy is read.
func (r *InboxRepository) OnNewMessage(ctx context.Context, convID, senderID uint, restrictTo []uint) error {
	now := time.Now()
	q := r.db.WithContext(ctx).Model(&models.ConversationParticipant{}).
		Where("conversation_id = ? AND user_id <> ? AND COALESCE(workflow_state, '') <> ?", convID, senderID, models.ConversationParticipantRemoved)
	if len(restrictTo) > 0 {
		q = q.Where("user_id IN ?", restrictTo)
	}
	if err := q.Updates(map[string]any{"workflow_state": models.ConversationParticipantUnread, "updated_at": now}).Error; err != nil {
		return err
	}
	return r.db.WithContext(ctx).Model(&models.ConversationParticipant{}).
		Where("conversation_id = ? AND user_id = ? AND COALESCE(workflow_state, '') <> ?", convID, senderID, models.ConversationParticipantRemoved).
		Updates(map[string]any{"workflow_state": models.ConversationParticipantRead, "last_read_at": now, "updated_at": now}).Error
}

// AddMessageRecipients restricts a message to userIDs (plus its author).
func (r *InboxRepository) AddMessageRecipients(ctx context.Context, messageID uint, userIDs []uint) error {
	if len(userIDs) == 0 {
		return nil
	}
	rows := make([]models.ConversationMessageRecipient, 0, len(userIDs))
	for _, id := range userIDs {
		rows = append(rows, models.ConversationMessageRecipient{ConversationMessageID: messageID, UserID: id})
	}
	return r.db.WithContext(ctx).Clauses(clause.OnConflict{DoNothing: true}).Create(&rows).Error
}

// LinkAttachments attaches files to a message.
func (r *InboxRepository) LinkAttachments(ctx context.Context, messageID uint, attachmentIDs []uint) error {
	if len(attachmentIDs) == 0 {
		return nil
	}
	rows := make([]models.ConversationMessageAttachment, 0, len(attachmentIDs))
	for _, id := range attachmentIDs {
		rows = append(rows, models.ConversationMessageAttachment{ConversationMessageID: messageID, AttachmentID: id})
	}
	return r.db.WithContext(ctx).Clauses(clause.OnConflict{DoNothing: true}).Create(&rows).Error
}

// AttachmentsForMessages returns the live attachments of each message.
func (r *InboxRepository) AttachmentsForMessages(ctx context.Context, messageIDs []uint) (map[uint][]models.Attachment, error) {
	out := map[uint][]models.Attachment{}
	if len(messageIDs) == 0 {
		return out, nil
	}
	var rows []struct {
		MessageID uint
		models.Attachment
	}
	err := r.db.WithContext(ctx).Table("conversation_message_attachments AS cma").
		Select("cma.conversation_message_id AS message_id, attachments.*").
		Joins("JOIN attachments ON attachments.id = cma.attachment_id").
		Where("cma.conversation_message_id IN ? AND attachments.workflow_state <> 'deleted'", messageIDs).
		Order("cma.id ASC").
		Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	for _, row := range rows {
		out[row.MessageID] = append(out[row.MessageID], row.Attachment)
	}
	return out, nil
}

// RecipientsForMessages returns the restricted recipient set of each
// restricted message (unrestricted messages are absent from the map).
func (r *InboxRepository) RecipientsForMessages(ctx context.Context, messageIDs []uint) (map[uint][]uint, error) {
	out := map[uint][]uint{}
	if len(messageIDs) == 0 {
		return out, nil
	}
	var rows []models.ConversationMessageRecipient
	if err := r.db.WithContext(ctx).Where("conversation_message_id IN ?", messageIDs).Order("id ASC").Find(&rows).Error; err != nil {
		return nil, err
	}
	for _, row := range rows {
		out[row.ConversationMessageID] = append(out[row.ConversationMessageID], row.UserID)
	}
	return out, nil
}

func (r *InboxRepository) visibleMessages(ctx context.Context, convID, userID uint) *gorm.DB {
	return r.db.WithContext(ctx).Table("conversation_messages AS m").
		Joins("JOIN conversation_participants p ON p.conversation_id = m.conversation_id AND p.user_id = ? AND COALESCE(p.workflow_state, '') <> ?", userID, models.ConversationParticipantRemoved).
		Where("m.conversation_id = ?", convID).
		Where(visibleMessageSQL)
}

// ListVisibleMessages pages through the messages the user can see, oldest
// first.
func (r *InboxRepository) ListVisibleMessages(ctx context.Context, convID, userID uint, params repository.PaginationParams) (*repository.PaginatedResult[models.ConversationMessage], error) {
	var total int64
	if err := r.visibleMessages(ctx, convID, userID).Count(&total).Error; err != nil {
		return nil, err
	}
	if params.PerPage <= 0 {
		params.PerPage = 50
	}
	if params.Page <= 0 {
		params.Page = 1
	}
	var msgs []models.ConversationMessage
	err := r.visibleMessages(ctx, convID, userID).Select("m.*").
		Order("m.created_at ASC, m.id ASC").
		Offset((params.Page - 1) * params.PerPage).Limit(params.PerPage).
		Scan(&msgs).Error
	if err != nil {
		return nil, err
	}
	return &repository.PaginatedResult[models.ConversationMessage]{Items: msgs, TotalCount: total, Page: params.Page, PerPage: params.PerPage}, nil
}

// VisibleMessages returns the listed messages of the conversation that
// the user can see (used to forward them).
func (r *InboxRepository) VisibleMessages(ctx context.Context, convID, userID uint, messageIDs []uint) ([]models.ConversationMessage, error) {
	q := r.visibleMessages(ctx, convID, userID)
	if len(messageIDs) > 0 {
		q = q.Where("m.id IN ?", messageIDs)
	}
	var msgs []models.ConversationMessage
	err := q.Select("m.*").Order("m.created_at ASC, m.id ASC").Scan(&msgs).Error
	return msgs, err
}

// CanSeeAttachment reports whether the user can see a message of the
// conversation that carries the attachment.
func (r *InboxRepository) CanSeeAttachment(ctx context.Context, convID, userID, attachmentID uint) (bool, error) {
	var n int64
	err := r.visibleMessages(ctx, convID, userID).
		Where("EXISTS (SELECT 1 FROM conversation_message_attachments a WHERE a.conversation_message_id = m.id AND a.attachment_id = ?)", attachmentID).
		Count(&n).Error
	return n > 0, err
}
