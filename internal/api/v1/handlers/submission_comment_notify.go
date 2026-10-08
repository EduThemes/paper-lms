package handlers

import (
	"context"
	"fmt"
	"html"

	"github.com/EduThemes/paper-lms/internal/domain/models"
)

// notifySubmissionComment tells the submission's owner that someone else
// commented on it (Canvas "Submission Comment", G091). A comment on a
// submission the student cannot see yet (manual post policy, not posted)
// stays quiet until the grades are posted, like the comment itself.
// Best-effort: never fails the request.
func (h *SubmissionHandler) notifySubmissionComment(ctx context.Context, assignment *models.Assignment, sub *models.Submission, authorID uint, authorName, comment string) {
	if h.notificationDeliveryService == nil || assignment == nil || sub == nil || sub.UserID == 0 || authorID == sub.UserID {
		return
	}
	if assignment.UsesManualPostPolicy() && sub.PostedAt == nil {
		return
	}
	if authorName == "" {
		authorName = "Your instructor"
	}
	subject := fmt.Sprintf("New comment on %s", assignment.Name)
	body := fmt.Sprintf("<p><strong>%s</strong> commented on your submission for <strong>%s</strong>:</p><p>%s</p>",
		html.EscapeString(authorName), html.EscapeString(assignment.Name), html.EscapeString(comment))
	_ = h.notificationDeliveryService.QueueNotification(ctx, sub.UserID, "submission_comment", subject, body, "Assignment", assignment.ID)
}
