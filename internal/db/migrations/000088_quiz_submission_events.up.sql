-- 000088_quiz_submission_events.up.sql
--
-- G057: quiz log auditing. One row per attempt event: server-recorded
-- (start, answer saved, submitted, auto-submitted, extended, graded,
-- regraded) and client-reported (page focus/blur, question viewed /
-- flagged). Read by course staff on the attempt grading page.

BEGIN;

CREATE TABLE IF NOT EXISTS quiz_submission_events (
    id                 bigserial PRIMARY KEY,
    quiz_submission_id bigint NOT NULL REFERENCES quiz_submissions(id) ON DELETE CASCADE,
    attempt            bigint NOT NULL DEFAULT 1,
    event_type         text   NOT NULL,
    event_data         text,
    source             text   NOT NULL DEFAULT 'server',
    client_timestamp   timestamptz,
    created_at         timestamptz,
    CONSTRAINT quiz_submission_events_source_check CHECK (source IN ('server', 'client'))
);

CREATE INDEX IF NOT EXISTS idx_quiz_submission_events_submission
    ON quiz_submission_events (quiz_submission_id);

COMMIT;
