-- 000086_quiz_lifecycle_columns.up.sql
--
-- Quiz lifecycle (Canvas parity, quiz-engine lane):
--   G041  quizzes.assignment_id links a graded quiz to its online_quiz
--         gradebook assignment (dropped in 000028, re-added here).
--   G043  answer-key release window + last-attempt-only + one-time results.
--   G042  quiz_submissions.fudge_points and per-question grader comments.
--   G053  quiz_submissions.score_before_regrade.

BEGIN;

ALTER TABLE quizzes ADD COLUMN IF NOT EXISTS assignment_id bigint
    REFERENCES assignments(id) ON DELETE SET NULL;
ALTER TABLE quizzes ADD COLUMN IF NOT EXISTS show_correct_answers_at timestamptz;
ALTER TABLE quizzes ADD COLUMN IF NOT EXISTS hide_correct_answers_at timestamptz;
ALTER TABLE quizzes ADD COLUMN IF NOT EXISTS show_correct_answers_last_attempt boolean NOT NULL DEFAULT false;
ALTER TABLE quizzes ADD COLUMN IF NOT EXISTS one_time_results boolean NOT NULL DEFAULT false;

CREATE INDEX IF NOT EXISTS idx_quizzes_assignment_id ON quizzes (assignment_id);

ALTER TABLE quiz_submissions ADD COLUMN IF NOT EXISTS fudge_points double precision;
ALTER TABLE quiz_submissions ADD COLUMN IF NOT EXISTS score_before_regrade double precision;
ALTER TABLE quiz_submissions ADD COLUMN IF NOT EXISTS results_seen_at timestamptz;

-- The overdue auto-submit job scans untaken attempts by end_at.
CREATE INDEX IF NOT EXISTS idx_quiz_submissions_untaken_end_at
    ON quiz_submissions (end_at) WHERE workflow_state = 'untaken' AND end_at IS NOT NULL;

ALTER TABLE quiz_submission_answers ADD COLUMN IF NOT EXISTS comment text;

COMMIT;
