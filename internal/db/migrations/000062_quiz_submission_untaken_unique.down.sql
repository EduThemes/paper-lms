-- 000062_quiz_submission_untaken_unique.down.sql

BEGIN;

DROP INDEX IF EXISTS idx_quiz_submissions_one_untaken_per_user;

COMMIT;
