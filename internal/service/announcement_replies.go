package service

import (
	"context"
	"errors"
	"strings"

	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/repository"
)

// Announcement replies (G090). Canvas announcements are discussion topics,
// so readers can reply when the announcement allows comments. Replies are
// threaded one level deep or more via ParentID, and the announcement's
// author is notified ("announcement_reply", category "Announcement Created
// By You").

// ErrRepliesDisabled is returned when the announcement does not allow
// comments.
var ErrRepliesDisabled = errors.New("this announcement does not allow comments")

// ErrRepliesUnavailable is returned when reply storage is not wired.
var ErrRepliesUnavailable = errors.New("announcement replies are not available")

// ErrReplyNotFound is returned for a missing reply / parent.
var ErrReplyNotFound = errors.New("reply not found")

// RepliesEnabled reports whether reply storage is wired.
func (s *AnnouncementService) RepliesEnabled() bool { return s.extras != nil }

// CanReply reports whether userID may post a reply: the announcement must
// be visible to them, and either allow comments or be managed by them.
func (s *AnnouncementService) CanReply(a *models.Announcement, canManage bool) error {
	if s.extras == nil {
		return ErrRepliesUnavailable
	}
	if !a.AllowComments && !canManage {
		return ErrRepliesDisabled
	}
	return nil
}

// CreateReply stores a reply (message already sanitized by the caller) and
// notifies the announcement's author.
func (s *AnnouncementService) CreateReply(ctx context.Context, a *models.Announcement, userID uint, message string, parentID *uint) (*models.AnnouncementReply, error) {
	if s.extras == nil {
		return nil, ErrRepliesUnavailable
	}
	if strings.TrimSpace(message) == "" {
		return nil, errors.New("message is required")
	}
	if parentID != nil {
		if _, err := s.extras.FindReply(ctx, a.ID, *parentID); err != nil {
			return nil, ErrReplyNotFound
		}
	}
	reply := &models.AnnouncementReply{
		AnnouncementID: a.ID,
		UserID:         userID,
		ParentID:       parentID,
		Message:        message,
		WorkflowState:  models.AnnouncementReplyActive,
	}
	if err := s.extras.CreateReply(ctx, reply); err != nil {
		return nil, err
	}
	if s.notifier != nil && a.UserID != 0 && a.UserID != userID {
		contextType, contextID := "Announcement", a.ID
		if a.CourseID != nil {
			contextType, contextID = "Course", *a.CourseID
		}
		_ = s.notifier.QueueNotification(ctx, a.UserID, "announcement_reply", "New reply to: "+a.Title, message, contextType, contextID)
	}
	return reply, nil
}

// ListReplies pages through the replies of an announcement.
func (s *AnnouncementService) ListReplies(ctx context.Context, announcementID uint, params repository.PaginationParams) (*repository.PaginatedResult[models.AnnouncementReply], error) {
	if s.extras == nil {
		return nil, ErrRepliesUnavailable
	}
	return s.extras.ListReplies(ctx, announcementID, params)
}

// GetReply loads a live reply of the announcement.
func (s *AnnouncementService) GetReply(ctx context.Context, announcementID, replyID uint) (*models.AnnouncementReply, error) {
	if s.extras == nil {
		return nil, ErrRepliesUnavailable
	}
	r, err := s.extras.FindReply(ctx, announcementID, replyID)
	if err != nil {
		return nil, ErrReplyNotFound
	}
	return r, nil
}

// UpdateReply rewrites a reply's message.
func (s *AnnouncementService) UpdateReply(ctx context.Context, r *models.AnnouncementReply, message string) error {
	if strings.TrimSpace(message) == "" {
		return errors.New("message is required")
	}
	r.Message = message
	return s.extras.UpdateReply(ctx, r)
}

// DeleteReply soft-deletes a reply.
func (s *AnnouncementService) DeleteReply(ctx context.Context, r *models.AnnouncementReply) error {
	r.WorkflowState = models.AnnouncementReplyDeleted
	return s.extras.UpdateReply(ctx, r)
}

// ReplyCounts returns the live reply count per announcement (empty when
// replies are not wired).
func (s *AnnouncementService) ReplyCounts(ctx context.Context, ids []uint) map[uint]int64 {
	if s.extras == nil {
		return map[uint]int64{}
	}
	m, err := s.extras.ReplyCounts(ctx, ids)
	if err != nil {
		return map[uint]int64{}
	}
	return m
}

// SectionsFor returns the targeted sections per announcement.
func (s *AnnouncementService) SectionsFor(ctx context.Context, ids []uint) map[uint][]uint {
	if s.extras == nil {
		return map[uint][]uint{}
	}
	m, err := s.extras.SectionsFor(ctx, ids)
	if err != nil {
		return map[uint][]uint{}
	}
	return m
}
