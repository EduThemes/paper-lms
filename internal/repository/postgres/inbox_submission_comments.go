package postgres

import (
	"context"
	"time"

	"gorm.io/gorm"

	"github.com/EduThemes/paper-lms/internal/repository"
)

// Submission comments in the inbox (G092; Canvas's "Submission Comments"
// mailbox). A thread is one submission with at least one comment the
// caller can read:
//
//   - the caller's own submission, in a course they are actively
//     enrolled in, on a published assignment: every comment except other
//     authors' drafts, and, until the submission is posted, only the
//     caller's own comments (the instructor's feedback is part of the
//     hidden grade);
//   - a submission in a course the caller actively teaches (teacher or
//     TA): every comment except other authors' drafts.
//
// These are the rules of GET .../submissions/:user_id/comments.

// SubmissionCommentThread is one row of the submission-comments view.
type SubmissionCommentThread struct {
	SubmissionID    uint       `json:"submission_id"`
	CourseID        uint       `json:"course_id"`
	CourseName      string     `json:"course_name"`
	AssignmentID    uint       `json:"assignment_id"`
	AssignmentName  string     `json:"assignment_name"`
	StudentID       uint       `json:"student_id"`
	StudentName     string     `json:"student_name"`
	CommentCount    int64      `json:"comment_count"`
	LastComment     string     `json:"last_comment"`
	LastAuthorID    uint       `json:"last_author_id"`
	LastAuthorName  string     `json:"last_author_name"`
	LastCommentAt   *time.Time `json:"last_comment_at"`
	CallerIsStudent bool       `json:"caller_is_student"`
}

// submissionThreadScopeSQL: submission alias s (assignment a) is one the
// caller may read comments on. Params: caller, account, caller, account.
const submissionThreadScopeSQL = `(
	(s.user_id = ?
	 AND EXISTS (SELECT 1 FROM enrollments e JOIN courses ec ON ec.id = e.course_id
	              WHERE e.user_id = s.user_id AND e.course_id = a.course_id
	                AND e.workflow_state = 'active' AND ec.account_id = ?)
	 AND (a.workflow_state = 'published' OR (COALESCE(a.workflow_state, '') = '' AND a.published)))
	OR EXISTS (SELECT 1 FROM enrollments e JOIN courses ec ON ec.id = e.course_id
	            WHERE e.user_id = ? AND e.course_id = a.course_id AND e.workflow_state = 'active'
	              AND e.type IN ('TeacherEnrollment', 'TaEnrollment') AND ec.account_id = ?)
)`

// submissionCommentVisibleSQL: comment alias sc of submission s is
// readable by the caller. Params: caller, caller.
const submissionCommentVisibleSQL = `(
	(NOT COALESCE(sc.draft, false) OR sc.author_id = ?)
	AND (s.user_id <> ? OR s.posted_at IS NOT NULL OR sc.author_id = s.user_id)
)`

// ListSubmissionCommentThreads pages through the caller's submission
// comment threads, newest comment first.
func (r *InboxRepository) ListSubmissionCommentThreads(ctx context.Context, userID, accountID uint, params repository.PaginationParams) ([]SubmissionCommentThread, int64, error) {
	if params.PerPage <= 0 {
		params.PerPage = 20
	}
	if params.Page <= 0 {
		params.Page = 1
	}
	visible := []any{userID, userID}
	base := func() *gorm.DB {
		return r.db.WithContext(ctx).Table("submissions AS s").
			Joins("JOIN assignments a ON a.id = s.assignment_id").
			Joins("JOIN courses co ON co.id = a.course_id").
			Where("co.account_id = ? AND co.workflow_state <> 'deleted' AND COALESCE(a.workflow_state, '') <> 'deleted'", accountID).
			Where(submissionThreadScopeSQL, userID, accountID, userID, accountID).
			Where("EXISTS (SELECT 1 FROM submission_comments sc WHERE sc.submission_id = s.id AND "+submissionCommentVisibleSQL+")", visible...)
	}

	var total int64
	if err := base().Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var rows []SubmissionCommentThread
	err := base().
		Select(`s.id AS submission_id, co.id AS course_id, co.name AS course_name,
			a.id AS assignment_id, a.name AS assignment_name,
			s.user_id AS student_id, COALESCE(stu.name, '') AS student_name,
			(SELECT COUNT(*) FROM submission_comments sc WHERE sc.submission_id = s.id AND `+submissionCommentVisibleSQL+`) AS comment_count,
			COALESCE(lc.comment, '') AS last_comment, COALESCE(lc.author_id, 0) AS last_author_id,
			COALESCE(au.name, '') AS last_author_name, lc.created_at AS last_comment_at,
			(s.user_id = ?) AS caller_is_student`, userID, userID, userID).
		Joins(`LEFT JOIN LATERAL (
			SELECT sc.comment, sc.author_id, sc.created_at FROM submission_comments sc
			 WHERE sc.submission_id = s.id AND `+submissionCommentVisibleSQL+`
			 ORDER BY sc.created_at DESC, sc.id DESC LIMIT 1) lc ON true`, visible...).
		Joins("LEFT JOIN users stu ON stu.id = s.user_id").
		Joins("LEFT JOIN users au ON au.id = lc.author_id").
		Order("lc.created_at DESC, s.id DESC").
		Offset((params.Page - 1) * params.PerPage).Limit(params.PerPage).
		Scan(&rows).Error
	return rows, total, err
}
