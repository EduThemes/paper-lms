package v1

import (
	"github.com/gofiber/fiber/v2"

	"github.com/EduThemes/paper-lms/internal/api/v1/middleware"
)

// registerNotifyInboxRoutes mounts the notify-inbox lane: inbox state and
// attachments (G092), the recipient address book (G093), announcement
// replies (G090) and the per-category x per-channel notification
// preferences (G091).
//
// It is called BEFORE the core conversation routes: the static paths
// /conversations/unread_count and /conversations/contexts would otherwise
// be captured by GET /conversations/:id.
func (r *Router) registerNotifyInboxRoutes(protected fiber.Router) {
	// Inbox (any authenticated user; participant-gated in the handler).
	protected.Get("/conversations/unread_count", r.ConversationHandler.UnreadCount)
	protected.Get("/conversations/contexts", r.ConversationHandler.ListInboxContexts)
	protected.Get("/conversations/submission_comments", r.ConversationHandler.ListSubmissionCommentThreads)
	protected.Put("/conversations", r.ConversationHandler.BatchUpdateConversations)
	protected.Post("/conversations/attachments", middleware.UploadRateLimit(), middleware.EnforceUploadSize(r.SettingsLookup), r.ConversationHandler.UploadAttachment)
	protected.Delete("/conversations/:id", r.ConversationHandler.DeleteConversation)
	protected.Post("/conversations/:id/forward", r.ConversationHandler.ForwardMessages)
	protected.Get("/conversations/:id/attachments/:attachment_id", r.ConversationHandler.DownloadAttachment)

	// Announcement replies (visibility + course-role checks in the handler;
	// the routes carry no :course_id).
	protected.Get("/announcements/:id/replies", r.AnnouncementHandler.ListReplies)
	protected.Post("/announcements/:id/replies", r.AnnouncementHandler.CreateReply)
	protected.Put("/announcements/:id/replies/:reply_id", r.AnnouncementHandler.UpdateReply)
	protected.Delete("/announcements/:id/replies/:reply_id", r.AnnouncementHandler.DeleteReply)

	// Notification preferences grid (self only).
	protected.Get("/users/self/notification_policies", r.NotificationDeliveryHandler.GetNotificationPolicies)
	protected.Put("/users/self/notification_policies", r.NotificationDeliveryHandler.UpdateNotificationPolicies)
	protected.Get("/users/self/communication_channel_types", r.NotificationDeliveryHandler.ListChannelTypes)
}
