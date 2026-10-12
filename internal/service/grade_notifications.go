package service

import (
	"context"
	"fmt"
	"html"

	"github.com/EduThemes/paper-lms/internal/domain/models"
)

// NotifyGradePosted queues the "grade posted" notification (Canvas
// "Submission Graded") for a submission whose grade the student can see:
// it must be posted and graded or excused. Shared by the grading handlers
// and the machine grade writers (SubmissionService.SetGradeNotifier).
// Best-effort: never fails the caller.
func NotifyGradePosted(ctx context.Context, nds *NotificationDeliveryService, assignment *models.Assignment, sub *models.Submission) {
	if nds == nil || assignment == nil || sub == nil || sub.UserID == 0 || sub.PostedAt == nil || (sub.Score == nil && !sub.Excused) {
		return
	}
	assignmentName := html.EscapeString(assignment.Name)
	subject := fmt.Sprintf("Grade posted: %s", assignment.Name)
	var body string
	if sub.Excused {
		body = fmt.Sprintf("You have been excused from <strong>%s</strong>.", assignmentName)
	} else {
		gradeDisplay := ""
		if sub.Grade != nil {
			gradeDisplay = *sub.Grade
		}
		body = fmt.Sprintf("Your submission for <strong>%s</strong> has been graded. You received a score of <strong>%s</strong>.", assignmentName, html.EscapeString(gradeDisplay))
	}
	_ = nds.QueueNotification(ctx, sub.UserID, "submission_grade", subject, body, "Assignment", assignment.ID)
}
