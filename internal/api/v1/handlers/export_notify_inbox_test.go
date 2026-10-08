package handlers

import (
	"context"

	"github.com/EduThemes/paper-lms/internal/domain/models"
)

// NotifySubmissionCommentForTest exposes notifySubmissionComment to the
// external DB tests (compiled only into test binaries).
func (h *SubmissionHandler) NotifySubmissionCommentForTest(ctx context.Context, a *models.Assignment, sub *models.Submission, authorID uint, authorName, comment string) {
	h.notifySubmissionComment(ctx, a, sub, authorID, authorName, comment)
}
