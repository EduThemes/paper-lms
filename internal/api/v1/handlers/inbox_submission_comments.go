package handlers

import (
	"github.com/gofiber/fiber/v2"

	"github.com/EduThemes/paper-lms/internal/api/v1/middleware"
	"github.com/EduThemes/paper-lms/internal/api/v1/responses"
)

// ListSubmissionCommentThreads handles GET /conversations/submission_comments:
// the inbox's "Submission Comments" mailbox (G092). Students see the
// comment threads on their own submissions, teachers and TAs those on
// submissions in the courses they teach, under the same visibility rules
// as the submission comments endpoint (no one else's drafts; nothing but
// the student's own comments before the submission is posted). Replies go
// through the submission comments endpoint.
func (h *ConversationHandler) ListSubmissionCommentThreads(c *fiber.Ctx) error {
	userID, err := getUserID(c)
	if err != nil {
		return err
	}
	if h.inbox == nil {
		return c.JSON([]fiber.Map{})
	}
	params := middleware.GetPagination(c)
	rows, total, err := h.inbox.ListSubmissionCommentThreads(c.Context(), userID, callerAccountID(c), params)
	if err != nil {
		return responses.InternalError(c, "Could not fetch submission comments")
	}
	responses.SetPaginationHeaders(c, total, params.Page, params.PerPage)
	out := make([]fiber.Map, len(rows))
	for i, r := range rows {
		preview := []rune(r.LastComment)
		if len(preview) > 140 {
			preview = append(preview[:139], '…')
		}
		out[i] = fiber.Map{
			"submission_id":     r.SubmissionID,
			"course_id":         r.CourseID,
			"course_name":       r.CourseName,
			"assignment_id":     r.AssignmentID,
			"assignment_name":   r.AssignmentName,
			"student_id":        r.StudentID,
			"student_name":      r.StudentName,
			"comment_count":     r.CommentCount,
			"last_comment":      string(preview),
			"last_author_id":    r.LastAuthorID,
			"last_author_name":  r.LastAuthorName,
			"last_comment_at":   r.LastCommentAt,
			"caller_is_student": r.CallerIsStudent,
		}
	}
	return c.JSON(out)
}
