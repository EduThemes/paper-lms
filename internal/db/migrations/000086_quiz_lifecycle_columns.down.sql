-- 000086_quiz_lifecycle_columns.down.sql

BEGIN;

ALTER TABLE quiz_submission_answers DROP COLUMN IF EXISTS comment;

DROP INDEX IF EXISTS idx_quiz_submissions_untaken_end_at;
ALTER TABLE quiz_submissions DROP COLUMN IF EXISTS results_seen_at;
ALTER TABLE quiz_submissions DROP COLUMN IF EXISTS score_before_regrade;
ALTER TABLE quiz_submissions DROP COLUMN IF EXISTS fudge_points;

DROP INDEX IF EXISTS idx_quizzes_assignment_id;
ALTER TABLE quizzes DROP COLUMN IF EXISTS one_time_results;
ALTER TABLE quizzes DROP COLUMN IF EXISTS show_correct_answers_last_attempt;
ALTER TABLE quizzes DROP COLUMN IF EXISTS hide_correct_answers_at;
ALTER TABLE quizzes DROP COLUMN IF EXISTS show_correct_answers_at;
ALTER TABLE quizzes DROP COLUMN IF EXISTS assignment_id;

COMMIT;
