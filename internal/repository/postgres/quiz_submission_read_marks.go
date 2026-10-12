package postgres

import (
	"context"
	"time"
)

// AppendReadQuestionIfInProgress atomically adds questionID to the
// attempt's read_questions JSON array (the can't-go-back lock), but only
// while the attempt is still 'untaken'. A targeted UPDATE — never a full
// row save — so a lock recorded while the attempt is being completed can
// never reset its workflow_state. Returns the stored array and whether the
// attempt was still in progress.
func (r *quizSubmissionRepo) AppendReadQuestionIfInProgress(ctx context.Context, submissionID, questionID uint) (string, bool, error) {
	var out struct{ ReadQuestions string }
	res := r.db.WithContext(ctx).Raw(`
		UPDATE quiz_submissions
		   SET read_questions = CASE
		         WHEN COALESCE(NULLIF(read_questions, ''), '[]')::jsonb @> jsonb_build_array(?::bigint)
		           THEN read_questions
		         ELSE (COALESCE(NULLIF(read_questions, ''), '[]')::jsonb || jsonb_build_array(?::bigint))::text
		       END,
		       updated_at = ?
		 WHERE id = ? AND workflow_state = 'untaken'
		RETURNING read_questions`, questionID, questionID, time.Now(), submissionID).Scan(&out)
	if res.Error != nil {
		return "", false, res.Error
	}
	if res.RowsAffected == 0 {
		return "", false, nil
	}
	return out.ReadQuestions, true, nil
}

var _ interface {
	AppendReadQuestionIfInProgress(ctx context.Context, submissionID, questionID uint) (string, bool, error)
} = (*quizSubmissionRepo)(nil)
