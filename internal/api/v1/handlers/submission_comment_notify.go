package handlers

import (
	"context"
	"fmt"
	"html"
	"strings"

	"github.com/EduThemes/paper-lms/internal/domain/models"
)

// submissionCommentNotification is the notification type of a comment
// notice (also the dedupe key for held-back comments).
const submissionCommentNotification = "submission_comment"

// notifySubmissionComment tells the submission's owner that someone else
// commented on it (Canvas "Submission Comment", G091). A comment on a
// submission the student cannot see yet (manual post policy, not posted)
// stays quiet until the grades are posted, like the comment itself; so
// does a moderated assignment's before the moderator publishes (provisional
// graders' comments are hidden from the student until then).
// Best-effort: never fails the request.
func (h *SubmissionHandler) notifySubmissionComment(ctx context.Context, assignment *models.Assignment, sub *models.Submission, authorID uint, authorName, comment string) {
	if h.notificationDeliveryService == nil || assignment == nil || sub == nil || sub.UserID == 0 || authorID == sub.UserID {
		return
	}
	if !assignment.GradesPublished() {
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
	_ = h.notificationDeliveryService.QueueNotification(ctx, sub.UserID, submissionCommentNotification, subject, body, "Assignment", assignment.ID)
}

// notifyHeldComments runs when posting grades makes a submission visible:
// comments others left while it was hidden (manual post policy) were not
// sent at the time, so the student gets one notice listing them. A
// comment already covered by an earlier comment notice for this
// assignment (created before it) is not repeated, so hiding and
// re-posting does not resend old feedback. No author names: a moderated
// assignment's graders may be anonymous to the student.
func (h *SubmissionHandler) notifyHeldComments(ctx context.Context, assignment *models.Assignment, sub *models.Submission) {
	if h.notificationDeliveryService == nil || h.commentRepo == nil || assignment == nil || sub == nil || sub.UserID == 0 || sub.PostedAt == nil {
		return
	}
	// accountID=0: the submission was loaded under the caller's tenant
	// by the post path.
	comments, err := h.commentRepo.ListBySubmissionID(ctx, sub.ID, 0)
	if err != nil || len(comments) == 0 {
		return
	}
	since := h.notificationDeliveryService.LatestNotificationAt(ctx, sub.UserID, submissionCommentNotification, "Assignment", assignment.ID)
	var texts []string
	for _, cm := range comments {
		if cm.Draft || cm.AuthorID == sub.UserID {
			continue
		}
		if since != nil && !cm.CreatedAt.After(*since) {
			continue
		}
		text := strings.TrimSpace(cm.Comment)
		if text == "" {
			text = "(attachment)"
		}
		texts = append(texts, text)
	}
	if len(texts) == 0 {
		return
	}
	subject := fmt.Sprintf("New comments on %s", assignment.Name)
	var b strings.Builder
	if len(texts) == 1 {
		fmt.Fprintf(&b, "<p>There is a new comment on your submission for <strong>%s</strong>:</p>", html.EscapeString(assignment.Name))
	} else {
		fmt.Fprintf(&b, "<p>There are %d new comments on your submission for <strong>%s</strong>:</p>", len(texts), html.EscapeString(assignment.Name))
	}
	for _, t := range texts {
		fmt.Fprintf(&b, "<p>%s</p>", html.EscapeString(t))
	}
	_ = h.notificationDeliveryService.QueueNotification(ctx, sub.UserID, submissionCommentNotification, subject, b.String(), "Assignment", assignment.ID)
}
