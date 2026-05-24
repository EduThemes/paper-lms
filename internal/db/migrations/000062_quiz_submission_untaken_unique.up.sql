-- 000062_quiz_submission_untaken_unique.up.sql
--
-- F-049 — close the StartSubmission race that lets a student blow
-- past quiz.allowed_attempts by firing N parallel POST
-- /submissions requests. The pre-fix path computed
-- attempt = existing.Attempt + 1 from a single SELECT, then issued
-- a separate INSERT. Two parallel requests both saw the same
-- "existing" row, both computed the same attempt number, and both
-- inserted — N concurrent untaken submissions for the same (quiz,
-- user).
--
-- A partial UNIQUE index on (quiz_id, user_id) WHERE
-- workflow_state = 'untaken' is the smallest fix: it permits the
-- normal multi-attempt history (one row per attempt in a
-- non-untaken state) AND rejects the race at the DB level
-- regardless of how many goroutines or pods are running.
--
-- The service's existing "FindByQuizAndUser → if untaken, return
-- it" branch handles the second-writer recovery path: when the
-- partial-unique constraint trips, the caller retries the find and
-- gets the winning row.

BEGIN;

CREATE UNIQUE INDEX IF NOT EXISTS
    idx_quiz_submissions_one_untaken_per_user
    ON quiz_submissions (quiz_id, user_id)
    WHERE workflow_state = 'untaken';

COMMIT;
