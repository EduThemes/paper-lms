package v1

import (
	"github.com/gofiber/fiber/v2"

	"github.com/EduThemes/paper-lms/internal/api/v1/middleware"
)

// registerDiscussionParityRoutes mounts the discussion endpoints added
// for Canvas parity: post attachments, the @mention picker, and the
// graded-discussion lookups for the assignment page and SpeedGrader.
// Topic CRUD, entries and the threaded view stay in router.go.
func (r *Router) registerDiscussionParityRoutes(protected fiber.Router, enrolled, instructor fiber.Handler) {
	if r.DiscussionParityHandler == nil {
		return
	}
	h := r.DiscussionParityHandler
	protected.Post("/courses/:course_id/discussion_topics/:topic_id/attachments",
		middleware.UploadRateLimit(), middleware.EnforceUploadSize(r.SettingsLookup), enrolled, h.UploadAttachment)
	protected.Get("/courses/:course_id/discussion_topics/:topic_id/entries/:entry_id/attachment", enrolled, h.DownloadEntryAttachment)
	protected.Get("/courses/:course_id/discussion_topics/:topic_id/mentionable_users", enrolled, h.MentionableUsers)
	protected.Get("/courses/:course_id/assignments/:assignment_id/discussion_topic", enrolled, h.AssignmentDiscussionTopic)
	protected.Get("/courses/:course_id/assignments/:assignment_id/discussion_entries", instructor, h.StudentEntries)
}
